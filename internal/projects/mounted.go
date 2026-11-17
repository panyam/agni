package projects

import (
	"context"
	"fmt"
	"sort"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// MountedDesigns reads the designs declared in mounted folders as standalone designs, each named
// "designs/{design}" by the id its `design.yaml` declares (agni issue 1693). It has no writes, so a
// service.DesignCatalog refuses a write to one of its designs as read-only.
//
// A design that extends a project keeps that project's resource name in config.extends. Two
// designs declaring one id anywhere in the trees is an error, since a design id is unique per server.
type MountedDesigns struct {
	fs *FSStore
}

// NewMountedDesigns reads the given trees. Leave out a tree a DirStore keeps, or each of its designs
// is held twice.
func NewMountedDesigns(trees ...Tree) *MountedDesigns {
	return &MountedDesigns{fs: NewFSStore(trees...)}
}

var _ service.DesignReader = (*MountedDesigns)(nil)

// Design returns one mounted design by id.
func (m *MountedDesigns) Design(ctx context.Context, id string) (*webapi.Design, error) {
	all, err := m.Designs(ctx)
	if err != nil {
		return nil, err
	}
	name := service.StandaloneDesignName(id)
	for _, d := range all {
		if d.GetName() == name {
			return d, nil
		}
	}
	return nil, fmt.Errorf("%w: no mounted design %s", service.ErrNotFound, name)
}

// Designs returns every mounted design, ordered by name.
func (m *MountedDesigns) Designs(context.Context) ([]*webapi.Design, error) {
	var out []*webapi.Design
	seen := map[string]string{}
	for _, t := range m.fs.trees {
		dirs, err := m.fs.walks.find(t, ".", DesignDescriptor)
		if err != nil {
			return nil, err
		}
		for _, dir := range dirs {
			id, d, err := m.fs.loadDesign(t, dir)
			if err != nil {
				return nil, err
			}
			where := t.Mount + ":" + displayDir(dir)
			if first, dup := seen[id]; dup {
				return nil, fmt.Errorf("duplicate design id %q, declared at %s and at %s: a design id is unique per server", id, first, where)
			}
			seen[id] = where
			if ext := d.GetConfig().GetExtends(); ext != "" {
				p, err := m.fs.projectAt(ext)
				if err != nil {
					return nil, fmt.Errorf("%s: extends: %w", where, err)
				}
				d.Config.Extends = p.GetName()
			}
			d.Name = service.StandaloneDesignName(id)
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GetName() < out[j].GetName() })
	return out, nil
}
