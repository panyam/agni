package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/panyam/agni/artifact"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/service"
)

// partSpecSuffix is the sibling a datasheet's shared PartSpec is written to: LM1117.pdf pairs with
// LM1117.partspec.json (protojson, visible to anyone who mounts the folder). It is a DRAFT: saved
// unvalidated, and never read by param.LoadSet. `agni params promote` validates it into a corpus.
const partSpecSuffix = ".partspec.json"

// osPartSpecStore is the OS-backed service.PartSpecStore, reading and writing a datasheet's shared
// PartSpec as a sibling file in the mount (CONSTRAINTS C1/C13). Save is compare-and-swap under a
// per-file lock, and the version is the sibling's content hash. This is the one write path the
// workspace exposes, contained to the sibling by mounts.Resolve. The lock is per PROCESS, so two
// serve instances on one filesystem can still clobber each other.
type osPartSpecStore struct {
	mounts []mounts.Mount
	locks  sync.Map // abs path -> *sync.Mutex
}

func (s *osPartSpecStore) lockFor(abs string) *sync.Mutex {
	m, _ := s.locks.LoadOrStore(abs, &sync.Mutex{})
	return m.(*sync.Mutex)
}

// partSpecSibling maps a datasheet source path to its PartSpec sibling: foo/LM1117.pdf ->
// foo/LM1117.partspec.json.
func partSpecSibling(path string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + partSpecSuffix
}

func versionOf(b []byte) string {
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}

// Get reads the datasheet's PartSpec sibling. Absence is (nil, "", false, nil), the normal first-open
// state. version is the file's content hash, passed back as base_version on save.
func (s *osPartSpecStore) Get(ctx context.Context, uri artifact.URI) (*parampb.PartSpec, string, bool, error) {
	abs, err := resolveSibling(s.mounts, uri, partSpecSibling)
	if err != nil {
		return nil, "", false, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, "", false, nil
		}
		return nil, "", false, err
	}
	spec := &parampb.PartSpec{}
	if err := protojson.Unmarshal(data, spec); err != nil {
		return nil, "", false, err
	}
	return spec, versionOf(data), true, nil
}

// Save writes the PartSpec sibling with compare-and-swap: under the per-file lock it reads the
// current version, requires it to equal baseVersion (empty means "expected absent"), then writes.
// A mismatch is service.ErrConflict. The returned version is the hash of the bytes written, which
// a subsequent Get reproduces from the same file bytes.
func (s *osPartSpecStore) Save(ctx context.Context, uri artifact.URI, spec *parampb.PartSpec, baseVersion string) (string, error) {
	abs, err := resolveSibling(s.mounts, uri, partSpecSibling)
	if err != nil {
		return "", err
	}
	lock := s.lockFor(abs)
	lock.Lock()
	defer lock.Unlock()

	cur := ""
	if data, err := os.ReadFile(abs); err == nil {
		cur = versionOf(data)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if cur != baseVersion {
		return "", service.ErrConflict
	}

	out, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(spec)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, out, 0o644); err != nil {
		return "", err
	}
	return versionOf(out), nil
}
