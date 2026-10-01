package main

import (
	"context"
	"fmt"

	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/mounts"
	"github.com/panyam/agni/service"
	"github.com/panyam/agni/stdlib/lib"
	"github.com/panyam/agni/stdlib/profiles"
	"github.com/panyam/agni/stdlib/rules/intent"
	"github.com/panyam/jaala/datalog"
)

// osProjectConfig is the OS-backed service.ConfigResolver. It reads the interface profiles and
// seeded parameters a project names, from the mounts.
//
// It holds NO CACHE. An operator edits a profile or seeds a part while the server runs, and an index
// answering with the previous version would produce a confident wrong verdict. If re-reading a large
// parameter corpus per request ever costs too much, copy internal/projects/cache.go, which caches
// and still stats every file it depends on before answering.
type osProjectConfig struct {
	mounts []mounts.Mount
}

// ResolveConfig loads what an AnalysisConfig's URIs point at, whether it came from a project
// descriptor or from a request.
//
// A tier that fails to load is an ERROR, not a skip, since an operator who wrote a profiles
// directory and silently got the built-ins would read the clean report as a clean design (C24). A
// tier the config does not name is not an error, because the loops below run zero times.
func (c *osProjectConfig) ResolveConfig(_ context.Context, cfg *webapi.AnalysisConfig, namespace string) (service.ResolvedConfig, error) {
	var out service.ResolvedConfig
	// read records every host path this resolution opened, so the digest covers what was read
	// rather than what the config named. One URI can resolve to different bytes on two servers.
	var read []string
	for _, uri := range cfg.GetProfileUris() {
		dir, err := c.dir(uri)
		if err != nil {
			return service.ResolvedConfig{}, err
		}
		read = append(read, dir)
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
		read = append(read, dir)
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
	// Intent composes as its own rule source. A config declaring none contributes nothing, so the
	// intent-bound checklist items read needs-design-intent rather than passing.
	if uri := cfg.GetIntentUri(); uri != "" {
		abs, err := c.file(uri)
		if err != nil {
			return service.ResolvedConfig{}, err
		}
		read = append(read, abs)
		decl, err := intent.LoadFile(abs)
		if err != nil {
			return service.ResolvedConfig{}, fmt.Errorf("%s intent %s: %w", namespace, uri, err)
		}
		out.Sources = append(out.Sources, intent.Source("intent", decl))
		out.Intent = true
	}
	// A project's own library is read here and composed by the service (Overlay.Registry), so a module
	// that does not parse or collides is reported when a query first runs over it, naming the file.
	for _, uri := range cfg.GetLibraryUris() {
		dir, err := c.dir(uri)
		if err != nil {
			return service.ResolvedConfig{}, err
		}
		read = append(read, dir)
		mods, docs, err := lib.Read(os.DirFS(dir))
		if err != nil {
			return service.ResolvedConfig{}, fmt.Errorf("%s library %s: %w", namespace, uri, err)
		}
		for _, m := range mods {
			out.Library = append(out.Library, service.LibraryModule{
				Path: m.Path, Language: datalog.LanguageName, Text: m.Text, Source: strings.TrimSuffix(uri, "/") + "/" + m.File,
			})
		}
		for p, d := range docs {
			if out.LibraryDocs == nil {
				out.LibraryDocs = map[string]string{}
			}
			out.LibraryDocs[p] = d
		}
	}
	// A resolution that read nothing still gets a digest identifying it as such. Symbol paths go in
	// as NAMES, since this call never opens them and statting a URI would fail.
	digest, err := digestConfig(read, cfg.GetSymbolPathUris())
	if err != nil {
		return service.ResolvedConfig{}, fmt.Errorf("%s config digest: %w", namespace, err)
	}
	out.Digest = digest
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

// sourceName is the catalog namespace a project's interface profiles appear under, so a
// finding reads `gateway-profiles/can-esd-missing` and says which project asked for it.
func sourceName(namespace string) string {
	if id, ok := service.ProjectID(namespace); ok {
		return id + "-profiles"
	}
	// A namespace that is not a project resource name is a request's. It keeps the `-profiles` suffix
	// and cannot collide with a project's, because a project id can never be the literal "request".
	return namespace + "-profiles"
}

// refuseProfilePathTheProjectOwns rejects a --profile-path naming a directory the design's own
// project already composes.
//
// The flag would load the project's own profiles a SECOND time under a second source name. The
// namespaces differ, so nothing collides and both copies run, reporting every profile finding twice.
// On the tutorial board that turned 15 findings into 18 and 201 considered subjects into 213 (agni
// issue 450). It refuses rather than dropping the duplicate, as --conventions does (see
// service/overlay.go).
//
// A design in NO project, a project declaring no profiles, or a flag naming somewhere else return
// nil. So does any failure to resolve the project, which whichever call needs the project reports.
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
