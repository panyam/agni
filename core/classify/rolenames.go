package classify

import (
	"fmt"
	"regexp"
	"strings"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// RoleToken renders a role as the lowercase token the query surface and the config lexicon speak:
// ROLE_GATE_DRIVE becomes "gate_drive". It is DERIVED from the generated enum name rather than read
// from a table, so the tokens cannot drift from the vocabulary and a language other than Go can
// perform the identical transform on its own generated enum.
//
// ROLE_UNSPECIFIED renders empty, so a zero value never reaches a fact row looking like a real role.
func RoleToken(r ir.Role) string {
	if r == ir.Role_ROLE_UNSPECIFIED {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(r.String(), "ROLE_"))
}

// ParseRole maps a token back to its role, reporting whether the vocabulary has one. It is the
// LOUD half of a closed vocabulary: a reader translating a source format's own netclass, or a config
// naming a role, gets a false here rather than a value that silently matches nothing. That silence is
// what agni 677 is about on the classification side.
func ParseRole(token string) (ir.Role, bool) {
	for _, r := range AllNetRoles() {
		if RoleToken(r) == token {
			return r, true
		}
	}
	return ir.Role_ROLE_UNSPECIFIED, false
}

// AllNetRoles is every role the engine stamps, in declaration order and without the zero value. It
// reads the GENERATED enum rather than a hand-kept list, so a role added to the proto is projected,
// parseable and configurable without anyone remembering a second place.
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
// It exists because a role can be KNOWN rather than guessed. Most formats make the engine read a
// net's purpose out of its name, which is why RoleVocab exists at all; IPC-2581 instead declares it
// on LogicalNet/@netClass as a closed enum, so a net called "N$17" can be authoritatively GROUND
// with nothing in the name to go on. Discarding that in favour of a name guess would be a strictly
// worse read.
//
// The translation happens in the READER, not here: C9's left-shift rule puts convention
// interpretation at the edge and keeps normalized facts in the core, so this package never learns a
// format's enum. A second format that declares roles writes this same key. An UNMAPPABLE source
// term is simply not written (the reader keeps it verbatim in its own attribute), so this key always
// holds a valid NetRole token or is absent.
//
// Kept in the open attributes map rather than a typed field on purpose: C9 admits a typed semantic
// field only once a second format populates it, and one format declares roles today.
const AttrDeclaredRole = "declared_role"

// RoleVocab is the naming lexicon: the regex sets that decide a net's electrical ROLE by name (rail,
// ground, feedback), for the cases where a directionless netlist carries the name as the only
// evidence. It exists so these heuristics stop being frozen Go literals: a project whose house naming
// differs can extend or replace each vocabulary via config (WS3-069) instead of patching the engine.
//
// It lives in classify (moved from check by WS3-072) so the ingestion pass can apply it without
// importing check, the same relocation the class lexicon made in WS3-071; check re-exports the names.
//
// Matching is on the hierarchy LEAF ("/psu/12V" -> "12V", the WS3-006 convention) and case-insensitive.
// The zero value is not usable; build one with DefaultRoleVocab (optionally then Extend/Replace).
//
// rail/ground/feedback classify NET names; supplyPin classifies a component's PIN names (which supply
// pin a part CONSUMES, for the WS3-072 POWER_IN stamp). supplyPin is a DISTINCT, stricter vocabulary,
// not a reuse of rail: a supply PIN is named VDD/VIN, never "3V3", and a bare "+" is a polarized-part
// terminal, not a supply — so the net-name forms (^+, digit-then-V) and the supply OUTPUT name VOUT are
// deliberately absent from it. It lives here so both the net-role and pin-supply naming conventions are
// one config-overridable lexicon (WS3-069), not a frozen literal.
type RoleVocab struct {
	// The patterns this vocabulary was built from, so a caller can read back the EFFECTIVE lexicon a
	// project installed rather than only asking it yes/no questions. ActiveRoleVocab's doc has promised
	// that inspection since WS3-069 and could not deliver it while the patterns were discarded at
	// compile time.
	//
	// Embedded by POINTER, deliberately. A protobuf-go message carries a MessageState holding a
	// DoNotCopy, so embedding by value makes every copy of a RoleVocab a vet copylocks violation.
	*configpb.NamingLexicon

	// The compiled form, derived from the patterns above at construction and never written again.
	// That immutability is the whole safety argument for the embedding: a decorator over a mutable
	// value would be two sources of truth wearing one struct, where a derived-and-frozen half cannot
	// go stale. Replace a vocabulary by building a new one (SetActiveRoleVocab), never by reaching in.
	rail, ground, feedback, switching, control, gateDrive, supplyPin []*regexp.Regexp
	// Transistor TERMINAL pin names (WS3-117), each its own vocabulary because the three are
	// independent conventions a house can spell differently (a gate is "G", "GATE", sometimes "DRV"
	// on a driver). They are consumed ONLY where the component's class is a transistor — see
	// classifyPinRole — because these are the shortest, most collision-prone pin names on a board:
	// bare "S" and "D" mean something on almost every part, and an ungated match would mis-role most
	// of a design. A wrong role is worse than a missing one, since a topology rule then walks a path
	// that does not exist.
	gate, source, drain []*regexp.Regexp
}

func mustCompileRole(pats ...string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(pats))
	for i, p := range pats {
		out[i] = regexp.MustCompile("(?i)" + p)
	}
	return out
}

// DefaultRoleVocab is the built-in lexicon: the rail/ground/feedback conventions the engine ships,
// the historical Go literals re-expressed as RE2. A project's config merges onto (or replaces) these.
// defaultLexicon is the built-in naming policy, expressed in the SAME schema a project's
// conventions.yaml carries. It is a proto rather than Go literals so there is exactly one answer to
// "which vocabularies exist and what do they hold", readable by the engine, the wire and the viewer
// alike (C2). A hand-written Go twin of this shape is what let three vocabularies go missing twice:
// gate/source/drain in WS3-117, and switching/control/gate_drive in agni 680.
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
			// switching: a regulator's power-stage node, which inherits a rail's name because it is named
			// after the rail it produces. _SW and _PHASE are the switch node, _BOOT the bootstrap cap node
			// riding above it, _LX the same node in the vendor spelling most common outside the US.
			//
			// A LOAD SWITCH OUTPUT COLLIDES WITH THIS and the name alone cannot separate them: "3V3_SW"
			// is a switch node on a buck and a switched rail on a load switch, and both are real house
			// conventions. Reading it as a regulator internal is the safe direction, because the cost of
			// being wrong is one rail going unprobed while the other way round asks a factory test to put
			// a probe on a node swinging to the input rail at the switching frequency. A project whose
			// convention is the other one narrows this vocabulary in conventions.yaml (WS3-069).
			Switching: vp(`_SW$`, `_BOOT$`, `_PHASE$`, `_LX$`),
			// control: a regulator's configuration and enable inputs, named after the rail whose converter
			// they configure. "20V_EN" enables the 20V converter and is driven by whatever logic the
			// sequencer runs at, never by 20V; "12V_MODE1" selects the 12V converter's operating mode.
			//
			// These differ from feedback and switching in WHY they are not rails. Those two are the
			// regulator's power plumbing and must not be probed. These are ordinary control signals that a
			// test point is welcome on. What they share, and the only thing this vocabulary claims, is that
			// the voltage token in the name identifies the CONVERTER rather than the net.
			Control: vp(`_EN$`, `_MODE\d*$`, `_SEL\d*$`),
			// gateDrive: the supply a regulator's gate driver runs from, again named after the rail the
			// converter produces. Its own right to the word "rail" is arguable, since it genuinely is a
			// supply, which is why it is its own role rather than lumped in with control. What is not
			// arguable is the number: "12V_VDRV" is a gate-drive supply on the 12V converter and sits at
			// whatever that part's driver rail is, commonly 5V.
			GateDrive: vp(`_VDRV$`, `_VDRIVE$`, `_VGATE$`),
		},
		Pin: &configpb.PinNameVocab{
			// supplyPin: a power-supply INPUT pin name, by prefix (VDD covers VDDA/VDDIO/VDDQ, VCC covers
			// VCCIO). Stricter than rail on purpose: no bare "+", no digit-then-V net form, and VOUT (a
			// supply output) is excluded.
			Supply: vp(`^(VCC|VDD|VIN|VBAT|VBUS|VSUP|VPP|AVDD|DVDD|VAUX|VCORE|VEE)`),
			// Transistor terminals, whole-name anchored. Anchoring is what keeps them safe even inside
			// the class gate: an unanchored "S" would match SDA, SCLK, SENSE and every other S-name on
			// the part.
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

// IsRail / IsGround / IsFeedback classify a NET name against the vocabulary; IsSupplyPin classifies a
// component's PIN name as a power-supply input (a distinct, stricter vocabulary — see the type doc).
func (v *RoleVocab) IsRail(name string) bool      { return anyRoleMatch(name, v.rail) }
func (v *RoleVocab) IsGround(name string) bool    { return anyRoleMatch(name, v.ground) }
func (v *RoleVocab) IsFeedback(name string) bool  { return anyRoleMatch(name, v.feedback) }
func (v *RoleVocab) IsSwitching(name string) bool { return anyRoleMatch(name, v.switching) }
func (v *RoleVocab) IsControl(name string) bool   { return anyRoleMatch(name, v.control) }
func (v *RoleVocab) IsGateDrive(name string) bool { return anyRoleMatch(name, v.gateDrive) }
func (v *RoleVocab) IsSupplyPin(name string) bool { return anyRoleMatch(name, v.supplyPin) }

// IsGate / IsSource / IsDrain classify a TRANSISTOR's pin name. The caller is responsible for the
// class gate: these vocabularies are deliberately short and would collide badly if applied to any
// part (see the type doc).
func (v *RoleVocab) IsGate(name string) bool   { return anyRoleMatch(name, v.gate) }
func (v *RoleVocab) IsSource(name string) bool { return anyRoleMatch(name, v.source) }
func (v *RoleVocab) IsDrain(name string) bool  { return anyRoleMatch(name, v.drain) }

// activeRoleVocab is the process-level lexicon every classifier consults. It is deployment/project
// config (set once at startup from --conventions, immutable after), not per-design, so a package
// default is the right shape. The is*Name helpers (in check) delegate here so their call sites are
// unchanged, and StampNetRoles reads it at ingestion.
var activeRoleVocab = DefaultRoleVocab()

// SetActiveRoleVocab replaces the process-level lexicon (the CLI calls this after loading a project's
// --conventions lexicon block). Passing nil restores the defaults. It must run before ingestion
// (ReadDesign), since StampNetRoles stamps net.role with the active vocab.
func SetActiveRoleVocab(v *RoleVocab) {
	if v == nil {
		v = DefaultRoleVocab()
	}
	activeRoleVocab = v
}

// ActiveRoleVocab returns the process-level lexicon currently in effect (the defaults unless a project
// config replaced it). Exposed so a caller can inspect what a --conventions lexicon installed.
func ActiveRoleVocab() *RoleVocab { return activeRoleVocab }

// BuildRoleVocab merges a project's overrides onto the built-in lexicon and compiles the result,
// VALIDATING every pattern — config is operator input, so a bad regex is a returned error rather than
// a bind-time panic. A nil override yields the built-ins. Patterns are RE2, matched case-insensitively
// on the hierarchy leaf (write ^/$ for whole-leaf anchoring).
//
// It takes the generated config message rather than a Go mirror of it. The mirror it replaced had one
// production caller whose only job was to copy ten fields across by hand, and that copy silently lost
// a vocabulary twice.
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

// mergeVocab applies one vocabulary's override: patterns are ADDED to the built-ins unless replace is
// set, in which case they become the whole set. Replace with no patterns is a deliberate empty
// vocabulary, which is how a project turns a built-in role off.
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

// StampNetRoles fills each net's roles SET from the active naming lexicon, once at ingestion (WS3-072),
// so the core reads a normalized net.role fact instead of re-running name matching per-net per-rule. The
// loader calls it right after Stamp, so every format is stamped by the same conventions. Idempotent: it
// recomputes and overwrites, so a re-stamp after a re-read is safe. A net matching no vocabulary gets an
// empty set (a plain signal net), so a consumer reads the absence as "no role", the same way an empty
// device_classes reads as "unknown".
// It stamps from the PROCESS-level lexicon; a read carrying its own conventions calls
// (*Lexicon).StampNetRoles instead (WS3-106).
func StampNetRoles(d *ir.Design) { ActiveLexicon().StampNetRoles(d) }

// rolesFor is the per-net projection StampNetRoles applies: the role the SOURCE declared (if any),
// then every vocabulary the NAME matches, in a stable order and without repeats. A rail-named
// feedback node ("VCC1V2_FB") matches BOTH rail and feedback and carries both roles; precedence
// between them is the consumer's call, not the stamp's.
//
// The declared role goes first because it is evidence rather than inference, and it is UNIONED with
// the name reading rather than replacing it: the two answer the same question from different
// sources, and a source that says GROUND does not thereby say "and nothing else". A design can
// legitimately declare a net GROUND while naming it something the feedback vocabulary also matches.
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
// already present. It is the single home of what a duplicate means, shared by every evidence tier:
// the ingestion pass that reads names and format declarations, and the params tier that adds what a
// datasheet's pin functions establish.
//
// The merge is why a role can be established twice without the weaker source overwriting the
// stronger. A declared ground whose name also spells "GND" is declared, not a convention; recording
// the weaker of two true sources would understate what is known and is the one way this can lose
// information. It also makes every tier idempotent, so running a pass twice over one design merges
// rather than duplicating.
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

func rolesFor(v *RoleVocab, n *ir.Net) []*ir.NetRole {
	stub := &ir.Net{}
	add := func(role ir.Role, src ir.RoleSource) { AddNetRole(stub, role, src) }
	// The declared role arrives as TEXT, because a reader translates its format's own vocabulary at the
	// edge and writes the token into an attribute (C9). This is the one place a string crosses into the
	// closed vocabulary, so it is the one place a parse can fail. A failure means a reader wrote a token
	// outside the enum, which is a reader bug rather than a property of the design; it is dropped here
	// because StampNetRoles has no error channel, and AttrDeclaredRole's contract is that a reader
	// writes a valid token or writes nothing.
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
