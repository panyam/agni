package query

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
)

// A rule built from a broken query used to compile without complaint and then report a clean pass,
// because RuleFromQuery returned no error and its Eval swallowed the failure into nil findings
// (agni issue 540). These pin the construction half: every one of these queries is wrong on its own
// terms, before any design is read.
func TestRuleFromQueryRejectsABrokenQuery(t *testing.T) {
	for _, tc := range []struct {
		name, query, wants string
	}{
		{
			name:  "unknown relation",
			query: `component-on-nett(?r, ?n) => ?r`,
			wants: "unknown relation",
		},
		{
			// The atom that is wrong is NOT the driver. An empty fact base stops solving after the
			// first atom yields nothing, so a validator built by evaluating against one would never
			// look here.
			name:  "wrong arity on a later atom",
			query: `component-on-net(?r, ?n), rail(?n, ?extra) => ?r`,
			wants: "takes 1 args",
		},
		{
			name:  "negation with nothing to range over",
			query: `component-on-net(?r, ?n), not rail(?other) => ?r`,
			wants: "shares no variable",
		},
		{
			name:  "projecting a variable no positive relation binds",
			query: `component-on-net(?r, ?n) => ?r, ?nothing`,
			wants: "not bound by a positive relation",
		},
		{
			name:  "rule head redefining a fact relation",
			query: `rail(?n) :- component-on-net(?r, ?n); rail(?n) => ?n`,
			wants: "redefines a fact relation",
		},
		{
			name:  "unknown aggregate",
			query: `component-on-net(?r, ?n) => median(?r)`,
			wants: "unknown aggregate",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q, err := Parse(tc.query)
			if err != nil {
				// A parse failure would make this test prove nothing about RuleFromQuery, so it is a
				// fatal setup error rather than a pass.
				t.Fatalf("this query must PARSE and fail later: %v", err)
			}
			_, err = RuleFromQuery(FindingQuery{
				Rule:       check.Rule{Name: "t", Severity: "error"},
				Query:      q,
				Kind:       check.KindComponent,
				SubjectVar: "r",
				Message:    "{r}",
			})
			if err == nil {
				t.Fatalf("built a rule from a broken query; it would have reported a clean pass")
			}
			if !strings.Contains(err.Error(), tc.wants) {
				t.Errorf("error %q does not say %q, so an author cannot tell what is wrong", err, tc.wants)
			}
		})
	}
}

// TestRuleFromQueryAcceptsAValidQuery is the positive control. A validator that rejected everything
// would pass every case above while making the engine useless.
func TestRuleFromQueryAcceptsAValidQuery(t *testing.T) {
	q := MustParse(`component-on-net(?r, ?n), rail(?n) => ?r`)
	rule, err := RuleFromQuery(FindingQuery{
		Rule:       check.Rule{Name: "t", Severity: "error"},
		Query:      q,
		Kind:       check.KindComponent,
		SubjectVar: "r",
		Message:    "{r}",
	})
	if err != nil {
		t.Fatalf("a valid query was rejected: %v", err)
	}
	if rule == nil {
		t.Fatal("no rule and no error")
	}
}

// TestValidateNeedsNoDesign: validation reads the query and the relation vocabulary, never a design.
// That is what lets a rule be checked where it is BUILT rather than where it first runs, which is the
// whole reason the construction-time half can exist.
func TestValidateNeedsNoDesign(t *testing.T) {
	if err := Validate(MustParse(`component-on-net(?r, ?n), rail(?n) => ?r`), facts.DefaultRegistry()); err != nil {
		t.Errorf("valid query rejected with no design: %v", err)
	}
	if err := Validate(MustParse(`nosuchrelation(?x) => ?x`), facts.DefaultRegistry()); err == nil {
		t.Error("unknown relation accepted with no design")
	}
}

// evalFailingQuery validates but cannot be solved: `contains` needs its argument bound, and no
// relation binds ?loose. Validation does not catch it because binding ORDER is what the solver
// establishes, and reproducing that here would mean reimplementing solve.
//
// That gap is why the eval-time half of agni issue 540 matters. Construction catches the queries an
// author gets wrong most often, and this one still slips through to the evaluator, so what the
// evaluator does with a failure is not academic.
const evalFailingQuery = `component-on-net(?r, ?n), contains(?loose, "x") => ?r`

// TestEvalFailureIsInconclusiveNotClean: a rule whose query cannot be evaluated reports that it could
// not decide. It used to return no findings, which every consumer reads as a design with no defects.
func TestEvalFailureIsInconclusiveNotClean(t *testing.T) {
	q, err := Parse(evalFailingQuery)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := Validate(q, facts.DefaultRegistry()); err != nil {
		t.Fatalf("this query must PASS validation and fail at eval, else it tests the wrong half: %v", err)
	}
	rule, err := RuleFromQuery(FindingQuery{
		Rule:       check.Rule{Name: "loose-filter", Severity: "error"},
		Query:      q,
		Kind:       check.KindComponent,
		SubjectVar: "r",
		Message:    "{r}",
	})
	if err != nil {
		t.Fatalf("construction rejected it, so the eval path is untested: %v", err)
	}

	fs := rule.Findings(check.NewModel(chainDesign()))
	if len(fs) != 1 {
		t.Fatalf("findings = %d, want one inconclusive; a rule that could not run must not report a clean pass", len(fs))
	}
	if !fs[0].Inconclusive {
		t.Errorf("finding is a DEFECT, not an inconclusive: %+v", fs[0])
	}
	if !strings.Contains(fs[0].Message, "could not run") || !strings.Contains(fs[0].Message, "loose-filter") {
		t.Errorf("message %q names neither the rule nor what happened", fs[0].Message)
	}
}
