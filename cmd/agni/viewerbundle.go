package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/panyam/agni/internal/version"
	"github.com/panyam/agni/internal/webbundle"
	"github.com/panyam/agni/web"
)

// viewerBundle supplies the viewer when nothing names a web dir and there is no ./web, which is how
// an installed binary runs (agni WS16-003). A release binary writes out the bundle it carries, and a
// `go install` binary downloads its release's. A variable so a test stands in for the cache and the
// network.
var viewerBundle = func() (dir, how string, err error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", "", fmt.Errorf("no cache directory to keep the viewer in: %w", err)
	}
	src := webbundle.Source{
		Version:   version.Version(),
		Releases:  webbundle.DefaultReleases,
		CacheRoot: filepath.Join(cache, "agni"),
		Client:    &http.Client{Timeout: 2 * time.Minute},
		Notes:     os.Stderr,
	}
	if fsys, ok := web.Bundle(); ok {
		src.Embedded = fsys
	}
	return src.Dir(context.Background())
}

// noViewerHint finishes the sentence a refusal starts when viewerBundle had nothing to give.
func noViewerHint(err error) string {
	if errors.Is(err, webbundle.ErrNoBundle) {
		return "This build carries no viewer and is not a release, so it has none to download. A release binary " +
			"(https://github.com/panyam/agni/releases) or `go install github.com/panyam/agni/cmd/agni@latest` has one"
	}
	return fmt.Sprintf("Getting the viewer failed: %v", err)
}
