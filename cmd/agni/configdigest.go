package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/panyam/agni/fshost"
)

// digestConfig is fshost.DigestConfig over host paths, each a file or a directory.
func digestConfig(paths, names []string) (string, error) {
	roots := make([]fshost.DigestRoot, 0, len(paths))
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return "", fmt.Errorf("digest %s: %w", p, err)
		}
		if info.IsDir() {
			roots = append(roots, fshost.DigestRoot{FS: os.DirFS(p), Name: "."})
		} else {
			roots = append(roots, fshost.DigestRoot{FS: os.DirFS(filepath.Dir(p)), Name: filepath.Base(p)})
		}
	}
	return fshost.DigestConfig(roots, names)
}
