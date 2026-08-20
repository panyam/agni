package check

import (
	"fmt"
	"math"
	"strings"
)

// PROTOTYPE (stage 1 of the proof-on-pass work). Nothing serializes this yet and no proto mirrors
// it, deliberately: the shape is being discovered against two real rules before it is committed to
// a wire form. The `ruledef.proto` header records what happens otherwise, a schema validated only by
// round-tripping its own producer, which proves it is faithful and cannot tell us it is convenient.
//
// WHAT THIS IS FOR. `Eval func(Model) []Finding` returns violations, so a pass is the ABSENCE of a
// finding and there is nowhere to record what the rule looked at. That is fine for "what is wrong
// with this board" and useless for "prove this pin is fine", which is the question a reviewer asks
// of a design they are being asked to sign off.
//
// The engine already defends a pass at the RULE level: Reads and OptionalReads gate on fact tiers,
// RequiresCapability on what a source format can express, ParamSymbols on whether a symbol was
// seeded at all. Each exists because its absence produced a silent false pass. A Verdict is the same
// argument one level down, at the individual subject and the individual datasheet row.
type Verdict struct {
	Rule    string
	Outcome Outcome
	Kind    string // KindNet | KindComponent | KindPin
	Subject string // net name (KindNet) or ref_des (KindComponent | KindPin)
	Pin     string // pin designator, set only when Kind == KindPin
	// NetID is the per-instance net identity (ir.Net.id) for a net subject, so two nets sharing a
	// name are distinguishable, matching Finding.NetID. Empty for any other kind.
	NetID string

	// Witness is what the outcome rests on. It is REQUIRED on Pass and Fail and is what makes a
	// pass evidence rather than silence. Nil is legitimate only on NoLimit and NotConsidered, where
	// the point of the verdict is that there was nothing to rest on.
	Witness *Witness

	// Reason says why a NotConsidered verdict could not be decided, in the rule author's words
	// ("pin could not be resolved to a datasheet terminal"). Empty for every other outcome.
	//
	// An open string rather than an enum, for the reason Rule.Tags and ContextSubject.Role are open:
	// the useful vocabulary is rule-specific, and a closed set defined in this package would either
	// collapse distinctions the rule depends on or grow a member per rule family. What the engine
	// requires is that the reason EXISTS, not that it comes from a list this package knows.
	Reason string

	// Finding is the existing violation form, set only when Outcome is Fail. It is carried rather
	// than replaced so the `check` path keeps its exact current output while the two rules below
	// grow a second projection (see VerdictsToFindings).
	Finding *Finding
}

// Outcome is what a rule concluded about ONE subject, which is a narrower question than the review
// layer's per-ITEM outcome vocabulary and deliberately does not reuse its spellings. A review
// outcome answers "did we get an answer to this question", mostly from preconditions decided around
// the rule. This answers "what did the rule conclude about this thing", decided inside it.
type Outcome string

const (
	// Pass: the comparison was made and the design is on the right side of it.
	Pass Outcome = "pass"
	// Fail: the comparison was made and the design is on the wrong side of it.
	Fail Outcome = "fail"
	// NotConsidered: the rule applied to this subject and never reached a comparison, with Reason
	// naming the step that stopped it.
	//
	// This is what makes a considered set honest, and it is why the verdict list IS the considered
	// set rather than something computed beside it. An enumerator that drops a subject silently
	// reports the same nothing as a rule that never looked, so a report built from the survivors
	// claims coverage it does not have. Under an addressable model it is worse: a dropped subject
	// answers 404, which reads as "no such pin" when the truth is "this pin exists and the rule
	// could not judge it".
	//
	// Distinct from NoLimit, which is a subject that DID reach the comparison and found the row
	// stating no bound. Here there was no comparison to reach.
	NotConsidered Outcome = "not-considered"
	// NoLimit: there was no bound to compare against, so nothing was checked.
	//
	// THIS IS THE ONE THAT DID NOT EXIST. Before this type, a datasheet row stating no maximum and
	// a design sitting comfortably under a stated maximum took the same silent `return` out of a
	// rule, which is the false-pass shape the rule-level gates spend four separate mechanisms
	// preventing. It is distinct from Finding.Inconclusive, which is reserved for a rule that had
	// everything it needed and still could not conclude. Here a specific input is simply absent.
	NoLimit Outcome = "no-limit"
)

// Witness is why a verdict holds: a one-line statement a person can read, the facts that statement
// rests on, and the datasheet provenance behind them.
//
// The Terms list is ordered and open rather than a fixed measured/limit pair, because the next
// witness-producing family is not a comparison at all. The protection and pull-up rules resolve
// through `reaches`, and their proof is a PATH ("SCL -> R7 -> +3V3"), which is this same list with
// the hops as terms. Fixing the shape to a comparison now would mean rewriting it at stage 2.
type Witness struct {
	// Statement is the human rendering, always set. It is the whole witness for a text consumer.
	Statement string
	// Terms are the facts Statement rests on, kept separately so a UI can lay them out and a test
	// can assert on a value without parsing prose.
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
// It exists to pick the BINDING row when one terminal carries several of the same kind. A datasheet
// can state more than one limit of a kind for one pin (different conditions), and the constraint
// that governs is the one the design is closest to violating, never the first one enumerated.
// Selecting by smallest margin does that in a single comparison for every bound shape: a violated
// row has a negative margin so it beats any passing row, and an unstated bound has no margin at all
// so it loses to any real limit.
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
// from one call.
//
// The one-call shape is the point rather than a convenience. If a rule decided the outcome and then
// separately assembled a witness, nothing would fail when the second step was forgotten, and a pass
// with no evidence is exactly what this work exists to remove. Producing both together makes the
// evidence structural: there is no way to reach a Pass without the statement that justifies it.
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

// VerdictsToFindings projects a verdict list down to the findings the `check` path already reports,
// so a rule can produce verdicts as its single source of truth and still return exactly what
// Eval has always returned. Only Fail carries a finding, which is the definition of the current
// contract: a pass and an unchecked row are both silence to `check`.
func VerdictsToFindings(vs []Verdict) []Finding {
	// NON-NIL on empty, matching Report, which is the constructor every hand-written Eval already
	// returns through. TestSpecParity compares a rule's Eval against its declarative twin with
	// reflect.DeepEqual, and a nil slice is not DeepEqual to an empty one, so a rule converted to
	// verdicts would diverge from its twin on any design with nothing wrong. That is the parity
	// break that reads as "the twin disagrees" when the two agree about every finding.
	out := []Finding{}
	for _, v := range vs {
		if v.Outcome == Fail && v.Finding != nil {
			out = append(out, *v.Finding)
		}
	}
	return out
}

// Render writes a witness the way a terminal shows it: the statement, then its terms, then any
// citation. It exists so the text form is defined in one place rather than per consumer.
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

// fmtQty renders a value with its unit, trimming the trailing zeros %g already handles so a limit
// reads as "3.6 V" rather than "3.600000 V".
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
