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
	r.Eval = func(m check.Model) []check.Finding {
		rows, err := Naive{}.Eval(q, NewBase(m))
		if err != nil {
			return nil
		}
		out := []check.Finding{}
		for _, row := range rows {
			subj := row.Bind[Var(fq.SubjectVar)].S
			f := check.Finding{
				Kind:    fq.Kind,
				Subject: subj,
				Message: interpolate(fq.Message, row),
				Prov:    provFor(m, fq.Kind, subj),
			}
			if fq.PinVar != "" {
				f.Pin = row.Bind[Var(fq.PinVar)].S
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
				f.Context = append(f.Context, check.ContextSubject{Kind: cv.Kind, Subject: ref, Role: cv.Role})
			}
			if fq.ParamSymbol != "" && fq.Kind == check.KindComponent {
				if dp := check.DatasheetProvFor(m, subj, fq.ParamSymbol); dp != nil {
					f.DatasheetProv = []*check.DatasheetCitation{dp}
				}
			}
			out = append(out, f)
		}
		return out
	}
	return &r
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
