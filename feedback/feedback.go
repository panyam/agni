// Package feedback is the report a trial of agni sends back from a machine that holds designs we
// never see (agni issue 1057). Its confidentiality guarantee is STRUCTURAL, as intake's is: the
// Report type has no field that can hold a net name, a reference designator, a part number, a design
// or project name, a path, or an error message, so a report cannot carry any of them however it is
// filled. It is stricter than intake.Skeleton, which keeps ref-des and MPNs because it stays inside
// the boundary; this crosses it, into a public issue.
//
// Every field is one of four things: a count, a duration, a value from a fixed vocabulary this
// package declares (Step, Status, ErrorKind, the design's format), or the name of a rule agni ships.
// A rule a project wrote is counted in aggregate and never named, because its namespace is the
// project's own name.
//
// Its JSON is hand-rolled rather than a proto, for the reason intake.Skeleton gives: a proto twin
// would have to carry this guarantee into a file people edit for other reasons.
package feedback

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Step is one thing a trial does, from a fixed list, so a step's name can never be text a design
// supplied.
type Step string

// The steps a trial walks, in order.
const (
	StepRead       Step = "read the design"
	StepCheck      Step = "run every rule"
	StepReview     Step = "score the project's checklists"
	StepQuerySets  Step = "answer the project's query sets"
	StepTrace      Step = "trace a signal through a series part"
	StepSaveReport Step = "save the check report"
)

// Status is how a step ended.
type Status string

// The outcomes a step can have. Skipped means the design gave the step nothing to do (a design in
// no project has no checklist), which is not a failure.
const (
	StatusOK      Status = "ok"
	StatusFailed  Status = "failed"
	StatusSkipped Status = "skipped"
)

// ErrorKind classifies why a step failed without carrying the error's text, which names files and
// entities. ClassifyError maps an error onto one.
type ErrorKind string

// The kinds a failure is reported as.
const (
	ErrorNone            ErrorKind = ""
	ErrorNotFound        ErrorKind = "not-found"
	ErrorInvalidArgument ErrorKind = "invalid-argument"
	ErrorUnavailable     ErrorKind = "unavailable"
	ErrorTimeout         ErrorKind = "timeout"
	ErrorInternal        ErrorKind = "internal"
)

// Report is one trial's sanitized result. Nothing in it is text a design or a project supplied.
type Report struct {
	Version  string       `json:"version"`  // the engine's build version
	Platform string       `json:"platform"` // GOOS/GOARCH, and whether it ran in a container
	Shape    Shape        `json:"shape"`
	Steps    []StepResult `json:"steps"`
	// Rules are the shipped rules that reported anything, by name. ProjectRules counts the rest.
	Rules        []RuleTally  `json:"rules"`
	ProjectRules ProjectRules `json:"project_rules"`
	Review       ReviewTally  `json:"review"`
	Queries      QueryTally   `json:"queries"`
}

// Shape is the design as counts. The format is the reader's name for it (kicad, edif), from the
// registry rather than the file.
type Shape struct {
	Format           string `json:"format"`
	Components       int    `json:"components"`
	Nets             int    `json:"nets"`
	Sheets           int    `json:"sheets"`
	HasBoard         bool   `json:"has_board"`
	UnexpandedBlocks int    `json:"unexpanded_blocks"` // hierarchical blocks the read left out, counted
}

// StepResult is one step's outcome and how long it took.
type StepResult struct {
	Step     Step          `json:"step"`
	Status   Status        `json:"status"`
	Duration time.Duration `json:"duration_ns"`
	Error    ErrorKind     `json:"error,omitempty"`
}

// RuleTally is one shipped rule's counts. Rule is a ShippedRule, which only Shipped can make, so a
// project's rule name cannot get here.
type RuleTally struct {
	Rule         ShippedRule `json:"rule"`
	Severity     string      `json:"severity"`
	Findings     int         `json:"findings"`
	Inconclusive int         `json:"inconclusive"`
	Pass         int         `json:"pass"`
	Fail         int         `json:"fail"`
}

// ProjectRules is what rules the design's project wrote reported, in aggregate.
type ProjectRules struct {
	Rules    int `json:"rules"`
	Findings int `json:"findings"`
}

// ReviewTally is what scoring the project's checklists produced, counted across them. A checklist's
// name and its items' titles are the project's words, so neither is kept.
type ReviewTally struct {
	Checklists int `json:"checklists"`
	Items      int `json:"items"`
	Answered   int `json:"answered"`
	Pass       int `json:"pass"`
	Fail       int `json:"fail"`
}

// QueryTally is what answering the project's query sets produced, counted across them.
type QueryTally struct {
	Sets     int `json:"sets"`
	Queries  int `json:"queries"`
	Answered int `json:"answered"`
	Failed   int `json:"failed"`
}

// ShippedRule is the name of a rule agni ships. It has an unexported field so that only Shipped can
// make one, from a name the caller's list of shipped rules holds.
type ShippedRule struct{ name string }

// String is the rule's name.
func (r ShippedRule) String() string { return r.name }

// MarshalText writes the rule's name, so a report's JSON names the rule.
func (r ShippedRule) MarshalText() ([]byte, error) { return []byte(r.name), nil }

// Shipped returns name as a ShippedRule when shipped holds it, and false for any other name, which a
// caller counts under ProjectRules instead.
func Shipped(name string, shipped map[string]bool) (ShippedRule, bool) {
	if !shipped[name] {
		return ShippedRule{}, false
	}
	return ShippedRule{name: name}, true
}

// ClassifyError reduces an error to its kind, dropping its text. The checks read the text only for
// the sentinel wording the service tier wraps every error in, never to keep any of it.
func ClassifyError(err error) ErrorKind {
	if err == nil {
		return ErrorNone
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "not found"), strings.Contains(s, "no such file"):
		return ErrorNotFound
	case strings.Contains(s, "invalid argument"):
		return ErrorInvalidArgument
	case strings.Contains(s, "unavailable"):
		return ErrorUnavailable
	case strings.Contains(s, "deadline exceeded"), strings.Contains(s, "context canceled"):
		return ErrorTimeout
	}
	return ErrorInternal
}

// Markdown renders the report for a person to read before they send it.
func (r Report) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "**agni %s** on %s\n\n", r.Version, r.Platform)
	s := r.Shape
	fmt.Fprintf(&b, "Design: %s, %d components, %d nets, %d sheets, board %s", s.Format, s.Components, s.Nets, s.Sheets, yesNo(s.HasBoard))
	if s.UnexpandedBlocks > 0 {
		fmt.Fprintf(&b, ", %d hierarchical blocks not read", s.UnexpandedBlocks)
	}
	b.WriteString(".\n\n| step | status | time |\n|---|---|---|\n")
	for _, st := range r.Steps {
		status := string(st.Status)
		if st.Error != ErrorNone {
			status += " (" + string(st.Error) + ")"
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", st.Step, status, st.Duration.Round(time.Millisecond))
	}
	if len(r.Rules) > 0 {
		b.WriteString("\n| shipped rule | severity | findings | inconclusive | pass | fail |\n|---|---|---|---|---|---|\n")
		rules := append([]RuleTally(nil), r.Rules...)
		sort.Slice(rules, func(i, j int) bool { return rules[i].Rule.name < rules[j].Rule.name })
		for _, t := range rules {
			fmt.Fprintf(&b, "| %s | %s | %d | %d | %d | %d |\n", t.Rule, t.Severity, t.Findings, t.Inconclusive, t.Pass, t.Fail)
		}
	}
	fmt.Fprintf(&b, "\nProject rules: %d reported %d findings.\n", r.ProjectRules.Rules, r.ProjectRules.Findings)
	fmt.Fprintf(&b, "Checklists: %d, %d of %d items answered, %d pass, %d fail.\n", r.Review.Checklists, r.Review.Answered, r.Review.Items, r.Review.Pass, r.Review.Fail)
	fmt.Fprintf(&b, "Query sets: %d, %d of %d queries answered, %d failed.\n", r.Queries.Sets, r.Queries.Answered, r.Queries.Queries, r.Queries.Failed)
	return b.String()
}

// issueBodyLimit keeps a prefilled issue link under the length browsers and GitHub accept.
const issueBodyLimit = 6000

// IssueURL is a link that opens a new issue on repo ("panyam/agni") prefilled with the report, for
// the person to read and submit. A body past the limit is cut at a line, saying so.
func (r Report) IssueURL(repo string) string {
	body := "<!-- Read this before submitting: it holds counts, timings and agni's own rule names, and nothing from your design. Add what you saw below it. -->\n\n" + r.Markdown() + "\n**What I saw:**\n\n"
	if len(body) > issueBodyLimit {
		cut := strings.LastIndex(body[:issueBodyLimit], "\n")
		body = body[:cut] + "\n\n(cut to fit a link; the full report is in the file `agni trial` wrote)\n"
	}
	v := url.Values{}
	v.Set("title", fmt.Sprintf("Trial report: agni %s", r.Version))
	v.Set("body", body)
	v.Set("labels", "trial-feedback")
	return "https://github.com/" + repo + "/issues/new?" + v.Encode()
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
