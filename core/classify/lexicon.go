package classify

import ir "github.com/panyam/agni/gen/go/agni/v1/ir"

// Lexicon is the naming vocabulary one READ is performed with: the role vocabulary (rail / ground /
// feedback / supply-pin names) and the classification vocabulary (device-class token hints). It is a
// VALUE carried by the loader, not a process global, so two designs read in one process can be
// stamped with different project conventions (WS3-106) — the property a served request needs and a
// package-level var cannot provide.
//
// It belongs to the read rather than to the rule catalog because that is where it does its work: the
// stamps below turn a vocabulary into DATA (ir.Net.roles, ir.Component.device_classes, POWER_IN pin
// directions), after which rules read the data and the vocabulary is spent (WS3-072's left-shift).
// Resolving names per rule instead would undo that.
//
// A nil *Lexicon means the process defaults, so every existing caller and any embedder that never
// declares a convention is unchanged.
type Lexicon struct {
	Role  *RoleVocab
	Class *ClassVocab
	// Value carries the bare-number unit conventions (WS3-118). A distinct name space from Role and
	// Class, so a distinct vocabulary: what unit "100" means on a capacitor has nothing to do with
	// what makes a net a rail.
	Value *ValueVocab
}

// DefaultLexicon returns the engine's built-in vocabularies, the value a read with no project
// convention uses.
func DefaultLexicon() *Lexicon {
	return &Lexicon{Role: DefaultRoleVocab(), Class: DefaultClassVocab(), Value: DefaultValueVocab()}
}

// ActiveLexicon captures the process-level vocabularies as a value. It is the bridge for callers that
// still install a convention globally (the CLI's --conventions today): the globals are read ONCE here,
// at read time, instead of being consulted again by every downstream name match.
func ActiveLexicon() *Lexicon {
	return &Lexicon{Role: activeRoleVocab, Class: activeClassVocab, Value: DefaultValueVocab()}
}

// role resolves the role vocabulary, falling back to the process default so a partially-filled or nil
// Lexicon is usable rather than a panic. Config is operator input; a missing half means "unspecified",
// which is the default, not an error.
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

// RoleVocab returns the role vocabulary in effect (the process default when unset), for the consumers
// that must match a bare NAME after the read: the spec-language name FFIs and pin-role derivation,
// which have no net to read a stamped role from.
func (l *Lexicon) RoleVocab() *RoleVocab { return l.role() }

// ClassVocab returns the classification vocabulary in effect (the process default when unset).
func (l *Lexicon) ClassVocab() *ClassVocab { return l.class() }

// Stamp runs the classification pass with this lexicon, filling each component's device_classes SET.
// See the package-level Stamp for the pass's contract; this is the per-read form.
func (l *Lexicon) Stamp(d *ir.Design) {
	index := PartIndex(d)
	for _, c := range d.GetComponents() {
		c.DeviceClasses = ClassesOf(l.Classify(c, FirstPart(index, c)))
	}
}

// StampNetRoles fills each net's roles SET from this lexicon. See the package-level StampNetRoles.
func (l *Lexicon) StampNetRoles(d *ir.Design) {
	v := l.role()
	for _, n := range d.GetNets() {
		n.Roles = rolesFor(v, n.GetName())
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
