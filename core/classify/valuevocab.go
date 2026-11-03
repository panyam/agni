package classify

import (
	"fmt"
	"math"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// ValueVocab is the CONVENTION half of reading a component's value, kept out of the parser (WS3-118).
//
// ParseQuantity implements the specified part of the notation, IEC 60062's RKM code and the SI
// prefixes. No specification says what unit a BARE number is in, because that depends on the
// component's class and on house habit. A bare "100" on a resistor is 100 Ω by universal convention;
// on a capacitor it could be farads, microfarads or picofarads depending on the era and the tool that
// wrote it. So it is a naming convention, declared under the lexicon's value_unit like the rail and
// ground patterns in RoleVocab.
//
// The DEFAULT covers resistors only. Guessing a bare capacitor or inductor unit would put a number
// three or six orders of magnitude out into a field that reads as authoritative. An unset class
// yields an empty unit. Quantity exists to represent this "number known, unit not" state.
type ValueVocab struct {
	// dimension maps a class to the unit its value is measured in. It applies whenever the NOTATION
	// stated no unit. It is not a guess, since a capacitor's value is a capacitance and "10u" on one
	// is 10 uF.
	// Almost nobody writes the F or the H, so without it most passives would get no unit.
	dimension map[ComponentClass]string
	// bareUnit is the narrower, uncertain case of a value with NO multiplier prefix either, where the
	// magnitude is unstated and historic conventions disagree.
	bareUnit map[ComponentClass]string
	// bareExp maps a class to the power of ten of a bare number on it. So a house declaring pF has a
	// bare capacitor "100" read as 100 pF. Absent is 0.
	bareExp map[ComponentClass]int
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
		bareExp:  map[ComponentClass]int{},
	}
}

// BuildValueVocab returns a vocabulary from a convention's value_unit block. The block maps a
// device-class name to the unit of a bare number on that class. The unit is an SI base unit ("F",
// "ohm", "henry") or one with a multiplier prefix ("pF", "uH", "kohm"), and the prefix scales the
// bare number. An empty or nil config yields the defaults. Naming a class replaces that class's
// default, and every class the config leaves out keeps its default. A class the engine does not ship,
// or a unit it cannot read, is an error, since config is operator input and a skipped line reads as
// applied.
func BuildValueVocab(bareUnits map[string]string) (*ValueVocab, error) {
	v := DefaultValueVocab()
	for cls, unit := range bareUnits {
		key, ok := ParseComponentClass(strings.ToLower(strings.TrimSpace(cls)))
		if !ok {
			return nil, fmt.Errorf("value_unit: unknown component class %q", cls)
		}
		u, exp := scaledUnit(unit)
		if u == "" {
			return nil, fmt.Errorf("value_unit %q: unit %q is not one the engine reads", cls, unit)
		}
		v.bareExp[key] = exp
		// A declared convention settles the ambiguous bare case AND confirms the dimension, since a
		// house saying "a bare capacitor value is in farads" has told us both.
		v.bareUnit[key] = u
		v.dimension[key] = u
	}
	return v, nil
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

// scaledUnit splits a declared unit into its base unit and its prefix's power of ten. It returns ""
// when the text is neither a unit nor a prefixed one.
func scaledUnit(s string) (string, int) {
	s = strings.TrimSpace(s)
	if u := canonicalUnit(s); u != "" {
		return u, 0
	}
	if rest, ok := strings.CutPrefix(s, "µ"); ok {
		if u := canonicalUnit(rest); u != "" {
			return u, -6
		}
	}
	if len(s) > 1 {
		if exp, ok := siPrefixes[s[0]]; ok && exp != 0 {
			if u := canonicalUnit(s[1:]); u != "" {
				return u, exp
			}
		}
	}
	return "", 0
}

// bareScale scales a bare number on class by the class's declared power of ten.
func (v *ValueVocab) bareScale(class ComponentClass, value float64) float64 {
	switch exp := v.bareExp[class]; {
	case exp > 0:
		return value * math.Pow10(exp)
	case exp < 0:
		return value / math.Pow10(-exp)
	}
	return value
}

// UnitFor returns the unit for a value on this component class when the notation stated none.
// prefixed says whether the value had a multiplier ("10u" yes, "100" no), and it selects between the
// dimension and bareUnit cases. Empty is a real answer, not a failure.
func (v *ValueVocab) UnitFor(class ComponentClass, prefixed bool) string {
	if v == nil {
		v = DefaultValueVocab()
	}
	if prefixed {
		return v.dimension[class]
	}
	return v.bareUnit[class]
}

// valueAttrKeys are the built-in attribute.value keys. KiCad, IPC-2581 and gEDA all normalize to
// "Value" at ingestion; xschem writes lowercase "value"; EDIF aggregates section properties with the
// SOURCE spelling and normalizes only MPN, so its value key is whatever the exporting tool wrote. No
// wider EDIF aliases are guessed, because the public corpus is FPGA netlists with no board parts and a
// wrong alias would read values from the wrong property.
var valueAttrKeys = []string{"value", "val", "partvalue", "component_value"}


// StampValues fills each component's value Quantity from its value attribute, under whatever key its
// format used (WS3-118). A rule then reads a NUMBER rather than re-parsing text per format. It is
// idempotent, recomputing and overwriting.
//
// A component with no value attribute gets NO Quantity (nil). If its value text cannot be parsed, it
// gets a Quantity with the input and no number. Only that second case is a gap worth reporting.
func (l *Lexicon) StampValues(d *ir.Design) {
	vocab := l.value()
	for _, c := range d.GetComponents() {
		text := l.role().ValueText(c)
		if text == "" {
			c.Value = nil
			continue
		}
		q := &ir.Quantity{Input: text}
		class := MostSpecific(ClassNames(c))
		if v, unit, prefixed, ok := parseQuantityDetail(text); ok {
			q.Value = &v
			if unit == "" {
				// The notation gave a number but no unit, so the class decides. The unit can still be
				// empty (a bare value on an ambiguous class), and then it is left empty, not guessed.
				unit = vocab.UnitFor(class, prefixed)
				if !prefixed {
					v = vocab.bareScale(class, v)
				}
			}
			q.Unit = unit
		} else if v, unit, ok := vocab.valueFromName(text, class); ok {
			q.Value, q.Unit = &v, unit
		}
		c.Value = q
	}
}

// valueFromName reads a value out of a library part name such as `R_10k_0402` or `C_4u7_25V_0402`
// (agni issue 1115). A library uses such a name as the Value, and the whole name parses to nothing.
// A token counts only when it has an SI prefix or a unit, so a package size (`0402`) is never read as
// ohms. It also counts only when its unit is the class's own (ohms, farads, henries), so a rating
// (`25V`, `22.4A`) is never read as the value. A name holding two different such values says nothing
// reliable and reads as none. So does a class with no unit of its own.
func (v *ValueVocab) valueFromName(text string, class ComponentClass) (float64, string, bool) {
	want := v.UnitFor(class, true)
	if want == "" {
		return 0, "", false
	}
	var found *float64
	for _, tok := range strings.FieldsFunc(text, func(r rune) bool { return r == '_' || r == ' ' || r == '/' || r == ',' }) {
		val, unit, prefixed, ok := parseQuantityDetail(tok)
		if !ok || (unit == "" && !prefixed) {
			continue
		}
		if unit == "" {
			unit = v.UnitFor(class, prefixed)
		}
		if unit != want {
			continue
		}
		if found != nil && *found != val {
			return 0, "", false
		}
		found = &val
	}
	if found == nil {
		return 0, "", false
	}
	return *found, want, true
}

// StampValues is the process-level form of (*Lexicon).StampValues, whose doc states the contract.
func StampValues(d *ir.Design) { ActiveLexicon().StampValues(d) }
