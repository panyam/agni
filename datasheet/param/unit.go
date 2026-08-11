package param

import (
	"math"

	"google.golang.org/protobuf/proto"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// Unit conversion for seeded datasheet parameters (agni issue 148).
//
// A PartSpec stores every row AS PRINTED, which is a provenance property rather than a stylistic one:
// a fixture that says 800 mA can be checked against the datasheet page by eye, and a reviewer
// verifying a citation reads the same number the vendor printed. The design side stores the opposite
// convention, because ir.Quantity is normalized to its SI base unit once at ingestion and keeps the
// source text in `input` so the normalization stays non-lossy.
//
// Both halves of every datasheet comparison were therefore stored under contradictory rules, with
// nothing bridging them. The extractors gated on the printed unit string, so a row in a prefixed unit
// was dropped from the slice they returned and the rule concluded from an empty list: a milliamp
// output rating read as no rating at all, and the rail scored a PASS.
//
// This file is the bridge, and it mirrors ir.Quantity's split rather than changing how specs are
// stored. The spec keeps the printed row, playing `input`'s role. An extractor returns a converted
// copy, playing `value`'s role. Nothing that displays a parameter (the params panel, the `param`
// relations, a citation) sees a rewritten number, and nothing that COMPARES one sees a prefixed unit.
//
// THE SCALE LIVES HERE AND NOWHERE ELSE. The standing objection to converting was that a silent scale
// factor inside a pass/fail rule is where a unit bug hides, and that objection is right. It is an
// argument about location, not about refusal: one table under the comparison contract, next to
// UnderSpecified and MachineComparable, is auditable in a way that ten hand-written unit gates were
// not. No rule contains a number.
//
// WHY THIS TABLE IS NOT core/classify's. That package parses a component's value TEXT ("10k", "4R7")
// and already owns a prefix table, so sharing looks free. It is not. IEC 60062's RKM code reads M as
// MEGA and is case-insensitive on k and u, which is correct for a schematic value field and inverts
// three orders of magnitude on a printed unit symbol, where mV is unambiguously milli and case is
// normative. Reusing it would import a decision that is right there and wrong here. The two agree on
// the base-unit vocabulary, which is the part that could genuinely drift, so a tripwire test in
// core/check (the one package that imports both layers) holds them to each other.

// UnitOhm is the canonical ohm symbol a converted parameter carries: GREEK CAPITAL LETTER OMEGA
// (U+03A9). It is the same symbol ir.Quantity documents as canonical, deliberately, so a rule
// comparing a design-side resistance against a datasheet-side one compares two identical strings.
// Declared here rather than imported from core/classify because the datasheet tier imports nothing
// from core (C17), and one duplicated constant is a smaller cost than that edge.
const UnitOhm = "Ω" // Ω

// ohmSign is the deprecated OHM SIGN codepoint (U+2126), visually identical to U+03A9. Unicode
// normalizes it away, but a transcribed spec carries the raw bytes and nothing here normalizes first.
const ohmSign = "Ω" // Ω (deprecated codepoint)

// prefixableUnits maps every spelling of a base unit a datasheet may print to its canonical symbol.
// These are the units an SI multiplier may be attached to, so the flat lookup is their cross product
// with siPrefixes.
//
// The five bases ir.Quantity names (Ω F H A V) are the ones a shipped extractor reads today. W, s and
// Hz are here because a parameter layer that knows about volts and not about seconds would send the
// next timing row down exactly the path this file exists to close, and a base unit costs one row.
var prefixableUnits = map[string]string{
	"V":     "V",
	"A":     "A",
	"F":     "F",
	"H":     "H",
	"W":     "W",
	"s":     "s",
	"Hz":    "Hz",
	UnitOhm: UnitOhm, ohmSign: UnitOhm,
	"Ohm": UnitOhm, "ohm": UnitOhm, "Ohms": UnitOhm, "ohms": UnitOhm,
}

// unprefixedUnits are the units that carry NO multiplier, so no prefixed form of them is recognized.
// Temperature is the one that matters: a datasheet states a junction limit in degrees Celsius and
// never in millidegrees, so admitting "mC" would only ever match a typo and scale it by a thousand.
var unprefixedUnits = map[string]string{
	"C": "C", "°C": "C", // °C
}

// siPrefixes maps a multiplier symbol to its power of ten.
//
// CASE IS NORMATIVE AND THERE IS NO FALLBACK. SI writes milli lowercase and mega uppercase, so mΩ and
// MΩ differ by nine orders of magnitude and mHz and MHz by the same. A case-insensitive lookup, or a
// "try the other case" retry, would resolve that ambiguity by guessing, and guessing wrong here
// produces a confidently wrong number rather than a skip. A spelling this table does not hold is
// refused, which is the direction the whole params layer fails in.
//
// Micro appears under both codepoints designs and datasheets actually carry: MICRO SIGN (U+00B5) and
// GREEK SMALL LETTER MU (U+03BC). No ASCII "u", which would collide with nothing but is not a unit
// symbol a datasheet prints.
//
// Kilo is lowercase k only. Uppercase K is kelvin, and a table that accepted it as kilo would read a
// temperature row as a thousand of something.
var siPrefixes = map[string]int{
	"p": -12,
	"n": -9,
	"µ": -6, "μ": -6, // µ, μ
	"m": -3,
	"k": 3,
	"M": 6,
	"G": 9,
	"T": 12,
}

// unitScales is the flat spelling -> (base, exponent) lookup, built once from the three tables above.
// Flattening at init rather than parsing a prefix off the front at call time is what makes the
// vocabulary CLOSED: every string this package will ever accept exists as a key, so a test can
// enumerate them and a reviewer can print them, and no unforeseen spelling resolves by accident.
var unitScales = func() map[string]unitScale {
	out := make(map[string]unitScale, len(prefixableUnits)*(len(siPrefixes)+1)+len(unprefixedUnits))
	for spelling, base := range prefixableUnits {
		out[spelling] = unitScale{base, 0}
		for prefix, exp := range siPrefixes {
			out[prefix+spelling] = unitScale{base, exp}
		}
	}
	for spelling, base := range unprefixedUnits {
		out[spelling] = unitScale{base, 0}
	}
	return out
}()

// unitScale is a printed unit's canonical base symbol and the decimal exponent between them.
type unitScale struct {
	base string
	exp  int
}

// BaseUnit reports the SI base unit a printed unit symbol reduces to, and the decimal exponent from
// the printed unit to that base ("mV" -> "V", -3; "kΩ" -> "Ω", 3; "V" -> "V", 0).
//
// ok is false for a unit this layer does not recognize, INCLUDING the empty one. An unstated unit is
// not evidence that a number is in the unit a caller happens to want, which is the same posture
// check.ComponentValueIn takes on a bare component value. A caller must treat false as "not
// comparable" and skip, never as "assume base".
func BaseUnit(unit string) (base string, exp int, ok bool) {
	s, hit := unitScales[unit]
	if !hit {
		return "", 0, false
	}
	return s.base, s.exp, true
}

// InBaseUnit returns p expressed in the SI base unit of whatever unit it was printed in, so a caller
// may compare its bounds against a number of any other provenance. ok is false when p is nil or its
// unit is not one BaseUnit recognizes, and a caller must then skip the row rather than compare it.
//
// The RETURNED ROW IS ALWAYS IN THE BASE UNIT, which is the contract that makes this safe: an
// extractor that filters on the returned Unit cannot admit a prefixed row, so the wrong-pass this
// closes cannot be reintroduced by forgetting a conversion at a call site. There is nothing to
// forget.
//
// p is returned UNCHANGED, same pointer, when it is already in the canonical base spelling. That is
// the common case (a corpus states most rows in base units), so the ordinary path allocates nothing,
// and a resolver that stores the returned row alongside the spec it came from keeps pointer identity
// with it. Otherwise the result is a deep copy with Unit and every present bound rewritten; p itself
// is never mutated, because the spec is shared across every rule in a run and the printed row is what
// a citation and the params panel must keep showing.
//
// Conditions are deliberately NOT converted. A condition's unit qualifies the row rather than
// carrying its value, MachineComparable only requires that conditions be structured rather than
// evaluated, and nothing compares against one today. Scaling a number no consumer reads would be the
// speculative half of this change.
func InBaseUnit(p *parampb.Parameter) (*parampb.Parameter, bool) {
	if p == nil {
		return nil, false
	}
	base, exp, ok := BaseUnit(p.GetUnit())
	if !ok {
		return nil, false
	}
	if exp == 0 && base == p.GetUnit() {
		return p, true
	}
	q := proto.Clone(p).(*parampb.Parameter)
	q.Unit = base
	if v := q.GetValue(); v != nil {
		v.Min, v.Typ, v.Max = scaleBound(v.Min, exp), scaleBound(v.Typ, exp), scaleBound(v.Max, exp)
	}
	return q, true
}

// scaleBound applies the exponent to one optional bound, leaving absent absent. An absent bound is a
// real state (a row stating only a max), so it must survive the conversion as absent rather than
// arriving downstream as a scaled zero.
func scaleBound(v *float64, exp int) *float64 {
	if v == nil || exp == 0 {
		return v
	}
	s := scalePow10(*v, exp)
	return &s
}

// scalePow10 multiplies v by ten to the exp, DIVIDING on the negative branch rather than multiplying
// by a negative power. The distinction is not cosmetic and is the same one core/classify's value
// parser documents: a positive power of ten is exactly representable in a double, a negative one is
// not, so 50 / 1e3 and 50 * 1e-3 land on different doubles. Dividing means a row transcribed as
// 50 mV and the same row transcribed as 0.05 V compare EQUAL, which is what lets a diff and a
// threshold comparison agree about a value that was merely respelled.
func scalePow10(v float64, exp int) float64 {
	if exp > 0 {
		return v * math.Pow10(exp)
	}
	return v / math.Pow10(-exp)
}
