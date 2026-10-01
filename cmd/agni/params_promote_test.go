package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/panyam/agni/datasheet/param"
)

// promoteProject copies the tutorial project and turns U2's seeded spec into a workbench DRAFT in the
// same directory, the state a transcription is in before anyone promotes it. It returns the project
// root and the draft.
func promoteProject(t *testing.T) (root, draft string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	root = filepath.Join(t.TempDir(), "tutorial-project")
	if err := os.CopyFS(root, os.DirFS("../../examples/tutorial-project")); err != nil {
		t.Fatal(err)
	}
	seeded := filepath.Join(root, "params", "acme-ldo-1v8.textproto")
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
	draft = filepath.Join(root, "params", "ACME-LDO-1V8.partspec.json")
	if err := os.MkdirAll(filepath.Dir(draft), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(draft, js, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	return root, draft
}

func absMaxVerdicts(t *testing.T) string {
	t.Helper()
	return runCLI(t, rootCmd(), "check", "designs/gateway", "--rule", "supply-exceeds-abs-max", "--verdicts")
}

// A draft reaches no check until it is promoted, and after promotion the rule it feeds fires. The U2
// verdict is the observable: its only datasheet is the draft.
func TestPromotedDraftReachesTheCheck(t *testing.T) {
	root, draft := promoteProject(t)
	if before := absMaxVerdicts(t); strings.Contains(before, "U2.1") {
		t.Fatalf("U2 was judged from a draft nobody promoted:\n%s", before)
	}
	out := runCLI(t, rootCmd(), "params", "promote", draft, "--to", filepath.Join(root, "params"))
	if !strings.Contains(out, "promoted ACME-LDO-1V8") {
		t.Errorf("promote output = %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "params", "ACME-LDO-1V8.textproto")); err != nil {
		t.Fatalf("no seeded file written: %v", err)
	}
	if after := absMaxVerdicts(t); !strings.Contains(after, "fail  U2.1") {
		t.Errorf("the promoted spec did not reach the check:\n%s", after)
	}
	// Running it again updates the same file rather than refusing its own earlier promotion.
	if out := runCLI(t, rootCmd(), "params", "promote", draft, "--to", filepath.Join(root, "params")); !strings.Contains(out, "updated ACME-LDO-1V8") {
		t.Errorf("re-promotion output = %q", out)
	}
}

// A refusal writes nothing, so a failed promotion cannot leave the corpus in a state no check loads.
func TestRefusedPromotionWritesNothing(t *testing.T) {
	root, _ := promoteProject(t)
	empty := filepath.Join(root, "params", "SEEDED-EMPTY.partspec.json")
	if err := os.WriteFile(empty, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadDir(filepath.Join(root, "params"))
	cmd := rootCmd()
	cmd.SetArgs([]string{"params", "promote", empty, "--to", filepath.Join(root, "params")})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "not ready for the corpus") {
		t.Fatalf("an empty draft was not refused: %v", err)
	}
	after, _ := os.ReadDir(filepath.Join(root, "params"))
	if len(after) != len(before) {
		t.Errorf("a refused promotion changed the corpus: %d files before, %d after", len(before), len(after))
	}
}

// `agni params <mpn>` still answers with a subcommand beside it.
func TestParamsStillLooksUpAnMPN(t *testing.T) {
	out := runCLI(t, rootCmd(), "params", "ACME-BUCK-3V3", "--params", "../../examples/tutorial-project/params")
	if !strings.Contains(out, "ACME-BUCK-3V3") {
		t.Errorf("params lookup output = %q", out)
	}
}
