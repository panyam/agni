package main

import (
	"context"
	"fmt"

	"os"
	"path/filepath"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/fshost"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/mounts"
	"github.com/panyam/agni/service"
)

// osProjectConfig is the OS-backed service.ConfigResolver: fshost.ConfigResolver over each mount's
// os.DirFS, with symbol libraries spelled as host paths because the CLI's and the server's loaders
// read the host filesystem.
type osProjectConfig struct {
	mounts []mounts.Mount
}

// ResolveConfig loads what an AnalysisConfig's URIs point at. See fshost.ConfigResolver.
func (c *osProjectConfig) ResolveConfig(ctx context.Context, cfg *webapi.AnalysisConfig, namespace string) (service.ResolvedConfig, error) {
	ms := make([]fshost.Mount, 0, len(c.mounts))
	for _, m := range c.mounts {
		ms = append(ms, fshost.Mount{Name: m.Name, FS: os.DirFS(m.Root)})
	}
	r := &fshost.ConfigResolver{Mounts: ms, SymbolPath: func(u artifact.URI) (string, error) {
		return mounts.Resolve(c.mounts, u)
	}}
	return r.ResolveConfig(ctx, cfg, namespace)
}

// dir resolves a project-config URI to a host directory inside its mount.
func (c *osProjectConfig) dir(uri string) (string, error) {
	u, err := artifact.Parse(uri)
	if err != nil {
		return "", err
	}
	return mounts.Resolve(c.mounts, u)
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
