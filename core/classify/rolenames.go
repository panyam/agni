package classify

import (
	"fmt"
	"regexp"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// The net-role tokens stamped onto ir.Net.roles and read back by the core. A net may carry more than
// one (a rail-named feedback node is both "rail" and "feedback"), the same way the device_classes SET
// (WS3-071) records every matched class and the reader picks the specific one.
//
// FEEDBACK AND SWITCHING BOTH MEAN "A RAIL-NAMED NET THAT IS NOT A RAIL", and unlike the other roles
// their precedence against rail is NOT the consumer's call. It is decided once, in Model.IsRailNet,
// because leaving it to each consumer is what agni issues 679 and 680 measured: seven rail-quantified
// consumers, of which exactly one remembered to exclude feedback. A regulator's switch node is the
// highest dV/dt net in the design, so a rule that treats it as a rail does not give weaker advice, it
// gives the opposite advice.
const (
	NetRoleRail      = "rail"
	NetRoleGround    = "ground"
	NetRoleFeedback  = "feedback"
	NetRoleSwitching = "switching"
	NetRoleControl   = "control"
	NetRoleGateDrive = "gate_drive"
)

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
func DefaultRoleVocab() *RoleVocab {
	return &RoleVocab{
		// rail: a "+" prefix, the supply-name prefixes, or a "12V"/"3V3"/"5V0" digits-then-V form.
		rail: mustCompileRole(`^\+`, `^(VCC|VDD|VEE|VBUS|VIN|VOUT|VBAT|VSUP|PWR)`, `^[0-9]+V`),
		// ground: GND or EARTH anywhere, or a VSS prefix.
		ground: mustCompileRole(`GND`, `EARTH`, `^VSS`),
		// feedback: a regulator sense node named with an _FB / feedback / sense suffix.
		feedback: mustCompileRole(`_FB$`, `_VFB$`, `_FEEDBACK$`, `_VSENSE$`, `_SENSE$`, `_SNS$`, `^V?FB$`),
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
		switching: mustCompileRole(`_SW$`, `_BOOT$`, `_PHASE$`, `_LX$`),
		// control: a regulator's configuration and enable inputs, named after the rail whose converter
		// they configure. "20V_EN" enables the 20V converter and is driven by whatever logic the
		// sequencer runs at, never by 20V; "12V_MODE1" selects the 12V converter's operating mode.
		//
		// These differ from feedback and switching in WHY they are not rails. Those two are the
		// regulator's power plumbing and must not be probed. These are ordinary control signals that a
		// test point is welcome on. What they share, and the only thing this vocabulary claims, is that
		// the voltage token in the name identifies the CONVERTER rather than the net.
		control: mustCompileRole(`_EN$`, `_MODE\d*$`, `_SEL\d*$`),
		// gateDrive: the supply a regulator's gate driver runs from, again named after the rail the
		// converter produces. Its own right to the word "rail" is arguable, since it genuinely is a
		// supply, which is why it is its own role rather than lumped in with control. What is not
		// arguable is the number: "12V_VDRV" is a gate-drive supply on the 12V converter and sits at
		// whatever that part's driver rail is, commonly 5V.
		gateDrive: mustCompileRole(`_VDRV$`, `_VDRIVE$`, `_VGATE$`),
		// supplyPin: a power-supply INPUT pin name, by prefix (VDD covers VDDA/VDDIO/VDDQ, VCC covers
		// VCCIO). Stricter than rail on purpose: no bare "+", no digit-then-V net form, and VOUT (a
		// supply output) is excluded.
		supplyPin: mustCompileRole(`^(VCC|VDD|VIN|VBAT|VBUS|VSUP|VPP|AVDD|DVDD|VAUX|VCORE|VEE)`),
		// Transistor terminals, whole-name anchored. Anchoring is what keeps them safe even inside
		// the class gate: an unanchored "S" would match SDA, SCLK, SENSE and every other S-name on
		// the part.
		gate:   mustCompileRole(`^G$`, `^GATE$`),
		source: mustCompileRole(`^S$`, `^SOURCE$`, `^SRC$`),
		drain:  mustCompileRole(`^D$`, `^DRAIN$`, `^DRN$`),
	}
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

// RoleVocabConfig is the per-vocabulary override set BuildRoleVocab applies. Named fields rather
// than positional arguments (WS3-117): every vocabulary has the same type, so a positional signature
// makes a transposition compile cleanly and silently cross two vocabularies — the kind of bug that
// surfaces as a rule quietly matching the wrong pin names. An omitted field leaves that vocabulary
// at its default.
type RoleVocabConfig struct {
	Rail      VocabPatterns
	Ground    VocabPatterns
	Feedback  VocabPatterns
	Switching VocabPatterns
	Control   VocabPatterns
	GateDrive VocabPatterns
	SupplyPin VocabPatterns
	Gate      VocabPatterns
	Source    VocabPatterns
	Drain     VocabPatterns
}

// BuildRoleVocab applies per-vocabulary overrides onto DefaultRoleVocab, compiling and VALIDATING every
// pattern — config is operator input, so a bad regex is a returned error, not a bind-time panic. An
// empty override leaves that vocabulary at its default. Patterns are RE2, matched case-insensitively on
// the hierarchy leaf (write ^/$ for whole-leaf anchoring).
func BuildRoleVocab(cfg RoleVocabConfig) (*RoleVocab, error) {
	def := DefaultRoleVocab()
	build := func(base []*regexp.Regexp, o VocabPatterns) ([]*regexp.Regexp, error) {
		var out []*regexp.Regexp
		if !o.Replace {
			out = append(out, base...)
		}
		for _, p := range o.Patterns {
			re, err := regexp.Compile("(?i)" + p)
			if err != nil {
				return nil, fmt.Errorf("pattern %q: %w", p, err)
			}
			out = append(out, re)
		}
		return out, nil
	}
	var v RoleVocab
	for _, d := range []struct {
		name string
		base []*regexp.Regexp
		over VocabPatterns
		dst  *[]*regexp.Regexp
	}{
		{"rail", def.rail, cfg.Rail, &v.rail},
		{"ground", def.ground, cfg.Ground, &v.ground},
		{"feedback", def.feedback, cfg.Feedback, &v.feedback},
		{"switching", def.switching, cfg.Switching, &v.switching},
		{"control", def.control, cfg.Control, &v.control},
		{"gate_drive", def.gateDrive, cfg.GateDrive, &v.gateDrive},
		{"supply_pin", def.supplyPin, cfg.SupplyPin, &v.supplyPin},
		{"gate", def.gate, cfg.Gate, &v.gate},
		{"source", def.source, cfg.Source, &v.source},
		{"drain", def.drain, cfg.Drain, &v.drain},
	} {
		out, err := build(d.base, d.over)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", d.name, err)
		}
		*d.dst = out
	}
	return &v, nil
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
		out = append(out, r.GetRole())
	}
	return out
}

// ConventionRoles builds the role set a naming convention would stamp, for an IR assembled by hand
// rather than by the ingestion pass (a test fixture, an overlay composing a design in memory). It
// names CONVENTION explicitly rather than leaving the source unspecified, so a hand-built net states
// the same thing the pass would have stated about the same name.
func ConventionRoles(roles ...string) []*ir.NetRole {
	out := make([]*ir.NetRole, 0, len(roles))
	for _, r := range roles {
		out = append(out, &ir.NetRole{Role: r, Source: ir.RoleSource_ROLE_SOURCE_CONVENTION})
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
func AddNetRole(n *ir.Net, role string, src ir.RoleSource) {
	if role == "" {
		return
	}
	for _, r := range n.GetRoles() {
		if r.GetRole() == role {
			if src > r.GetSource() {
				r.Source = src
			}
			return
		}
	}
	n.Roles = append(n.Roles, &ir.NetRole{Role: role, Source: src})
}

func rolesFor(v *RoleVocab, n *ir.Net) []*ir.NetRole {
	stub := &ir.Net{}
	add := func(role string, src ir.RoleSource) { AddNetRole(stub, role, src) }
	add(n.GetAttributes()[AttrDeclaredRole], ir.RoleSource_ROLE_SOURCE_DECLARED)
	name := n.GetName()
	if v.IsRail(name) {
		add(NetRoleRail, ir.RoleSource_ROLE_SOURCE_CONVENTION)
	}
	if v.IsGround(name) {
		add(NetRoleGround, ir.RoleSource_ROLE_SOURCE_CONVENTION)
	}
	if v.IsFeedback(name) {
		add(NetRoleFeedback, ir.RoleSource_ROLE_SOURCE_CONVENTION)
	}
	if v.IsSwitching(name) {
		add(NetRoleSwitching, ir.RoleSource_ROLE_SOURCE_CONVENTION)
	}
	if v.IsControl(name) {
		add(NetRoleControl, ir.RoleSource_ROLE_SOURCE_CONVENTION)
	}
	if v.IsGateDrive(name) {
		add(NetRoleGateDrive, ir.RoleSource_ROLE_SOURCE_CONVENTION)
	}
	return stub.Roles
}
