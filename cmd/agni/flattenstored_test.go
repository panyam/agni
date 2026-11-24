package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/mountset"
)

// hierDescriptor names hier_blocks.edn the entry, with flatten as the flatten_hierarchy line, or no
// such line when flatten is "".
func hierDescriptor(flatten string) string {
	s := "name: hb\nentry: hier_blocks.edn\n"
	if flatten != "" {
		s += "flatten_hierarchy: " + flatten + "\n"
	}
	return s
}

// hierFolder writes the EDIF reader's hierarchical fixture and a design.yaml into dir/hw, and
// returns that folder.
func hierFolder(t *testing.T, dir, descriptor string) string {
	t.Helper()
	hw := filepath.Join(dir, "hw")
	if err := os.MkdirAll(hw, 0o750); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "readers", "edif", "testdata", "hier_blocks.edn"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hw, "hier_blocks.edn"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hw, "design.yaml"), []byte(descriptor), 0o600); err != nil {
		t.Fatal(err)
	}
	return hw
}

// componentCount is how many components a query of design reads, with extra args such as --remote.
func componentCount(t *testing.T, design string, extra ...string) int {
	t.Helper()
	out, err := runAgni(t, append([]string{"query", design, "component.net_count(?r, ?n) => ?r", "--format", "json"}, extra...)...)
	if err != nil {
		t.Fatalf("query %s: %v\n%s", design, err, out)
	}
	var resp struct {
		Rows []json.RawMessage `json:"rows"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("query %s: %v\n%s", design, err, out)
	}
	return len(resp.Rows)
}

// storedFlatten is a stored design's flatten_hierarchy.
func storedFlatten(t *testing.T, url, id string) bool {
	t.Helper()
	d, err := webapiconnect.NewDesignServiceClient(http.DefaultClient, url).GetDesign(context.Background(), connect.NewRequest(&webapi.GetDesignRequest{Name: "designs/" + id}))
	if err != nil {
		t.Fatal(err)
	}
	return d.Msg.GetFlattenHierarchy()
}

// A design pushed, added or linked from a folder whose design.yaml sets flatten_hierarchy reads its
// netlist whole, as a query of the folder does. A later design.yaml setting it false turns it off,
// and one with no such line leaves it as it was, as an intent: section does (agni issue 1887).
func TestAStoredDesignReadsWholeWhenItsDesignYAMLSaysSo(t *testing.T) {
	whole := componentCount(t, hierFolder(t, t.TempDir(), hierDescriptor("true")))
	top := componentCount(t, hierFolder(t, t.TempDir(), hierDescriptor("")))
	if whole <= top {
		t.Fatalf("the folder reads %d components flattened and %d at its top cell; want more flattened, or the stored reads below prove nothing", whole, top)
	}
	cases := map[string]func(t *testing.T, url, lab string){
		"push": func(t *testing.T, url, lab string) {
			if out, err := runDesigns(t, url, "push", filepath.Join(lab, "hw"), "--id", "hb"); err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
		},
		"add": func(t *testing.T, url, _ string) {
			if out, err := runDesigns(t, url, "add", "lab/hw", "--only", "hw=hb", "--update"); err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
		},
	}
	for name, store := range cases {
		t.Run(name, func(t *testing.T) {
			lab := t.TempDir()
			hw := hierFolder(t, lab, hierDescriptor("true"))
			srv := designServer(t, t.TempDir(), t.TempDir(), mountset.Folder("lab", lab))
			store(t, srv.URL, lab)
			if !storedFlatten(t, srv.URL, "hb") {
				t.Errorf("designs/hb keeps flatten_hierarchy = false; want its design.yaml's true")
			}
			if got := componentCount(t, "designs/hb", "--remote", srv.URL); got != whole {
				t.Errorf("designs/hb reads %d components; want %d, as its folder reads flattened", got, whole)
			}
			if err := os.WriteFile(filepath.Join(hw, "design.yaml"), []byte(hierDescriptor("")), 0o600); err != nil {
				t.Fatal(err)
			}
			store(t, srv.URL, lab)
			if !storedFlatten(t, srv.URL, "hb") {
				t.Errorf("a design.yaml with no flatten_hierarchy line cleared it; want it left, as intent is left")
			}
			if err := os.WriteFile(filepath.Join(hw, "design.yaml"), []byte(hierDescriptor("false")), 0o600); err != nil {
				t.Fatal(err)
			}
			store(t, srv.URL, lab)
			if storedFlatten(t, srv.URL, "hb") {
				t.Errorf("a design.yaml setting flatten_hierarchy: false left it on; want it off")
			}
			if got := componentCount(t, "designs/hb", "--remote", srv.URL); got != top {
				t.Errorf("designs/hb reads %d components once flattening is off; want %d, its top cell", got, top)
			}
		})
	}
}

// A netlist linked from a GitHub repo whose design.yaml sets flatten_hierarchy reads whole, and a
// link to a folder whose design.yaml sets it false turns it off (agni issue 1887).
func TestALinkedDesignReadsWholeWhenItsDesignYAMLSaysSo(t *testing.T) {
	edn, err := os.ReadFile(filepath.Join("..", "..", "readers", "edif", "testdata", "hier_blocks.edn"))
	if err != nil {
		t.Fatal(err)
	}
	whole := componentCount(t, hierFolder(t, t.TempDir(), hierDescriptor("true")))
	top := componentCount(t, hierFolder(t, t.TempDir(), hierDescriptor("")))
	gh, _ := privateGitHubRepo(t)
	gh.AddRepo("acme", "open", false, map[string]string{
		"whole/design.yaml": hierDescriptor("true"), "whole/hier_blocks.edn": string(edn),
		"top/design.yaml": hierDescriptor("false"), "top/hier_blocks.edn": string(edn),
	})
	dir := t.TempDir()
	url := serveViewer(t, viewerOpts{store: testStore(t, filepath.Join(dir, "store")), uploadsDir: filepath.Join(dir, "uploads"), githubAPI: gh.URL, githubRaw: gh.RawURL()})
	must := func(out string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
	}
	must(runMounts(t, "add", "open", "github://acme/open", "--remote", url))
	must(runDesigns(t, url, "create", "hb"))
	must(runDesigns(t, url, "link", "hb", "netlist", "open/whole/hier_blocks.edn@main"))
	if !storedFlatten(t, url, "hb") {
		t.Errorf("designs/hb keeps flatten_hierarchy = false; want its design.yaml's true")
	}
	if out, err := runDesigns(t, url, "get", "hb"); err != nil || !strings.Contains(out, "reads every hierarchical block") {
		t.Errorf("designs get hb = %q, %v; want it to say the design reads every block", out, err)
	}
	if got := componentCount(t, "designs/hb", "--remote", url); got != whole {
		t.Errorf("designs/hb reads %d components; want %d, as its folder reads flattened", got, whole)
	}
	must(runDesigns(t, url, "link", "hb", "netlist", "open/top/hier_blocks.edn@main"))
	if storedFlatten(t, url, "hb") {
		t.Errorf("a link to a design.yaml setting flatten_hierarchy: false left it on; want it off")
	}
	if got := componentCount(t, "designs/hb", "--remote", url); got != top {
		t.Errorf("designs/hb reads %d components once flattening is off; want %d, its top cell", got, top)
	}
}

// runAgniStderr runs the whole CLI with args and returns what it printed to stdout and to stderr.
func runAgniStderr(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := rootCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.Background())
	return out.String(), errOut.String(), err
}

// A remote check or review of a design read at its top cell alone prints the note a local run
// prints, since the server's result lists the blocks the read left out. A design read whole prints
// none (agni issue 1887).
func TestARemoteRunNotesTheBlocksItsReadLeftOut(t *testing.T) {
	lab := t.TempDir()
	hierFolder(t, lab, hierDescriptor(""))
	srv := designServer(t, t.TempDir(), t.TempDir(), mountset.Folder("lab", lab))
	if out, err := runDesigns(t, srv.URL, "push", filepath.Join(lab, "hw"), "--id", "hb"); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	runs := map[string][]string{
		"check":  {"check", "designs/hb", "--remote", srv.URL, "--format", "json"},
		"review": {"review", "designs/hb", "--checklist", filepath.Join("..", "..", "examples", "design-review", "checklist.yaml"), "--remote", srv.URL, "--format", "json"},
	}
	for name, args := range runs {
		t.Run(name, func(t *testing.T) {
			_, top, err := runAgniStderr(t, args...)
			if err != nil {
				t.Fatalf("%v\n%s", err, top)
			}
			if !strings.Contains(top, "note: ") || !strings.Contains(top, "--flatten-hierarchy") {
				t.Errorf("a remote %s of a design read at its top cell wrote %q to stderr; want the note naming --flatten-hierarchy", name, top)
			}
			_, whole, err := runAgniStderr(t, append(args, "--flatten-hierarchy")...)
			if err != nil {
				t.Fatalf("%v\n%s", err, whole)
			}
			if strings.Contains(whole, "--flatten-hierarchy") {
				t.Errorf("a remote %s of a design read whole wrote %q to stderr; want no note", name, whole)
			}
		})
	}
}
