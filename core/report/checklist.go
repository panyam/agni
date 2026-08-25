package report

import "io"

// Checklist is one review run, aggregated for reading.
//
// It is a SECOND page shape over the same machinery rather than a filter of the check report, and
// the difference is the axis. The check report is rule-major: one section per rule, every subject it
// considered underneath. A checklist is question-major: the team's own items, in the team's own
// order, with the evidence for each. A reviewer walking a checklist is answering "did we ask this",
// and a rule-major document makes them assemble that answer themselves.
//
// The two share Design/Generated/ContentHash/URLBase/MountPath and the stylesheet, so they read as
// one product and a link means the same thing on both.
type Checklist struct {
	Design      string // the design this run read, as the caller names it
	Name        string // the checklist's own name, from the manifest
	Generated   string // caller-supplied timestamp; empty is fine
	ContentHash string // the bytes this run saw
	URLBase     string // empty means emit no links at all
	MountPath   string // empty means emit no links at all
	Summary     string // the one-line tally, rendered by the caller so both surfaces read alike
	Covered     int
	Answered    int
	Total       int
	Areas       []ChecklistArea
}

// ChecklistArea is one group of items, in manifest order. Areas are NOT sorted by how bad they are,
// unlike the check report's rules: a checklist's order is the team's, and rearranging it would mean
// an item that has been third in their process for years moves every time the board changes.
type ChecklistArea struct {
	Name    string
	Summary string
	Items   []ChecklistItem
}

// ChecklistItem is one question and what the run made of it.
type ChecklistItem struct {
	ID       string // the team's own identifier, preserved from the manifest
	Title    string
	Outcome  string // review.Outcome as a string; the stylesheet keys off it
	Note     string // why it did not apply, or who owns it when nothing automated can answer
	Evidence []ChecklistEvidence
}

// ChecklistEvidence is one finding behind an item, with the link to its proof.
type ChecklistEvidence struct {
	Rule    string
	Subject string
	Message string
	URL     string // empty when no link could be promised
}

// Failed reports whether this item is worth opening on arrival. Everything that is not a plain pass
// qualifies, because the states between pass and fail are the ones a reader most needs to see: an
// item nobody answered is the whole reason the review layer exists.
func (i ChecklistItem) Failed() bool { return i.Outcome != "pass" }

// ChecklistHTML writes the checklist as one self-contained page.
//
// html/template for the reason HTML gives: every title, subject and message came out of a design file
// the engine did not author, and per-context escaping is the only thing that makes that safe.
func ChecklistHTML(w io.Writer, c Checklist) error {
	t, err := parse("checklist.html.tmpl")
	if err != nil {
		return err
	}
	return t.Execute(w, c)
}
