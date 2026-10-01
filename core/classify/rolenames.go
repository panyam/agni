package classify

import (
	"fmt"
	"regexp"
	"strings"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// RoleToken renders a role as the lowercase token the query language and the config lexicon use, so
// ROLE_GATE_DRIVE becomes "gate_drive". It is DERIVED from the generated enum name rather than read
// from a table, so the tokens cannot drift from the vocabulary and another language can apply the
// same transform to its own generated enum.
//
// ROLE_UNSPECIFIED renders empty, so a zero value never reaches a fact row looking like a real role.
func RoleToken(r ir.Role) string {
	if r == ir.Role_ROLE_UNSPECIFIED {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(r.String(), "ROLE_"))
}

// ParseRole maps a token back to its role, reporting whether the vocabulary has one. A reader
// translating a format's netclass, or a config naming a role, gets false for an unknown token rather
// than a value that silently matches nothing (agni 677 is the same failure for classes).
func ParseRole(token string) (ir.Role, bool) {
	for _, r := range AllNetRoles() {
		if RoleToken(r) == token {
			return r, true
		}
	}
	return ir.Role_ROLE_UNSPECIFIED, false
}

// AllNetRoles is every role the engine stamps, in declaration order and without the zero value. It
// reads the GENERATED enum, so a role added to the proto is projected, parseable and configurable
// with no second list to update.
func AllNetRoles() []ir.Role {
	out := make([]ir.Role, 0, len(ir.Role_name)-1)
	for i := int32(1); ; i++ {
		name, ok := ir.Role_name[i]
		if !ok {
			break
		}
		_ = name
		out = append(out, ir.Role(i))
	}
	return out
}

// AttrDeclaredRole is the ir.Net.attributes key carrying a role the SOURCE FILE stated outright,
// already translated into the NetRole vocabulary above by the reader that understood the format.
// StampNetRoles unions it with what the naming lexicon infers.
//
// A role can be KNOWN rather than guessed. IPC-2581 declares it on LogicalNet/@netClass as a closed
// enum, so a net called "N$17" can be authoritatively GROUND with nothing in the name to go on.
//
// The READER translates, per C9, so this package never learns a format's enum, and a second format
// that declares roles writes this same key. An UNMAPPABLE source term is not written (the reader
// keeps it verbatim in its own attribute), so this key holds a valid NetRole token or is absent. It
// is an attribute rather than a typed field because C9 admits a typed field only once a second format
// populates it.
const AttrDeclaredRole = "declared_role"

// RoleVocab is the naming lexicon, the regex sets that decide a net's electrical ROLE by name (rail,
// ground, feedback), for the cases where a directionless netlist carries the name as the only
// evidence. A project whose house naming differs extends or replaces each vocabulary via config
// (WS3-069). check re-exports the names (WS3-072).
//
// Matching is on the hierarchy LEAF ("/psu/12V" -> "12V", the WS3-006 convention) and case-insensitive.
// The zero value is not usable; build one with DefaultRoleVocab or BuildRoleVocab.
//
// The net vocabularies classify NET names; supplyPin classifies a component's PIN names (which supply
// a part CONSUMES, for the WS3-072 POWER_IN stamp). supplyPin is stricter than rail. A supply PIN is
// named VDD/VIN, never "3V3", and a bare "+" is a polarized-part terminal, so the net-name forms (^+,
// digit-then-V) and the supply OUTPUT name VOUT are absent from it.
type RoleVocab struct {
	// The EFFECTIVE patterns this vocabulary was built from, so a caller can read back what a project
	// installed rather than only asking yes/no questions. Embedded by POINTER because a protobuf-go
	// message holds a DoNotCopy, so a value embedding makes every RoleVocab copy a vet copylocks error.
	*configpb.NamingLexicon

	// The compiled form, derived from the patterns above at construction and never written again, so
	// it cannot go stale against them. Replace a vocabulary by building a new one, never by reaching in.
	rail, ground, feedback, switching, control, gateDrive, supplyPin []*regexp.Regexp
	// Transistor TERMINAL pin names (WS3-117), each its own vocabulary because a house can spell them
	// independently. Consumed ONLY where the component's class is a transistor (see classifyPinRole),
	// because bare "S" and "D" mean something on almost every part and an ungated match would mis-role
	// most of a design. A wrong role is worse than a missing one, since a topology rule then walks a
	// path that does not exist.
	gate, source, drain []*regexp.Regexp
}

func mustCompileRole(pats ...string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(pats))
	for i, p := range pats {
		out[i] = regexp.MustCompile("(?i)" + p)
	}
	return out
}

// defaultLexicon is the built-in naming policy, in the SAME schema a project's conventions.yaml
// carries, so the engine, the wire and the viewer read one definition of the vocabularies (C2). A
// hand-written Go twin lost three vocabularies twice (WS3-117, agni 680). What the four
// "named after a rail, not a rail" roles mean is in docsite/content/guide/naming-conventions.md.
func defaultLexicon() *configpb.NamingLexicon {
	vp := func(pats ...string) *configpb.VocabPatterns { return &configpb.VocabPatterns{Patterns: pats} }
	return &configpb.NamingLexicon{
		Net: &configpb.NetNameVocab{
			// rail: a "+" prefix, the supply-name prefixes, or a "12V"/"3V3"/"5V0" digits-then-V form.
			Rail: vp(`^\+`, `^(VCC|VDD|VEE|VBUS|VIN|VOUT|VBAT|VSUP|PWR)`, `^[0-9]+V`),
			// ground: GND or EARTH anywhere, or a VSS prefix.
			Ground: vp(`GND`, `EARTH`, `^VSS`),
			// feedback: a regulator sense node named with an _FB / feedback / sense suffix.
			Feedback: vp(`_FB$`, `_VFB$`, `_FEEDBACK$`, `_VSENSE$`, `_SENSE$`, `_SNS$`, `^V?FB$`),
			// switching: a regulator's power-stage node (_SW, _PHASE, _LX) and bootstrap node (_BOOT).
			// "3V3_SW" is also a load-switch output in some houses; reading it as a regulator internal
			// is the safe direction, and a project with the other convention narrows this vocabulary.
			Switching: vp(`_SW$`, `_BOOT$`, `_PHASE$`, `_LX$`),
			// control: enable and configuration inputs, where the voltage token names the CONVERTER
			// ("20V_EN" enables the 20V converter and is driven at logic level).
			Control: vp(`_EN$`, `_MODE\d*$`, `_SEL\d*$`),
			// gateDrive: the gate driver's own supply, named after the converter rather than its
			// voltage ("12V_VDRV" commonly sits at 5V).
			GateDrive: vp(`_VDRV$`, `_VDRIVE$`, `_VGATE$`),
		},
		Pin: &configpb.PinNameVocab{
			// supplyPin: a power-supply INPUT pin name, by prefix (VDD covers VDDA/VDDIO/VDDQ, VCC covers
			// VCCIO). Stricter than rail (see the RoleVocab doc).
			Supply: vp(`^(VCC|VDD|VIN|VBAT|VBUS|VSUP|VPP|AVDD|DVDD|VAUX|VCORE|VEE)`),
			// Transistor terminals, whole-name anchored, because even inside the class gate an
			// unanchored "S" would match SDA, SCLK and SENSE.
			Gate:   vp(`^G$`, `^GATE$`),
			Source: vp(`^S$`, `^SOURCE$`, `^SRC$`),
			Drain:  vp(`^D$`, `^DRAIN$`, `^DRN$`),
		},
	}
}

// DefaultRoleVocab is the built-in lexicon, compiled. A project's config merges onto (or replaces)
// these; see BuildRoleVocab.
func DefaultRoleVocab() *RoleVocab {
	v, err := BuildRoleVocab(nil)
	if err != nil {
		panic("classify: the built-in naming lexicon does not compile: " + err.Error())
	}
	return v
}

func roleLeaf(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}

func anyRoleMatch(name string, pats []*regexp.Regexp) bool {
	name = roleLeaf(name)
	for _, p := range pats {
		if p.MatchString(name) {
			return true
		}
	}
	return false
}

// IsRail through IsGateDrive classify a NET name against the vocabulary. IsSupplyPin classifies a
// component's PIN name as a power-supply input, a stricter vocabulary (see the type doc).
func (v *RoleVocab) IsRail(name string) bool      { return anyRoleMatch(name, v.rail) }
func (v *RoleVocab) IsGround(name string) bool    { return anyRoleMatch(name, v.ground) }
func (v *RoleVocab) IsFeedback(name string) bool  { return anyRoleMatch(name, v.feedback) }
func (v *RoleVocab) IsSwitching(name string) bool { return anyRoleMatch(name, v.switching) }
func (v *RoleVocab) IsControl(name string) bool   { return anyRoleMatch(name, v.control) }
func (v *RoleVocab) IsGateDrive(name string) bool { return anyRoleMatch(name, v.gateDrive) }
func (v *RoleVocab) IsSupplyPin(name string) bool { return anyRoleMatch(name, v.supplyPin) }

// IsGate / IsSource / IsDrain classify a TRANSISTOR's pin name. The caller must apply the class gate,
// because these vocabularies are short and collide on any other part (see the type doc).
func (v *RoleVocab) IsGate(name string) bool   { return anyRoleMatch(name, v.gate) }
func (v *RoleVocab) IsSource(name string) bool { return anyRoleMatch(name, v.source) }
func (v *RoleVocab) IsDrain(name string) bool  { return anyRoleMatch(name, v.drain) }

// activeRoleVocab is the process-level lexicon, set once at startup and read by check's Is*Name
// helpers and by StampNetRoles. A read carrying its own conventions uses a Lexicon instead; see
// lexicon.go for the split.
var activeRoleVocab = DefaultRoleVocab()

// SetActiveRoleVocab replaces the process-level lexicon (agni serve installs its server default this
// way). Passing nil restores the defaults. It must run before ingestion (ReadDesign), since
// StampNetRoles stamps net.role with the active vocab.
func SetActiveRoleVocab(v *RoleVocab) {
	if v == nil {
		v = DefaultRoleVocab()
	}
	activeRoleVocab = v
}

// ActiveRoleVocab returns the process-level lexicon currently in effect (the defaults unless a project
// config replaced it), so a caller can inspect what was installed.
func ActiveRoleVocab() *RoleVocab { return activeRoleVocab }

// BuildRoleVocab merges a project's overrides onto the built-in lexicon and compiles the result,
// VALIDATING every pattern, since config is operator input and a bad regex should be a returned
// error rather than a panic. A nil override yields the built-ins. Patterns are RE2, matched
// case-insensitively on the hierarchy leaf (write ^/$ for whole-leaf anchoring).
func BuildRoleVocab(over *configpb.NamingLexicon) (*RoleVocab, error) {
	merged := mergeLexicon(defaultLexicon(), over)
	v := &RoleVocab{NamingLexicon: merged}
	net, pin := merged.GetNet(), merged.GetPin()
	for _, d := range []struct {
		name string
		src  *configpb.VocabPatterns
		dst  *[]*regexp.Regexp
	}{
		{"rail", net.GetRail(), &v.rail},
		{"ground", net.GetGround(), &v.ground},
		{"feedback", net.GetFeedback(), &v.feedback},
		{"switching", net.GetSwitching(), &v.switching},
		{"control", net.GetControl(), &v.control},
		{"gate_drive", net.GetGateDrive(), &v.gateDrive},
		{"supply_pin", pin.GetSupply(), &v.supplyPin},
		{"gate", pin.GetGate(), &v.gate},
		{"source", pin.GetSource(), &v.source},
		{"drain", pin.GetDrain(), &v.drain},
	} {
		out, err := compilePatterns(d.src)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", d.name, err)
		}
		*d.dst = out
	}
	return v, nil
}

// mergeLexicon resolves a project's overrides against the built-ins, vocabulary by vocabulary, and
// returns the EFFECTIVE lexicon. That value is what RoleVocab embeds, so reading the patterns back off
// a vocabulary answers what it actually matches rather than what was shipped.
func mergeLexicon(base, over *configpb.NamingLexicon) *configpb.NamingLexicon {
	if over == nil {
		return base
	}
	bn, bp := base.GetNet(), base.GetPin()
	on, op := over.GetNet(), over.GetPin()
	out := &configpb.NamingLexicon{
		Net: &configpb.NetNameVocab{
			Rail:      mergeVocab(bn.GetRail(), on.GetRail()),
			Ground:    mergeVocab(bn.GetGround(), on.GetGround()),
			Feedback:  mergeVocab(bn.GetFeedback(), on.GetFeedback()),
			Switching: mergeVocab(bn.GetSwitching(), on.GetSwitching()),
			Control:   mergeVocab(bn.GetControl(), on.GetControl()),
			GateDrive: mergeVocab(bn.GetGateDrive(), on.GetGateDrive()),
		},
		Pin: &configpb.PinNameVocab{
			Supply: mergeVocab(bp.GetSupply(), op.GetSupply()),
			Gate:   mergeVocab(bp.GetGate(), op.GetGate()),
			Source: mergeVocab(bp.GetSource(), op.GetSource()),
			Drain:  mergeVocab(bp.GetDrain(), op.GetDrain()),
		},
	}
	// The class block is a project's alone; there is no built-in set to merge it against.
	if cls := over.GetClass(); len(cls) > 0 {
		out.Class = cls
	}
	return out
}

// mergeVocab applies one vocabulary's override. Patterns are ADDED to the built-ins unless replace is
// set, in which case they become the whole set. Replace with no patterns yields an empty vocabulary,
// which is how a project turns a built-in role off.
func mergeVocab(base, over *configpb.VocabPatterns) *configpb.VocabPatterns {
	if over == nil || (len(over.GetPatterns()) == 0 && !over.GetReplace()) {
		return base
	}
	if over.GetReplace() {
		return &configpb.VocabPatterns{Patterns: over.GetPatterns(), Replace: true}
	}
	return &configpb.VocabPatterns{Patterns: append(append([]string{}, base.GetPatterns()...), over.GetPatterns()...)}
}

// compilePatterns compiles one vocabulary, case-insensitively.
func compilePatterns(v *configpb.VocabPatterns) ([]*regexp.Regexp, error) {
	pats := v.GetPatterns()
	if len(pats) == 0 {
		return nil, nil
	}
	out := make([]*regexp.Regexp, 0, len(pats))
	for _, p := range pats {
		re, err := regexp.Compile("(?i)" + p)
		if err != nil {
			return nil, fmt.Errorf("pattern %q: %w", p, err)
		}
		out = append(out, re)
	}
	return out, nil
}

// StampNetRoles fills each net's roles SET once at ingestion (WS3-072), so the core reads a normalized
// net.role fact instead of re-running name matching per net per rule. It recomputes and overwrites. A
// net matching no vocabulary gets an empty set, which consumers read as "no role", the same way an
// empty device_classes reads as "unknown". This is the process-level form of (*Lexicon).StampNetRoles.
func StampNetRoles(d *ir.Design) { ActiveLexicon().StampNetRoles(d) }

// RoleTokens returns just the role tokens a net carries, dropping the evidence. For the many callers
// that ask WHICH roles rather than how each was established; a caller that needs the source reads
// the NetRole values, or asks check.NetRoleSource.
func RoleTokens(n *ir.Net) []string {
	roles := n.GetRoles()
	if len(roles) == 0 {
		return nil
	}
	out := make([]string, 0, len(roles))
	for _, r := range roles {
		out = append(out, RoleToken(r.GetRoleKind()))
	}
	return out
}

// NetRoles returns the roles a net carries, dropping the evidence. The typed counterpart of
// RoleTokens, for a caller comparing against the vocabulary rather than rendering it.
func NetRoles(n *ir.Net) []ir.Role {
	roles := n.GetRoles()
	if len(roles) == 0 {
		return nil
	}
	out := make([]ir.Role, 0, len(roles))
	for _, r := range roles {
		out = append(out, r.GetRoleKind())
	}
	return out
}

// ConventionRoles builds the role set a naming convention would stamp, for an IR assembled by hand
// rather than by the ingestion pass (a test fixture, an overlay composing a design in memory). It
// names CONVENTION explicitly rather than leaving the source unspecified, so a hand-built net states
// the same thing the pass would have stated about the same name.
func ConventionRoles(roles ...ir.Role) []*ir.NetRole {
	out := make([]*ir.NetRole, 0, len(roles))
	for _, r := range roles {
		out = append(out, &ir.NetRole{RoleKind: r, Source: ir.RoleSource_ROLE_SOURCE_CONVENTION})
	}
	return out
}

// AddNetRole merges one role fact into a net's set, keeping the STRONGER evidence when that role is
// already present. Every evidence tier merges through it: the ingestion pass (names and format
// declarations) and the params tier (a datasheet's pin functions). A declared ground whose name also
// spells "GND" stays declared, and running a pass twice merges rather than duplicating.
func AddNetRole(n *ir.Net, role ir.Role, src ir.RoleSource) {
	if role == ir.Role_ROLE_UNSPECIFIED {
		return
	}
	for _, r := range n.GetRoles() {
		if r.GetRoleKind() == role {
			if src > r.GetSource() {
				r.Source = src
			}
			return
		}
	}
	n.Roles = append(n.Roles, &ir.NetRole{RoleKind: role, Source: src})
}

// rolesFor is the per-net projection StampNetRoles applies. It emits the role the SOURCE declared
// (if any), then every vocabulary the NAME matches, in a stable order and without repeats. A rail-named
// feedback node ("VCC1V2_FB") carries both rail and feedback, and precedence is the consumer's call.
// The declared role is UNIONED with the name reading rather than replacing it, because a source that
// says GROUND does not thereby say "and nothing else".
func rolesFor(v *RoleVocab, n *ir.Net) []*ir.NetRole {
	stub := &ir.Net{}
	add := func(role ir.Role, src ir.RoleSource) { AddNetRole(stub, role, src) }
	// A token outside the enum is a reader bug. It is dropped because StampNetRoles has no error
	// channel, and AttrDeclaredRole's contract is a valid token or nothing.
	if declared, ok := ParseRole(n.GetAttributes()[AttrDeclaredRole]); ok {
		add(declared, ir.RoleSource_ROLE_SOURCE_DECLARED)
	}
	name := n.GetName()
	if v.IsRail(name) {
		add(ir.Role_ROLE_RAIL, ir.RoleSource_ROLE_SOURCE_CONVENTION)
	}
	if v.IsGround(name) {
		add(ir.Role_ROLE_GROUND, ir.RoleSource_ROLE_SOURCE_CONVENTION)
	}
	if v.IsFeedback(name) {
		add(ir.Role_ROLE_FEEDBACK, ir.RoleSource_ROLE_SOURCE_CONVENTION)
	}
	if v.IsSwitching(name) {
		add(ir.Role_ROLE_SWITCHING, ir.RoleSource_ROLE_SOURCE_CONVENTION)
	}
	if v.IsControl(name) {
		add(ir.Role_ROLE_CONTROL, ir.RoleSource_ROLE_SOURCE_CONVENTION)
	}
	if v.IsGateDrive(name) {
		add(ir.Role_ROLE_GATE_DRIVE, ir.RoleSource_ROLE_SOURCE_CONVENTION)
	}
	return stub.Roles
}
