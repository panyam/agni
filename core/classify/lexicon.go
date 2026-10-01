package classify

import ir "github.com/panyam/agni/gen/go/agni/v1/ir"

// Lexicon is the naming vocabulary one READ is performed with: the role vocabulary (rail, ground,
// feedback, switching, control, gate-drive and supply-pin names), the classification vocabulary
// (device-class token hints) and the value vocabulary. It is a VALUE carried by the loader, not a
// process global, so two designs read in one process can be stamped with different project
// conventions (WS3-106).
//
// The stamps below turn a vocabulary into DATA (ir.Net.roles, ir.Component.device_classes, POWER_IN
// pin directions), after which rules read the data and the vocabulary is spent (WS3-072's
// left-shift). The package-level Stamp, StampNetRoles, StampValues and StampPowerInPins are the
// process-level forms of the methods here, reading the globals through ActiveLexicon.
//
// A nil *Lexicon means the process defaults.
type Lexicon struct {
	Role  *RoleVocab
	Class *ClassVocab
	// Value carries the bare-number unit conventions (WS3-118).
	Value *ValueVocab
}

// DefaultLexicon returns the engine's built-in vocabularies, the value a read with no project
// convention uses.
func DefaultLexicon() *Lexicon {
	return &Lexicon{Role: DefaultRoleVocab(), Class: DefaultClassVocab(), Value: DefaultValueVocab()}
}

// ActiveLexicon captures the process-level vocabularies as a value, for callers that install a
// convention globally (`agni serve`, through naming.ApplyLexicon). The globals are read ONCE here, at
// read time, rather than by every downstream name match.
func ActiveLexicon() *Lexicon {
	return &Lexicon{Role: activeRoleVocab, Class: activeClassVocab, Value: DefaultValueVocab()}
}

// role resolves the role vocabulary, falling back to the process default, so a nil or partially
// filled Lexicon is usable. A missing half in operator config means the default, not an error.
func (l *Lexicon) role() *RoleVocab {
	if l == nil || l.Role == nil {
		return activeRoleVocab
	}
	return l.Role
}

// class resolves the classification vocabulary, with the same nil-means-default contract as role.
func (l *Lexicon) class() *ClassVocab {
	if l == nil || l.Class == nil {
		return activeClassVocab
	}
	return l.Class
}

// value resolves the bare-number unit vocabulary, with the same nil-means-default contract as role.
func (l *Lexicon) value() *ValueVocab {
	if l == nil || l.Value == nil {
		return DefaultValueVocab()
	}
	return l.Value
}

// ValueVocab returns the bare-number unit vocabulary in effect (the built-in default when unset).
func (l *Lexicon) ValueVocab() *ValueVocab { return l.value() }

// RoleVocab returns the role vocabulary in effect (the process default when unset), for consumers
// that must match a bare NAME after the read, such as the spec-language name FFIs and pin-role
// derivation, which have no net to read a stamped role from.
func (l *Lexicon) RoleVocab() *RoleVocab { return l.role() }

// ClassVocab returns the classification vocabulary in effect (the process default when unset).
func (l *Lexicon) ClassVocab() *ClassVocab { return l.class() }

// Stamp runs the classification pass with this lexicon, filling each component's device_classes SET.
// See the package-level Stamp for the pass's contract.
//
// It REPLACES the set, which keeps a re-stamp idempotent and also drops any datasheet tag an earlier
// StampClassesFromSpecs added, so StampClassesFromSpecs must run after it. See
// docsite/content/architecture/ingestion-and-ir.md#derived-fields-and-the-tiers-that-fill-them.
func (l *Lexicon) Stamp(d *ir.Design) {
	index := PartIndex(d)
	for _, c := range d.GetComponents() {
		c.DeviceClasses = TagsOf(l.Classify(c, FirstPart(index, c)), ir.ClassSource_CLASS_SOURCE_CONVENTION)
	}
}

// StampNetRoles fills each net's roles SET from this lexicon. See the package-level StampNetRoles.
func (l *Lexicon) StampNetRoles(d *ir.Design) {
	v := l.role()
	for _, n := range d.GetNets() {
		n.Roles = rolesFor(v, n)
	}
}

// StampPowerInPins promotes under-typed supply pins to POWER_IN using this lexicon's supply-pin names.
// See the package-level StampPowerInPins.
func (l *Lexicon) StampPowerInPins(d *ir.Design) {
	v := l.role()
	for _, lib := range d.GetLibraries() {
		for _, pt := range lib.GetParts() {
			for _, pin := range pt.GetPins() {
				if underspecifiedInputDir(pin.GetDirection()) && v.IsSupplyPin(pin.GetName()) {
					pin.Direction = ir.PinDirection_PIN_DIRECTION_POWER_IN
				}
			}
		}
	}
}
