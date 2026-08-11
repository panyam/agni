package main

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/internal/service"
	"github.com/panyam/agni/project"
)

// osProjects is the OS-backed service.ProjectStore: it maps the server's mounts onto the descriptor
// discovery in package project, one tree per mount. All filesystem access stays at the cmd edge
// (C1/C13), and here that is one `os.DirFS` call.
//
// Containment is structural rather than checked. A mount becomes an fs.FS rooted at the mount, and an
// fs.FS has no parent to climb into, so an upward resolution walk cannot reach a descriptor outside
// the mount and a ref carrying `..` is rejected by fs.ValidPath before any file is opened.
type osProjects struct {
	mounts []mounts.Mount
}

// tree returns the descriptor tree for one mount.
func (s *osProjects) tree(m mounts.Mount) project.Tree {
	return project.Tree{FS: os.DirFS(m.Root)}
}

// Project returns one project by its declared id, or ErrNotFound.
func (s *osProjects) Project(ctx context.Context, projectID string) (service.ProjectInfo, error) {
	all, err := s.Projects(ctx)
	if err != nil {
		return service.ProjectInfo{}, err
	}
	for _, p := range all {
		if p.ID == projectID {
			return p, nil
		}
	}
	return service.ProjectInfo{}, fmt.Errorf("%w: no project %q on any mount", service.ErrNotFound, projectID)
}

// Projects discovers every project across the mounts, ordered by id.
//
// A duplicate id ACROSS mounts is rejected here rather than in package project, which can only see
// one tree at a time. The reasoning is the same: two projects claiming one name means one of them is
// unreachable through its own resource name.
func (s *osProjects) Projects(_ context.Context) ([]service.ProjectInfo, error) {
	var out []service.ProjectInfo
	seen := map[string]string{}
	for _, m := range s.mounts {
		found, err := s.tree(m).Projects()
		if err != nil {
			return nil, err
		}
		for _, p := range found {
			where := m.Name + ":" + p.Dir
			if first, dup := seen[p.Name]; dup {
				return nil, fmt.Errorf("duplicate project id %q, declared at %s and at %s: a project id is its resource name, so two projects cannot share one", p.Name, first, where)
			}
			seen[p.Name] = where
			out = append(out, service.ProjectInfo{ID: p.Name, Title: p.DisplayName(), Mount: m.Name, DirRef: p.Dir})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Design returns one design within a project, or ErrNotFound.
func (s *osProjects) Design(ctx context.Context, projectID, designID string) (service.DesignInfo, error) {
	all, err := s.Designs(ctx, projectID)
	if err != nil {
		return service.DesignInfo{}, err
	}
	for _, d := range all {
		if d.ID == designID {
			return d, nil
		}
	}
	return service.DesignInfo{}, fmt.Errorf("%w: project %q has no design %q", service.ErrNotFound, projectID, designID)
}

// Designs returns a project's designs, ordered by id.
func (s *osProjects) Designs(ctx context.Context, projectID string) ([]service.DesignInfo, error) {
	p, err := s.Project(ctx, projectID)
	if err != nil {
		return nil, err
	}
	m, ok := mounts.Find(s.mounts, p.Mount)
	if !ok {
		return nil, fmt.Errorf("%w: mount %q is no longer configured", service.ErrNotFound, p.Mount)
	}
	found, err := s.tree(m).Designs(p.DirRef)
	if err != nil {
		return nil, err
	}
	out := make([]service.DesignInfo, 0, len(found))
	for _, d := range found {
		out = append(out, designInfo(p, d))
	}
	return out, nil
}

// Resolve maps a mount-relative ref to the design containing it and that design's project.
func (s *osProjects) Resolve(_ context.Context, mount, ref string) (service.DesignInfo, service.ProjectInfo, bool, error) {
	m, ok := mounts.Find(s.mounts, mount)
	if !ok {
		return service.DesignInfo{}, service.ProjectInfo{}, false, fmt.Errorf("no such mount %q: %w", mount, service.ErrNotFound)
	}
	if _, err := mounts.SafeResolve(m.Root, ref); err != nil {
		return service.DesignInfo{}, service.ProjectInfo{}, false, fmt.Errorf("%w: %s", service.ErrInvalidPath, err)
	}
	d, p, found, err := s.tree(m).Resolve(ref)
	if err != nil || !found {
		return service.DesignInfo{}, service.ProjectInfo{}, false, err
	}
	pi := service.ProjectInfo{ID: p.Name, Title: p.DisplayName(), Mount: m.Name, DirRef: p.Dir}
	return designInfo(pi, d), pi, true, nil
}

// designInfo projects a located descriptor into the port's value type. The entry and companion names
// arrive already joined to the tree root, which is the mount, so nothing here re-derives a path.
func designInfo(p service.ProjectInfo, d project.LocatedDesign) service.DesignInfo {
	return service.DesignInfo{
		ProjectID:     p.ID,
		ID:            d.Name,
		Title:         d.DisplayName(),
		Mount:         p.Mount,
		DirRef:        d.Dir,
		EntryRef:      d.EntryRef(),
		CompanionRefs: d.CompanionRefs(),
	}
}
