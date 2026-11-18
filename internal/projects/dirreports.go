package projects

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// DirReports keeps reports as one protojson file each, `<design>/<report>.json`, in a folder (agni
// issue 1704). A report is a few hundred findings, so a file each is enough. A write goes to a
// temporary file and is renamed into place, so a reader sees the whole report or none. One mutex
// serialises every write.
type DirReports struct {
	dir string
	mu  sync.Mutex
	now func() time.Time
}

var _ service.ReportStore = (*DirReports)(nil)

// NewDirReports returns reports kept in dir, created if absent.
func NewDirReports(dir string) (*DirReports, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, err
	}
	return &DirReports{dir: abs, now: time.Now}, nil
}

func (r *DirReports) ids(design, id string) error {
	if !idPattern.MatchString(design) {
		return fmt.Errorf("%w: design id %q is not a valid id", service.ErrInvalidArgument, design)
	}
	if !idPattern.MatchString(id) {
		return fmt.Errorf("%w: report id %q is not a valid id", service.ErrInvalidArgument, id)
	}
	return nil
}

// mint is a report id that sorts by creation time, with a random suffix so two in one second differ.
func mint(now time.Time) (string, error) {
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	return strings.ToLower(now.Format("20060102T150405")) + "-" + hex.EncodeToString(suffix[:]), nil
}

// CreateReport keeps a new report.
func (r *DirReports) CreateReport(_ context.Context, design, id string, in *webapi.Report) (*webapi.Report, error) {
	now := r.now().UTC()
	if id == "" {
		var err error
		if id, err = mint(now); err != nil {
			return nil, err
		}
	}
	if err := r.ids(design, id); err != nil {
		return nil, err
	}
	out := proto.Clone(in).(*webapi.Report)
	out.Name = service.ReportName(design, id)
	out.CreateTime = timestamppb.New(now)
	out.DesignChanged, out.PacksChanged = false, nil
	b, err := protojson.MarshalOptions{Multiline: true}.Marshal(out)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	dir := filepath.Join(r.dir, design)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	dst := filepath.Join(dir, id+".json")
	if _, err := os.Stat(dst); err == nil {
		return nil, fmt.Errorf("%w: %s", service.ErrAlreadyExists, out.GetName())
	}
	tmp, err := os.CreateTemp(dir, ".new-*.json")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return nil, err
	}
	return out, nil
}

// Report reads one report, or answers ErrNotFound.
func (r *DirReports) Report(_ context.Context, design, id string) (*webapi.Report, error) {
	if err := r.ids(design, id); err != nil {
		return nil, fmt.Errorf("%w: no report %s", service.ErrNotFound, service.ReportName(design, id))
	}
	return r.load(design, id)
}

func (r *DirReports) load(design, id string) (*webapi.Report, error) {
	root, err := os.OpenRoot(r.dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	b, err := root.ReadFile(filepath.Join(design, id+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: no report %s", service.ErrNotFound, service.ReportName(design, id))
	}
	if err != nil {
		return nil, err
	}
	out := &webapi.Report{}
	if err := protojson.Unmarshal(b, out); err != nil {
		return nil, fmt.Errorf("%s/%s.json: %w", design, id, err)
	}
	return out, nil
}

// Reports reads every report of a design, newest first.
func (r *DirReports) Reports(_ context.Context, design string) ([]*webapi.Report, error) {
	if !idPattern.MatchString(design) {
		return nil, nil
	}
	entries, err := os.ReadDir(filepath.Join(r.dir, design))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []*webapi.Report
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok || e.IsDir() || !idPattern.MatchString(id) {
			continue
		}
		rep, err := r.load(design, id)
		if err != nil {
			return nil, err
		}
		out = append(out, rep)
	}
	sort.SliceStable(out, func(i, j int) bool {
		ti, tj := out[i].GetCreateTime().AsTime(), out[j].GetCreateTime().AsTime()
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return out[i].GetName() > out[j].GetName()
	})
	return out, nil
}

// DeleteReport removes one report, or answers ErrNotFound.
func (r *DirReports) DeleteReport(_ context.Context, design, id string) error {
	if err := r.ids(design, id); err != nil {
		return fmt.Errorf("%w: no report %s", service.ErrNotFound, service.ReportName(design, id))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := os.Remove(filepath.Join(r.dir, design, id+".json")); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: no report %s", service.ErrNotFound, service.ReportName(design, id))
	} else if err != nil {
		return err
	}
	return nil
}

// DeleteReports removes a design's reports.
func (r *DirReports) DeleteReports(_ context.Context, design string) error {
	if !idPattern.MatchString(design) {
		return fmt.Errorf("%w: design id %q is not a valid id", service.ErrInvalidArgument, design)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return os.RemoveAll(filepath.Join(r.dir, design))
}
