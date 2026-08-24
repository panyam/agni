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
	// Subjects is the tuple of entities this verdict is ABOUT, in the rule's own order, and it is the
	// verdict's IDENTITY. Never empty.
	//
	// A TUPLE BECAUSE SOME RULES ASK ABOUT A RELATION, and a relation belongs to no single entity.
	// copper-clearance measures a distance between two nets. regulator-output-exceeds-abs-max compares
	// a regulator against a part it feeds ACROSS a named rail, so only all three pin the answer down:
	// one source feeding one load over two supplied rails is two different answers. A strap group is a
	// device and the N nets encoding its value. Keying those by one entity issues one id for several
	// answers, which was invisible while verdicts only projected down to findings and is wrong now
	// that they are addressable: the report links every row by VerdictID.
	//
	// ORDER IS THE RULE'S AND IS SIGNIFICANT. Some relations are directional and the direction is the
	// claim: pin-tracking bounds subject-pin minus reference-pin, so swapping them inverts the sign,
	// and regulator-output reads source then load. A symmetric relation canonicalises INSIDE the rule
	// (copper-clearance orders its pair by name) rather than leaving it to the framework, because a
	// framework that sorted would destroy the directional ones.
	//
	// ARITY IS FIXED PER RULE, declared as Rule.SubjectShape, so a consumer can index a rule's
	// verdicts and a person can construct an id without running the check first. A rule emitting a
	// 2-tuple on one design and a 3-tuple on another is a bug, and TestSubjectShapeHolds says so.
	Subjects []Entity

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

	// Context are the design entities this verdict's proof NAMES but is not ABOUT, typed so a
	// consumer can highlight them: the resistor and rail a pull-up passes through, the rail a pin
	// sits on. Ordered, and Role is the author's word for the part each plays.
	//
	// THIS IS THE HIGHLIGHTABLE HALF, and the division from Witness.Terms is exactly that. A Term is
	// a Label and a bare string, so "pull-up=R1" cannot be resolved to anything: nothing says whether
	// R1 is a component, a net or a pin. A ContextSubject carries Kind, which is what HighlightSpec
	// joins on. The test is whether clicking it should light something up.
	//
	// Excludes the subject, which is already named by Kind/Subject/Pin above, so a consumer draws
	// subject-as-figure and Context-as-ground. That is the split focusStack already implements.
	//
	// Finding.Context is the projection of this for a failing verdict, and differs legitimately: a
	// Finding about a COMPONENT lists the pin in its context, where a pin-subject verdict does not,
	// because for the verdict the pin is the subject.
	Context []ContextSubject

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
	// Inconclusive: the rule had everything it needed, REACHED its decision, and could not decide.
	//
	// Distinct from NotConsidered, where there was no decision to reach, and from NoLimit, where a
	// specific input was absent. Here the inputs are present and the DISCRIMINATION is impossible: a
	// transistor in a power path is either an ideal-diode controller providing reverse protection or
	// an ordinary switch providing none, and a netlist cannot tell them apart.
	//
	// It is the outcome form of Finding.Inconclusive, which already ships, and carries that field's
	// contract: a consumer must NOT count it as a failure. That is why it cannot simply be Fail, and
	// why mapping it to NotConsidered would be worse still, since NotConsidered reaches no output and
	// the mapping would silently delete a finding the check path reports today.
	//
	// NO RULE PRODUCES THIS YET. It is here before the wire form rather than after, because
	// reverse-blocking-absent is the rule that needs it and adding an enum member now is free where
	// adding one to a shipped schema is not.
	Inconclusive Outcome = "inconclusive"
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
	// Terms are the VALUES Statement rests on, kept separately so a UI can lay them out and a test
	// can assert on one without parsing prose: a measured voltage, a stated limit, a hop bound.
	//
	// Values only, never entities. An entity belongs in Verdict.Context, which carries the Kind a
	// highlight needs; a Term's Value is a bare string that no consumer can resolve. A witness whose
	// proof is entirely a path therefore has NO terms, and that is correct rather than a gap: the
	// facts it rests on are all things you can point at.
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

// VerdictID is a verdict's stable name, `<rule>:(<kind>:<ref>,...)`, DERIVED from the verdict rather
// than assigned to it. Nothing persists a verdict, so a CLI run and a server run have to compute the same
// name for the same verdict without talking to each other, which is the argument mount:// already won
// one level up restated one level down.
//
// WHAT IT IS BUILT FROM, and what it deliberately is not. Rule, Kind and the kind's own reference,
// and nothing else. Not run order, not the message text, and NOT the outcome. Leaving the outcome out
// is the valuable part: the same URL then addresses the same check on the same pin across revisions,
// so a link filed last month still resolves after the answer flips, and can say that it flipped.
// Include it and every flip breaks every link, exactly when the link matters most.
//
// The REF's grammar belongs to the kind rather than being a positional tuple of every kind's fields.
// checks.Subject is already a widening union (ref, pin, net_id, bus_id, and counting), and a
// positional key over those changes format every time a kind is added, invalidating every id ever
// issued. Here a seventh kind adds a grammar and leaves existing ids untouched.
//
// Readable, not hashed, because under recompute-on-demand an id is a QUESTION YOU CAN POSE and not
// merely a label you receive: someone worried about a terminal can construct
// `pin-exceeds-abs-max:pin:U12.7` without running check first to discover the name of the thing they
// wanted to ask about. A hash makes that impossible.
//
// GENERATED, NEVER PARSED. One function builds it and nothing splits it back apart, because the
// structure travels in Subjects where a consumer reads it typed. That is what lets a ref keep its own
// colons (`symbol:Library:Symbol`) and its own commas (an endpoint's `0,0`) behind one escape rule
// rather than an encoding designed around a parser nobody needs.
//
// KNOWN LIMIT: two nets sharing a name share an id, because using NetID instead would make the id
// unconstructible (nobody can type a net id). That matches how Subject already behaves on the wire,
// where a consumer joins by name, and duplicate net names are themselves a reported defect.
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

// encodeRef percent-escapes the four characters the tuple syntax uses, so two distinct tuples can
// never produce the same id.
//
// THIS IS NOT AN ACADEMIC CASE. KindEndpoint's ref is literally "0,0", a comma sitting in the
// delimiter position, and a net name is passed through from a source file that may contain anything.
// Without the escape ("A,B") and ("A", "B") are one string, and two different verdicts answer to one
// name.
//
// The colon is deliberately NOT escaped. Kind is a closed vocabulary containing none of these
// characters, so a "kind:ref" element stays unambiguous even where the ref carries colons of its own,
// and KindSymbol's real spelling (Library:Symbol) stays readable instead of becoming
// Library%3ASymbol in every id a person might type.
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
		// Fail AND Inconclusive, because both reach the check path today. An inconclusive result is
		// not a defect and must not be counted as one, which the Finding carries in its own
		// Inconclusive flag; what it must not be is silent, since a bound review item reading silence
		// as a pass is the failure reverse-blocking-absent's doc describes.
		if (v.Outcome == Fail || v.Outcome == Inconclusive) && v.Finding != nil {
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
