package classify

import (
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// ValueVocab is the CONVENTION half of reading a component's value, kept out of the parser (WS3-118).
//
// ParseQuantity implements the part of the notation that is specified: IEC 60062's RKM code and the
// SI prefixes. No specification says what unit a BARE number carries, because that depends on the
// component's class and on house habit. A bare "100" on a resistor is 100 Ω by universal convention;
// on a capacitor it could be farads, microfarads or picofarads depending on the era and the tool that
// wrote it. So it is a naming convention, declared through --conventions like the rail and ground
// patterns in RoleVocab.
//
// The DEFAULT covers resistors only. Guessing a bare capacitor or inductor unit would put a number
// three or six orders of magnitude out into a field that reads as authoritative. An unset class
// yields an empty unit, the "number known, unit not" state Quantity exists to represent.
type ValueVocab struct {
	// dimension is the unit a class's value is measured in, applied whenever the NOTATION stated no
	// unit. It is not a guess, since a capacitor's value is a capacitance and "10u" on one is 10 uF.
	// Almost nobody writes the F or the H, so without it most passives would get no unit.
	dimension map[ComponentClass]string
	// bareUnit is the narrower, uncertain case of a value with NO multiplier prefix either, where the
	// magnitude is unstated and historic conventions disagree.
	bareUnit map[ComponentClass]string
}

// DefaultValueVocab returns the built-in bare-number conventions. A resistor's bare value is ohms and
// nothing else is assumed; the ValueVocab doc says why the default is this narrow.
func DefaultValueVocab() *ValueVocab {
	return &ValueVocab{
		dimension: map[ComponentClass]string{
			ClassResistor:  UnitOhm,
			ClassCapacitor: "F",
			ClassInductor:  "H",
			ClassFerrite:   UnitOhm, // a bead is specified by its impedance at a frequency
		},
		// Only the resistor (see the ValueVocab doc).
		bareUnit: map[ComponentClass]string{ClassResistor: UnitOhm},
	}
}

// BuildValueVocab returns a vocabulary from declared conventions, a map of device-class name to the SI
// BASE unit a bare number on that class means ("ohm"/"farad"/"henry" are accepted alongside the
// symbols). An empty or nil config yields the defaults. A class the config names replaces the default
// for that class; classes it does not name keep theirs.
func BuildValueVocab(bareUnits map[string]string) *ValueVocab {
	v := DefaultValueVocab()
	for cls, unit := range bareUnits {
		u := canonicalUnit(unit)
		if u == "" {
			continue // an unreadable unit is skipped, never guessed, as in the params tier
		}
		// A declared convention settles the ambiguous bare case AND confirms the dimension, since a
		// house saying "a bare capacitor value is in farads" has told us both.
		key := ComponentClass(strings.ToLower(strings.TrimSpace(cls)))
		v.bareUnit[key] = u
		v.dimension[key] = u
	}
	return v
}

// canonicalUnit maps a declared unit spelling to its canonical SI base symbol, or "" when it names no
// unit this engine models.
func canonicalUnit(s string) string {
	if u, ok := unitSuffixes[strings.ToUpper(strings.TrimSpace(s))]; ok {
		return u
	}
	if u, ok := unitSuffixes[strings.TrimSpace(s)]; ok {
		return u
	}
	return ""
}

// UnitFor returns the unit a value on this component class is measured in when the notation stated
// none. prefixed says whether the value carried a multiplier ("10u" yes, "100" no), which selects
// between the dimension and bareUnit cases. Empty is a real answer, not a failure.
func (v *ValueVocab) UnitFor(class ComponentClass, prefixed bool) string {
	if v == nil {
		v = DefaultValueVocab()
	}
	if prefixed {
		return v.dimension[class]
	}
	return v.bareUnit[class]
}

// valueAttrKeys are the attribute names a component's value arrives under. KiCad, IPC-2581 and gEDA all
// normalize to "Value" at ingestion; xschem writes lowercase "value"; EDIF aggregates section properties
// with the SOURCE spelling and normalizes only MPN, so its value key is whatever the exporting tool
// wrote.
//
// The match is case-insensitive, so Value/VALUE/value need one entry. The remaining aliases are the
// spellings seen in this repo's readers. No wider EDIF aliases are guessed, because the public corpus
// is FPGA netlists with no board parts and a wrong alias would read values from the wrong property.
var valueAttrKeys = []string{"value", "val", "partvalue", "component_value"}

// valueTextOf returns the value text a component carries, or "" when it carries none. Checked in valueAttrKeys order so an explicit "Value" wins a looser alias.
func valueTextOf(c *ir.Component) string {
	for _, want := range valueAttrKeys {
		for k, v := range c.GetAttributes() {
			if strings.ToLower(k) == want && strings.TrimSpace(v) != "" {
				return v
			}
		}
	}
	return ""
}

// StampValues fills each component's value Quantity from whatever attribute its format spelled the
// value in (WS3-118), so a rule reads a NUMBER rather than re-parsing text per format. It is idempotent,
// recomputing and overwriting.
//
// A component with no value attribute gets NO Quantity (nil). One whose value text could not be parsed
// gets a Quantity carrying the input with the number absent, and only that second case is a gap worth
// reporting.
func (l *Lexicon) StampValues(d *ir.Design) {
	vocab := l.value()
	for _, c := range d.GetComponents() {
		text := valueTextOf(c)
		if text == "" {
			c.Value = nil
			continue
		}
		q := &ir.Quantity{Input: text}
		if v, unit, prefixed, ok := parseQuantityDetail(text); ok {
			q.Value = &v
			if unit == "" {
				// The notation gave a number but no unit, so the class decides. Still possibly empty
				// (a bare value on an ambiguous class), and that stays empty rather than a guess.
				unit = vocab.UnitFor(MostSpecific(ClassNames(c)), prefixed)
			}
			q.Unit = unit
		}
		c.Value = q
	}
}

// StampValues is the process-level form of (*Lexicon).StampValues, which carries the contract.
func StampValues(d *ir.Design) { ActiveLexicon().StampValues(d) }
