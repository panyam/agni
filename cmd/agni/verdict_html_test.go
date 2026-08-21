package main

import (
	"strings"
	"testing"
)

// THE BOUNDARY that keeps a link a promise rather than a guess (agni issue 392).
//
// A bare file path is minted a mount name locally, from the enclosing project or as "local", and that
// name means nothing on a server the operator did not start with it. --url-base says where the viewer
// is; it does not say that this design is on it. Both are needed, and neither implies the other.
func TestHTMLReportOnlyLinksAMountTheOperatorNamed(t *testing.T) {
	loose := runCheck(t, "--verdicts", "--format", "html", "--url-base", "http://localhost:8080",
		"testdata/conformance/showcase.fires.kicad_sch")
	if strings.Contains(loose, "<a href") {
		t.Error("a locally-minted mount must not produce links; they resolve on nobody's server")
	}
	// The report is still a report: it just names subjects as plain text.
	if !strings.Contains(loose, "agni check") || !strings.Contains(loose, "i2c-pull-up") {
		t.Error("dropping the links must not drop the report")
	}
}

// Without --url-base there is nothing to link to, and the report says so by saying nothing.
func TestHTMLReportHasNoLinksWithoutABase(t *testing.T) {
	out := runCheck(t, "--verdicts", "--format", "html", "testdata/conformance/showcase.fires.kicad_sch")
	if strings.Contains(out, "<a href") {
		t.Error("no base means no links")
	}
}

// html is the verdict report and has no findings-only form, so asking for it IS asking for the
// considered set. It turns --verdicts on rather than refusing: a flag with no alternative is not a
// choice the reader should have to make twice.
func TestHTMLImpliesVerdicts(t *testing.T) {
	bare := runCheck(t, "--format", "html", "testdata/conformance/showcase.fires.kicad_sch")
	explicit := runCheck(t, "--verdicts", "--format", "html", "testdata/conformance/showcase.fires.kicad_sch")
	// Same document either way. Compare a pass row rather than the whole page, whose header carries a
	// timestamp that differs between two runs.
	if !strings.Contains(bare, "<td class=\"o pass\">") {
		t.Error("html without --verdicts must render the considered set, not a findings-only table")
	}
	if strings.Count(bare, "<details") != strings.Count(explicit, "<details") {
		t.Errorf("html with and without --verdicts must render the same rules: %d vs %d",
			strings.Count(bare, "<details"), strings.Count(explicit, "<details"))
	}
}

// The page must stand on its own: emailed, committed, opened from a file:// path, read with scripts
// off. Collapsing is <details>, so a reader with no JavaScript still sees everything.
func TestHTMLReportIsSelfContainedAndNeedsNoScript(t *testing.T) {
	out := runCheck(t, "--verdicts", "--format", "html", "testdata/conformance/showcase.fires.kicad_sch")
	if strings.Contains(out, "<script") {
		t.Error("the report must not depend on scripts to be readable")
	}
	if !strings.Contains(out, "<details") {
		t.Error("collapsing should be <details>, which works without scripts")
	}
	for _, external := range []string{"http://cdn", "https://cdn", "<link rel=\"stylesheet\""} {
		if strings.Contains(out, external) {
			t.Errorf("the report must be self-contained, found %q", external)
		}
	}
}
