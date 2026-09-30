package check

import (
	"fmt"
	"math"
	"strings"
)

// Verdict is what one rule concluded about one subject, passes included, with the evidence the
// conclusion rests on. A finding records only a violation, so a pass is the absence of one; a
// Verdict is how a rule proves a pin is fine rather than merely not reporting it (#387). Its wire
// form is checks.Verdict in checks.proto. See
// docsite/content/build/check-rule.md#say-what-you-looked-at-not-only-what-failed.
type Verdict struct {
	Rule    string
	Outcome Outcome
	// Subjects is the tuple of entities this verdict is ABOUT, in the rule's own order, and it is the
	// verdict's IDENTITY. Never empty.
	//
	// A tuple because a rule about a RELATION (copper-clearance between two nets, a regulator feeding
	// a load across a rail) needs every entity in it to give each answer its own VerdictID (#404).
	// ORDER IS SIGNIFICANT (pin-tracking bounds subject-pin minus reference-pin), so the framework
	// never sorts and a symmetric relation canonicalises INSIDE the rule. ARITY IS FIXED PER RULE,
	// declared as Rule.SubjectShape and held by TestSubjectShapeHolds. See
	// docsite/content/build/check-rule.md#a-tuple-in-the-verdict-one-entity-in-the-finding.
	Subjects []Entity

	// Witness is what the outcome rests on. REQUIRED on Pass and Fail, since without it a pass is
	// silence. Nil only on NoLimit and NotConsidered, where there was nothing to rest on.
	Witness *Witness

	// Reason says why a NotConsidered verdict could not be decided, in the rule author's words
	// ("pin could not be resolved to a datasheet terminal"). Empty for every other outcome.
	//
	// An open string, like Rule.Tags and ContextSubject.Role, because the useful vocabulary is
	// rule-specific. The engine requires only that the reason EXISTS.
	Reason string

	// Context are the design entities this verdict's proof NAMES but is not ABOUT, typed so a
	// consumer can highlight them: the resistor and rail a pull-up passes through, the rail a pin
	// sits on. Ordered, and Role is the author's word for the part each plays.
	//
	// The split from Witness.Terms is whether clicking it should light something up. A Term's value
	// is a bare string ("R1" could be a component, net or pin), where a ContextSubject carries the
	// Kind a highlight joins on (#388).
	//
	// Excludes the entities in Subjects, so a consumer draws subject-as-figure and Context-as-ground,
	// the split focusStack implements in the web viewer.
	//
	// Finding.Context is the projection of this for a failing verdict, and can differ. A Finding
	// about a COMPONENT lists the pin in its context, where a pin-subject verdict does not, because
	// for the verdict the pin is the subject.
	Context []ContextSubject

	// Finding is the violation form, set on Fail and Inconclusive. VerdictsToFindings projects it
	// out so the `check` path keeps its existing output.
	Finding *Finding
}

// Outcome is what a rule concluded about ONE subject. It is narrower than the review layer's
// per-ITEM outcome and does not reuse its spellings. A review outcome answers "did we get an answer
// to this question", mostly from preconditions around the rule, and this answers "what did the rule
// conclude about this thing", decided inside it. The five outcomes are tabled in
// docsite/content/build/check-rule.md#five-outcomes-and-the-three-that-are-not-a-pass.
type Outcome string

const (
	// Pass: the comparison was made and the design is on the right side of it.
	Pass Outcome = "pass"
	// Fail: the comparison was made and the design is on the wrong side of it.
	Fail Outcome = "fail"
	// Inconclusive: the rule had every input and reached its decision, but the design cannot
	// discriminate between the cases (a power-path transistor may be an ideal-diode controller or a
	// plain switch, and a netlist cannot tell which).
	//
	// The outcome form of Finding.Inconclusive, so a consumer must NOT count it as a failure. Never
	// map it to NotConsidered, which produces no finding and would delete one the check path reports.
	Inconclusive Outcome = "inconclusive"
	// NotConsidered: the rule applied to this subject and never reached a comparison, with Reason
	// naming the step that stopped it.
	//
	// The verdict list IS the considered set, so an enumerator must emit this rather than drop a
	// subject. A dropped subject reports the same nothing as a rule that never looked, and under an
	// addressable id it answers 404, which reads as "no such pin" rather than "this rule could not
	// judge it".
	NotConsidered Outcome = "not-considered"
	// NoLimit: the subject reached the comparison and the datasheet row stated no bound, so nothing
	// was checked. Without it, a row stating no maximum and a design under a stated maximum take the
	// same silent return out of a rule (#387). CompareToBound returns it for an unstated Bound.
	NoLimit Outcome = "no-limit"
)

// Witness is why a verdict holds: a one-line statement a person can read, the facts that statement
// rests on, and the datasheet provenance behind them.
//
// Terms is an ordered open list rather than a measured/limit pair because a path proof from
// `reaches` ("SCL -> R7 -> +3V3") uses the same list with the hops as terms.
type Witness struct {
	// Statement is the human rendering, always set. It is the whole witness for a text consumer.
	Statement string
	// Terms are the VALUES Statement rests on, kept separately so a UI can lay them out and a test
	// can assert on one without parsing prose: a measured voltage, a stated limit, a hop bound.
	//
	// Values only, never entities, which go in Verdict.Context with a Kind. So a witness whose proof
	// is entirely a path has NO terms, and that is correct.
	Terms []WitnessTerm
	// Datasheet is the provenance of any seeded value the verdict used, the same citation form a
	// Finding carries. Empty for a witness resting on nothing seeded.
	Datasheet []*DatasheetCitation
}

// WitnessTerm is one labelled fact inside a witness.
type WitnessTerm struct {
	Label string // "measured", "absolute maximum", "hop 1"
	Value string // "3.3 V", "3.6 V", "R7"
}

// Bound is an optional two-sided limit, matching what a datasheet row can state: a maximum only, a
// minimum only, both, or neither. Neither is the case that produces NoLimit.
type Bound struct {
	Min *float64
	Max *float64
}

// Stated reports whether the bound constrains anything at all.
func (b Bound) Stated() bool { return b.Min != nil || b.Max != nil }

// Margin reports how much room a measured value has before it crosses the bound, in the bound's own
// unit. Negative means the bound is already violated and the magnitude says by how much. An unstated
// bound returns +Inf, since nothing constrains the value.
//
// Use it to pick the BINDING row when a datasheet states several limits of one kind for a pin (under
// different conditions). The smallest margin governs, not the first row enumerated. That holds for
// every bound shape, since a violated row goes negative and beats any passing row, and an unstated
// bound loses to any real limit.
func (b Bound) Margin(measured float64) float64 {
	m := math.Inf(1)
	if b.Max != nil {
		m = math.Min(m, *b.Max-measured)
	}
	if b.Min != nil {
		m = math.Min(m, measured-*b.Min)
	}
	return m
}

// CompareToBound is the single comparison a limit rule makes, returning the outcome AND the witness
// from one call so a rule cannot reach a Pass without the statement that justifies it. An unstated
// bound returns NoLimit and a nil witness.
//
// quantity names what is being measured for the human statement ("nominal"), limitName names the
// bound as the datasheet spells it ("absolute maximum", "recommended range").
func CompareToBound(measured float64, unit string, b Bound, quantity, limitName string) (Outcome, *Witness) {
	if !b.Stated() {
		return NoLimit, nil
	}

	terms := []WitnessTerm{{Label: quantity, Value: fmtQty(measured, unit)}}
	switch {
	case b.Min != nil && b.Max != nil:
		terms = append(terms, WitnessTerm{Label: limitName, Value: fmtRange(*b.Min, *b.Max, unit)})
	case b.Max != nil:
		terms = append(terms, WitnessTerm{Label: limitName, Value: fmtQty(*b.Max, unit)})
	default:
		terms = append(terms, WitnessTerm{Label: limitName, Value: fmtQty(*b.Min, unit)})
	}

	w := &Witness{Terms: terms}
	switch {
	case b.Max != nil && measured > *b.Max:
		w.Statement = fmt.Sprintf("%s exceeds the %s of %s",
			fmtQty(measured, unit), limitName, fmtQty(*b.Max, unit))
		return Fail, w
	case b.Min != nil && measured < *b.Min:
		w.Statement = fmt.Sprintf("%s is below the %s of %s",
			fmtQty(measured, unit), limitName, fmtQty(*b.Min, unit))
		return Fail, w
	}

	switch {
	case b.Min != nil && b.Max != nil:
		w.Statement = fmt.Sprintf("%s is inside the %s %s",
			fmtQty(measured, unit), limitName, fmtRange(*b.Min, *b.Max, unit))
	case b.Max != nil:
		w.Statement = fmt.Sprintf("%s is within the %s of %s",
			fmtQty(measured, unit), limitName, fmtQty(*b.Max, unit))
	default:
		w.Statement = fmt.Sprintf("%s is at or above the %s of %s",
			fmtQty(measured, unit), limitName, fmtQty(*b.Min, unit))
	}
	return Pass, w
}

// VerdictID is a verdict's stable name, `<rule>:(<kind>:<ref>,...)`, DERIVED from the verdict so a
// CLI run and a server run compute the same name without persisting anything.
//
// Built from Rule, Kind and the kind's own reference, and NOT the outcome, so a link filed last month
// still resolves after the answer flips. Each kind owns its REF grammar (EntityRef) rather than the
// id being a positional tuple of checks.Subject's fields, so adding a kind leaves existing ids
// untouched.
//
// Readable, not hashed, so a person can construct `pin-exceeds-abs-max:(pin:U12.7)` and ask about a
// terminal without running check first.
//
// GENERATED, NEVER PARSED. The structure travels typed in Subjects, which is why a ref may keep its
// own colons (`symbol:Library:Symbol`) and commas (an endpoint's `0,0`) behind encodeRef alone.
//
// KNOWN LIMIT: two nets sharing a name share an id, because a NetID would make the id impossible to
// type. The wire Subject joins by name too, and duplicate net names are themselves a reported defect.
func VerdictID(v Verdict) string {
	parts := make([]string, 0, len(v.Subjects))
	for _, e := range v.Subjects {
		parts = append(parts, e.Kind+":"+encodeRef(EntityRef(e)))
	}
	return v.Rule + ":(" + strings.Join(parts, ",") + ")"
}

// EntityRef is the kind-owned half of one element's name. A pin joins its ref-des and designator
// with a dot, which is already this codebase's pin-key spelling (a KiCad pad key is built as
// RefDes+"."+Number).
func EntityRef(e Entity) string {
	if e.Kind == KindPin && e.Pin != "" {
		return e.Ref + "." + e.Pin
	}
	return e.Ref
}

// encodeRef percent-escapes the four characters the tuple syntax uses (% , ( )), so two distinct
// tuples never produce the same id. KindEndpoint's ref is literally "0,0", and a net name can hold
// anything, so without it ("A,B") and ("A", "B") collide.
//
// The colon is NOT escaped. Kind is a closed vocabulary with no colons, so "kind:ref" stays
// unambiguous and KindSymbol's Library:Symbol stays readable in an id a person might type.
func encodeRef(s string) string {
	if !strings.ContainsAny(s, "%,()") {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '%':
			b.WriteString("%25")
		case ',':
			b.WriteString("%2C")
		case '(':
			b.WriteString("%28")
		case ')':
			b.WriteString("%29")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// SubjectRefs joins a verdict's subject refs for the callers that ORDER or DISPLAY a tuple as one
// string (the report's row sort, the CLI table). The kinds are left out because those callers are
// arranging rather than naming; VerdictID is the name.
func SubjectRefs(v Verdict) string {
	parts := make([]string, 0, len(v.Subjects))
	for _, e := range v.Subjects {
		parts = append(parts, EntityRef(e))
	}
	return strings.Join(parts, ",")
}

// VerdictsToFindings projects a verdict list down to the findings the `check` path reports, which is
// how Rule.Findings and a spec derive findings from verdicts. Fail and Inconclusive carry a finding;
// Pass, NotConsidered and NoLimit are silence to `check`.
func VerdictsToFindings(vs []Verdict) []Finding {
	// NON-NIL on empty, matching Report. TestSpecParity compares with reflect.DeepEqual, where nil is
	// not equal to an empty slice, so a nil here breaks parity with a spec twin on every clean design.
	out := []Finding{}
	for _, v := range vs {
		// Inconclusive is not a defect (its Finding says so in its own Inconclusive flag), but it must
		// not be silent, or a review item bound to the rule reads the silence as a pass.
		if (v.Outcome == Fail || v.Outcome == Inconclusive) && v.Finding != nil {
			out = append(out, *v.Finding)
		}
	}
	return out
}

// Render writes a witness the way a terminal shows it, the statement and then one line per
// datasheet citation. A nil witness renders as "".
func (w *Witness) Render() string {
	if w == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(w.Statement)
	for _, c := range w.Datasheet {
		if c == nil {
			continue
		}
		b.WriteString("\n  ")
		b.WriteString(citationLine(c))
	}
	return b.String()
}

func citationLine(c *DatasheetCitation) string {
	doc := c.Doc
	if doc == "" {
		doc = "unknown source"
	}
	s := doc
	if c.Page > 0 {
		s += fmt.Sprintf(", p.%d", c.Page)
	}
	if c.Section != "" {
		s += ", " + c.Section
	}
	return s
}

// fmtQty renders a value with its unit via %g, so a limit reads as "3.6 V" rather than "3.600000 V".
func fmtQty(v float64, unit string) string {
	if unit == "" {
		return fmt.Sprintf("%g", v)
	}
	return fmt.Sprintf("%g %s", v, unit)
}

func fmtRange(lo, hi float64, unit string) string {
	if unit == "" {
		return fmt.Sprintf("%g to %g", lo, hi)
	}
	return fmt.Sprintf("%g to %g %s", lo, hi, unit)
}
