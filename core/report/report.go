// Package report aggregates a check run into the shape a person reads: what was checked, what it
// rests on, and what to do about the parts that failed.
//
// It lives in core/ rather than cmd/ deliberately. Issue 380 records what happened the last time a
// check renderer landed in cmd/: the example module could not reach it, wrote its own, and the two
// drifted. A report is a projection over the run, so it belongs where a second consumer can import
// it, and it does no I/O of its own (CONSTRAINTS C1).
//
// The aggregation here is the judgment; the HTML in html.go is a rendering of it. That split is what
// keeps the layout free to change without re-deciding what a reader should see first, and it is what
// would make a caller-supplied template cheap later without freezing a template context now.
package report

import (
	"net/url"
	"sort"
	"strings"

	"github.com/panyam/agni/core/check"
)

// Report is one check run, aggregated.
//
// Generated and Design are supplied by the caller rather than read here, because this package does no
// I/O and does not read the clock: a report built twice from the same run must be identical, which is
// what lets a committed report be diffed across board revisions.
type Report struct {
	Design      string // the design this run read, as the caller names it
	Generated   string // caller-supplied timestamp; empty is fine
	ContentHash string // the bytes this run saw, so a stale link can be detected (issue 392)
	URLBase     string // e.g. "http://localhost:8080"; empty means emit no links at all
	MountPath   string // e.g. "demo/board.kicad_sch"; empty means emit no links at all
	Totals      Totals
	Rules       []RuleReport
}

// Totals are the run's headline numbers.
//
// Considered and Findings are counted separately and neither is derivable from the other, which is
// the point: a rule that states a considered set contributes to both, and a rule that reports only
// failures contributes to Findings alone. Presenting one number would hide which kind of coverage the
// run actually has.
type Totals struct {
	RulesReporting    int // rules that stated a considered set
	RulesFindingsOnly int // rules that reported violations without stating what they looked at
	Considered        int // verdicts from rules that state a considered set
	Pass              int
	Fail              int
	NotConsidered     int
	NoLimit           int
	Inconclusive      int
	Findings          int // every violation, including from rules that state no considered set
}

// RuleReport is one rule's section.
type RuleReport struct {
	Name     string
	Severity string
	Summary  string
	Impact   string
	Remedy   string
	// StatesConsideredSet is carried into the report because a reader must be able to tell "this rule
	// examined 41 rails and cleared 39" from "this rule reported two problems and said nothing about
	// what else it looked at". Collapsing them is the false-coverage claim the verdict work removes,
	// and a report is exactly where it would be most convincing and most wrong.
	StatesConsideredSet bool
	Counts              map[check.Outcome]int
	Rows                []Row
}

// Failed reports whether this rule has anything a reader must act on.
func (r RuleReport) Failed() bool {
	return r.Counts[check.Fail] > 0 || r.Counts[check.Inconclusive] > 0
}

// Row is one subject's outcome under one rule.
type Row struct {
	ID      string
	Outcome check.Outcome
	// Subjects is what the row is about, in the rule's order: one entity for most rules, the whole
	// tuple for a rule whose question is a relation. A reader of a copper-clearance row wants both
	// nets, and a row that named one of them would be asking them to guess the other.
	Subjects []check.Entity
	Message  string // the violation sentence, set on a failure
	Witness  string // what the outcome rests on
	Reason   string // why an undecidable subject could not be judged
	Terms    []Term
	Context  []string // other entities the proof names
	URL      string   // empty when the caller supplied no base
}

// Term is one labelled fact behind a row's witness.
type Term struct{ Label, Value string }

// Build aggregates a run into a Report.
//
// verdicts come from check.RunVerdicts (rules that state a considered set) and findings from
// check.Run (every rule). A rule appears in the report if it produced either, so a findings-only rule
// is present and visibly labelled rather than silently absent.
//
// ORDERING IS THE MAIN JUDGMENT HERE. Rules with something to act on come first, then the rest
// alphabetically; within a rule, failures and undecidables come before passes. A reader opening a
// report on a board with three problems and two thousand passes should meet the three problems.
func Build(verdicts []check.Verdict, findings []check.Finding, rules []*check.Rule, meta Report) Report {
	byName := map[string]*check.Rule{}
	for _, r := range rules {
		byName[r.Name] = r
	}

	sections := map[string]*RuleReport{}
	section := func(name string) *RuleReport {
		s, ok := sections[name]
		if ok {
			return s
		}
		s = &RuleReport{Name: name, Counts: map[check.Outcome]int{}}
		if r := byName[name]; r != nil {
			s.Severity, s.Summary, s.Impact, s.Remedy = r.Severity, r.Summary, r.Impact, r.Remedy
			s.StatesConsideredSet = r.StatesConsideredSet
		}
		sections[name] = s
		return s
	}

	out := meta
	for _, v := range verdicts {
		s := section(v.Rule)
		s.Counts[v.Outcome]++
		s.Rows = append(s.Rows, rowOf(v, out))
		out.Totals.Considered++
		switch v.Outcome {
		case check.Pass:
			out.Totals.Pass++
		case check.Fail:
			out.Totals.Fail++
		case check.NotConsidered:
			out.Totals.NotConsidered++
		case check.NoLimit:
			out.Totals.NoLimit++
		case check.Inconclusive:
			out.Totals.Inconclusive++
		}
	}

	// Findings from rules that state no considered set have no verdict to carry them, so they are
	// added here. A rule that DOES state one already contributed its failures above; adding them
	// again would double-count, which is why this is keyed on the rule rather than on the finding.
	for _, f := range findings {
		out.Totals.Findings++
		if r := byName[f.Rule]; r != nil && r.StatesConsideredSet {
			continue
		}
		s := section(f.Rule)
		s.Counts[check.Fail]++
		s.Rows = append(s.Rows, Row{
			Outcome:  check.Fail,
			Subjects: []check.Entity{f.Subject},
			Message:  f.Message,
		})
	}

	for _, s := range sections {
		sortRows(s.Rows)
		if s.StatesConsideredSet {
			out.Totals.RulesReporting++
		} else {
			out.Totals.RulesFindingsOnly++
		}
		out.Rules = append(out.Rules, *s)
	}
	sort.SliceStable(out.Rules, func(i, j int) bool {
		if a, b := out.Rules[i].Failed(), out.Rules[j].Failed(); a != b {
			return a // anything to act on comes first
		}
		return out.Rules[i].Name < out.Rules[j].Name
	})
	return out
}

// outcomeRank orders the rows inside a rule: what a reader must act on, then what could not be
// decided, then the evidence that everything else is fine.
func outcomeRank(o check.Outcome) int {
	switch o {
	case check.Fail:
		return 0
	case check.Inconclusive:
		return 1
	case check.NoLimit:
		return 2
	case check.NotConsidered:
		return 3
	}
	return 4 // Pass
}

func sortRows(rows []Row) {
	sort.SliceStable(rows, func(i, j int) bool {
		ri, rj := outcomeRank(rows[i].Outcome), outcomeRank(rows[j].Outcome)
		if ri != rj {
			return ri < rj
		}
		return rowRef(rows[i]) < rowRef(rows[j])
	})
}

// rowRef is a row's subject tuple as one string, for ordering only. Rows sort by the tuple in the
// rule's own order rather than by a canonicalised one, so two rows of the same rule sort the way the
// rule reads them.
func rowRef(r Row) string {
	parts := make([]string, 0, len(r.Subjects))
	for _, e := range r.Subjects {
		parts = append(parts, check.EntityRef(e))
	}
	return strings.Join(parts, ",")
}

func rowOf(v check.Verdict, meta Report) Row {
	r := Row{
		ID:       check.VerdictID(v),
		Outcome:  v.Outcome,
		Subjects: v.Subjects,
		Reason:   v.Reason,
	}
	if v.Witness != nil {
		r.Witness = v.Witness.Statement
		for _, t := range v.Witness.Terms {
			r.Terms = append(r.Terms, Term{Label: t.Label, Value: t.Value})
		}
	}
	if v.Finding != nil {
		r.Message = v.Finding.Message
	}
	for _, c := range v.Context {
		r.Context = append(r.Context, check.EntityRef(c.Entity))
	}
	r.URL = verdictURL(meta, r.ID)
	return r
}

// verdictURL builds the link that opens this verdict's proof in a running viewer, or "" when the
// caller gave no base.
//
// A MISSING LINK IS THE CORRECT ANSWER for a loose file, not a gap to fill with a guess (issue 392).
// A URL is a promise the reader can follow, and one assembled from an invented mount resolves on
// nobody's server, which reads as a broken tool rather than a mismatched setup.
//
// The content hash rides along so the viewer can say "this link was computed against different bytes"
// instead of silently highlighting whatever now sits at that subject. A link that quietly points at
// the wrong pin is the same false-confidence failure this whole layer exists to remove, relocated
// into the browser.
func verdictURL(meta Report, id string) string {
	if meta.URLBase == "" || meta.MountPath == "" {
		return ""
	}
	u := meta.URLBase + "/designs/" + meta.MountPath + "/view?verdict=" + url.QueryEscape(id)
	if meta.ContentHash != "" {
		u += "&hash=" + url.QueryEscape(meta.ContentHash)
	}
	return u
}
