package main

import (
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
