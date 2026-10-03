package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/encoding/protojson"
)

// sharedChecklistTree is a project that extends a config-only one, both under one mount that an
// agni.yaml declares, which is how a team shares its checklists (agni issue 829).
func sharedChecklistTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	indent := func(path string) string {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return "    " + strings.ReplaceAll(strings.TrimRight(string(b), "\n"), "\n", "\n    ") + "\n"
	}
	design, err := os.ReadFile("testdata/review/can-broken.edn")
	if err != nil {
		t.Fatal(err)
	}
	write("house/project.yaml", "name: house\ntitle: House\nchecklists:\n  rails:\n"+indent("testdata/intent/rails-checklist.yaml"))
	write("proj/project.yaml", "name: proj\ntitle: Proj\nextends: projects/house\nchecklists:\n  review:\n"+indent("testdata/review/mini.yaml"))
	write("proj/designs/d/board.edn", string(design))
	write("proj/designs/d/design.yaml", "name: d\ntitle: D\nentry: board.edn\n")
	write("agni.yaml", "mounts:\n  t: "+root+"\n")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	t.Chdir(root)
	return root
}

func runRoot(t *testing.T, args ...string) string {
	t.Helper()
	cmd := rootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out.String())
	}
	return out.String()
}

// `agni checklists` reads the resolver ListChecklists and `review --checklist` read, so an inherited
// checklist is listed, first, and is the one review runs by default.
func TestChecklistsListsInheritedOnesFirst(t *testing.T) {
	sharedChecklistTree(t)
	var resp webapi.ListChecklistsResponse
	if err := protojson.Unmarshal([]byte(runRoot(t, "checklists", "proj/designs/d", "--format", "json")), &resp); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range resp.GetChecklists() {
		names = append(names, c.GetName())
	}
	if resp.GetProject() != "projects/proj" || strings.Join(names, ",") != "rails,review" {
		t.Errorf("checklists = %s %v, want projects/proj with rails (inherited) then review", resp.GetProject(), names)
	}
	if text := runRoot(t, "checklists", "proj/designs/d"); !strings.Contains(text, "rails  Rail sizing review  (default)") {
		t.Errorf("text = %q, want the inherited checklist marked as the default", text)
	}
	if out := runRoot(t, "review", "proj/designs/d"); !strings.Contains(out, "# Review: Rail sizing review") {
		t.Errorf("review with no --checklist ran %q, want the inherited default", strings.SplitN(out, "\n", 2)[0])
	}
}
