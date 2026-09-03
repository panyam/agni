package main

import (
	"context"
	"fmt"

	"os"
	"path/filepath"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/datasheet/param"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/service"
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
	for _, uri := range cfg.GetSymbolPathUris() {
		dir, err := c.dir(uri)
		if err != nil {
			return service.ResolvedConfig{}, err
		}
		out.SymbolPaths = append(out.SymbolPaths, dir)
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

// refuseProfilePathTheProjectOwns rejects a --profile-path naming a directory the design's own
// project already composes.
//
// Pointing the flag at the project's own profiles/ reads as reasonable and is a mistake. The project
// composes that directory because it declared it, so the flag loads the same files a SECOND time under
// a second source name. Nothing collides, because the two namespaces differ, so both copies run: every
// profile finding is reported twice, and the coverage line counts each subject again. On the tutorial
// board that turned 15 findings into 18 and 201 considered subjects into 213, with the three extra
// findings being the same three on the same subjects (agni issue 450).
//
// It refuses rather than silently dropping the duplicate, on the same reasoning the convention path
// one layer down already uses: an operator passing --conventions for the file their project declares
// gets a duplicate-source error rather than a merge. Resolving it quietly would leave the operator
// believing the flag did something.
//
// A design in NO project, a project declaring no profiles, or a flag naming somewhere else are all
// ordinary and return nil. So is any failure to resolve the project: this function's job is to refuse
// a known-bad combination, and a resolution error is reported by whichever call needed the project to
// do real work.
func refuseProfilePathTheProjectOwns(ctx context.Context, designArg, profilePath string) error {
	_, p, err := cliResolveProject(ctx, designArg)
	if err != nil || p == nil {
		return nil
	}
	flagDir, err := filepath.Abs(profilePath)
	if err != nil {
		return nil
	}
	ws, err := workspace()
	if err != nil {
		return nil
	}
	cfg := &osProjectConfig{mounts: ws.Mounts()}
	for _, uri := range p.GetConfig().GetProfileUris() {
		owned, err := cfg.dir(uri)
		if err != nil {
			continue
		}
		if filepath.Clean(owned) != filepath.Clean(flagDir) {
			continue
		}
		return fmt.Errorf("--profile-path %s names the profiles project %q already composes, so passing it would load them twice and report every profile finding twice; drop the flag",
			profilePath, p.GetName())
	}
	return nil
}
