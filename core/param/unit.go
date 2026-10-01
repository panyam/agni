package param

import (
	"math"

	"google.golang.org/protobuf/proto"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// Unit conversion for seeded datasheet parameters (agni issue 148).
//
// A PartSpec stores every row AS PRINTED, so a fixture can be checked against its datasheet page by
// eye, while the design side's ir.Quantity is normalized to its SI base unit. An extractor reads
// through InBaseUnit and gets a converted copy, so nothing that displays a parameter sees a rewritten
// number and nothing that COMPARES one sees a prefixed unit. An extractor that gates on the printed
// unit string instead drops every prefixed row, and its rule passes over an empty list.
//
// THE SCALE LIVES HERE AND NOWHERE ELSE. No rule contains a number.
//
// This table is not core/classify's. That one parses value text by IEC 60062's RKM code, which reads
// M as MEGA and ignores case, and would invert three orders of magnitude on a printed unit symbol.
// TestUnitVocabulariesAgree in core/check holds the two base-unit vocabularies to each other. See
// docsite/content/architecture/datasheet-layer.md#comparison-semantics.

// UnitOhm is the canonical ohm symbol a converted parameter carries: GREEK CAPITAL LETTER OMEGA
// (U+03A9). It is the same symbol ir.Quantity documents as canonical, so a rule comparing a
// design-side resistance against a datasheet-side one compares two identical strings. Declared here
// rather than imported from core/classify because the datasheet tier imports nothing from core (C17).
const UnitOhm = "Ω" // Ω

// ohmSign is the deprecated OHM SIGN codepoint (U+2126), visually identical to U+03A9. Unicode
// normalizes it away, but a transcribed spec carries the raw bytes and nothing here normalizes first.
const ohmSign = "Ω" // Ω (deprecated codepoint)

// prefixableUnits maps every spelling of a base unit a datasheet may print to its canonical symbol.
// These are the units an SI multiplier may be attached to, so the flat lookup is their cross product
// with siPrefixes.
//
// The five bases ir.Quantity names (Ω F H A V) are the ones a shipped extractor reads today. W, s and
// Hz are here so the first timing or power row converts rather than being dropped.
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
// Temperature is the one that matters, since a datasheet states a junction limit in degrees Celsius and
// never in millidegrees, so admitting "mC" would only ever match a typo and scale it by a thousand.
var unprefixedUnits = map[string]string{
	"C": "C", "°C": "C", // °C
}

// siPrefixes maps a multiplier symbol to its power of ten.
//
// CASE IS NORMATIVE AND THERE IS NO FALLBACK. SI writes milli lowercase and mega uppercase, so mΩ and
// MΩ differ by nine orders of magnitude. A case-insensitive lookup or a retry in the other case would
// guess, and a wrong guess yields a confidently wrong number rather than a skip. A spelling this table
// does not hold is refused.
//
// Micro appears under both codepoints designs and datasheets carry, MICRO SIGN (U+00B5) and GREEK
// SMALL LETTER MU (U+03BC). There is no ASCII "u", since a datasheet does not print one.
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
// Flattening at init rather than parsing a prefix at call time keeps the vocabulary CLOSED. Every
// string this package accepts is a key a test can enumerate, and no unforeseen spelling resolves by
// accident.
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
// ok is false for a unit this layer does not recognize, INCLUDING the empty one, the same posture
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
// The RETURNED ROW IS ALWAYS IN THE BASE UNIT, so an extractor that filters on the returned Unit
// cannot admit a prefixed row.
//
// p is returned UNCHANGED, same pointer, when it is already in the canonical base spelling. That is
// the common case, so the ordinary path allocates nothing and a resolver storing the returned row
// keeps pointer identity with the spec. Otherwise the result is a deep copy with Unit and every
// present bound rewritten. p itself is never mutated, because the spec is shared across every rule in
// a run and a citation and the params panel must keep showing the printed row.
//
// Conditions are NOT converted. A condition's unit qualifies the row rather than carrying its value,
// and nothing compares against one.
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

// scaleBound applies the exponent to one optional bound. An absent bound is a real state (a row
// stating only a max), so it stays absent rather than becoming a scaled zero.
func scaleBound(v *float64, exp int) *float64 {
	if v == nil || exp == 0 {
		return v
	}
	s := scalePow10(*v, exp)
	return &s
}

// scalePow10 multiplies v by ten to the exp, DIVIDING on the negative branch rather than multiplying
// by a negative power, as core/classify's value parser does. A positive power of ten is exact in a
// double and a negative one is not, so 50 / 1e3 and 50 * 1e-3 land on different doubles. Dividing
// makes a row transcribed as 50 mV and the same row as 0.05 V compare EQUAL.
func scalePow10(v float64, exp int) float64 {
	if exp > 0 {
		return v * math.Pow10(exp)
	}
	return v / math.Pow10(-exp)
}
