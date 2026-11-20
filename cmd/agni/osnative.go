package main

import (
	"context"
	"errors"

	"github.com/panyam/goapplib/storage/mounts"
	"github.com/panyam/agni/internal/native"
	"github.com/panyam/agni/mountset"
	"github.com/panyam/agni/service"
)

// osNative is the OS-backed service.NativeRenderer adapter. It checks the gate and shells out to the
// format's own golden tool, the kicad-cli cache. It translates the cmd gate errors to the service's
// ErrNative* sentinels, so the service maps them to Connect codes without importing the cache.
type osNative struct {
	mounts  *mountset.Table
	enabled map[string]bool
	cache   *native.Cache
	// guard refuses a render of a private source the caller can't read (agni issue 1728).
	guard *service.AccessGuard
}

// Available reports whether a NATIVE render can be offered for the file (a tool exists for the
// format and the operator enabled it). An unresolvable path is simply unavailable.
func (n *osNative) Available(uri mounts.URI) bool {
	abs, err := n.mounts.Resolve(uri)
	if err != nil {
		return false
	}
	return native.Available(abs, n.enabled)
}

// Render shells out to the native tool for the 1-based page, mapping the cmd gate errors to the
// service's ErrNative* sentinels.
func (n *osNative) Render(ctx context.Context, uri mounts.URI, page int) (string, error) {
	if err := n.guard.URI(ctx, uri); err != nil {
		return "", err
	}
	abs, err := n.mounts.Resolve(uri)
	if err != nil {
		return "", err
	}
	svg, err := n.cache.Render(ctx, abs, page, n.enabled)
	if err != nil {
		return "", mapNativeErr(err)
	}
	return svg, nil
}

// mapNativeErr translates the cmd native-cache gate errors to the service's runtime-neutral
// sentinels.
func mapNativeErr(err error) error {
	switch {
	case errors.Is(err, native.ErrNoTool):
		return service.ErrNativeNoTool
	case errors.Is(err, native.ErrNotEnabled):
		return service.ErrNativeNotEnabled
	case errors.Is(err, native.ErrNotFound):
		return service.ErrNativeNotFound
	default:
		return err
	}
}
