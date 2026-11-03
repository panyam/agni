package query

import (
	"context"
	"fmt"
	"regexp"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/jaala/datalog"
)

// FindingQuery declares a datalog-backed check rule (WS3-038). Every answer row of its datalog
// program becomes a check.Finding. Through it, a datalog query becomes a catalog rule with a
// severity, a subject and a doc, rather than only an ad-hoc search.
//
// Rule holds the rule's identity and metadata (Name, Severity, Summary, Tags, Detail, Reads), and
// RuleFromQuery fills its Eval. Query is the COMPILED datalog program (WS3-043). The caller
// produces it with MustParse for a hand-authored string or Build for a generated AST. The goal MUST
// project SubjectVar, and PinVar too when Kind is KindPin. Message is a template, and each answer
// row fills its {var} placeholders. If Rule.Reads is empty, it is derived from the query as the EDB
// relations the query reads, so check.Available gates it correctly.
type FindingQuery struct {
	Rule  check.Rule
	Query Query
	// Vocabulary resolves the query's names. It holds the shipped relations and library, plus any
	// library the run brings, either a project's lib/ or modules sent with the request (agni issues
	// 779, 788). The rule validates against it, declares what it reads from it, and evaluates
	// against it. Nil means the process default, which suits a query written into this repo.
	Vocabulary *facts.Registry
	Kind       string // check.KindNet | check.KindComponent | check.KindPin
	SubjectVar string // projected variable name (no leading ?) bound to Finding.Subject
	PinVar     string // projected variable name bound to Finding.Pin (KindPin only; "" otherwise)
	Message    string // template; each {var} is replaced by that variable's bound value in the row
	// ParamSymbol names the datasheet symbol to check, such as "IOUT", on a component-subject
	// datasheet rule. RuleFromQuery then resolves that parameter's citation from the subject's
	// seeded spec and attaches it to Finding.DatasheetProv. So a datalog-authored datasheet finding
	// has the same doc/page/section/confidence as a built-in datasheet rule's finding. Empty means
	// no citation.
	ParamSymbol string
	// ContextVars are further projected variables, each set as a CONTEXT entity on each finding.
	// The message names a context entity but is not about it (agni issue 349). Most rules leave it
	// empty, because their message names only the subject.
	//
	// It is a SLICE rather than a map keyed by role, for two reasons. Order matters, since chips
	// render in the message's order. And two entities may share a role ("A and B both strap to
	// address N"). See
	// docsite/content/build/check-rule.md#if-the-sentence-names-two-entities-carry-both.
	ContextVars []ContextVar
	// TupleVars are further projected variables that join SubjectVar to form the VERDICT's subject
	// tuple, in the rule's own order. Empty for the ordinary one-subject case.
	//
	// These are not ContextVars. The message NAMES a context entity, while a tuple element is part
	// of what the verdict IS, and so part of its id. Imagine a rule reporting one row per (host,
	// required signal). It needs the signal here, or every row for one host answers to one name.
	// Finding.Subject is singular either way
	// (docsite/content/build/check-rule.md#a-tuple-in-the-verdict-one-entity-in-the-finding).
	TupleVars []TupleVar
	// Domain declares the rule's CONSIDERED SET as a second goal beside the finding goal. Its rows
	// are every subject the rule examined, and the finding goal's rows are the subset that came out
	// wrong (agni issue 424). Without a Domain the rule keeps the failures-only shape. That shape
	// under-reports the rule and never overstates it.
	//
	// The domain is DECLARED RATHER THAN DERIVED. For the ESD and pull-up requirements, the domain
	// is the finding goal's body minus its negated literal. But signal-dangling ends in a
	// comparison. Deriving its domain would report the failures AS the considered set.
	// StatesConsideredSet exists to prevent that false coverage claim.
	Domain *Domain
}

// Domain is a rule's considered set, expressed as a goal over the same datalog program as the rule.
// Its rows must project SubjectVar, PinVar and every TupleVar, so a domain row and a finding row
// key the same way. The passing subjects are then the domain rows minus the finding rows.
type Domain struct {
	// Query is the compiled domain program. It shares the finding query's rule set by convention
	// rather than by construction, since a scope goal usually needs one rule of its own (the finding
	// goal's body with the failing condition dropped).
	Query Query
	// Witness is the template for a passing verdict's statement, interpolated from the domain row the
	// same way Message is from a finding row. REQUIRED when Domain is set.
	//
	// It may assert the positive condition ("{n} reaches an ESD clamp") even though the domain goal
	// does not prove it. A subject in the domain and not in the failures is one the finding goal
	// declined to report. For these rules, that is the good case.
	Witness string
	// Evidence names the entities that prove a PASSING subject passes, through the rule's own
	// ContextVars. Its rows are matched to passing subjects by the subject tuple.
	//
	// It is a third goal rather than a widening of the domain. The domain is the CONSIDERED SET,
	// passing and failing alike, and a failing subject has no proof to bind. So one query cannot
	// both enumerate the failures and bind the evidence (agni issue 662). The Go-walk requirements
	// get the same thing from check.PullUpVerdict, which returns its hops as entities.
	//
	// Evidence is optional. A passing subject with no evidence row keeps an empty context, since
	// some requirements prove a negative and have nothing to point at.
	Evidence *Query
}

// TupleVar binds one projected datalog variable to an element of the verdict's subject tuple. Kind
// is required, for the same reason as on ContextVar. A datalog variable is a bare string binding,
// and nothing about the column says whether it holds a net name, a ref des or a required-signal
// role.
type TupleVar struct {
	Var  string // projected variable name, no leading "?"
	Kind string // check.KindNet | check.KindComponent | check.KindSignal | ...
}

// ContextVar binds one projected datalog variable to a context entity on every finding a rule emits.
//
// Kind is the entity's subject kind. It is NOT inferable from the variable. The same projected
// column could be a net name or a ref des, depending on the relation it came from. A wrong Kind
// produces a chip that highlights nothing, so Kind is required rather than defaulted.
type ContextVar struct {
	Var  string // projected variable name, no leading "?"
	Kind string // check.KindNet | check.KindComponent | check.KindPin | check.KindBus
	Role string // the part it plays in the message ("terminal", "rail", "source")
}

var placeholderRe = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

// RuleFromQuery compiles a FindingQuery into a check.Rule whose Eval runs the query over a design
// and maps each answer row to a Finding. The Finding's Prov is resolved from the Model by subject,
// so a query-backed finding is as locatable as a hand-written one.
//
// IT VALIDATES THE QUERY AND REPORTS WHY IT CANNOT RUN, so a misspelled relation is an error at
// construction rather than an empty answer that reads as a CLEAN PASS (agni issue 540).
//
// Use it where the query came from a person (a review manifest, a wire RuleDef, an overlay). For a
// query written into this repo as code, use MustRuleFromQuery.
func RuleFromQuery(fq FindingQuery) (*check.Rule, error) {
	if err := Validate(fq.Query, fq.vocabulary()); err != nil {
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
// column name. If the goal does not project the subject as a plain column, the subject comes back
// as the empty string, and the finding is about nothing. An aggregate does not count. count(?r) is
// labelled count(r), and over an empty design it is still one row (agni issue 726).
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
// This repo's tests run the query of every built-in rule and every compiled profile. So a query
// there that does not validate is a programmer error. A package-level `var r = RuleFromQuery(q)`
// has nowhere to return an error to, so it panics at init rather than dropping the error.
// profiles.Compile panics on a malformed profile for the same reason.
func MustRuleFromQuery(fq FindingQuery) *check.Rule {
	r, err := RuleFromQuery(fq)
	if err != nil {
		panic("query: " + fq.Rule.Name + ": " + err.Error())
	}
	return r
}

// evalFailed builds the finding for a rule whose query could not be evaluated against a design.
//
// It names no subject, because the failure belongs to the RULE rather than to any entity. Naming an
// arbitrary component would send a reader to a part that is fine. The finding is Inconclusive
// (docsite/content/build/check-rule.md#five-outcomes-and-the-three-that-are-not-a-pass).
func evalFailed(rule string, err error) check.Finding {
	return check.Finding{
		Inconclusive: true,
		Message:      fmt.Sprintf("%s could not run: %v", rule, err),
	}
}

// vocabulary returns the registry the rule's names resolve against, or the process default when
// none is set.
//
// The default is looked up each time, never captured when the rule is built. Built-in rules compile
// in package init() before any relation catalog has registered (agni issue 540). A captured default
// would be an empty registry, and every such rule would report that it could not decide.
func (fq FindingQuery) vocabulary() *facts.Registry {
	if fq.Vocabulary != nil {
		return fq.Vocabulary
	}
	return facts.DefaultRegistry()
}

// baseKey keys the fact base cached on a model, one per vocabulary.
type baseKey struct{ reg *facts.Registry }

// sharedBase returns the model's fact base over reg, built once per model and shared by every
// query-backed rule that runs over it (agni issue 810). Building it projects the whole design into
// tuples. That took 64ms on a 1123-component board, and each of 34 rules spent that time to run a
// query that usually matched nothing. Sharing is safe because jaala keeps a query's derived
// relations on a per-query copy of the Base, never on the shared one.
func sharedBase(reg *facts.Registry, m check.Model) *Base {
	return check.Memo(m, baseKey{reg}, func() any { return NewBaseFrom(reg, m) }).(*Base)
}

// buildRule is RuleFromQuery's construction half, after validation has passed.
func buildRule(fq FindingQuery) *check.Rule {
	q := fq.Query
	r := fq.Rule
	if len(r.Reads) == 0 {
		r.Reads = ReadsFrom(fq.vocabulary(), q)
	}
	if fq.bindsScope() {
		r.Scoping = check.ScopeSubjects
	}
	// findingFor projects one answer row onto the violation it reports. Both eval shapes below
	// share it, so a rule's finding cannot depend on whether its author declared a domain.
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
	// A datalog goal yields the rows that MATCHED. Without a declared Domain, the answer leaves out
	// every subject that did not fail, so the only accurate report is failures-only.
	if fq.Domain == nil {
		r.Eval = check.FailuresOnly(func(ctx context.Context, m check.Model) []check.Finding {
			opts, none := fq.scopeOptions(ctx)
			if none {
				return []check.Finding{}
			}
			rows, err := Default.Eval(ctx, q, sharedBase(fq.vocabulary(), m), opts...)
			if err != nil {
				// Construction validated this query, so this is the ENGINE failing on a design. An
				// inconclusive finding says the rule could not decide, while nil would read as a
				// clean pass (agni issue 540).
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
	// The failing verdicts are built here rather than through check.FailuresOnly. That adapter sees
	// only the Finding, whose subject is singular, and the subject tuple has to come from the ROW.
	r.Eval = func(ctx context.Context, m check.Model) []check.Verdict {
		opts, none := fq.scopeOptions(ctx)
		if none {
			return nil
		}
		base := sharedBase(fq.vocabulary(), m)
		var vs []check.Verdict
		failed := map[string]bool{}
		rows, err := Default.Eval(ctx, q, base, opts...)
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
			// The FAILING verdict gets a witness too, which check.FailuresOnly omits. Here the
			// answer row IS the proof, and the message states it. Verdict.Witness is REQUIRED on
			// Pass and Fail. The wire Verdict holds no Finding, since a defect travels once, in the
			// findings array. So without a witness, a failing verdict read back from the service
			// renders an empty sentence.
			vs = append(vs, check.Verdict{
				Outcome:  outcome,
				Subjects: subjects,
				Context:  f.Context,
				Witness:  &check.Witness{Statement: f.Message},
				Finding:  &f,
			})
		}
		drows, err := Default.Eval(ctx, fq.Domain.Query, base, opts...)
		if err != nil {
			// Keep the findings. A defect must never disappear because the coverage half failed, even
			// though the rule then reports fewer passes than it examined.
			return vs
		}
		// evidence maps each subject tuple to the entities that prove its pass. It is evaluated
		// once for the whole rule, since evaluating per subject would re-derive the entire program
		// for each row. An evaluation failure means no evidence rather than an error, so a pass
		// loses its chips and is still a pass.
		evidence := map[string][]check.ContextSubject{}
		if fq.Domain.Evidence != nil {
			if erows, err := Default.Eval(ctx, *fq.Domain.Evidence, base, opts...); err == nil {
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

// contextOf projects a row's declared ContextVars as context entities, in the author's order. That
// is also the order in which the message names them.
//
// A context var that did not bind in this row contributes nothing rather than an empty chip. An
// unbound one means the rule was mis-declared, and a blank chip would hide that.
//
// The finding path and the evidence path share it, so the two cannot drift about what a context
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

// tuple builds the verdict's subject tuple from one answer row. The tuple is the primary subject,
// then each declared TupleVar in the author's order. An unbound primary subject yields no tuple at
// all, since a verdict with no subject has no id and could not be addressed.
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

// bindsScope reports whether a scope can be bound into the rule's goals (agni issue 897). That
// needs the subject variable to be the only element of the verdict's tuple that a scope can name. A
// tuple element of another kind names nothing a scope holds, so it does not get in the way. One
// example is the required signal that a profile rule pairs with its host. A second net or part in
// the tuple does get in the way. A scope naming it reaches verdicts that the subject variable's
// binding would leave out. So that rule runs whole, and the run filters it.
func (fq FindingQuery) bindsScope() bool {
	for _, tv := range fq.TupleVars {
		switch tv.Kind {
		case check.KindNet, check.KindComponent, check.KindPin:
			return false
		}
	}
	return true
}

// scopeOptions returns the evaluation options for one run. They hold the work budget, plus the
// scope's subjects bound to SubjectVar when the rule can bind them. A pin-subject rule binds the
// parts the scope reaches. When the scope names pins alone, it also binds the pins' designators to
// PinVar. none reports that the scope holds nothing of the rule's kind. Such a run needs no
// evaluation, since it would keep none of its rows.
func (fq FindingQuery) scopeOptions(ctx context.Context) (opts []datalog.Option, none bool) {
	opts = EvalOptions(ctx)
	s := check.SubjectScope(ctx)
	if s == nil || !fq.bindsScope() {
		return opts, false
	}
	var names []string
	if fq.Kind == check.KindNet {
		names = s.NetNames()
	} else {
		names = s.RefDes()
	}
	if len(names) == 0 {
		return nil, true
	}
	opts = append(opts, BindEach(fq.SubjectVar, texts(names)))
	if fq.Kind == check.KindPin && fq.PinVar != "" {
		// Asked about U7.3 alone, the rule evaluates that pin rather than every pin of U7. Two pins
		// of two parts bind both designators on both parts. That asks about a few more pins than
		// requested, and the run narrows them.
		if pins, ok := s.PinDesignators(); ok {
			opts = append(opts, BindEach(fq.PinVar, texts(pins)))
		}
	}
	return opts, false
}

func texts(ss []string) []Value {
	out := make([]Value, len(ss))
	for i, s := range ss {
		out[i] = Text(s)
	}
	return out
}

// tupleKey names a subject tuple for set membership. It reuses VerdictID's escaping rather than
// joining refs with a separator. A net name passes through from a source file and may contain
// anything, including whatever separator looked safe.
func tupleKey(es []check.Entity) string {
	return check.VerdictID(check.Verdict{Subjects: es})
}

// interpolate replaces each {var} in the template with that variable's bound value in the row, as
// Display shows it. An unbound {var} renders empty. The row could not have been projected without
// it, so this case is only a safeguard.
func interpolate(tmpl string, row Row) string {
	return placeholderRe.ReplaceAllStringFunc(tmpl, func(m string) string {
		return Display(row.Bind[Var(m[1:len(m)-1])])
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
