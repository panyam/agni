package projects

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/records"
	"github.com/panyam/agni/service"
)

// DirProjects keeps stored projects as one protojson record each, `<id>.json` (agni issues 1694,
// 1733). A stored project is a title and a list of design names, a few hundred bytes. The records
// sit in a folder or in sqlite, as records.Store keeps them, and each write replaces a record whole.
type DirProjects struct {
	recs records.Store
}

// NewDirProjects returns records kept as files in dir, created if absent.
func NewDirProjects(dir string) (*DirProjects, error) {
	d, err := records.NewDir(dir)
	if err != nil {
		return nil, err
	}
	return NewProjects(d), nil
}

// NewProjects returns projects kept in recs, each at `<id>.json`.
func NewProjects(recs records.Store) *DirProjects { return &DirProjects{recs: recs} }

var _ service.ProjectRecords = (*DirProjects)(nil)

func (r *DirProjects) key(id string) (string, error) {
	if !idPattern.MatchString(id) {
		return "", fmt.Errorf("%w: project id %q is not a valid id", service.ErrInvalidArgument, id)
	}
	return id + ".json", nil
}

// Get reads one record, or answers ErrNotFound.
func (r *DirProjects) Get(ctx context.Context, id string) (*webapi.Project, error) {
	k, err := r.key(id)
	if err != nil {
		return nil, err
	}
	b, err := r.recs.Get(ctx, k)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: no project %s", service.ErrNotFound, service.ProjectName(id))
	}
	if err != nil {
		return nil, err
	}
	out := &webapi.Project{}
	if err := protojson.Unmarshal(b, out); err != nil {
		return nil, fmt.Errorf("project %s: %w", id, err)
	}
	return out, nil
}

// List reads every record, ordered by name.
func (r *DirProjects) List(ctx context.Context) ([]*webapi.Project, error) {
	entries, err := r.recs.List(ctx, "")
	if err != nil {
		return nil, err
	}
	var out []*webapi.Project
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name, ".json")
		if !ok || e.Dir || !idPattern.MatchString(id) {
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
func (r *DirProjects) Put(ctx context.Context, id string, p *webapi.Project) error {
	k, err := r.key(id)
	if err != nil {
		return err
	}
	b, err := protojson.Marshal(p)
	if err != nil {
		return err
	}
	return r.recs.Put(ctx, k, b)
}

// Delete removes a record, or answers ErrNotFound.
func (r *DirProjects) Delete(ctx context.Context, id string) error {
	k, err := r.key(id)
	if err != nil {
		return err
	}
	if err := r.recs.Delete(ctx, k); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: no project %s", service.ErrNotFound, service.ProjectName(id))
	} else if err != nil {
		return err
	}
	return nil
}
