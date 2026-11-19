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

	"github.com/panyam/goapplib/storage/mounts"
	dsapi "github.com/panyam/agni/datasheet/gen/go/agni/v1/dsapi"
)

// annotationsDirSuffix names the per-datasheet directory holding one region-annotation file per
// author, so LM1117.pdf pairs with LM1117.annotations/ and author "alice" writes
// LM1117.annotations/alice.json. One file per author means Save never contends and Get is a plain
// directory union (WS13-011).
const annotationsDirSuffix = ".annotations"

// osAnnotationStore is the OS-backed dsservice.AnnotationStore. It writes per-author
// region-annotation overlays into the corpus store, in an annotations folder under the datasheet's
// derived folder (see derived). It follows the stores' shared I/O discipline of a per-file lock and
// protojson, with two differences. There is NO compare-and-swap, since each author owns their file.
// And Get UNIONS every author's file for one datasheet. Anyone who can read the datasheet sees every
// overlay, because the author is a namespace and not an authentication.
type osAnnotationStore struct {
	derived derived
	locks   sync.Map // abs author-file path -> *sync.Mutex
}

// annotationsFolder is the annotations' folder inside a datasheet's derived folder.
const annotationsFolder = "annotations"

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

// Get returns every author's overlay for the datasheet, ordered by author for a stable union. It
// reads the store, then the folder an older agnids wrote beside the PDF, and an author's overlay in
// the store wins over the same author's beside the PDF. For a datasheet nobody has annotated, Get
// returns (nil, nil). That is a normal state.
func (s *osAnnotationStore) Get(ctx context.Context, uri mounts.URI) ([]*dsapi.AnnotationSet, error) {
	var dirs []string
	if s.derived.corpus != "" {
		dir, err := s.derived.dir(uri)
		if err != nil {
			return nil, err
		}
		dirs = append(dirs, filepath.Join(dir, annotationsFolder))
	}
	old, err := s.derived.legacy(uri, annotationsDir)
	if err != nil {
		return nil, err
	}
	dirs = append(dirs, old)
	byFile := map[string]*dsapi.AnnotationSet{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || byFile[e.Name()] != nil {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name())) // #nosec G304 -- reads an annotation folder under the corpus or the mount
			if err != nil {
				return nil, err
			}
			set := &dsapi.AnnotationSet{}
			if err := protojson.Unmarshal(data, set); err != nil {
				return nil, err
			}
			byFile[e.Name()] = set
		}
	}
	var sets []*dsapi.AnnotationSet
	for _, set := range byFile {
		sets = append(sets, set)
	}
	sort.Slice(sets, func(i, j int) bool { return sets[i].GetAuthor() < sets[j].GetAuthor() })
	return sets, nil
}

// Save writes one author's overlay into the store, creating the annotation folder on first write and
// overwriting just that author's file. A server with no corpus returns dsservice.ErrNoCorpus. The
// per-file lock only serializes one author's concurrent writes, and there is no compare-and-swap.
func (s *osAnnotationStore) Save(ctx context.Context, uri mounts.URI, author string, set *dsapi.AnnotationSet) error {
	derivedDir, err := s.derived.dir(uri)
	if err != nil {
		return err
	}
	dir := filepath.Join(derivedDir, annotationsFolder)
	file := filepath.Join(dir, safeAuthor(author)+".json")
	lock := s.lockFor(file)
	lock.Lock()
	defer lock.Unlock()

	if err := os.MkdirAll(dir, 0o755); err != nil { // #nosec G301 -- the shared corpus, which the engine reads too
		return err
	}
	out, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(set)
	if err != nil {
		return err
	}
	return os.WriteFile(file, out, 0o644) // #nosec G306 -- the shared corpus, which the engine reads too
}
