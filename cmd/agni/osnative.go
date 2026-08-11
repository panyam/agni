package main

import (
	"context"
	"errors"

	"github.com/panyam/agni/internal/artifact"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/internal/native"
	"github.com/panyam/agni/internal/service"
)

// osNative is the OS-backed service.NativeRenderer adapter: it gates and shells out to the format's
// own golden tool (the kicad-cli cache), translating the cmd gate errors to the service's
// ErrNative* sentinels so the service maps them to Connect codes without importing the cache.
type osNative struct {
	mounts  []mounts.Mount
	enabled map[string]bool
	cache   *native.Cache
}

// Available reports whether a NATIVE render can be offered for the file (a tool exists for the
// format and the operator enabled it). An unresolvable path is simply unavailable.
func (n *osNative) Available(uri artifact.URI) bool {
	abs, err := mounts.Resolve(n.mounts, uri)
	if err != nil {
		return false
	}
	return native.Available(abs, n.enabled)
}

// Render shells out to the native tool for the 1-based page, mapping the cmd gate errors to the
// service's ErrNative* sentinels.
func (n *osNative) Render(ctx context.Context, uri artifact.URI, page int) (string, error) {
	abs, err := mounts.Resolve(n.mounts, uri)
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
