package projects

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/records"
	"github.com/panyam/agni/service"
)

// DirReports keeps reports as one protojson record each, `<design>/<report>.json` (agni issues
// 1704, 1733). A report is a few hundred findings, so a record each is enough. The records sit in a
// folder or in sqlite, as records.Store keeps them. Each is written whole, so a reader sees the whole
// report or none, and the store refuses a create of a taken id.
type DirReports struct {
	recs records.Store
	now  func() time.Time
}

var _ service.ReportStore = (*DirReports)(nil)

// NewDirReports returns reports kept as files in dir, created if absent.
func NewDirReports(dir string) (*DirReports, error) {
	d, err := records.NewDir(dir)
	if err != nil {
		return nil, err
	}
	return NewReports(d), nil
}

// NewReports returns reports kept in recs, each at `<design>/<report>.json`.
func NewReports(recs records.Store) *DirReports { return &DirReports{recs: recs, now: time.Now} }

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
func (r *DirReports) CreateReport(ctx context.Context, design, id string, in *webapi.Report) (*webapi.Report, error) {
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
	if err := r.recs.Create(ctx, records.Join(design, id+".json"), b); errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("%w: %s", service.ErrAlreadyExists, out.GetName())
	} else if err != nil {
		return nil, err
	}
	return out, nil
}

// Report reads one report, or answers ErrNotFound.
func (r *DirReports) Report(ctx context.Context, design, id string) (*webapi.Report, error) {
	if err := r.ids(design, id); err != nil {
		return nil, fmt.Errorf("%w: no report %s", service.ErrNotFound, service.ReportName(design, id))
	}
	return r.load(ctx, design, id)
}

func (r *DirReports) load(ctx context.Context, design, id string) (*webapi.Report, error) {
	b, err := r.recs.Get(ctx, records.Join(design, id+".json"))
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
func (r *DirReports) Reports(ctx context.Context, design string) ([]*webapi.Report, error) {
	if !idPattern.MatchString(design) {
		return nil, nil
	}
	entries, err := r.recs.List(ctx, design)
	if err != nil {
		return nil, err
	}
	var out []*webapi.Report
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name, ".json")
		if !ok || e.Dir || !idPattern.MatchString(id) {
			continue
		}
		rep, err := r.load(ctx, design, id)
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
func (r *DirReports) DeleteReport(ctx context.Context, design, id string) error {
	if err := r.ids(design, id); err != nil {
		return fmt.Errorf("%w: no report %s", service.ErrNotFound, service.ReportName(design, id))
	}
	if err := r.recs.Delete(ctx, records.Join(design, id+".json")); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: no report %s", service.ErrNotFound, service.ReportName(design, id))
	} else if err != nil {
		return err
	}
	return nil
}

// DeleteReports removes a design's reports.
func (r *DirReports) DeleteReports(ctx context.Context, design string) error {
	if !idPattern.MatchString(design) {
		return fmt.Errorf("%w: design id %q is not a valid id", service.ErrInvalidArgument, design)
	}
	return r.recs.DeleteDir(ctx, design)
}
