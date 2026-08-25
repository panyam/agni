package main

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

const urlBaseFlag = "http://localhost:8080"

// withMount declares a mount for one test.
//
// Two things have to be undone, and the second is the one that bites. --mount is a root PERSISTENT
// flag, so the variable is set directly rather than passed to a check command built on its own. And
// the mount TABLE is parsed once per process behind a sync.Once, which is right for a CLI serving one
// invocation and wrong inside a test binary where every test shares the process: without the reset
// these tests pass alone and fail in the package run, because whichever test resolves a mount first
// fixes the table for all of them.
func withMount(t *testing.T) {
	t.Helper()
	prevSpecs, prevVal, prevErr := cliMountSpecs, cliWSVal, cliWSErr
	cliMountSpecs = []string{"demo=testdata/conformance"}
	cliWSOnce, cliWSVal, cliWSErr = sync.Once{}, nil, nil
	t.Cleanup(func() {
		cliMountSpecs, cliWSVal, cliWSErr = prevSpecs, prevVal, prevErr
		cliWSOnce = sync.Once{}
	})
}

// mountedShowcase runs the showcase board through a named mount, which is what makes a link possible:
// the viewer's path is /designs/<mount>/<path>/view, and a CLI reading a loose file has no mount to
// put there.
func mountedShowcase(t *testing.T, args ...string) string {
	t.Helper()
	withMount(t)
	return runCheck(t, append([]string{
		"--verdicts", "--url-base", urlBaseFlag, "mount://demo/showcase.fires.kicad_sch",
	}, args...)...)
}

// A row for a registered design carries a URL that opens that verdict's proof (agni issue 392,
// acceptance 1). The id is already in the row; what was missing is the CLI knowing what to put in
// front of it.
func TestVerdictRowsCarryAProofURL(t *testing.T) {
	for _, tc := range []struct{ name, out string }{
		{"text", mountedShowcase(t)},
		{"csv", mountedShowcase(t, "--format", "csv")},
	} {
		if !strings.Contains(tc.out, urlBaseFlag+"/designs/demo/showcase.fires.kicad_sch/view?verdict=") {
			t.Errorf("%s: no verdict link in:\n%s", tc.name, tc.out)
		}
		// The hash rides along so a stale link can be told apart from a live one, which is the whole
		// reason the URL is worth more than the id.
		if !strings.Contains(tc.out, "hash=sha256") {
			t.Errorf("%s: the link carries no content hash, so nothing downstream can detect a stale one", tc.name)
		}
	}
}

// A row for a loose file carries NO URL rather than a broken one (acceptance 2). A bare path is
// minted a mount locally, and that name means nothing on a server the operator did not start with it,
// so a link built from it resolves nowhere. Silence is the correct answer.
func TestALooseFileCarriesNoURL(t *testing.T) {
	text := runCheck(t, "--verdicts", "--url-base", urlBaseFlag, "testdata/conformance/showcase.fires.kicad_sch")
	if strings.Contains(text, urlBaseFlag) {
		t.Errorf("a loose file has no mount the server would recognise, so it must emit no link:\n%s", text)
	}
	csv := runCheck(t, "--verdicts", "--format", "csv", "--url-base", urlBaseFlag,
		"testdata/conformance/showcase.fires.kicad_sch")
	if strings.Contains(csv, urlBaseFlag) {
		t.Errorf("same for the csv, whose url column stays empty:\n%s", csv)
	}
	// The COLUMN still exists, so a consumer binding to the header does not have to know whether the
	// run happened to be linkable.
	if !strings.HasPrefix(csv, strings.Join(verdictCSVColumns, ",")) {
		t.Error("the url column must be present even when every cell is empty")
	}
}

// Without --url-base there is no link anywhere, in any format. The operator names the viewer or
// nothing is guessed on their behalf.
func TestNoURLBaseMeansNoLinks(t *testing.T) {
	withMount(t)
	out := runCheck(t, "--verdicts", "mount://demo/showcase.fires.kicad_sch")
	if strings.Contains(out, "http://") {
		t.Errorf("a link was assembled with no --url-base to build it from:\n%s", out)
	}
}

// The three renderers compose one link, so a row in a terminal, a cell in a spreadsheet and an anchor
// on the page all address the same verdict. They read it from different places (Row.URL for two of
// them, a per-row call for the csv, which emits in the run's order rather than the report's), which is
// exactly the split that lets two spellings drift apart.
func TestEveryFormatComposesTheSameLink(t *testing.T) {
	text, csv, html := mountedShowcase(t), mountedShowcase(t, "--format", "csv"), mountedShowcase(t, "--format", "html")
	const want = urlBaseFlag + "/designs/demo/showcase.fires.kicad_sch/view?verdict=esd-protection%3A%28net%3AUSB_D%2B%29"
	for name, out := range map[string]string{"text": text, "csv": csv, "html": html} {
		if !strings.Contains(out, want) {
			t.Errorf("%s does not carry the shared link form %q", name, want)
		}
	}
}

// The point of agni issue 459: a link depends on whether the mount is real, not on how the design was
// spelled. Both forms address the same file through the same declared mount, so both are equally
// followable and both must be emitted.
func TestAPlainPathThroughADeclaredMountIsLinkable(t *testing.T) {
	withMount(t)
	plain := runCheck(t, "--verdicts", "--format", "csv", "--url-base", urlBaseFlag,
		"testdata/conformance/showcase.fires.kicad_sch")
	if !strings.Contains(plain, urlBaseFlag+"/designs/demo/showcase.fires.kicad_sch/view?verdict=") {
		t.Fatalf("a plain path under a declared mount must be linkable:\n%s", plain)
	}

	spelled := runCheck(t, "--verdicts", "--format", "csv", "--url-base", urlBaseFlag,
		"mount://demo/showcase.fires.kicad_sch")
	if plain != spelled {
		t.Errorf("the two spellings address one design and must produce identical output.\nplain:\n%s\nspelled:\n%s", plain, spelled)
	}
}

// The refusal is what the widening must not cost. A mount this run MINTED still gets no link, because
// its name means nothing on a server the operator did not start with it.
func TestAMintedMountIsStillNotLinkable(t *testing.T) {
	// No withMount, so nothing is declared and the argument is minted a mount of its own.
	csv := runCheck(t, "--verdicts", "--format", "csv", "--url-base", urlBaseFlag,
		"testdata/conformance/showcase.fires.kicad_sch")
	if strings.Contains(csv, urlBaseFlag) {
		t.Errorf("a minted mount resolves on nobody's server, so it must emit no link:\n%s", csv)
	}
	// Spelling it mount:// does not rescue it. An undeclared mount name is refused at URI resolution,
	// well before a link could be composed from it, so the two routes to a link agree that DECLARED is
	// the requirement.
	cmd := checkCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--verdicts", "--url-base", urlBaseFlag, "mount://demo/showcase.fires.kicad_sch"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("a mount:// URI naming an undeclared mount must be refused")
	}
	if !strings.Contains(err.Error(), "not declared") {
		t.Errorf("the refusal should say the mount was not declared, got %v", err)
	}
}

// Declared separates the two tables the workspace keeps. inDeclared would answer yes to a minted
// mount as well, since it searches Mounts(), which is why the predicate reads the field directly.
func TestWorkspaceDeclaredSeparatesNamedFromMinted(t *testing.T) {
	withMount(t)
	ws, err := workspace()
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	if !ws.Declared("demo") {
		t.Error("demo was passed as --mount and must read as declared")
	}
	if ws.Declared("local") {
		t.Error("local is the name mint invents; it must never read as declared")
	}
	if ws.Declared("never-named") {
		t.Error("an unknown name is not declared")
	}

	// Minting one does not promote it.
	if _, err := ws.URI("testdata/conformance/showcase.passes.kicad_sch"); err != nil {
		t.Fatalf("URI: %v", err)
	}
	for _, m := range ws.Mounts() {
		if m.Name != "demo" && ws.Declared(m.Name) {
			t.Errorf("mount %q was minted and must not read as declared", m.Name)
		}
	}
}

// A nil workspace yields no link rather than a panic. The caller ignores workspace()'s error, because
// a failure there is reported by whichever call needed it for real work, and fail-closed is the same
// answer every other unlinkable case gets.
func TestLinkTargetIsNilSafe(t *testing.T) {
	got, why := linkTarget(nil, "mount://demo/x.edn")
	if got != "" {
		t.Errorf("linkTarget(nil) = %q, want no link", got)
	}
	if why == "" {
		t.Error("linkTarget(nil) gave no reason; refusing to link has to say why (the silence this replaced)")
	}
}

// Every refusal carries a reason. This is the property the note on stderr rests on: if any path can
// return ("", "") the operator gets "no verdict links were emitted: " with nothing after the colon.
func TestLinkTargetAlwaysExplainsARefusal(t *testing.T) {
	ws, err := newCLIWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	for _, uri := range []string{"", "not a uri", "mount://undeclared/x.edn", "mount://demo/"} {
		got, why := linkTarget(ws, uri)
		if got != "" {
			t.Errorf("linkTarget(%q) = %q, want no link", uri, got)
			continue
		}
		if why == "" {
			t.Errorf("linkTarget(%q) refused with no reason", uri)
		}
	}
}
