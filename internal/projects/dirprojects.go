package projects

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// DirProjects keeps stored projects as one protojson file each, `<id>.json`, in a folder (agni issue
// 1694). A stored project is a title and a list of design names, a few hundred bytes. A write goes
// to a temporary file and is renamed into place, so a reader sees the old record or the new one.
type DirProjects struct {
	dir string
	mu  sync.Mutex
}

// NewDirProjects returns records kept in dir, created if absent.
func NewDirProjects(dir string) (*DirProjects, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, err
	}
	return &DirProjects{dir: abs}, nil
}

var _ service.ProjectRecords = (*DirProjects)(nil)

func (r *DirProjects) path(id string) (string, error) {
	if !idPattern.MatchString(id) {
		return "", fmt.Errorf("%w: project id %q is not a valid id", service.ErrInvalidArgument, id)
	}
	return filepath.Join(r.dir, id+".json"), nil
}

// Get reads one record, or answers ErrNotFound.
func (r *DirProjects) Get(_ context.Context, id string) (*webapi.Project, error) {
	p, err := r.path(id)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p) // #nosec G304 -- p is the store's own folder joined with a checked id
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: no project %s", service.ErrNotFound, service.ProjectName(id))
	}
	if err != nil {
		return nil, err
	}
	out := &webapi.Project{}
	if err := protojson.Unmarshal(b, out); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	return out, nil
}

// List reads every record, ordered by name.
func (r *DirProjects) List(ctx context.Context) ([]*webapi.Project, error) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}
	var out []*webapi.Project
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok || e.IsDir() || !idPattern.MatchString(id) {
			continue
		}
		p, err := r.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GetName() < out[j].GetName() })
	return out, nil
}

// Put writes a record, replacing any it had.
func (r *DirProjects) Put(_ context.Context, id string, p *webapi.Project) error {
	dst, err := r.path(id)
	if err != nil {
		return err
	}
	b, err := protojson.MarshalOptions{Multiline: true}.Marshal(p)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	tmp, err := os.CreateTemp(r.dir, ".new-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dst)
}

// Delete removes a record, or answers ErrNotFound.
func (r *DirProjects) Delete(_ context.Context, id string) error {
	p, err := r.path(id)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := os.Remove(p); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: no project %s", service.ErrNotFound, service.ProjectName(id))
	} else if err != nil {
		return err
	}
	return nil
}
