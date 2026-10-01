package main

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/panyam/agni/artifact"
	dsapi "github.com/panyam/agni/gen/go/agni/v1/dsapi"
	"github.com/panyam/agni/mounts"
)

// annotationsDirSuffix names the per-datasheet directory holding one region-annotation file per
// author, so LM1117.pdf pairs with LM1117.annotations/ and author "alice" writes
// LM1117.annotations/alice.json. One file per author means Save never contends and Get is a plain
// directory union (WS13-011).
const annotationsDirSuffix = ".annotations"

// osAnnotationStore is the OS-backed dsservice.AnnotationStore, writing per-author region-annotation
// overlays under the mount. It follows osPartSpecStore's I/O discipline (mounts.Resolve containment,
// per-file lock, protojson) except that there is NO compare-and-swap, since each author owns their
// file, and Get UNIONS every author's file for one datasheet. Anyone who mounts the folder sees every
// overlay, because author namespaces and does not authenticate (mounts are the security boundary).
type osAnnotationStore struct {
	mounts []mounts.Mount
	locks  sync.Map // abs author-file path -> *sync.Mutex
}

func (s *osAnnotationStore) lockFor(abs string) *sync.Mutex {
	m, _ := s.locks.LoadOrStore(abs, &sync.Mutex{})
	return m.(*sync.Mutex)
}

// annotationsDir maps a datasheet source path to its per-author annotation directory:
// foo/LM1117.pdf -> foo/LM1117.annotations.
func annotationsDir(path string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + annotationsDirSuffix
}

// safeAuthor maps a client-supplied author id to a single filename component, keeping [A-Za-z0-9_-]
// and replacing everything else with '_', so an author cannot traverse out of its datasheet's
// annotation directory (mounts.Resolve is the containment backstop). Distinct exotic authors may
// collide after mapping.
func safeAuthor(author string) string {
	var b strings.Builder
	for _, r := range author {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "_"
	}
	return b.String()
}

// Get returns every author's overlay for the datasheet, ordered by author for a stable union. A
// datasheet nobody has annotated (the directory does not exist) is (nil, nil), a normal state.
func (s *osAnnotationStore) Get(ctx context.Context, uri artifact.URI) ([]*dsapi.AnnotationSet, error) {
	dir, err := resolveSibling(s.mounts, uri, annotationsDir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var sets []*dsapi.AnnotationSet
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		set := &dsapi.AnnotationSet{}
		if err := protojson.Unmarshal(data, set); err != nil {
			return nil, err
		}
		sets = append(sets, set)
	}
	sort.Slice(sets, func(i, j int) bool { return sets[i].GetAuthor() < sets[j].GetAuthor() })
	return sets, nil
}

// Save writes one author's overlay, creating the annotation directory on first write and
// overwriting just that author's file. The per-file lock only serializes one author's concurrent
// writes, and there is no compare-and-swap.
func (s *osAnnotationStore) Save(ctx context.Context, uri artifact.URI, author string, set *dsapi.AnnotationSet) error {
	dir, err := resolveSibling(s.mounts, uri, annotationsDir)
	if err != nil {
		return err
	}
	file := filepath.Join(dir, safeAuthor(author)+".json")
	lock := s.lockFor(file)
	lock.Lock()
	defer lock.Unlock()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	out, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(set)
	if err != nil {
		return err
	}
	return os.WriteFile(file, out, 0o644)
}
