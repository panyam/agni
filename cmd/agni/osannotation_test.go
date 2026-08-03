package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/mounts"
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
	dir := t.TempDir()
	st := &osAnnotationStore{mounts: []mounts.Mount{{Name: "m", Root: dir}}}
	ctx := context.Background()
	region := func(id, typ string) []*webapi.RegionAnnotation {
		return []*webapi.RegionAnnotation{{RegionId: id, Type: typ}}
	}

	// Absent: an empty union, not an error.
	sets, err := st.Get(ctx, "m", "d.pdf")
	if err != nil || len(sets) != 0 {
		t.Fatalf("absent => sets=%v err=%v", sets, err)
	}

	// Two authors annotate the same datasheet.
	if err := st.Save(ctx, "m", "d.pdf", "alice", &webapi.AnnotationSet{DocId: "d", Author: "alice", Annotations: region("p1.t1", "table")}); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(ctx, "m", "d.pdf", "bob", &webapi.AnnotationSet{DocId: "d", Author: "bob", Annotations: region("p2.f1", "schematic")}); err != nil {
		t.Fatal(err)
	}

	// Get unions both, ordered by author for a stable read.
	sets, err = st.Get(ctx, "m", "d.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 2 || sets[0].GetAuthor() != "alice" || sets[1].GetAuthor() != "bob" {
		t.Fatalf("union = %d sets %v, want [alice bob]", len(sets), sets)
	}

	// An author overwrites ONLY its own file (no CAS); the other author's overlay is untouched.
	if err := st.Save(ctx, "m", "d.pdf", "alice", &webapi.AnnotationSet{DocId: "d", Author: "alice", Annotations: region("p3.t2", "chart")}); err != nil {
		t.Fatal(err)
	}
	sets, _ = st.Get(ctx, "m", "d.pdf")
	if len(sets) != 2 || sets[0].GetAnnotations()[0].GetRegionId() != "p3.t2" || sets[1].GetAnnotations()[0].GetRegionId() != "p2.f1" {
		t.Fatalf("after alice overwrite: %v", sets)
	}

	// The overlay landed in the per-datasheet annotation directory as <author>.json.
	if _, err := os.Stat(filepath.Join(dir, "d.annotations", "alice.json")); err != nil {
		t.Fatalf("alice's overlay not written to the annotation dir: %v", err)
	}
}
