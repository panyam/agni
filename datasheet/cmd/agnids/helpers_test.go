package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/panyam/agni/mountset"
	"github.com/panyam/goapplib/storage/mounts"
)

// mustURI builds a mount-relative URI a test names directly, failing the test rather than the
// assertion when the literal is malformed.
func mustURI(mount, p string) mounts.URI {
	u, err := mounts.NewURI(mount, p)
	if err != nil {
		panic(err)
	}
	return u
}

// writePDF writes body as a stand-in PDF at path. The derived store keys by the bytes' hash, so a test
// that reads the store back needs the file to exist.
func writePDF(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// sha256Hex is the derived store's folder name for a PDF whose bytes are body.
func sha256Hex(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

// tableOf is a fixed mount table over ms, as a server started with those flags serves.
func tableOf(ms []mounts.Mount) *mountset.Table { return mountset.NewTable(ms...) }
