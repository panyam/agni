package report

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
)

var anchor = regexp.MustCompile(`<a [^>]*>`)

// Every link in both HTML reports opens in the viewer's named window, and each page carries the
// script that moves an open viewer there once, so a click reuses one viewer rather than reloading it
// (agni issue 1156).
func TestReportLinksOpenTheNamedViewer(t *testing.T) {
	var verdictPage, checklistPage bytes.Buffer
	r := Build([]check.Verdict{verdict("converted", "N", check.Pass)}, nil, rules(), Report{URLBase: "http://h:1", MountPath: "m/d"})
	if err := HTML(&verdictPage, r); err != nil {
		t.Fatal(err)
	}
	c := Checklist{Design: "d", Name: "n", URLBase: "http://h:1", MountPath: "m/d", Areas: []ChecklistArea{{
		Name:  "a",
		Items: []ChecklistItem{{ID: "1", Title: "t", Outcome: "fail", Evidence: []ChecklistEvidence{{Rule: "r", Subject: "N", URL: "http://h:1/designs/m/d/view?verdict=x"}}}},
	}}}
	if err := ChecklistHTML(&checklistPage, c); err != nil {
		t.Fatal(err)
	}
	for name, page := range map[string]string{"verdict report": verdictPage.String(), "checklist": checklistPage.String()} {
		var links int
		for _, a := range anchor.FindAllString(page, -1) {
			if !strings.Contains(a, "href=") {
				continue
			}
			links++
			if !strings.Contains(a, `target="`+ViewerTarget+`"`) {
				t.Errorf("%s: link %s does not open in %s", name, a, ViewerTarget)
			}
		}
		if links == 0 {
			t.Errorf("%s: rendered no link, so nothing above was checked", name)
		}
		if n := strings.Count(page, "<script>"); n != 1 {
			t.Errorf("%s: carries %d scripts, want the one that moves an open viewer", name, n)
		}
		if !strings.Contains(page, `method: "ApplyLocation"`) {
			t.Errorf("%s: the script does not ask the viewer to apply the link", name)
		}
	}
}

// The script asks only the viewer's window. A report is opened from disk and from mail, so it makes
// no request of its own and names no host.
func TestViewerScriptMakesNoRequest(t *testing.T) {
	for _, banned := range []string{"fetch(", "XMLHttpRequest", "sendBeacon", "http://", "https://", "import("} {
		if strings.Contains(viewerLinkJS, banned) {
			t.Errorf("viewerlink.js contains %q", banned)
		}
	}
}
