package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/panyam/goapplib/storage/mounts/github/githubtest"
)

// v1Sheet is a KiCad schematic with one part, U1, and v2Sheet the same sheet with U2 added.
const v1Sheet = `(kicad_sch
	(lib_symbols
		(symbol "test:DUAL"
			(property "Reference" "U")
			(symbol "DUAL_1_1" (pin input line (name "A") (number "1")))
			(symbol "DUAL_2_1" (pin output line (name "Y") (number "2")))))
	(symbol (lib_id "test:DUAL") (unit 1) (uuid "u1a")
		(property "Reference" "U1") (property "Value" "DUAL"))
	(symbol (lib_id "test:DUAL") (unit 2) (uuid "u1b")
		(property "Reference" "U1") (property "Value" "DUAL")))`

var v2Sheet = strings.TrimSuffix(v1Sheet, ")") + `
	(symbol (lib_id "test:DUAL") (unit 1) (uuid "u2a")
		(property "Reference" "U2") (property "Value" "DUAL")))`

// A linked design read at two commits diffs as agni diff does on the files of those commits, its
// revisions are listed, and a report names the commit it ran on (agni issue 1725).
func TestADiffOfTwoRevisionsMatchesADiffOfTheirFiles(t *testing.T) {
	gh := githubtest.New(t)
	repo := gh.AddRepo("acme", "board", false, map[string]string{"hw/bb.kicad_sch": v1Sheet})
	c1 := repo.Head("main")
	c2 := repo.Commit("main", map[string]string{"hw/bb.kicad_sch": v2Sheet}, nil)
	dir := t.TempDir()
	url := serveViewer(t, viewerOpts{storeDir: filepath.Join(dir, "store"), uploadsDir: filepath.Join(dir, "uploads"), githubAPI: gh.URL, githubRaw: gh.RawURL()})
	must := func(out string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return out
	}
	must(runMounts(t, "add", "board", repo.Source(), "--remote", url))
	must(runDesigns(t, url, "create", "bb"))
	must(runDesigns(t, url, "link", "bb", "schematic", "board/hw/bb.kicad_sch@main"))

	for i, sheet := range []string{v1Sheet, v2Sheet} {
		p := filepath.Join(dir, []string{"a", "b"}[i], "bb.kicad_sch")
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(sheet), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	local := must(runAgni(t, "diff", filepath.Join(dir, "a", "bb.kicad_sch"), filepath.Join(dir, "b", "bb.kicad_sch"), "--format", "json"))
	remote := must(runAgni(t, "diff", "designs/bb@"+c1, "designs/bb@"+c2, "--remote", url, "--format", "json"))
	if !strings.Contains(local, `"U2"`) || remote != local {
		t.Errorf("agni diff designs/bb@c1 designs/bb@c2 --remote =\n%s\nwant the same as on the two commits' files:\n%s", remote, local)
	}

	revs := must(runDesigns(t, url, "revisions", "bb"))
	if !strings.Contains(revs, "refs "+c1) || !strings.Contains(revs, "current") || !strings.Contains(revs, "schematic at "+c2) {
		t.Errorf("designs revisions bb =\n%s\nwant the c1 read named by its commit, and the current one at c2", revs)
	}
	if got := must(runDesigns(t, url, "get", "bb@"+c1)); !strings.HasPrefix(got, "designs/bb@") || strings.Contains(got, "artifacts/") {
		t.Errorf("designs get bb@c1 = %q; want the revision alone", got)
	}
	if got := must(runReports(t, url, "create", "bb")); !strings.Contains(got, "schematic at "+c2[:12]) {
		t.Errorf("reports create bb = %q; want it to name the commit it ran on", got)
	}

	// A public repo's tags list with no connection, through GitHub's REST API (agni issue 1788).
	repo.Tag("v1", c1, false)
	repo.Tag("v2", c2, true)
	repo.Tag("v0", c1, false)
	public := must(runDesigns(t, url, "revisions", "bb", "--tags"))
	if !strings.Contains(public, "designs/bb@v2\t") || !strings.Contains(public, "schematic at "+c2) || !strings.Contains(public, "designs/bb@v1\t") || !strings.Contains(public, "designs/bb@v0\t") {
		t.Errorf("designs revisions bb --tags over a public mount with no connection =\n%s\nwant v2, v1 and v0", public)
	}

	// Through a connection, the repo's tags list as revisions, read or not. A tag whose commit the
	// server has read carries that revision, whatever name the read used.
	gh.AddToken(secretToken)
	t.Setenv("AGNI_TEST_GITHUB_TOKEN", secretToken)
	must(runCobra(t, connectionsCmd(), "add", "acme", "--github-token", "env:AGNI_TEST_GITHUB_TOKEN", "--remote", url))
	must(runMounts(t, "add", "tagged", repo.Source(), "--connection", "acme", "--remote", url))
	must(runDesigns(t, url, "create", "bt"))
	must(runDesigns(t, url, "link", "bt", "schematic", "tagged/hw/bb.kicad_sch@main"))
	line := func(out, name string) string {
		for _, l := range strings.Split(out, "\n") {
			if strings.HasPrefix(l, name+"\t") {
				return l
			}
		}
		return ""
	}
	tags := must(runDesigns(t, url, "revisions", "bt", "--tags"))
	if l := line(tags, "designs/bt@v1"); !strings.Contains(l, "not read yet") || !strings.Contains(l, "schematic at "+c1) {
		t.Errorf("designs revisions bt --tags =\n%s\nwant v1 at c1, not read yet", tags)
	}
	if l := line(tags, "designs/bt@v2"); !strings.Contains(l, "current") || !strings.Contains(l, "schematic at "+c2) {
		t.Errorf("designs revisions bt --tags =\n%s\nwant v2 at c2, the commit the design holds", tags)
	}
	must(runDesigns(t, url, "get", "bt@"+c1))
	tags = must(runDesigns(t, url, "revisions", "bt", "--tags"))
	for _, tag := range []string{"designs/bt@v1", "designs/bt@v0"} {
		if l := line(tags, tag); !strings.Contains(l, "; read ") || strings.Contains(l, "not read yet") {
			t.Errorf("after reading bt at c1, designs revisions bt --tags =\n%s\nwant %s read, since it names c1", tags, tag)
		}
	}
}

// A report can run on a revision of a design, and names the revision it read. The server keeps that
// revision while the report exists, even an unnamed one the design has moved on from (agni issue
// 1774).
func TestAReportKeepsTheRevisionItRanOn(t *testing.T) {
	gh := githubtest.New(t)
	repo := gh.AddRepo("acme", "board", false, map[string]string{"hw/bb.kicad_sch": v1Sheet})
	c1 := repo.Head("main")
	c2 := repo.Commit("main", map[string]string{"hw/bb.kicad_sch": v2Sheet}, nil)
	dir := t.TempDir()
	url := serveViewer(t, viewerOpts{storeDir: filepath.Join(dir, "store"), uploadsDir: filepath.Join(dir, "uploads"), githubAPI: gh.URL, githubRaw: gh.RawURL()})
	must := func(out string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return out
	}
	must(runMounts(t, "add", "board", repo.Source(), "--remote", url))
	must(runDesigns(t, url, "create", "bb"))
	// Following main, the design's revision has no name.
	must(runDesigns(t, url, "link", "bb", "schematic", "board/hw/bb.kicad_sch@main"))
	revision := regexp.MustCompile(`; revision ([0-9a-f]{12})`)
	onMain := must(runReports(t, url, "create", "bb", "--id", "on-main"))
	m := revision.FindStringSubmatch(onMain)
	if m == nil || !strings.Contains(onMain, "schematic at "+c2[:12]) {
		t.Fatalf("reports create bb = %q; want it to name c2 and the revision it read", onMain)
	}
	followed := m[1]

	onC1 := must(runReports(t, url, "create", "bb@"+c1, "--id", "on-c1"))
	if m := revision.FindStringSubmatch(onC1); m == nil || m[1] == followed || !strings.Contains(onC1, "schematic at "+c1[:12]) {
		t.Errorf("reports create bb@c1 = %q; want it run on c1's revision, not %s", onC1, followed)
	}

	// Pinning the link at c1 moves the design off the revision the first report ran on.
	must(runDesigns(t, url, "link", "bb", "schematic", "board/hw/bb.kicad_sch@"+c1))
	if revs := must(runDesigns(t, url, "revisions", "bb")); !strings.Contains(revs, "@"+followed) {
		t.Errorf("designs revisions bb after the move =\n%s\nwant %s kept, since the report on-main ran on it", revs, followed)
	}
	if got := must(runReports(t, url, "list", "bb")); !strings.Contains(got, "on-c1") || !strings.Contains(got, "on-main") {
		t.Errorf("reports list bb = %q; want both reports", got)
	}
}
