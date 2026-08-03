package classify

import (
	"fmt"
	"regexp"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// The net-role tokens stamped onto ir.Net.roles and read back by the core. A net may carry more than
// one (a rail-named feedback node is both "rail" and "feedback"); consumers decide precedence, the same
// way the device_classes SET (WS3-071) records every matched class and the reader picks the specific one.
const (
	NetRoleRail     = "rail"
	NetRoleGround   = "ground"
	NetRoleFeedback = "feedback"
)

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
	rail, ground, feedback, supplyPin []*regexp.Regexp
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
		// supplyPin: a power-supply INPUT pin name, by prefix (VDD covers VDDA/VDDIO/VDDQ, VCC covers
		// VCCIO). Stricter than rail on purpose: no bare "+", no digit-then-V net form, and VOUT (a
		// supply output) is excluded.
		supplyPin: mustCompileRole(`^(VCC|VDD|VIN|VBAT|VBUS|VSUP|VPP|AVDD|DVDD|VAUX|VCORE|VEE)`),
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
func (v *RoleVocab) IsSupplyPin(name string) bool { return anyRoleMatch(name, v.supplyPin) }

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

// BuildRoleVocab applies per-vocabulary overrides onto DefaultRoleVocab, compiling and VALIDATING every
// pattern — config is operator input, so a bad regex is a returned error, not a bind-time panic. An
// empty override leaves that vocabulary at its default. Patterns are RE2, matched case-insensitively on
// the hierarchy leaf (write ^/$ for whole-leaf anchoring).
func BuildRoleVocab(rail, ground, feedback, supplyPin VocabPatterns) (*RoleVocab, error) {
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
	var err error
	if v.rail, err = build(def.rail, rail); err != nil {
		return nil, fmt.Errorf("rail: %w", err)
	}
	if v.ground, err = build(def.ground, ground); err != nil {
		return nil, fmt.Errorf("ground: %w", err)
	}
	if v.feedback, err = build(def.feedback, feedback); err != nil {
		return nil, fmt.Errorf("feedback: %w", err)
	}
	if v.supplyPin, err = build(def.supplyPin, supplyPin); err != nil {
		return nil, fmt.Errorf("supply_pin: %w", err)
	}
	return &v, nil
}

// StampNetRoles fills each net's roles SET from the active naming lexicon, once at ingestion (WS3-072),
// so the core reads a normalized net.role fact instead of re-running name matching per-net per-rule. The
// loader calls it right after Stamp, so every format is stamped by the same conventions. Idempotent: it
// recomputes and overwrites, so a re-stamp after a re-read is safe. A net matching no vocabulary gets an
// empty set (a plain signal net), so a consumer reads the absence as "no role", the same way an empty
// device_classes reads as "unknown".
func StampNetRoles(d *ir.Design) {
	v := activeRoleVocab
	for _, n := range d.GetNets() {
		n.Roles = rolesFor(v, n.GetName())
	}
}

// rolesFor is the per-net projection StampNetRoles applies: every vocabulary a name matches, in a
// stable order. A rail-named feedback node ("VCC1V2_FB") matches BOTH rail and feedback and carries
// both roles; precedence between them is the consumer's call, not the stamp's.
func rolesFor(v *RoleVocab, name string) []string {
	var out []string
	if v.IsRail(name) {
		out = append(out, NetRoleRail)
	}
	if v.IsGround(name) {
		out = append(out, NetRoleGround)
	}
	if v.IsFeedback(name) {
		out = append(out, NetRoleFeedback)
	}
	return out
}
