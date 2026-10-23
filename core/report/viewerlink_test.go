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

// A report with nothing to link to carries no script at all, so one written with no viewer stays
// exactly the plain document it was before (agni issue 1156).
func TestUnlinkedReportsCarryNoScript(t *testing.T) {
	var verdictPage, checklistPage bytes.Buffer
	r := Build([]check.Verdict{verdict("converted", "NET_ZQ7", check.Pass)}, nil, rules(), Report{})
	if err := HTML(&verdictPage, r); err != nil {
		t.Fatal(err)
	}
	label := r.Rules[0].Rows[0].SubjectLabel()
	c := Checklist{Design: "d", Name: "n", Areas: []ChecklistArea{{Name: "a", Items: []ChecklistItem{{ID: "1", Title: "t", Outcome: "fail", Evidence: []ChecklistEvidence{{Rule: "r", Subject: "NET_ZQ7"}}}}}}}
	if err := ChecklistHTML(&checklistPage, c); err != nil {
		t.Fatal(err)
	}
	for name, page := range map[string]string{"verdict report": verdictPage.String(), "checklist": checklistPage.String()} {
		if !strings.Contains(page, label) {
			t.Errorf("%s: the row is missing, so the absence below proves nothing", name)
		}
		if strings.Contains(page, "<script") {
			t.Errorf("%s: carries a script with no link for it to act on", name)
		}
	}
}

// A linked report never depends on its script: with the script removed, as a reader with scripts off
// or a mail filter would have it, every row, link and section is still there.
func TestLinkedReportReadsTheSameWithoutItsScript(t *testing.T) {
	vs := []check.Verdict{verdict("converted", "NET_ZQ7", check.Pass), verdict("converted", "NET_ZQ8", check.Fail)}
	var page bytes.Buffer
	if err := HTML(&page, Build(vs, nil, rules(), Report{URLBase: "http://h:1", MountPath: "m/d"})); err != nil {
		t.Fatal(err)
	}
	full := page.String()
	script := "<script>" + viewerLinkJS + "</script>"
	if strings.Count(full, script) != 1 {
		t.Fatalf("the linked report does not carry the script verbatim once")
	}
	stripped := strings.Replace(full, script, "", 1)
	if strings.Contains(stripped, "<script") {
		t.Error("removing the viewer script left another script behind")
	}
	if a, b := strings.Count(full, "<details"), strings.Count(stripped, "<details"); a == 0 || a != b {
		t.Errorf("sections: %d with the script, %d without", a, b)
	}
	if a, b := strings.Count(full, `target="`+ViewerTarget+`"`), strings.Count(stripped, `target="`+ViewerTarget+`"`); a != 2 || a != b {
		t.Errorf("viewer links: %d with the script, %d without, want 2 in both", a, b)
	}
	for _, subject := range []string{">NET_ZQ7<", ">NET_ZQ8<"} {
		if !strings.Contains(stripped, subject) {
			t.Errorf("row %s is missing without the script", subject)
		}
	}
}
