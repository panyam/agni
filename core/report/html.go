package report

import (
	"embed"
	"html/template"
	"io"

	"github.com/panyam/agni/core/check"
)

// tmplFS holds every page template. The folder is embedded whole, so a new template is picked up by
// adding the file; a list of names here compiled fine and failed at render time when one was missed.
//
//go:embed templates/*.tmpl
var tmplFS embed.FS

// CSS is the stylesheet both report pages share, so a pass is the same green on the check report
// and on the checklist. It is a file rather than a string literal so an editor treats it as CSS.
//
//go:embed style.css
var CSS string

// viewerLinkJS moves an open viewer when a report link is clicked rather than loading it again
// (agni issue 1156). Every report link targets the window it looks for, ViewerTarget.
//
//go:embed viewerlink.js
var viewerLinkJS string

// ViewerTarget is the window name every report link opens in, so clicks reuse one viewer tab, or a
// frame of that name beside the report, rather than opening a tab each.
const ViewerTarget = "agni-viewer"

// HTML writes the report as one self-contained page.
//
// html/template rather than string building, because every subject, message and witness came out of
// a design file the engine did not author, and a net name holding an angle bracket or a quote would
// otherwise corrupt or inject into the page. html/template escapes per context (text, attribute, URL).
//
// COLLAPSING IS <details>, NOT SCRIPT. The report has to survive being emailed, committed, opened
// from a file:// path and read with scripts disabled, and a section that cannot expand hides its rows.
// So it never DEPENDS on a script. A report that links into a viewer carries one script, which only
// moves a viewer already open rather than loading it again (viewerlink.js, agni issue 1156); with
// scripts off its links still open in that viewer's window. A report with no links carries none.
//
// Self-contained, with no external CSS or fonts, so it renders with no network.
func HTML(w io.Writer, r Report) error {
	t, err := parse("report.html.tmpl")
	if err != nil {
		return err
	}
	return t.Execute(w, r)
}

// parse builds one page template by name. Both pages go through it so they share one func map, and
// with it the stylesheet.
func parse(name string) (*template.Template, error) {
	return template.New(name).Funcs(funcs()).ParseFS(tmplFS, "templates/"+name, "templates/querysections.html.tmpl")
}

func funcs() template.FuncMap {
	return template.FuncMap{
		// css injects the shared stylesheet. template.CSS marks it pre-escaped, since it is ours and
		// html/template would otherwise escape the braces.
		"css": func() template.CSS { return template.CSS(CSS) }, // #nosec G203 -- CSS is the embedded stylesheet, not input
		// viewerScript is the embedded script that moves an open viewer, ours and never input.
		"viewerScript": func() template.JS { return template.JS(viewerLinkJS) }, // #nosec G203 -- the embedded viewerlink.js, not input
		"viewerTarget": func() string { return ViewerTarget },
		// setview lays a query set out for the shared querysections template, which the query-set page
		// and the check report's Queries sections both render (agni issue 1190).
		"setview": setView,
		// outcomeClass maps an outcome to its CSS class, keeping the vocabulary in one place.
		"outcomeClass": func(o check.Outcome) string {
			switch o {
			case check.Pass:
				return "pass"
			case check.Fail:
				return "fail"
			case check.Inconclusive:
				return "inconclusive"
			case check.NoLimit:
				return "nolimit"
			case check.NotConsidered:
				return "notconsidered"
			}
			return ""
		},
		// outcomeLabel is what a reader sees. "not considered" was never judged; "no limit" reached
		// the comparison and found nothing stated to compare against.
		"outcomeLabel": func(o check.Outcome) string {
			switch o {
			case check.Pass:
				return "pass"
			case check.Fail:
				return "fail"
			case check.Inconclusive:
				return "could not decide"
			case check.NoLimit:
				return "no limit stated"
			case check.NotConsidered:
				return "not considered"
			}
			return string(o)
		},
		// checkWord is one IO-map rule's outcome inside a pin-map row, where "not run" says the run
		// left that rule out rather than reading as a blank.
		"checkWord": func(o check.Outcome) string {
			if o == "" {
				return "not run"
			}
			return string(o)
		},
		"count":     func(m map[check.Outcome]int, o check.Outcome) int { return m[o] },
		"pass":      func() check.Outcome { return check.Pass },
		"fail":      func() check.Outcome { return check.Fail },
		"subjectOf": func(row Row) string { return row.SubjectLabel() },
	}
}
