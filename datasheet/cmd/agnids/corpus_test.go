package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/datasheet/corpus"
)

// promoteCorpus copies the tutorial project's params and turns ACME-LDO-1V8's seeded spec into a
// workbench DRAFT in the same directory, the state a transcription is in before anyone promotes it.
// It returns the corpus directory and the draft.
func promoteCorpus(t *testing.T) (dir, draft string) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), "params")
	if err := os.CopyFS(dir, os.DirFS("../../../examples/tutorial-project/params")); err != nil {
		t.Fatal(err)
	}
	seeded := filepath.Join(dir, "acme-ldo-1v8.textproto")
	b, err := os.ReadFile(seeded)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := param.Load(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	js, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(seeded); err != nil {
		t.Fatal(err)
	}
	// INSIDE the corpus directory, beside the seeded files, which is the case that proves a draft is not
	// read: a draft elsewhere would be unseen whatever LoadSet did.
	draft = filepath.Join(dir, "ACME-LDO-1V8.partspec.json")
	if err := os.WriteFile(draft, js, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, draft
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

// A draft is not seeded until it is promoted. Promotion writes the spec where LoadSet, which every
// check reads through, finds it, and records it in the index.
func TestPromotedDraftIsSeededAndIndexed(t *testing.T) {
	dir, draft := promoteCorpus(t)
	if set, err := param.LoadSet(os.DirFS(dir)); err != nil || set.Lookup("ACME-LDO-1V8") != nil {
		t.Fatalf("before promotion: err %v, or the draft was seeded", err)
	}
	out, err := runAgnids(t, "promote", draft, "--to", dir)
	if err != nil || !strings.Contains(out, "promoted ACME-LDO-1V8") || !strings.Contains(out, "index generation 1") {
		t.Fatalf("promote: %q, %v", out, err)
	}
	set, err := param.LoadSet(os.DirFS(dir))
	if err != nil || set.Lookup("ACME-LDO-1V8") == nil {
		t.Fatalf("after promotion LoadSet does not seed the part: %v", err)
	}
	ix, err := corpus.Read(os.DirFS(dir))
	if err != nil {
		t.Fatal(err)
	}
	if e, ok := ix.Lookup("ACME-LDO-1V8"); !ok || e.File != "ACME-LDO-1V8.textproto" || len(ix.Entries) != 2 {
		t.Errorf("index after promotion = %+v", ix)
	}
	// Running it again updates the same file rather than refusing its own earlier promotion.
	if out, err := runAgnids(t, "promote", draft, "--to", dir); err != nil || !strings.Contains(out, "updated ACME-LDO-1V8") {
		t.Errorf("re-promotion: %q, %v", out, err)
	}
	// The index promotion wrote is the one a rebuild of the files agrees with.
	if out, err := runAgnids(t, "index", dir, "--check"); err != nil {
		t.Errorf("index --check after promotion: %q, %v", out, err)
	}
}

// A refusal writes nothing, neither a spec nor an index, so a failed promotion cannot leave the corpus
// in a state no check loads.
func TestRefusedPromotionWritesNothing(t *testing.T) {
	dir, _ := promoteCorpus(t)
	empty := filepath.Join(dir, "SEEDED-EMPTY.partspec.json")
	if err := os.WriteFile(empty, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadDir(dir)
	_, err := runAgnids(t, "promote", empty, "--to", dir)
	if err == nil || !strings.Contains(err.Error(), "not ready for the corpus") {
		t.Fatalf("an empty draft was not refused: %v", err)
	}
	after, _ := os.ReadDir(dir)
	if len(after) != len(before) {
		t.Errorf("a refused promotion changed the corpus: %d files before, %d after", len(before), len(after))
	}
}

// --check is what a corpus repository's CI runs: missing and stale both fail and say what to run, and
// a rebuild makes it pass.
func TestIndexCheckCatchesAHandEdit(t *testing.T) {
	dir, _ := promoteCorpus(t)
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
