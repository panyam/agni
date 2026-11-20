package main

import (
	"os"
	"path/filepath"
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
}
