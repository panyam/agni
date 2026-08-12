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

// ResolveConfig loads what an AnalysisConfig's URIs point at, whether it came from a project
// descriptor or from a request.
//
// A tier that fails to load is an ERROR, not a skip. An operator who wrote a profiles directory and
// silently got the built-ins would read the resulting clean report as a clean design, which is the
// same silent-pass failure C24 was written for. A tier the config simply does not name never reaches
// here as an error: the loops below run zero times.
func (c *osProjectConfig) ResolveConfig(_ context.Context, cfg *webapi.AnalysisConfig, namespace string) (service.ResolvedConfig, error) {
	var out service.ResolvedConfig
	for _, uri := range cfg.GetProfileUris() {
		dir, err := c.dir(uri)
		if err != nil {
			return service.ResolvedConfig{}, err
		}
		ps, err := profiles.LoadDir(dir)
		if err != nil {
			return service.ResolvedConfig{}, fmt.Errorf("%s profiles %s: %w", namespace, uri, err)
		}
		out.Sources = append(out.Sources, profiles.Source(sourceName(namespace), ps))
		out.Profiles = true
	}
	for _, uri := range cfg.GetParamUris() {
		dir, err := c.dir(uri)
		if err != nil {
			return service.ResolvedConfig{}, err
		}
		set, err := param.LoadSet(os.DirFS(dir))
		if err != nil {
			return service.ResolvedConfig{}, fmt.Errorf("%s params %s: %w", namespace, uri, err)
		}
		out.Specs = set
	}
	// Intent composes as its own rule source. A config that declared none simply contributes nothing,
	// which is how the intent-bound checklist items read needs-design-intent rather than passing on an
	// architecture nobody stated.
	if uri := cfg.GetIntentUri(); uri != "" {
		abs, err := c.file(uri)
		if err != nil {
			return service.ResolvedConfig{}, err
		}
		decl, err := intent.LoadFile(abs)
		if err != nil {
			return service.ResolvedConfig{}, fmt.Errorf("%s intent %s: %w", namespace, uri, err)
		}
		out.Sources = append(out.Sources, intent.Source("intent", decl))
		out.Intent = true
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
func sourceName(namespace string) string {
	if id, ok := service.ProjectID(namespace); ok {
		return id + "-profiles"
	}
	// A namespace that is not a project resource name is a request's. It keeps the same `-profiles`
	// suffix so the two read alike in a catalog snapshot, and it cannot collide with a project's,
	// because a project id can never be the literal "request".
	return namespace + "-profiles"
}
