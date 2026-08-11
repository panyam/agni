package main

import (
	"context"
	"fmt"

	"os"

	"github.com/panyam/agni/datasheet/param"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/artifact"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/internal/service"
	"github.com/panyam/agni/stdlib/profiles"
	"github.com/panyam/agni/stdlib/rules/intent"
)

// osProjectConfig is the OS-backed service.ProjectConfigLoader: it reads the interface profiles and
// seeded parameters a project names, from the mounts. All filesystem access stays at the cmd edge
// (C1/C13).
//
// It holds NO CACHE. An operator edits a profile or seeds a part while the server runs, and an index
// that answered with the previous version would produce a confident wrong verdict, which is the
// failure this whole workstream exists to remove. A large parameter corpus re-read per request is a
// cost, and a visible one; a stale one is not.
//
// If a deployment ever feels that cost, internal/projects/cache.go is the shape to copy rather than
// the trade to reopen: it caches, and it still stats every file it depends on before answering, so
// the speedup never comes out of freshness.
type osProjectConfig struct {
	mounts []mounts.Mount
}

// ProjectConfig loads what a project's URIs point at.
//
// A tier that fails to load is an ERROR, not a skip. An operator who wrote a profiles directory and
// silently got the built-ins would read the resulting clean report as a clean design, which is the
// same silent-pass failure C24 was written for. A tier the project simply does not have never
// reaches here: the store only sets a URI for a file that exists.
func (c *osProjectConfig) ProjectConfig(_ context.Context, p *webapi.Project, d *webapi.Design) (service.ProjectConfig, error) {
	var out service.ProjectConfig
	for _, uri := range p.GetProfileUris() {
		dir, err := c.dir(uri)
		if err != nil {
			return service.ProjectConfig{}, err
		}
		ps, err := profiles.LoadDir(dir)
		if err != nil {
			return service.ProjectConfig{}, fmt.Errorf("%s profiles %s: %w", p.GetName(), uri, err)
		}
		// Namespaced by the project's id rather than by a fixed "profile-overlay", because two
		// projects on one server would otherwise contribute rule sources of the same name and the
		// second would collide with the first. The `-profiles` suffix keeps it clear of the project's
		// naming convention, which claims the bare id as its own namespace.
		out.Sources = append(out.Sources, profiles.Source(projectSourceName(p), ps))
	}
	for _, uri := range p.GetParamUris() {
		dir, err := c.dir(uri)
		if err != nil {
			return service.ProjectConfig{}, err
		}
		set, err := param.LoadSet(os.DirFS(dir))
		if err != nil {
			return service.ProjectConfig{}, fmt.Errorf("%s params %s: %w", p.GetName(), uri, err)
		}
		out.Specs = set
	}
	// Intent is the design's, not the project's, and composes as its own rule source. A design that
	// declared none simply contributes nothing, which is how the intent-bound checklist items read
	// needs-design-intent rather than passing on an architecture nobody stated.
	if uri := d.GetIntentUri(); uri != "" {
		abs, err := c.file(uri)
		if err != nil {
			return service.ProjectConfig{}, err
		}
		decl, err := intent.LoadFile(abs)
		if err != nil {
			return service.ProjectConfig{}, fmt.Errorf("%s intent %s: %w", d.GetName(), uri, err)
		}
		out.Sources = append(out.Sources, intent.Source("intent", decl))
	}
	return out, nil
}

// file resolves a config URI to a host file inside its mount.
func (c *osProjectConfig) file(uri string) (string, error) {
	u, err := artifact.Parse(uri)
	if err != nil {
		return "", err
	}
	return mounts.Resolve(c.mounts, u)
}

// dir resolves a project-config URI to a host directory inside its mount.
func (c *osProjectConfig) dir(uri string) (string, error) {
	u, err := artifact.Parse(uri)
	if err != nil {
		return "", err
	}
	return mounts.Resolve(c.mounts, u)
}

// projectSourceName is the catalog namespace a project's interface profiles appear under, so a
// finding reads `gateway-profiles/can-esd-missing` and says which project asked for it.
func projectSourceName(p *webapi.Project) string {
	id, ok := service.ProjectID(p.GetName())
	if !ok {
		id = "project"
	}
	return id + "-profiles"
}
