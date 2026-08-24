package report

import (
	"embed"
	"html/template"
	"io"

	"github.com/panyam/agni/core/check"
)

//go:embed report.html.tmpl
var tmplFS embed.FS

// HTML writes the report as one self-contained page.
//
// html/template rather than string building, and the reason is correctness rather than tidiness.
// Every subject, message and witness in this document came out of a design file the engine did not
// author: a net named with an angle bracket, or a message carrying a quote, would corrupt the page or
// inject into it. html/template escapes per context (text, attribute, URL) and knows the difference;
// concatenation does not.
//
// COLLAPSING IS <details>, NOT SCRIPT. The report has to survive being emailed, committed, opened
// from a file:// path and read with scripts disabled, and a reader who cannot expand a section
// because JavaScript did not run has a report that lies by omission. The reference tool this format
// borrows from hand-rolls a script and five buttons for the same effect.
//
// Self-contained on purpose: one file, no external CSS or fonts, so it still renders after being
// moved somewhere with no network.
func HTML(w io.Writer, r Report) error {
	t, err := template.New("report.html.tmpl").Funcs(funcs()).ParseFS(tmplFS, "report.html.tmpl")
	if err != nil {
		return err
	}
	return t.Execute(w, r)
}

func funcs() template.FuncMap {
	return template.FuncMap{
		// outcomeClass maps an outcome to its CSS class. Kept as a function rather than a field on Row
		// so the vocabulary lives in one place if a sixth outcome ever lands.
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
		// outcomeLabel is what a reader sees. "not considered" and "no limit" are spelled out because
		// the distinction between them is the substance: one was never judged, the other reached the
		// comparison and found nothing stated to compare against.
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
		"count":     func(m map[check.Outcome]int, o check.Outcome) int { return m[o] },
		"pass":      func() check.Outcome { return check.Pass },
		"fail":      func() check.Outcome { return check.Fail },
		"subjectOf": func(row Row) string { return row.SubjectLabel() },
	}
}
