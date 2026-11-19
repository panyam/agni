package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/datasheet/dsservice"
	dsapi "github.com/panyam/agni/datasheet/gen/go/agni/v1/dsapi"
	"github.com/panyam/agni/internal/mountset"
)

func TestAnnotationsDirAndSafeAuthor(t *testing.T) {
	if got := annotationsDir("ti/LM1117.pdf"); got != "ti/LM1117.annotations" {
		t.Errorf("annotationsDir = %q, want ti/LM1117.annotations", got)
	}
	if got := safeAuthor("../../etc/passwd"); strings.ContainsAny(got, `/.\`) {
		t.Errorf("safeAuthor kept a path char: %q", got)
	}
	if got := safeAuthor("alice-1_B"); got != "alice-1_B" {
		t.Errorf("safeAuthor mangled a safe id: %q", got)
	}
	if got := safeAuthor(""); got != "_" {
		t.Errorf("safeAuthor(empty) = %q, want _", got)
	}
}

func TestOsAnnotationStoreUnion(t *testing.T) {
	dir, corpus := t.TempDir(), t.TempDir()
	writePDF(t, filepath.Join(dir, "d.pdf"), "pdf-d")
	st := &osAnnotationStore{derived: derived{mounts: tableOf([]mountset.Mount{{Name: "m", Root: dir}}), corpus: corpus}}
	ctx := context.Background()
	region := func(id, typ string) []*dsapi.RegionAnnotation {
		return []*dsapi.RegionAnnotation{{RegionId: id, Type: typ}}
	}

	// An absent overlay reads as an empty union, not an error.
	sets, err := st.Get(ctx, mustURI("m", "d.pdf"))
	if err != nil || len(sets) != 0 {
		t.Fatalf("absent => sets=%v err=%v", sets, err)
	}

	// Two authors annotate the same datasheet.
	if err := st.Save(ctx, mustURI("m", "d.pdf"), "alice", &dsapi.AnnotationSet{DocId: "d", Author: "alice", Annotations: region("p1.t1", "table")}); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(ctx, mustURI("m", "d.pdf"), "bob", &dsapi.AnnotationSet{DocId: "d", Author: "bob", Annotations: region("p2.f1", "schematic")}); err != nil {
		t.Fatal(err)
	}

	// Get unions both, ordered by author for a stable read.
	sets, err = st.Get(ctx, mustURI("m", "d.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 2 || sets[0].GetAuthor() != "alice" || sets[1].GetAuthor() != "bob" {
		t.Fatalf("union = %d sets %v, want [alice bob]", len(sets), sets)
	}

	// An author overwrites ONLY its own file (no CAS); the other author's overlay is untouched.
	if err := st.Save(ctx, mustURI("m", "d.pdf"), "alice", &dsapi.AnnotationSet{DocId: "d", Author: "alice", Annotations: region("p3.t2", "chart")}); err != nil {
		t.Fatal(err)
	}
	sets, _ = st.Get(ctx, mustURI("m", "d.pdf"))
	if len(sets) != 2 || sets[0].GetAnnotations()[0].GetRegionId() != "p3.t2" || sets[1].GetAnnotations()[0].GetRegionId() != "p2.f1" {
		t.Fatalf("after alice overwrite: %v", sets)
	}

	// The overlay went into the corpus store, under the PDF's hash, as <author>.json, and nothing was
	// written into the mount (agni issue 1734).
	if _, err := os.Stat(filepath.Join(corpus, "derived", sha256Hex("pdf-d"), "annotations", "alice.json")); err != nil {
		t.Fatalf("alice's overlay not written to the store: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "d.annotations")); !os.IsNotExist(err) {
		t.Errorf("an annotation folder was written beside the PDF: %v", err)
	}
}

// An older agnids wrote annotations beside the PDF. Get still reads them, and the store's overlay
// for the same author wins. Without a corpus, Save has nowhere to write and says so.
func TestOsAnnotationStoreReadsTheOldLayout(t *testing.T) {
	dir, corpus := t.TempDir(), t.TempDir()
	writePDF(t, filepath.Join(dir, "d.pdf"), "pdf-d")
	old := filepath.Join(dir, "d.annotations")
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, a := range []string{"carol", "alice"} {
		body := `{"docId":"d","author":"` + a + `","annotations":[{"regionId":"old.` + a + `"}]}`
		if err := os.WriteFile(filepath.Join(old, a+".json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ms := []mountset.Mount{{Name: "m", Root: dir}}
	st := &osAnnotationStore{derived: derived{mounts: tableOf(ms), corpus: corpus}}
	ctx := context.Background()
	if err := st.Save(ctx, mustURI("m", "d.pdf"), "alice", &dsapi.AnnotationSet{DocId: "d", Author: "alice", Annotations: []*dsapi.RegionAnnotation{{RegionId: "new.alice"}}}); err != nil {
		t.Fatal(err)
	}
	sets, err := st.Get(ctx, mustURI("m", "d.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 2 || sets[0].GetAnnotations()[0].GetRegionId() != "new.alice" || sets[1].GetAnnotations()[0].GetRegionId() != "old.carol" {
		t.Fatalf("union = %v, want the store's alice and the old layout's carol", sets)
	}
	bare := &osAnnotationStore{derived: derived{mounts: tableOf(ms)}}
	if err := bare.Save(ctx, mustURI("m", "d.pdf"), "alice", &dsapi.AnnotationSet{Author: "alice"}); !errors.Is(err, dsservice.ErrNoCorpus) {
		t.Errorf("Save with no corpus = %v, want ErrNoCorpus", err)
	}
}
