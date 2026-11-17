package service

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/proto"
)

// MemProjectRecords keeps stored projects in memory, for tests and for a host whose projects end
// with the process (agni issue 1694).
type MemProjectRecords struct {
	mu sync.Mutex
	m  map[string]*webapi.Project
}

// NewMemProjectRecords returns an empty MemProjectRecords.
func NewMemProjectRecords() *MemProjectRecords {
	return &MemProjectRecords{m: map[string]*webapi.Project{}}
}

var _ ProjectRecords = (*MemProjectRecords)(nil)

// Get returns one record, or ErrNotFound.
func (r *MemProjectRecords) Get(_ context.Context, id string) (*webapi.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.m[id]
	if !ok {
		return nil, fmt.Errorf("%w: no project %s", ErrNotFound, ProjectName(id))
	}
	return proto.CloneOf(p), nil
}

// List returns every record, ordered by name.
func (r *MemProjectRecords) List(context.Context) ([]*webapi.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*webapi.Project, 0, len(r.m))
	for _, p := range r.m {
		out = append(out, proto.CloneOf(p))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GetName() < out[j].GetName() })
	return out, nil
}

// Put creates or replaces a record.
func (r *MemProjectRecords) Put(_ context.Context, id string, p *webapi.Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[id] = proto.CloneOf(p)
	return nil
}

// Delete removes a record, or answers ErrNotFound.
func (r *MemProjectRecords) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.m[id]; !ok {
		return fmt.Errorf("%w: no project %s", ErrNotFound, ProjectName(id))
	}
	delete(r.m, id)
	return nil
}
