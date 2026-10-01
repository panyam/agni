package param

import (
	"errors"
	"slices"
	"strings"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// Pin resolution is the spec-side half of the design-pin-to-datasheet-pin join, and the one
// place the precedence stated on Parameter.pin_refs is implemented.
//
// THE NAME LEADS AND THE NUMBER ONLY BREAKS TIES. A pin NUMBER is a fact about a PACKAGE and
// a datasheet parameter is a fact about the DIE, so a number-first join reports about the
// wrong terminal on a part seeded from one package and placed in another. The TXB0104 fixture
// carries the case, where number 11 is a data I/O in the TSSOP-14 and the B-side supply in the
// UQFN-12. See docsite/content/architecture/datasheet-layer.md#pin-binding.
//
// EVERYTHING HERE IS PER PART TYPE. No function takes a reference designator, so a caller
// resolves once per (part type, terminal) and fans the answer out across instances itself.

var (
	// ErrNoPinData means the spec declares no pins at all, as an older corpus does. Unlike
	// ErrPinUnknown, the caller should fall back to the part-level path rather than treat
	// this as a failed lookup (CONSTRAINTS C9, degrade-safe).
	ErrNoPinData = errors.New("param: spec declares no pins")

	// ErrPinUnknown means the spec has pins but neither channel matched this terminal.
	ErrPinUnknown = errors.New("param: no pin matches")

	// ErrPinAmbiguous means the evidence narrows to several pins and nothing separates them:
	// a name printed on more than one terminal with no identified package to break the tie,
	// or a number that means different pins in different packages. Callers skip and never
	// pick one.
	ErrPinAmbiguous = errors.New("param: pin reference is ambiguous")

	// ErrPinConflict means the two channels resolved to DIFFERENT pins, the repackaging
	// mismatch or a wrong symbol. Either channel alone would have given a confident wrong
	// answer.
	ErrPinConflict = errors.New("param: pin name and number disagree")
)

// ResolvePin maps one design terminal onto a pin of this spec, given the terminal's name and
// package-relative designator (both off the ir.Pin reached through the component's PartType)
// and the package the design places, when identified (PackageForMPN derives it from an
// orderable MPN).
//
// Any argument may be empty, and each empty one removes a channel. The precedence:
//
//	name matches one pin                  -> that pin (the number is not consulted)
//	name matches several                  -> the number breaks the tie, inside a known package
//	name matches none, number matches one -> that pin (a symbol whose pin names differ from
//	                                         the datasheet's is ordinary)
//	the two channels disagree             -> ErrPinConflict
//	nothing separates the candidates      -> ErrPinAmbiguous
//
// An empty packageRef does not disable the number channel. The number is looked up in every
// declared package and resolves when they all agree (VCCA is number 1 in both the TSSOP and
// the UQFN), refusing only where the packages disagree.
//
// A refusal always returns a nil pin. Skip-not-false-pass, since a rule that resolves the
// wrong terminal evaluates cleanly and reports about the wrong thing.
//
// A multi-SECTION component (a relay's coil and contacts) spreads its pins across several
// PartTypes, so the caller finds the ir.Pin for a designator by searching every section's
// PartType. Pin numbering is per physical package, so sections never enter the key.
func ResolvePin(spec *parampb.PartSpec, name, designator, packageRef string) (*parampb.Pin, error) {
	if len(spec.GetPins()) == 0 {
		return nil, ErrNoPinData
	}

	byName := PinsByName(spec, name)
	byNumber := PinsByNumber(spec, packageRef, designator)

	// The number channel speaks only when it lands on exactly one pin. Several hits means
	// the packages disagree about this number.
	var numbered *parampb.Pin
	if len(byNumber) == 1 {
		numbered = byNumber[0]
	}

	switch {
	case len(byName) == 0:
		switch {
		case numbered != nil:
			return numbered, nil
		case len(byNumber) > 1:
			return nil, ErrPinAmbiguous
		default:
			return nil, ErrPinUnknown
		}

	case len(byName) == 1:
		if numbered != nil && numbered.GetId() != byName[0].GetId() {
			return nil, ErrPinConflict
		}
		return byName[0], nil

	default:
		if numbered == nil {
			return nil, ErrPinAmbiguous
		}
		if slices.ContainsFunc(byName, func(p *parampb.Pin) bool { return p.GetId() == numbered.GetId() }) {
			return numbered, nil
		}
		return nil, ErrPinConflict
	}
}

// PinsByName returns every pin printed under a name, in declaration order. SEVERAL HITS IS
// THE NORMAL CASE for a part that prints one name across terminals (a large IC's VDD pins,
// the TXB0104's two NC pins), and collapsing it to the first match is the bug pin binding
// exists to prevent. An empty name matches nothing.
//
// Matching normalizes case and drops spaces and underscores, so "V CCA" from a document
// producer and "VCC_A" from a symbol library both reach "VCCA". Nothing fuzzier, since a
// near-miss name is a different pin.
func PinsByName(spec *parampb.PartSpec, name string) []*parampb.Pin {
	key := normalizePinName(name)
	if key == "" {
		return nil
	}
	var out []*parampb.Pin
	for _, p := range spec.GetPins() {
		if normalizePinName(p.GetName()) == key {
			out = append(out, p)
		}
	}
	return out
}

// PinsByNumber returns every pin carrying a designator in a package. With a packageRef it
// searches that package only and yields at most one pin, since Validate holds a number to
// one pin per package. With an EMPTY packageRef it searches all of them and yields the
// distinct pins the number reaches, so a caller can tell "every package agrees" (one hit)
// from "the packages disagree" (several).
func PinsByNumber(spec *parampb.PartSpec, packageRef, number string) []*parampb.Pin {
	key := normalizePinNumber(number)
	if key == "" {
		return nil
	}
	var out []*parampb.Pin
	for _, p := range spec.GetPins() {
		for _, n := range p.GetNumbers() {
			if packageRef != "" && n.GetPackageRef() != packageRef {
				continue
			}
			if normalizePinNumber(n.GetNumber()) == key {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// PinByID returns the pin with a spec-local id, or nil. Parameter.pin_refs names these ids and
// Validate guarantees every ref resolves, so a nil on a validated spec means the caller
// invented the id.
func PinByID(spec *parampb.PartSpec, id string) *parampb.Pin {
	if id == "" {
		return nil
	}
	for _, p := range spec.GetPins() {
		if p.GetId() == id {
			return p
		}
	}
	return nil
}

// PinParameters returns the parameters BOUND to a pin, in declaration order.
//
// Part-wide rows (an empty pin_refs) are EXCLUDED. An empty binding means "a fact about the
// die", and reading it as "a fact about every terminal" would credit a junction-temperature
// rating as a pin's own limit. A caller that wants the part-level fallback asks for it
// explicitly.
func PinParameters(spec *parampb.PartSpec, pinID string) []*parampb.Parameter {
	if pinID == "" {
		return nil
	}
	var out []*parampb.Parameter
	for _, p := range spec.GetParameters() {
		if slices.Contains(p.GetPinRefs(), pinID) {
			out = append(out, p)
		}
	}
	return out
}

// PinRelations returns every relation this pin takes part in, on EITHER end, since a relation
// between two terminals constrains both.
//
// The caller must read subject_pin_ref to tell the ends apart. The bound is on subject MINUS
// reference, so treating this pin as always the subject inverts the sign on half of them.
func PinRelations(spec *parampb.PartSpec, pinID string) []*parampb.PinRelation {
	if pinID == "" {
		return nil
	}
	var out []*parampb.PinRelation
	for _, r := range spec.GetRelations() {
		if r.GetSubjectPinRef() == pinID || r.GetReferencePinRef() == pinID {
			out = append(out, r)
		}
	}
	return out
}

// PackageForMPN narrows an orderable MPN to one declared package by its suffix, so a caller
// can supply ResolvePin's packageRef instead of leaving the number channel package-blind.
// It returns nil when no declared suffix matches, which includes the ordinary case of a
// design carrying only the base MPN. NIL MEANS "PACKAGE NOT IDENTIFIED", NEVER "NO PACKAGE",
// and the number channel then requires cross-package agreement rather than assuming a body.
//
// The match is a case-insensitive suffix test that tolerates one trailing packaging letter
// (the reel/tape "R", "T"), because "TXB0104PWR" is the tape-and-reel spelling of the same
// TSSOP part. The longest matching suffix wins. A package declaring no mpn_suffix never
// matches, since an undeclared suffix is not evidence.
func PackageForMPN(spec *parampb.PartSpec, mpn string) *parampb.Package {
	up := strings.ToUpper(strings.TrimSpace(mpn))
	if up == "" {
		return nil
	}
	var best *parampb.Package
	for _, pkg := range spec.GetPackages() {
		suffix := strings.ToUpper(pkg.GetMpnSuffix())
		if suffix == "" {
			continue
		}
		if !hasPackageSuffix(up, suffix) {
			continue
		}
		if best == nil || len(suffix) > len(best.GetMpnSuffix()) {
			best = pkg
		}
	}
	return best
}

// hasPackageSuffix reports whether an upper-cased MPN ends in a package code, allowing one
// trailing packaging-format letter after it ("PWR" is "PW" on tape and reel). Any longer tail
// is treated as a different part number, because guessing at it is how a near-miss MPN becomes
// the wrong package.
func hasPackageSuffix(mpn, suffix string) bool {
	if strings.HasSuffix(mpn, suffix) {
		return true
	}
	if len(mpn) > 0 {
		trimmed := mpn[:len(mpn)-1]
		last := mpn[len(mpn)-1]
		if (last == 'R' || last == 'T') && strings.HasSuffix(trimmed, suffix) {
			return true
		}
	}
	return false
}

// normalizePinName reduces a pin name to its comparison key, upper case with no spaces or
// underscores. It is narrower than the model layer's symbol normalization, which also drops
// parentheses, because stripping more would merge different terminals.
func normalizePinName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if r == ' ' || r == '_' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// normalizePinNumber reduces a designator to its comparison key, upper case with all
// whitespace removed. Ball designators are alphanumeric ("B2"), so numeric parsing is not an
// option. Leading zeros are NOT stripped, so "01" and "1" stay different keys, because a
// pinout that writes one never writes the other.
func normalizePinNumber(s string) string {
	return strings.ToUpper(strings.Join(strings.Fields(s), ""))
}
