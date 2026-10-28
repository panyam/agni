package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/datasheet/corpus"
	dsapi "github.com/panyam/agni/datasheet/gen/go/agni/v1/dsapi"
)

// draftCorpus copies the tutorial project's params and turns ACME-LDO-1V8's published spec into a
// DRAFT in the corpus's store, the state a transcription is in before anyone publishes it. It
// returns the corpus directory.
func draftCorpus(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "params")
	if err := os.CopyFS(dir, os.DirFS("../../../examples/tutorial-project/params")); err != nil {
		t.Fatal(err)
	}
	published := filepath.Join(dir, "acme-ldo-1v8.textproto")
	b, err := os.ReadFile(published)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := param.Load(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(published); err != nil {
		t.Fatal(err)
	}
	d := &dsapi.Draft{Mpn: spec.GetMpn(), Spec: spec, DocumentUris: []string{"mount://ds/acme/ldo.pdf"}}
	if _, err := newOSDraftStore(dir).Save(context.Background(), d, ""); err != nil {
		t.Fatal(err)
	}
	return dir
}

func runAgnids(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := rootCmd()
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	err := cmd.Execute()
	return out.String(), err
}

// A draft is not seeded until it is published. Publishing writes the spec where LoadSet, which every
// check reads through, finds it, and records it in the index; the draft stays.
func TestPublishedDraftIsSeededAndIndexed(t *testing.T) {
	dir := draftCorpus(t)
	if set, err := param.LoadSet(os.DirFS(dir)); err != nil || set.Lookup("ACME-LDO-1V8") != nil {
		t.Fatalf("before publishing: err %v, or the draft was seeded", err)
	}
	out, err := runAgnids(t, "publish", "acme-ldo-1v8", "--corpus", dir)
	if err != nil || !strings.Contains(out, "published acme-ldo-1v8") || !strings.Contains(out, "index generation 1") {
		t.Fatalf("publish: %q, %v", out, err)
	}
	set, err := param.LoadSet(os.DirFS(dir))
	if err != nil || set.Lookup("ACME-LDO-1V8") == nil {
		t.Fatalf("after publishing LoadSet does not seed the part: %v", err)
	}
	ix, err := corpus.Read(os.DirFS(dir))
	if err != nil {
		t.Fatal(err)
	}
	if e, ok := ix.Lookup("ACME-LDO-1V8"); !ok || e.File != "ACME-LDO-1V8.textproto" || len(ix.Entries) != 2 {
		t.Errorf("index after publishing = %+v", ix)
	}
	if _, found, _ := newOSDraftStore(dir).Get(context.Background(), "ACME-LDO-1V8"); !found {
		t.Error("publishing removed the draft, which is the start of the next edit")
	}
	if out, err := runAgnids(t, "publish", "ACME-LDO-1V8", "--corpus", dir); err != nil || !strings.Contains(out, "republished") {
		t.Errorf("publishing again: %q, %v", out, err)
	}
	if out, err := runAgnids(t, "index", dir, "--check"); err != nil {
		t.Errorf("index --check after publishing: %q, %v", out, err)
	}
}

// A refusal writes nothing, neither a spec nor an index, so a failed publish cannot leave the corpus
// in a state no check loads.
func TestRefusedPublishWritesNothing(t *testing.T) {
	dir := draftCorpus(t)
	empty := &dsapi.Draft{Mpn: "SEEDED-EMPTY", Spec: unprovenanced("SEEDED-EMPTY")}
	if _, err := newOSDraftStore(dir).Save(context.Background(), empty, ""); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadDir(dir)
	_, err := runAgnids(t, "publish", "SEEDED-EMPTY", "--corpus", dir)
	if err == nil || !strings.Contains(err.Error(), "not ready for the corpus") {
		t.Fatalf("an empty draft was not refused: %v", err)
	}
	after, _ := os.ReadDir(dir)
	if len(after) != len(before) {
		t.Errorf("a refused publish changed the corpus: %d entries before, %d after", len(before), len(after))
	}
}

// --check is what a corpus repository's CI runs: missing and stale both fail and say what to run, and
// a rebuild makes it pass.
func TestIndexCheckCatchesAHandEdit(t *testing.T) {
	dir := draftCorpus(t)
	if _, err := runAgnids(t, "index", dir, "--check"); err == nil || !strings.Contains(err.Error(), "has no "+corpus.IndexFile) {
		t.Fatalf("an unindexed corpus passed --check: %v", err)
	}
	if out, err := runAgnids(t, "index", dir); err != nil || !strings.Contains(out, "generation 1") {
		t.Fatalf("index: %q, %v", out, err)
	}
	spec := filepath.Join(dir, "acme-buck-3v3.textproto")
	b, err := os.ReadFile(spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(spec, append(b, []byte("\n# reviewed by hand\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = runAgnids(t, "index", dir, "--check")
	if err == nil || !strings.Contains(err.Error(), "changed ACME-BUCK-3V3") {
		t.Fatalf("--check missed a hand edit: %v", err)
	}
	if out, err := runAgnids(t, "index", dir); err != nil || !strings.Contains(out, "generation 2") {
		t.Fatalf("rebuild after the edit: %q, %v", out, err)
	}
	if _, err := runAgnids(t, "index", dir, "--check"); err != nil {
		t.Errorf("--check after a rebuild: %v", err)
	}
}

const pinMuxFixture = "../../../cmd/agni/testdata/pin-mux"

// muxCorpus is a corpus publishing the KW47 spec whose pins list no alternates, indexed, and the
// pin-mux reference that supplies them.
func muxCorpus(t *testing.T) (dir, mux string) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), "params")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(pinMuxFixture, "kw47.textproto"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kw47.textproto"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runAgnids(t, "index", dir); err != nil {
		t.Fatal(err)
	}
	return dir, filepath.Join(pinMuxFixture, "kw47.pin_mux.yaml")
}

// Merging a mux table rewrites the file that already seeds the MPN, whatever it is named, so the corpus
// still holds one file per MPN, and the index records the new content.
func TestMergeMuxRewritesThePublishedSpec(t *testing.T) {
	dir, mux := muxCorpus(t)
	out, err := runAgnids(t, "merge-mux", mux, "--corpus", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "merged into kw47.textproto for KW47; index generation 2") {
		t.Errorf("output: %q", out)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.textproto"))
	if len(files) != 1 {
		t.Fatalf("the corpus must still hold one spec file, got %v", files)
	}
	set, err := param.LoadSet(os.DirFS(dir))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range set.Lookup("KW47").GetPins() {
		if len(p.Alternates) == 0 {
			t.Errorf("%s has no alternates after the merge", p.Id)
		}
	}
	if _, err := runAgnids(t, "index", dir, "--check"); err != nil {
		t.Errorf("the index must match the merged file: %v", err)
	}
}

// An MPN the corpus does not seed is published from the reference alone, under SpecFileName.
func TestMergeMuxPublishesAnUnseededPart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "params")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	mux := filepath.Join(t.TempDir(), "kw47.pin_mux.yaml")
	if err := os.WriteFile(mux, []byte(`
mpn: KW47
docs: [{id: rm, title: KW47 Reference Manual Rev. 2}]
pins:
  - {id: ptb3, name: PTB3, alternates: [{name: PTB3}], prov: {doc_ref: rm, page: 58, method: hand, confidence: 1}}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runAgnids(t, "merge-mux", mux, "--corpus", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "published KW47.textproto for KW47") {
		t.Errorf("output: %q", out)
	}
	if set, err := param.LoadSet(os.DirFS(dir)); err != nil || set.Lookup("KW47") == nil {
		t.Errorf("the reference must be seeded: %v", err)
	}
}

// A merge whose result Validate rejects writes nothing and lists the problem.
func TestMergeMuxRefusesAnInvalidResultAndWritesNothing(t *testing.T) {
	dir, _ := muxCorpus(t)
	before, err := os.ReadFile(filepath.Join(dir, "kw47.textproto"))
	if err != nil {
		t.Fatal(err)
	}
	ix, err := os.ReadFile(filepath.Join(dir, corpus.IndexFile))
	if err != nil {
		t.Fatal(err)
	}
	mux := filepath.Join(t.TempDir(), "bad.pin_mux.yaml")
	if err := os.WriteFile(mux, []byte("mpn: KW47\npins: [{id: ptd0, name: PTD0, alternates: [{name: PTD0}]}]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = runAgnids(t, "merge-mux", mux, "--corpus", dir)
	if err == nil || !strings.Contains(err.Error(), "ptd0: no prov") {
		t.Fatalf("want the missing prov named, got %v", err)
	}
	after, _ := os.ReadFile(filepath.Join(dir, "kw47.textproto"))
	ixAfter, _ := os.ReadFile(filepath.Join(dir, corpus.IndexFile))
	if !bytes.Equal(before, after) || !bytes.Equal(ix, ixAfter) {
		t.Errorf("a refused merge changed the corpus")
	}
}
