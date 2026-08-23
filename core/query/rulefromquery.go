package query

import (
	"regexp"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// FindingQuery declares a datalog-backed check rule (WS3-038): a datalog program whose every
// answer row becomes a check.Finding. It is the bridge from the SEARCH surface (a query returns
// rows) to the RULE surface (a rule emits findings with a severity, a subject, and a doc), so a
// declarative datalog query is a first-class catalog rule, not only an ad-hoc search — the "prove
// it in datalog, optimize in Go later" path.
//
// Rule carries the rule's identity and metadata (Name, Severity, Summary, Tags, Detail, Reads);
// RuleFromQuery fills its Eval. Query is the COMPILED datalog program (WS3-043): the front end owns
// producing it — MustParse for a hand-authored string, Build for a generated AST — so this bridge no
// longer parses. The goal MUST project SubjectVar (and PinVar when Kind is KindPin). Message is a
// template whose {var} placeholders are filled from each answer row. If Rule.Reads is empty it is
// derived from the query (the EDB relations it reads), so check.Available gates it correctly.
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
	// ContextVars are further projected variables to carry as each finding's CONTEXT entities: the
	// entities the message names but is not about (agni issue 349). Empty for a rule whose message
	// names only its subject, which is most of them.
	//
	// SubjectVar already proves this surface can say which projected variable plays which part. This
	// is that same idea for a list, and it is why the fix is a field rather than a convention: the
	// binding exists in the row and was simply dropped when the Finding was built.
	//
	// A SLICE rather than a map keyed by role, for two reasons. Order is significant (it matches the
	// order the message names them, so a panel's chips read like the sentence), and two entities may
	// share a role: "A and B both strap to address N" has two entities playing "device".
	ContextVars []ContextVar
	// TupleVars are further projected variables that join SubjectVar to form the VERDICT's subject
	// tuple, in the rule's own order. Empty for the ordinary case: one subject, and the tuple is it.
	//
	// It is separate from ContextVars because the two answer different questions. A context entity is
	// something the message NAMES; a tuple element is part of what the verdict IS, and therefore part
	// of its id. A rule reporting one row per (host, required signal) needs the signal here, or every
	// row for one host answers to one name. Finding.Subject stays singular either way.
	TupleVars []TupleVar
	// Domain, when set, declares the rule's CONSIDERED SET as a second goal beside the finding goal:
	// its answer rows are every subject the rule examined, where the finding goal's rows are the
	// subset that came out wrong. Setting it is what lets a query-backed rule state a considered set
	// (agni issue 424); leaving it unset keeps the failures-only shape, which under-reports the rule
	// and never overstates it.
	//
	// DECLARED RATHER THAN DERIVED, which is the whole design. The domain LOOKS derivable. For the
	// ESD and pull-up requirements it is exactly the finding goal's body minus its negated literal,
	// but signal-dangling ends in a comparison rather than a negation, and taking its body as the
	// domain would report the failures AS the considered set. That is a coverage claim the run has
	// not earned, and it is the same lie StatesConsideredSet exists to prevent. An author knows the
	// answer; an inference that is right four times out of six is worse than no inference.
	Domain *Domain
}

// Domain is a rule's considered set, expressed the way the rule itself is: as a goal over the same
// datalog program. Its rows must project SubjectVar, PinVar and every TupleVar, so a domain row and
// a finding row key the same way and the passing subjects are the difference between them.
type Domain struct {
	// Query is the compiled domain program. It shares the finding query's rule set by convention
	// rather than by construction, because a scope goal usually needs one rule of its own (the
	// finding goal's body with the failing condition dropped) and building it beside the rest is
	// what keeps the two legible together.
	Query Query
	// Witness is the template for a passing verdict's statement, interpolated from the domain row the
	// same way Message is from a finding row. REQUIRED when Domain is set, because a Pass without a
	// witness is the silence verdicts exist to remove.
	//
	// It may assert the positive condition ("{n} reaches an ESD clamp"), which is sound even though
	// the domain goal does not prove it: a subject in the domain that is not in the failures is one
	// the finding goal declined to report, and for these rules that is precisely the good case.
	Witness string
}

// TupleVar binds one projected datalog variable to an element of the verdict's subject tuple. Kind is
// required for the reason it is on ContextVar: a datalog variable is a bare string binding, and
// nothing about the column says whether it holds a net name, a ref des or a required-signal role.
type TupleVar struct {
	Var  string // projected variable name, no leading "?"
	Kind string // check.KindNet | check.KindComponent | check.KindSignal | ...
}

// ContextVar binds one projected datalog variable to a context entity on every finding a rule emits.
//
// Kind is the entity's subject kind, which is NOT inferable from the variable: a datalog variable is
// just a string binding, and the same projected column could be a net name or a ref des depending on
// the relation it came from. Getting it wrong produces a chip that highlights nothing, so it is
// required rather than defaulted.
type ContextVar struct {
	Var  string // projected variable name, no leading "?"
	Kind string // check.KindNet | check.KindComponent | check.KindPin | check.KindBus
	Role string // the part it plays in the message ("terminal", "rail", "source")
}

var placeholderRe = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

// RuleFromQuery compiles a FindingQuery into a check.Rule whose Eval runs the (already-compiled)
// query over the design and maps each answer row to a Finding. At Eval time it builds a query Base
// from the Model, evaluates, and projects each row into a Finding. An eval error (which a validated
// query should not produce) yields no findings rather than a panic, so a rule can never crash a
// check run. The Finding's Prov is resolved from the Model by subject, so a query-backed finding
// stays as locatable as a hand-written one.
func RuleFromQuery(fq FindingQuery) *check.Rule {
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
		// In the author's declared order, which is the order the message names them. A context var
		// that did not bind in this row contributes nothing rather than an empty chip: the row could
		// not have been projected without it, so an unbound one means the rule was mis-declared and a
		// blank chip would hide that behind something that looks deliberate.
		for _, cv := range fq.ContextVars {
			ref := row.Bind[Var(cv.Var)].S
			if ref == "" {
				continue
			}
			f.Context = append(f.Context, check.ContextSubject{Entity: check.Entity{Kind: cv.Kind, Ref: ref}, Role: cv.Role})
		}
		if fq.ParamSymbol != "" && fq.Kind == check.KindComponent {
			if dp := check.DatasheetProvFor(m, subj, fq.ParamSymbol); dp != nil {
				f.DatasheetProv = []*check.DatasheetCitation{dp}
			}
		}
		return f
	}
	// A datalog goal yields the rows that MATCHED, so without a declared Domain the subjects the rule
	// silently passed over are not in the answer at all and the honest report is failures-only.
	if fq.Domain == nil {
		r.Eval = check.FailuresOnly(func(m check.Model) []check.Finding {
			rows, err := Naive{}.Eval(q, NewBase(m))
			if err != nil {
				return nil
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
	// can only see the Finding, whose subject is singular by contract. A rule reporting one row per
	// (host, required signal) needs its tuple, and the tuple has to come from the ROW.
	r.Eval = func(m check.Model) []check.Verdict {
		base := NewBase(m)
		var vs []check.Verdict
		failed := map[string]bool{}
		rows, err := Naive{}.Eval(q, base)
		if err != nil {
			// A validated query does not reach here. Reporting nothing beats reporting a considered set
			// whose failing half never ran.
			return nil
		}
		for _, row := range rows {
			f := findingFor(m, row)
			outcome := check.Fail
			if f.Inconclusive {
				outcome = check.Inconclusive
			}
			subjects := fq.tuple(row)
			failed[tupleKey(subjects)] = true
			// A witness on the FAILING verdict too, which check.FailuresOnly deliberately omits. There
			// it would be decoration, because an unconverted rule has no proof to show and inventing one
			// makes it look converted. Here the answer row IS the proof: the goal matched, and the
			// message is that match stated in words.
			//
			// It is also load-bearing rather than tidy. Verdict.Witness is REQUIRED on Pass and Fail,
			// and the wire form of a Verdict carries no Finding on purpose (a defect travels once, in
			// the findings array), so a failing row that crossed the seam with only a Finding rendered
			// with an empty sentence in every consumer that reads verdicts back from the service.
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
			// Keep the findings. A defect must never disappear because the coverage half misbehaved,
			// even though the rule then reports fewer passes than it examined.
			return vs
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
			})
		}
		return vs
	}
	return &r
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
// as locatable as a hand-written one: a pin/component subject is a ref-des, a net subject is a net
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
