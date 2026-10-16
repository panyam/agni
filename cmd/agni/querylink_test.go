package main

import (
	"bytes"
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rpt "github.com/panyam/agni/core/report"
)

// runQueryBoth runs `agni query` and returns its stdout and stderr, where a link is printed.
func runQueryBoth(t *testing.T, args ...string) (string, string) {
	t.Helper()
	cmd := queryCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("query %v: %v\n%s", args, err, errOut.String())
	}
	return out.String(), errOut.String()
}

// linkIn returns the one "look at ..." link a run printed, parsed.
func linkIn(t *testing.T, stderr, prefix string) *url.URL {
	t.Helper()
	for _, line := range strings.Split(stderr, "\n") {
		if rest, ok := strings.CutPrefix(line, prefix); ok {
			u, err := url.Parse(strings.TrimSpace(rest))
			if err != nil {
				t.Fatal(err)
			}
			return u
		}
	}
	t.Fatalf("no %q line in:\n%s", prefix, stderr)
	return nil
}

// A query answer opens in the viewer (agni issue 634): the link names the design's view and carries
// the question, its bindings and its title, so following it asks exactly what the run asked.
func TestAQueryLinksBackIntoTheViewer(t *testing.T) {
	withMount(t)
	q := `component.net(?r, ?n), ?n = ?net => ?r`
	_, stderr := runQueryBoth(t, "mount://demo/showcase.fires.kicad_sch", q, "--bind", "net=GND", "--title", "On ground", "--server", urlBaseFlag)
	u := linkIn(t, stderr, "look at it: ")
	if u.Path != "/designs/demo/showcase.fires.kicad_sch/view" {
		t.Errorf("link path %q, want the design's view", u.Path)
	}
	v := u.Query()
	if v.Get("query") != q || v.Get("title") != "On ground" || strings.Join(v["bind"], ",") != "net=GND" {
		t.Errorf("link asks %q titled %q bound %v, want the run's query, title and binding", v.Get("query"), v.Get("title"), v["bind"])
	}
	if v.Get("hash") != "" {
		t.Errorf("a query link carries the question and no content hash, got %q", v.Get("hash"))
	}
}

// Without --server a query prints no link and asks no server anything.
func TestAQueryWithoutAServerPrintsNoLink(t *testing.T) {
	withMount(t)
	_, stderr := runQueryBoth(t, "mount://demo/showcase.fires.kicad_sch", "component.net(?r, ?n) => ?r")
	if strings.Contains(stderr, "look at") {
		t.Errorf("a link with no --server:\n%s", stderr)
	}
}

// A set read from a file is not on any server, so each query's link carries its text as the set
// asks it, preamble in front, with the set's bindings spelled as --bind takes them.
func TestASetFromAFileLinksEachQueryByItsText(t *testing.T) {
	withMount(t)
	set := filepath.Join(t.TempDir(), "s.yaml")
	body := "preamble: |\n  on(?r) :- component.net(?r, ?_)\nqueries:\n  - name: parts\n    query: on(?r) => ?r\n  - name: three\n    query: net.pin_count(?n, ?c), ?c >= ?min, ?n != ?t => ?n\n    bind: {min: 3, t: \"3\"}\n"
	if err := os.WriteFile(set, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, stderr := runQueryBoth(t, "mount://demo/showcase.fires.kicad_sch", "--set", set, "--server", urlBaseFlag)
	parts := linkIn(t, stderr, "look at parts: ").Query()
	if parts.Get("query") != "on(?r) :- component.net(?r, ?_)\n;\non(?r) => ?r" {
		t.Errorf("parts link asks %q, want the preamble in front of the query", parts.Get("query"))
	}
	three := linkIn(t, stderr, "look at three: ").Query()
	if strings.Join(three["bind"], ",") != `min=3,t="3"` || three.Get("title") != "three" {
		t.Errorf("three link binds %v titled %q, want min=3 and t=\"3\" (text that reads as a number)", three["bind"], three.Get("title"))
	}
}

// A set the project keeps is linked by its name and the query's, so the link stays short however
// long the query is and follows the set's current text.
func TestAProjectSetLinksEachQueryByName(t *testing.T) {
	freshWorkspace(t)
	cliMountSpecs = []string{"tut=" + filepath.Join("..", "..", "examples", "tutorial-project")}
	_, stderr := runQueryBoth(t, "mount://tut/designs/gateway", "--set", "power-review", "--server", urlBaseFlag)
	n := strings.Count(stderr, "look at ")
	if n < 2 {
		t.Fatalf("%d links for the power-review set, want one per query:\n%s", n, stderr)
	}
	for _, line := range strings.Split(stderr, "\n") {
		if !strings.HasPrefix(line, "look at ") {
			continue
		}
		u, err := url.Parse(line[strings.Index(line, "http"):])
		if err != nil {
			t.Fatal(err)
		}
		v := u.Query()
		if v.Get("set") != "power-review" || v.Get("q") == "" || v.Get("query") != "" {
			t.Errorf("link %s: want set=power-review, a query name, and no query text", u)
		}
	}
}

// Under --server self the link's base is the one this process serves, so both commands that ask a
// question mint one. trace guarded its link on the url a remote server has, which self leaves empty,
// so `trace --server self` served with no link to follow.
func TestSelfMintsTheLinkItServes(t *testing.T) {
	withMount(t)
	spec, err := parseServerSpec("self")
	if err != nil {
		t.Fatal(err)
	}
	defer spec.Close()
	cmd := queryCmd()
	cmd.SetContext(context.Background())
	cmd.SetErr(&bytes.Buffer{})
	const design = "mount://demo/showcase.fires.kicad_sch"
	q := queryLinker(cmd, spec, design)(func(meta rpt.Report) string { return rpt.QueryURL(meta, "component.net(?r, ?n) => ?r", "", nil) })
	tr := traceLink(cmd, spec, design, "J1.1", "U1.1", 6)
	for name, u := range map[string]string{"query": q, "trace": tr} {
		if !strings.HasPrefix(u, spec.base()+"/designs/demo/showcase.fires.kicad_sch/view?") {
			t.Errorf("%s under --server self minted %q, want a link on %s", name, u, spec.base())
		}
	}
}
