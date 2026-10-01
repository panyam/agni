package query

import (
	"fmt"
	"regexp"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// FindingQuery declares a datalog-backed check rule (WS3-038), a datalog program whose every answer
// row becomes a check.Finding. It is how a datalog query becomes a catalog rule with a severity, a
// subject and a doc, rather than only an ad-hoc search.
//
// Rule carries the rule's identity and metadata (Name, Severity, Summary, Tags, Detail, Reads);
// RuleFromQuery fills its Eval. Query is the COMPILED datalog program (WS3-043), produced by the
// caller with MustParse for a hand-authored string or Build for a generated AST. The goal MUST
// project SubjectVar (and PinVar when Kind is KindPin). Message is a template whose {var}
// placeholders are filled from each answer row. If Rule.Reads is empty it is derived from the query
// (the EDB relations it reads), so check.Available gates it correctly.
type FindingQuery struct {
	Rule       check.Rule
	Query      Query
	Kind       string // check.KindNet | check.KindComponent | check.KindPin
	SubjectVar string // projected variable name (no leading ?) bound to Finding.Subject
	PinVar     string // projected variable name bound to Finding.Pin (KindPin only; "" otherwise)
	Message    string // template; each {var} is replaced by that variable's bound value in the row
	// ParamSymbol, when set on a component-subject datasheet rule, names the datasheet symbol the rule
	// checks (e.g. "IOUT"). RuleFromQuery then resolves that parameter's citation from the subject's
	// seeded spec and attaches it to Finding.DatasheetProv, so a datalog-authored datasheet finding
	// carries the same doc/page/section/confidence a built-in datasheet rule does. Empty = no citation.
	ParamSymbol string
	// ContextVars are further projected variables to carry as each finding's CONTEXT entities, the
	// entities the message names but is not about (agni issue 349). Empty for a rule whose message
	// names only its subject, which is most of them.
	//
	// A SLICE rather than a map keyed by role, because order is significant (chips render in the order
	// the message names them) and two entities may share a role ("A and B both strap to address N").
	// See docsite/content/build/check-rule.md#if-the-sentence-names-two-entities-carry-both.
	ContextVars []ContextVar
	// TupleVars are further projected variables that join SubjectVar to form the VERDICT's subject
	// tuple, in the rule's own order. Empty for the ordinary one-subject case.
	//
	// Not ContextVars, because a context entity is something the message NAMES and a tuple element is
	// part of what the verdict IS, and so of its id. A rule reporting one row per (host, required
	// signal) needs the signal here, or every row for one host answers to one name. Finding.Subject
	// stays singular either way
	// (docsite/content/build/check-rule.md#a-tuple-in-the-verdict-one-entity-in-the-finding).
	TupleVars []TupleVar
	// Domain, when set, declares the rule's CONSIDERED SET as a second goal beside the finding goal.
	// Its rows are every subject the rule examined, where the finding goal's rows are the subset that
	// came out wrong (agni issue 424). Unset keeps the failures-only shape, which under-reports the rule
	// and never overstates it.
	//
	// DECLARED RATHER THAN DERIVED. For the ESD and pull-up requirements the domain is the finding
	// goal's body minus its negated literal, but signal-dangling ends in a comparison, so deriving its
	// domain would report the failures AS the considered set, the false coverage claim
	// StatesConsideredSet exists to prevent.
	Domain *Domain
}

// Domain is a rule's considered set, expressed as a goal over the same datalog program as the rule.
// Its rows must project SubjectVar, PinVar and every TupleVar, so a domain row and a finding row key
// the same way and the passing subjects are the difference between them.
type Domain struct {
	// Query is the compiled domain program. It shares the finding query's rule set by convention
	// rather than by construction, since a scope goal usually needs one rule of its own (the finding
	// goal's body with the failing condition dropped).
	Query Query
	// Witness is the template for a passing verdict's statement, interpolated from the domain row the
	// same way Message is from a finding row. REQUIRED when Domain is set.
	//
	// It may assert the positive condition ("{n} reaches an ESD clamp") even though the domain goal
	// does not prove it, because a subject in the domain and not in the failures is one the finding
	// goal declined to report, which for these rules is the good case.
	Witness string
	// Evidence, when set, names the entities a PASSING subject's proof rests on, through the rule's
	// own ContextVars. Its rows are matched to passing subjects by the subject tuple.
	//
	// It is a third goal rather than a widening of the domain, because the domain is the CONSIDERED
	// SET, passing and failing alike, and a failing subject has no proof to bind. So one query cannot
	// both enumerate the failures and bind the evidence (agni issue 662). The Go-walk requirements get
	// the same thing from check.PullUpVerdict, which returns its hops as entities.
	//
	// Optional. A passing subject with no evidence row keeps an empty context, since some requirements
	// prove a negative and have nothing to point at.
	Evidence *Query
}

// TupleVar binds one projected datalog variable to an element of the verdict's subject tuple. Kind is
// required for the reason it is on ContextVar, since a datalog variable is a bare string binding and
// nothing about the column says whether it holds a net name, a ref des or a required-signal role.
type TupleVar struct {
	Var  string // projected variable name, no leading "?"
	Kind string // check.KindNet | check.KindComponent | check.KindSignal | ...
}

// ContextVar binds one projected datalog variable to a context entity on every finding a rule emits.
//
// Kind is the entity's subject kind, which is NOT inferable from the variable, because the same
// projected column could be a net name or a ref des depending on the relation it came from. A wrong
// Kind produces a chip that highlights nothing, so it is required rather than defaulted.
type ContextVar struct {
	Var  string // projected variable name, no leading "?"
	Kind string // check.KindNet | check.KindComponent | check.KindPin | check.KindBus
	Role string // the part it plays in the message ("terminal", "rail", "source")
}

var placeholderRe = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

// RuleFromQuery compiles a FindingQuery into a check.Rule whose Eval runs the query over a design and
// maps each answer row to a Finding. The Finding's Prov is resolved from the Model by subject, so a
// query-backed finding stays as locatable as a hand-written one.
//
// IT VALIDATES THE QUERY AND REPORTS WHY IT CANNOT RUN, so a misspelled relation is an error at
// construction rather than an empty answer that reads as a CLEAN PASS (agni issue 540).
//
// Use it where the query came from a person (a review manifest, a wire RuleDef, an overlay). For a
// query written into this repo as code, use MustRuleFromQuery.
func RuleFromQuery(fq FindingQuery) (*check.Rule, error) {
	if err := Validate(fq.Query, facts.DefaultRegistry()); err != nil {
		return nil, err
	}
	if err := fq.checkProjects(fq.Query, "query"); err != nil {
		return nil, err
	}
	if fq.Domain != nil {
		if err := fq.checkProjects(fq.Domain.Query, "domain"); err != nil {
			return nil, err
		}
	}
	return buildRule(fq), nil
}

// checkProjects rejects a goal whose rows cannot name the finding's subject. Each row is read by
// column name, so a subject the goal does not project as a plain column comes back as the empty
// string and becomes a finding about nothing. An aggregate does not count, because count(?r) is
// labelled count(r) and over an empty design it is still one row (agni issue 726).
func (fq FindingQuery) checkProjects(q Query, which string) error {
	cols := map[Var]bool{}
	for _, c := range q.Columns() {
		cols[c] = true
	}
	need := []string{fq.SubjectVar}
	if fq.PinVar != "" {
		need = append(need, fq.PinVar)
	}
	for _, tv := range fq.TupleVars {
		need = append(need, tv.Var)
	}
	for _, v := range need {
		if !cols[Var(v)] {
			return fmt.Errorf("query: %s: the %s does not project ?%s as a plain column, so its rows cannot name the finding's subject", fq.Rule.Name, which, v)
		}
	}
	return nil
}

// MustRuleFromQuery is RuleFromQuery for a query that ships as CODE, panicking if it does not
// compile.
//
// A built-in rule or a compiled profile carries a query this repo's tests run, so one that does not
// validate is a programmer error. A package-level `var r = RuleFromQuery(q)` has nowhere to return
// an error to, so it panics at init rather than dropping the error. profiles.Compile panics on a
// malformed profile for the same reason.
func MustRuleFromQuery(fq FindingQuery) *check.Rule {
	r, err := RuleFromQuery(fq)
	if err != nil {
		panic("query: " + fq.Rule.Name + ": " + err.Error())
	}
	return r
}

// evalFailed is the finding a rule reports when its query could not be evaluated against a design.
//
// It carries no subject, because the failure is the RULE's rather than any entity's, and naming an
// arbitrary component would send a reader to a part that is fine. It is Inconclusive
// (docsite/content/build/check-rule.md#five-outcomes-and-the-three-that-are-not-a-pass).
func evalFailed(rule string, err error) check.Finding {
	return check.Finding{
		Inconclusive: true,
		Message:      fmt.Sprintf("%s could not run: %v", rule, err),
	}
}

// buildRule is RuleFromQuery's construction half, after validation has passed.
func buildRule(fq FindingQuery) *check.Rule {
	q := fq.Query
	r := fq.Rule
	if len(r.Reads) == 0 {
		r.Reads = Reads(q)
	}
	// findingFor projects one answer row onto the violation it reports. It is shared by both eval
	// shapes below so the finding a rule emits cannot depend on whether its author declared a domain.
	findingFor := func(m check.Model, row Row) check.Finding {
		subj := row.Bind[Var(fq.SubjectVar)].S
		f := check.Finding{Subject: check.Entity{Kind: fq.Kind, Ref: subj}, Message: interpolate(fq.Message, row), Prov: provFor(m, fq.Kind, subj)}
		if fq.PinVar != "" {
			f.Subject.Pin = row.Bind[Var(fq.PinVar)].S
		}
		f.Context = append(f.Context, fq.contextOf(row)...)
		if fq.ParamSymbol != "" && fq.Kind == check.KindComponent {
			if dp := check.DatasheetProvFor(m, subj, fq.ParamSymbol); dp != nil {
				f.DatasheetProv = []*check.DatasheetCitation{dp}
			}
		}
		return f
	}
	// A datalog goal yields the rows that MATCHED, so without a declared Domain the subjects the rule
	// passed over are not in the answer and the only accurate report is failures-only.
	if fq.Domain == nil {
		r.Eval = check.FailuresOnly(func(m check.Model) []check.Finding {
			rows, err := Naive{}.Eval(q, NewBase(m))
			if err != nil {
				// Construction validated this query, so this is the ENGINE failing on a design. An
				// inconclusive finding says the rule could not decide, where nil would read as a clean
				// pass (agni issue 540).
				return []check.Finding{evalFailed(r.Name, err)}
			}
			out := []check.Finding{}
			for _, row := range rows {
				out = append(out, findingFor(m, row))
			}
			return out
		})
		return &r
	}
	r.StatesConsideredSet = true
	r.SubjectShape = fq.subjectShape()
	// The failing verdicts are built here rather than through check.FailuresOnly, because that adapter
	// sees only the Finding, whose subject is singular, and the subject tuple has to come from the ROW.
	r.Eval = func(m check.Model) []check.Verdict {
		base := NewBase(m)
		var vs []check.Verdict
		failed := map[string]bool{}
		rows, err := Naive{}.Eval(q, base)
		if err != nil {
			// The failing half never ran, so any considered set would report every subject as passing
			// on evidence never gathered. One inconclusive verdict says the rule could not decide
			// (agni issue 540).
			f := evalFailed(r.Name, err)
			return []check.Verdict{{
				Outcome: check.Inconclusive,
				Witness: &check.Witness{Statement: f.Message},
				Finding: &f,
			}}
		}
		for _, row := range rows {
			f := findingFor(m, row)
			outcome := check.Fail
			if f.Inconclusive {
				outcome = check.Inconclusive
			}
			subjects := fq.tuple(row)
			failed[tupleKey(subjects)] = true
			// A witness on the FAILING verdict too, which check.FailuresOnly omits. Here the answer row
			// IS the proof, and the message states it. Verdict.Witness is REQUIRED on Pass and Fail, and
			// the wire Verdict carries no Finding (a defect travels once, in the findings array), so
			// without it a failing verdict read back from the service renders an empty sentence.
			vs = append(vs, check.Verdict{
				Outcome:  outcome,
				Subjects: subjects,
				Context:  f.Context,
				Witness:  &check.Witness{Statement: f.Message},
				Finding:  &f,
			})
		}
		drows, err := Naive{}.Eval(fq.Domain.Query, base)
		if err != nil {
			// Keep the findings. A defect must never disappear because the coverage half failed, even
			// though the rule then reports fewer passes than it examined.
			return vs
		}
		// The entities a pass rests on, indexed by subject tuple. Evaluated once for the whole rule,
		// since per subject would re-derive the entire program for each row. An evaluation failure
		// means no evidence rather than an error, so a pass loses its chips and not itself.
		evidence := map[string][]check.ContextSubject{}
		if fq.Domain.Evidence != nil {
			if erows, err := (Naive{}).Eval(*fq.Domain.Evidence, base); err == nil {
				for _, row := range erows {
					subjects := fq.tuple(row)
					if len(subjects) == 0 {
						continue
					}
					key := tupleKey(subjects)
					if _, seen := evidence[key]; seen {
						continue // one proof is enough; a second path says nothing new about the pass
					}
					evidence[key] = fq.contextOf(row)
				}
			}
		}
		for _, row := range drows {
			subjects := fq.tuple(row)
			if len(subjects) == 0 || failed[tupleKey(subjects)] {
				continue
			}
			vs = append(vs, check.Verdict{
				Outcome:  check.Pass,
				Subjects: subjects,
				Witness:  &check.Witness{Statement: interpolate(fq.Domain.Witness, row)},
				Context:  evidence[tupleKey(subjects)],
			})
		}
		return vs
	}
	return &r
}

// contextOf projects a row's declared ContextVars as context entities, in the author's order, which
// is the order the message names them.
//
// A context var that did not bind in this row contributes nothing rather than an empty chip, because
// an unbound one means the rule was mis-declared and a blank chip would hide that.
//
// Shared by the finding path and the evidence path so the two cannot drift about what a context
// entity is.
func (fq FindingQuery) contextOf(row Row) []check.ContextSubject {
	var out []check.ContextSubject
	for _, cv := range fq.ContextVars {
		ref := row.Bind[Var(cv.Var)].S
		if ref == "" {
			continue
		}
		out = append(out, check.ContextSubject{Entity: check.Entity{Kind: cv.Kind, Ref: ref}, Role: cv.Role})
	}
	return out
}

// tuple builds the verdict's subject tuple from one answer row: the primary subject, then each
// declared TupleVar in the author's order. An unbound primary subject yields no tuple at all, since a
// verdict with no subject has no id and could not be addressed.
func (fq FindingQuery) tuple(row Row) []check.Entity {
	subj := row.Bind[Var(fq.SubjectVar)].S
	if subj == "" {
		return nil
	}
	e := check.Entity{Kind: fq.Kind, Ref: subj}
	if fq.PinVar != "" {
		e.Pin = row.Bind[Var(fq.PinVar)].S
	}
	out := []check.Entity{e}
	for _, tv := range fq.TupleVars {
		out = append(out, check.Entity{Kind: tv.Kind, Ref: row.Bind[Var(tv.Var)].S})
	}
	return out
}

// subjectShape is the tuple's kinds, declared so a consumer can index the rule's verdicts. Empty for
// the ordinary one-subject rule, matching what check.Rule.SubjectShape means by empty.
func (fq FindingQuery) subjectShape() []string {
	if len(fq.TupleVars) == 0 {
		return nil
	}
	out := []string{fq.Kind}
	for _, tv := range fq.TupleVars {
		out = append(out, tv.Kind)
	}
	return out
}

// tupleKey names a subject tuple for set membership. It reuses VerdictID's escaping rather than
// joining refs with a separator, because a net name is passed through from a source file and may
// contain anything, including whatever separator looked safe.
func tupleKey(es []check.Entity) string {
	return check.VerdictID(check.Verdict{Subjects: es})
}

// interpolate replaces each {var} in the template with that variable's bound string value in the
// row; an unbound {var} renders empty (the row could not have been projected without it, so this is
// belt-and-braces).
func interpolate(tmpl string, row Row) string {
	return placeholderRe.ReplaceAllStringFunc(tmpl, func(m string) string {
		return row.Bind[Var(m[1:len(m)-1])].S
	})
}

// provFor resolves the design-side provenance for a finding's subject so a query-backed finding is
// as locatable as a hand-written one. A pin/component subject is a ref-des, a net subject is a net
// name. Nil when the subject is not found (the message still carries the identity).
func provFor(m check.Model, kind, subject string) *ir.Provenance {
	switch kind {
	case check.KindNet:
		for _, n := range m.Nets() {
			if n.Name == subject {
				return n.Prov
			}
		}
	case check.KindComponent, check.KindPin:
		for _, c := range m.Components() {
			if c.RefDes == subject {
				return c.Prov
			}
		}
	}
	return nil
}
