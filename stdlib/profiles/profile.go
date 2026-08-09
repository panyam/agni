// Package profiles turns a declarative interface definition into check rules (WS3-034). An interface
// profile (SPI-NOR, eMMC, CAN, ...) names its required signals and support needs; Compile generates a
// datalog program per requirement and wraps each with query.RuleFromQuery (WS3-038), so adding an
// interface is a data value, not new code — the lever that collapses ~130 near-identical "verify
// signal X connected" review items into one mechanism (docs/19 §3). The generated datalog uses only
// the merged pin/net relations (component-on-net, the string/pattern predicates, reaches, rail,
// net.pin_count).
//
// A signal is matched by NET NAME, through one of the matcher forms in matcher.go (affix, glob, or
// regex), and the completeness check anchors on a designated always-present signal. A declared-host
// binding (identify the interface's chip and anchor on it) is the complementary path (WS3-042), so a
// wholly-absent interface is silent under the convention path by design (nothing declares it should
// exist yet).
package profiles

import (
	"fmt"
	"maps"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/core/query"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Profile is one interface definition. Name appears in rule names/messages and the "profile" tag;
// Anchor is the net-name suffix of the always-present signal the CONVENTION completeness check hangs
// on when no host is declared.
//
// Host binding (WS3-042): when a component DECLARES this interface via an attribute
// (component.attr(?ref, HostAttrKey, HostAttrVal), e.g. interface=SPI_NOR), an ADDITIONAL
// host-anchored completeness check runs — "this component is the flash, so its bus must have
// CS/SCLK/IO0-3" — precise (anchored on the declared host) and able to flag a WHOLLY-ABSENT bus (a
// host wired to none of its signals), which the convention path cannot. It complements, not
// replaces, the convention path (a design that declares no host, like the ACME EVT, still gets the
// convention + confidence-gate check).
//
// TWO host forms, either or both (WS3-044). The ATTRIBUTE form is a declaration: the design states
// its own intent, so it is authoritative and needs nothing seeded. The CLASS form is an inference
// from the datasheet — a part IS an SPI-NOR flash because its spec says device_class is
// spi_nor_flash — which lets host binding work on a design that carries MPNs but no interface
// annotation. They union: a component matching either is a host.
//
// Identifying a host by MPN prefix was rejected and stays rejected: a hardcoded part-family list is
// unverified and rots. The class form is not that. It reads a fact the datasheet states, and it is
// deliberately keyed on component.device_class rather than component.class, because the latter also
// carries keyword evidence derived from the ref-des and description — which would put the guess back
// in through the side door.
type Profile struct {
	Name    string
	Signals []Signal

	HostAttrKey string // a declared component attribute key (e.g. "interface"); "" = no host binding
	HostAttrVal string // ... and its value (e.g. "SPI_NOR")

	// HostClass binds the host by the DATASHEET's declared device class (e.g. "crystal"), matched
	// against component.device_class. "" = no class binding. It needs a seeded param set: without
	// --params no component has a spec, so no host is found and the host path stays silent rather
	// than guessing (the param tier's standing posture).
	//
	// Both sides go through classify.NormalizeDeviceClass, which folds only what the WS10-015
	// vocabulary KNOWS. For those, case and vendor aliases both fold: a datasheet saying "XTAL" or
	// "Crystal" matches a profile declaring "crystal". A class the vocabulary does NOT recognize
	// passes through unchanged INCLUDING ITS CASE, so "LDO" and "ldo" are different strings and do
	// not match. Two classes already in the seeded corpus ("ldo", "mcu") sit in exactly that
	// position, so a profile binding one of them must spell it as the datasheet does. Widening the
	// vocabulary is WS10-004's canonical-taxonomy work, not this field's.
	HostClass string

	// Requirements is the ordered list of checks this profile declares (WS3-045). Each ref names a
	// registered requirement-type compiler and carries its params; Compile iterates them uniformly.
	// Making the composition DATA (a slice) rather than a hardcoded Compile is what lets a new
	// interface — or a new requirement type like CAN's termination — be a declaration, not engine code.
	Requirements []Requirement
}

// Requirement is one declared check in a Profile: a registered compiler Type (signal-missing,
// missing-pullup, signal-dangling, host-incomplete, termination) plus its Params (empty for most;
// termination names the two bridged net suffixes). Adding a requirement TYPE is registering one
// compiler; adding a requirement to a profile is one slice entry.
type Requirement struct {
	Type   string
	Params map[string]string
}

// TagRequirement is the tag key Compile stamps on every generated rule, carrying the Type of the
// Requirement that produced it. It is the counterpart of the "profile" tag: that one says WHICH
// INTERFACE a rule belongs to, this one says WHICH ASK OF THAT INTERFACE it answers, so a consumer
// can select one requirement's rule instead of the profile's whole compiled set (WS3-115).
//
// Without it the only handle on a single requirement is the rule NAME, which encodes the type by
// convention and not faithfully — the termination requirement compiles to "<profile>-termination-missing"
// and esd to "<profile>-esd-missing" — so name-matching would be a second source of truth that drifts
// the moment a compiler picks a different suffix.
//
// The value is the requirement TYPE, which is unique within a profile by construction rather than by
// convention: two requirements of one type compile to the same rule NAME, and catalog composition
// rejects a duplicate composed name. A profile that would make this key ambiguous therefore already
// fails loudly at composition.
const TagRequirement = "requirement"

// TagProfile is the tag key every generated rule carries, valued with the profile's Name. WS9-041
// groups coverage by it, and catalog supersession selects a whole interface family with it (WS3-056).
const TagProfile = "profile"

// BuiltinSourceName is the catalog source name the built-in profiles register under, so their rules
// compose as "profile/<rule>". It is named here rather than repeated as a literal because supersession
// SELECTS on it: an overlay replaces the built-in reading of an interface and must not reach rules
// that merely share the interface tag from some other source.
const BuiltinSourceName = "profile"

// requirementCompiler turns one declared Requirement on a Profile into a check rule, or nil when the
// requirement does not apply to this profile (no host, no pull-up signal). It emits datalog via the
// query builder — the check LOGIC stays Go (a closed vocabulary); only the COMPOSITION is data.
type requirementCompiler func(Profile, Requirement) *check.Rule

// requirementValidator reports why a requirement's declared params cannot produce the check its type
// promises. It sees only the params, not the Profile: a param is valid or not on its own terms, and
// keeping the signature narrow means a validator cannot quietly grow into a second compiler.
//
// Most requirement types take no params and register none (nil is the norm, not an omission).
type requirementValidator func(params map[string]string) error

// requirementEntry pairs a requirement type's compiler with its optional param validator. The two
// live together because a type that needs params needs both: the compiler consumes them and the
// validator is what stops an incomplete declaration reaching it.
type requirementEntry struct {
	compile  requirementCompiler
	validate requirementValidator // nil when the type takes no params
}

// requirementRegistry maps a requirement Type to its compiler and optional param validator. Built-ins
// are registered here as a package var (initialized before register.go's init calls Compile, so no
// ordering hazard); an overlay adds its own via RegisterRequirement or
// RegisterRequirementWithValidator. The four original WS3-034 requirements plus WS3-045's
// termination, which is the only one taking params and so the only one with a validator.
var requirementRegistry = map[string]requirementEntry{
	"signal-missing":  {compile: func(p Profile, _ Requirement) *check.Rule { return p.signalMissingRule() }},
	"host-incomplete": {compile: func(p Profile, _ Requirement) *check.Rule { return p.hostIncompleteRule() }},
	"missing-pullup":  {compile: func(p Profile, _ Requirement) *check.Rule { return p.pullupRule() }},
	"signal-dangling": {compile: func(p Profile, _ Requirement) *check.Rule { return p.danglingRule() }},
	"termination":     {compile: terminationRule, validate: validateTermination},
	"esd":             {compile: esdRule},
}

// RegisterRequirement adds a requirement-type compiler under name (overwriting any existing entry).
// The extension seam for an out-of-module overlay to ship a proprietary requirement type, same
// open-core posture as check.RegisterSource / formats.Register.
//
// A type registered this way declares no params: any params a profile gives it are passed through to
// the compiler unchecked. Use RegisterRequirementWithValidator when the type needs params, so a
// customer's incomplete declaration is a teaching error at load rather than a surprise inside Compile.
func RegisterRequirement(name string, c func(Profile, Requirement) *check.Rule) {
	requirementRegistry[name] = requirementEntry{compile: c}
}

// RegisterRequirementWithValidator registers a requirement-type compiler together with a validator for
// its params (WS3-047). The validator runs at LOAD time for a YAML-authored profile and at Compile time
// for a Go-literal one, so an incomplete declaration is rejected wherever it arrives; its error text is
// shown to the profile author verbatim, so it should name the params it wanted.
//
// It is a separate function rather than a signature change to RegisterRequirement because that seam is
// public API an out-of-module overlay already builds against.
func RegisterRequirementWithValidator(name string, c func(Profile, Requirement) *check.Rule, v func(params map[string]string) error) {
	requirementRegistry[name] = requirementEntry{compile: c, validate: v}
}

// HasHost reports whether this profile is host-bound (WS3-042): a component must DECLARE the
// interface via HostAttrKey=HostAttrVal for the host completeness path to anchor. A host-bound
// profile whose host is declared nowhere cannot evaluate its host path, which the review gate
// treats distinctly from an absent interface (WS3-090).
func (p Profile) HasHost() bool { return p.HostAttrKey != "" || p.HostClass != "" }

// IsHost reports whether component c is a host of this profile: it carries the declared attribute, or
// its seeded datasheet declares the bound device class. This is the ONE definition of "host", read by
// all three consumers — the datalog hostRule, the review scope (Nets/Components), and HostDeclared.
//
// Keeping them on one predicate is not tidiness. They answer different questions about the same fact,
// and when they disagree the disagreement is invisible: a host the datalog anchors on but the scope
// does not recognise produces findings that HostDeclared simultaneously reports as unevaluable, so a
// review shows an interface both failing and not-automated. Before WS3-044 each consumer tested the
// attribute itself, and adding the class form to only one of them would have created exactly that.
//
// The class path yields nothing without a seeded param set (PartSpec is nil for every component), so a
// class-bound profile is silent rather than wrong on a design read without --params.
func (p Profile) IsHost(m check.Model, c *ir.Component) bool {
	if c == nil {
		return false
	}
	if p.HostAttrKey != "" && c.GetAttributes()[p.HostAttrKey] == p.HostAttrVal {
		return true
	}
	if p.HostClass == "" || m == nil {
		return false
	}
	spec := m.PartSpec(c.GetRefDes())
	if spec == nil || spec.GetDeviceClass() == "" {
		return false
	}
	return classify.NormalizeDeviceClass(spec.GetDeviceClass()) == classify.NormalizeDeviceClass(p.HostClass)
}

// anchorSignal returns the profile's anchor signal (the always-present line the convention
// completeness check hangs on), or nil when none is declared.
func (p Profile) anchorSignal() *Signal {
	for i := range p.Signals {
		if p.Signals[i].Anchor {
			return &p.Signals[i]
		}
	}
	return nil
}

// reqSignalMissing is the convention completeness requirement — the one built-in whose compiler can
// only hang on a declared anchor, hence the validation below.
const reqSignalMissing = "signal-missing"

// validateAnchorDeclared rejects a profile that declares the convention completeness requirement but
// gives signalMissingRule nothing to compile: no anchor signal, or an anchor with no OTHER signal left
// to report missing. Either way the requirement compiles to NOTHING, silently. Paired with any
// requirement that does compile (signal-dangling), the item then runs clean and scores a PASS while the
// check the author asked for never existed — the WS3-099 false-pass shape arriving through author error
// rather than design state. Rejecting it is the same posture Compile already takes for an unsound
// matcher or an unknown requirement type: a declaration that cannot do what it says is a bug in the
// declaration, not a silent no-op.
//
// Scoped to this one requirement type on purpose: an overlay-registered compiler owns its own
// applicability and may legitimately return nil (no host, no pull-up signal), so a blanket
// "every requirement must compile" rule would be wrong.
func validateAnchorDeclared(p Profile) error {
	for _, r := range p.Requirements {
		if r.Type != reqSignalMissing {
			continue
		}
		if p.anchorSignal() == nil {
			return fmt.Errorf("profile %q declares the %q requirement but marks no signal as the anchor: the convention completeness check has nothing to hang on, so it would compile to nothing",
				p.Name, reqSignalMissing)
		}
		if len(p.Signals) < 2 {
			return fmt.Errorf("profile %q declares the %q requirement but has no signal besides the anchor: there is nothing left to report missing, so it would compile to nothing",
				p.Name, reqSignalMissing)
		}
	}
	return nil
}

// anchorSuffix returns the net-name suffix of the profile's anchor signal, or "" when none is
// flagged — and also "" for a glob/regex-matched anchor, which has no suffix.
func (p Profile) anchorSuffix() string {
	for _, s := range p.Signals {
		if s.Anchor {
			return s.Suffix
		}
	}
	return ""
}

// hostRule derives host(?ref): a component that declares this interface via its attribute.
// hostRules are the datalog twin of IsHost: one Def per declared host form, sharing the head, which
// is how datalog spells a union. A profile declaring both is a host under either.
//
// The class clause reads component.device_class, the DATASHEET-declared class, not component.class —
// see the Profile doc for why the difference matters. It is empty without a seeded param set, so the
// clause contributes nothing rather than guessing.
func (p Profile) hostRules() []query.Rule {
	var out []query.Rule
	if p.HostAttrKey != "" {
		out = append(out, query.Def(query.Rel("host", query.V("ref")),
			query.Pos(query.Rel("component.attr", query.V("ref"), query.Str(p.HostAttrKey), query.Str(p.HostAttrVal)))))
	}
	if p.HostClass != "" {
		// Normalized at BUILD time: the relation now projects the canonical key (WS3-044), so the
		// literal compiled in here has to be canonical too or it could never match.
		cl := string(classify.NormalizeDeviceClass(p.HostClass))
		out = append(out, query.Def(query.Rel("host", query.V("ref")),
			query.Pos(query.Rel("component.device_class", query.V("ref"), query.Str(cl)))))
	}
	return out
}

// Signal is one interface member, identified by how its net is NAMED. A signal declares exactly one
// matcher form (WS3-057), all evaluated in matcher.go:
//
//   - Suffix, optionally narrowed by Prefix (conjunctive): the readable default, "the role is the
//     tail of the net name", with the prefix discriminating a bus whose suffix is shared with a
//     foreign one (prefix "PCIE_" + suffix "_TXP", so a UWB serdes _TXP cannot anchor a PCIe check).
//   - Glob: a whole-name shell-style pattern ("ETH_SW*_A_H"), for naming where the identity is the
//     PREFIX and the suffix is generic, which affix matching cannot tell apart.
//   - Regex: an unanchored RE2 escape hatch, for multi-instance naming a glob cannot express.
//
// PullUp marks a line that must reach a rail through a pull-up resistor.
type Signal struct {
	Name   string
	Prefix string
	Suffix string
	Glob   string
	Regex  string
	PullUp bool
	Anchor bool // the always-present signal the convention completeness check hangs on (at most one)
}

// Compile turns a Profile into its check rules by iterating its declared Requirements uniformly:
// each ref is looked up in the requirement registry and its compiler run, dropping the ones that do
// not apply (a nil result — no host, no pull-up signal). There is NO per-requirement special-casing
// here (that was the WS3-034 v0 smell WS3-045 removes); applicability lives in each compiler. The
// rules carry a "profile" tag (Profile.Name) so a consumer can group them by interface.
func Compile(p Profile) []*check.Rule {
	// Every signal must declare exactly one sound matcher before any rule is generated from it: a
	// matcher-less or over-broad signal compiles to a rule that selects every net, which anchors
	// completeness anywhere and reports noise. Parse/Load already reject these for YAML profiles, so
	// this is the gate for a Go-literal one — a programmer error, hence the same panic posture as the
	// unknown-requirement-type check below.
	for _, s := range p.Signals {
		if err := validateSignalMatcher(s); err != nil {
			panic(fmt.Sprintf("profiles: profile %q: %v", p.Name, err))
		}
	}
	// Likewise for a completeness requirement that would compile to nothing (WS3-099). Parse rejects it
	// for YAML profiles; this is the gate for a Go-literal one.
	if err := validateAnchorDeclared(p); err != nil {
		panic("profiles: " + err.Error())
	}
	// And for a requirement whose params cannot produce the check its type promises (WS3-047). Load
	// rejects this for YAML profiles; a Go literal reaches Compile without passing Load, so the gate is
	// repeated here rather than moved — the same twin posture as the two checks above.
	if err := ValidateRequirements(p); err != nil {
		panic("profiles: " + err.Error())
	}
	var rules []*check.Rule
	for _, req := range p.Requirements {
		if r := requirementRegistry[req.Type].compile(p, req); r != nil {
			rules = append(rules, stampRequirement(r, req.Type))
		}
	}
	return rules
}

// stampRequirement records on a compiled rule which Requirement produced it (TagRequirement). It is
// applied HERE rather than inside each compiler for the same reason the profile tag would be if it
// were being added today: the compilers are an open set — an overlay registers its own through
// RegisterRequirement — and a tag every selector depends on cannot be left to each of them to
// remember. Compile is the one place that knows both the rule and the requirement it came from.
//
// The tag map is REPLACED with a copy rather than written in place. A built-in compiler builds its
// map fresh per call (p.tags()), but an out-of-module compiler may hand back a package-level literal
// shared by every rule it emits, and writing through that would make the last requirement's type win
// for all of them. Copying costs one small map per rule at init and removes the whole class.
func stampRequirement(r *check.Rule, reqType string) *check.Rule {
	tags := make(map[string]string, len(r.Tags)+1)
	maps.Copy(tags, r.Tags)
	tags[TagRequirement] = reqType
	r.Tags = tags
	return r
}

func (p Profile) lname() string { return strings.ToLower(strings.ReplaceAll(p.Name, "-", "_")) }

// presenceRules are the IDB rules every requirement shares: has_signal("X") for each present signal,
// and the CONFIDENCE gate in_use — true only when TWO DISTINCT signals of the interface are present.
// Convention (net-name suffix) matching a lone signal is not evidence the interface exists: a real
// corpus has many `_CS` nets (flash, other chip-selects) whose buses are named nothing like SPI-NOR,
// and firing "missing SCLK" on each of them is pure noise. Requiring two matching signals before
// asserting completeness/support makes the profile robust to naming that isn't ours — the stopgap
// until a declared-host binding (WS3-042) removes the guessing entirely.
func (p Profile) presenceRules() []query.Rule {
	var rules []query.Rule
	for _, s := range p.Signals {
		body := append([]query.Literal{query.Pos(query.Rel("component-on-net", query.V("r"), query.V("n")))},
			netMatch(query.V("n"), s)...)
		rules = append(rules, query.Def(query.Rel("has_signal", query.Str(s.Name)), body...))
	}
	rules = append(rules, query.Def(query.Rel("in_use", query.V("x")),
		query.Pos(query.Rel("has_signal", query.V("x"))),
		query.Pos(query.Rel("has_signal", query.V("y"))),
		query.Cmp(query.V("x"), "!=", query.V("y"))))
	return rules
}

func (p Profile) tags() map[string]string {
	return map[string]string{
		check.KeyCategory:     check.CategoryConnectivity,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
		TagProfile:            p.Name, // WS9-041 groups findings by this
	}
}

// signalMissingRule (convention path) fires when the interface is in use — its anchor net exists and
// the in_use confidence gate holds — but a required signal net is absent. A design that declares a
// host additionally gets hostIncompleteRule; this one covers un-annotated designs.
func (p Profile) signalMissingRule() *check.Rule {
	anchorSig := p.anchorSignal()
	if anchorSig == nil {
		// Unreachable for a validated profile (validateAnchorDeclared rejects this at Parse/Compile);
		// kept so a hand-built Profile that bypasses both degrades instead of panicking here.
		return nil
	}
	rules := p.presenceRules()
	// When the profile can bind a host, suppress the convention path on a design that DECLARES one:
	// the precise host path covers it, so a signal is not reported twice (net + component).
	var guard []query.Literal
	if p.HasHost() {
		rules = append(rules, p.hostRules()...)
		rules = append(rules,
			query.Def(query.Rel("any_host", query.Str("y")), query.Pos(query.Rel("host", query.V("ref")))))
		guard = []query.Literal{query.Neg(query.Rel("any_host", query.Str("y")))}
	}
	n := 0
	for _, s := range p.Signals {
		if s.Anchor {
			continue
		}
		n++
		// The anchor net is matched by the anchor signal's FULL convention (suffix + optional prefix),
		// so a prefix-named interface anchors only on its own nets — not a foreign same-suffix serdes.
		body := append([]query.Literal{query.Pos(query.Rel("component-on-net", query.V("r"), query.V("a")))},
			netMatch(query.V("a"), *anchorSig)...)
		body = append(body,
			query.Pos(query.Rel("in_use", query.V("iu"))),
			query.Neg(query.Rel("has_signal", query.Str(s.Name))))
		body = append(body, guard...)
		rules = append(rules, query.Def(query.Rel("missing", query.V("a"), query.Str(s.Name)), body...))
	}
	if n == 0 {
		return nil
	}
	q := query.Build(rules,
		[]query.Literal{query.Pos(query.Rel("missing", query.V("a"), query.V("sig")))},
		query.V("a"), query.V("sig"))
	return query.RuleFromQuery(p.missingFindingQuery("-signal-missing", q, check.KindNet, "a",
		fmt.Sprintf("%s interface (anchored at net {a}) is missing required signal {sig}", p.Name)))
}

// hostIncompleteRule (host path, WS3-042) anchors completeness on a component that DECLARES the
// interface: present_X(?h) is derived when the host connects to a matching net; missing(?h,"X")
// fires per signal the host lacks — so a host wired to none of its bus fires for every signal
// (wholly-absent detection the convention path cannot do). Precise: one finding per host per missing
// signal, no net-name guessing.
func (p Profile) hostIncompleteRule() *check.Rule {
	if !p.HasHost() {
		return nil // requirement declared but the profile binds no host: nothing to anchor on
	}
	rules := p.hostRules()
	for _, s := range p.Signals {
		present := "present_" + s.Name
		presentBody := append([]query.Literal{
			query.Pos(query.Rel("host", query.V("h"))),
			query.Pos(query.Rel("component-on-net", query.V("h"), query.V("n"))),
		}, netMatch(query.V("n"), s)...)
		rules = append(rules,
			query.Def(query.Rel(present, query.V("h")), presentBody...),
			query.Def(query.Rel("missing", query.V("h"), query.Str(s.Name)),
				query.Pos(query.Rel("host", query.V("h"))),
				query.Neg(query.Rel(present, query.V("h")))))
	}
	q := query.Build(rules,
		[]query.Literal{query.Pos(query.Rel("missing", query.V("h"), query.V("sig")))},
		query.V("h"), query.V("sig"))
	return query.RuleFromQuery(p.missingFindingQuery("-host-incomplete", q, check.KindComponent, "h",
		fmt.Sprintf("%s host {h} declares the interface but is missing required signal {sig}", p.Name)))
}

func (p Profile) missingFindingQuery(nameSuffix string, q query.Query, kind, subjectVar, msg string) query.FindingQuery {
	return query.FindingQuery{
		Rule: check.Rule{
			Name:     p.lname() + nameSuffix,
			Severity: "error",
			Summary:  fmt.Sprintf("A required %s signal is absent.", p.Name),
			Impact:   fmt.Sprintf("An %s bus that has some of its signals but not all is a wiring omission: the interface will not work, and it reads at bring-up as a dead peripheral rather than a capture slip.", p.Name),
			Tags:     p.tags(),
			Detail:   ruleDoc("signal-missing"),
		},
		Query:      mustBindHeadFirst(q),
		Kind:       kind,
		SubjectVar: subjectVar,
		Message:    msg,
	}
}

// pullupRule fires when a pull-up signal net reaches no rail (no pull-up resistor to power/ground).
func (p Profile) pullupRule() *check.Rule {
	rules := p.presenceRules()
	needs := 0
	for _, s := range p.Signals {
		if !s.PullUp {
			continue
		}
		needs++
		body := append([]query.Literal{query.Pos(query.Rel("component-on-net", query.V("r"), query.V("n")))},
			netMatch(query.V("n"), s)...)
		rules = append(rules, query.Def(query.Rel("needs_pullup", query.V("n")), body...))
	}
	if needs == 0 {
		return nil
	}
	rules = append(rules,
		// A signal is pulled two ways, and two rules sharing a head is how datalog spells disjunction.
		//
		// The DIRECT form — a resistor sitting on this net and also on a rail — is the shape a pull-up
		// actually has, and it is the one that works on a real board. The reaches form cannot see it
		// (WS3-108): the series walk refuses to cross INTO a net whose fan-out exceeds maxWalkFan, which
		// is right for its own purpose, but a pull-up TERMINATES on a rail and a rail is wide almost by
		// definition. So the one destination this rule needs was the one kind of net the walk would not
		// enter, and `pulled` could not become true however correct the design was.
		//
		// The reaches form is KEPT rather than replaced. It covers the multi-hop cases that work today
		// (a pull-up behind a series element to a narrow rail), and adding a clause can only make more
		// nets pulled — so this change can remove a false positive and cannot introduce a finding.
		// Both clauses open with needs_pullup(?n) so the head variable is BOUND before anything
		// scans (WS3-114). Without it the direct form's first atom enumerates every (component,
		// net) pair on the board and the third re-scans per survivor; the reaches form walks from
		// every net. unpulled already conjoins needs_pullup, so this removes only head facts that
		// were computed and then thrown away.
		query.Def(query.Rel("pulled", query.V("n")),
			query.Pos(query.Rel("needs_pullup", query.V("n"))),
			query.Pos(query.Rel("component-on-net", query.V("pu"), query.V("n"))),
			query.Pos(query.Rel("component.class", query.V("pu"), query.Str("resistor"))),
			query.Pos(query.Rel("component-on-net", query.V("pu"), query.V("rail"))),
			query.Cmp(query.V("rail"), "!=", query.V("n")),
			query.Pos(query.Rel("rail", query.V("rail")))),
		query.Def(query.Rel("pulled", query.V("n")),
			query.Pos(query.Rel("needs_pullup", query.V("n"))),
			query.Pos(query.Rel("reaches", query.V("n"), query.V("rail"))),
			query.Pos(query.Rel("rail", query.V("rail")))),
		query.Def(query.Rel("unpulled", query.V("n")),
			query.Pos(query.Rel("needs_pullup", query.V("n"))),
			query.Pos(query.Rel("in_use", query.V("iu"))),
			query.Neg(query.Rel("pulled", query.V("n")))))
	q := query.Build(rules,
		[]query.Literal{query.Pos(query.Rel("unpulled", query.V("n")))}, query.V("n"))
	return query.RuleFromQuery(query.FindingQuery{
		Rule: check.Rule{
			Name:     p.lname() + "-missing-pullup",
			Severity: "warning",
			Summary:  fmt.Sprintf("A %s signal that needs a pull-up reaches no rail.", p.Name),
			Impact:   "An open-drain or chip-select line with no pull-up floats to an undefined level between drives, so the device can select or clock spuriously at power-up.",
			Tags:     p.tags(),
			Detail:   ruleDoc("missing-pullup"),
		},
		Query:      mustBindHeadFirst(q),
		Kind:       check.KindNet,
		SubjectVar: "n",
		Message:    fmt.Sprintf("%s signal net {n} needs a pull-up but reaches no rail", p.Name),
	})
}

// danglingRule fires when a signal net exists by name but carries fewer than two connections (present
// in the netlist but not actually wired through to both ends of the bus).
func (p Profile) danglingRule() *check.Rule {
	rules := p.presenceRules()
	for _, s := range p.Signals {
		body := append([]query.Literal{query.Pos(query.Rel("component-on-net", query.V("r"), query.V("n")))},
			netMatch(query.V("n"), s)...)
		rules = append(rules, query.Def(query.Rel("sig_net", query.V("n")), body...))
	}
	rules = append(rules, query.Def(query.Rel("dangling", query.V("n")),
		query.Pos(query.Rel("sig_net", query.V("n"))),
		query.Pos(query.Rel("in_use", query.V("iu"))),
		query.Pos(query.Rel("net.pin_count", query.V("n"), query.V("c"))),
		query.Cmp(query.V("c"), "<", query.Num(2))))
	q := query.Build(rules,
		[]query.Literal{query.Pos(query.Rel("dangling", query.V("n")))}, query.V("n"))
	return query.RuleFromQuery(query.FindingQuery{
		Rule: check.Rule{
			Name:     p.lname() + "-signal-dangling",
			Severity: "warning",
			Summary:  fmt.Sprintf("A %s signal net has fewer than two connections.", p.Name),
			Impact:   "A signal that is named but wired to only one pin is a half-made connection: the net exists, so presence checks pass, but the far end of the bus is not actually reached.",
			Tags:     p.tags(),
			Detail:   ruleDoc("signal-dangling"),
		},
		Query:      mustBindHeadFirst(q),
		Kind:       check.KindNet,
		SubjectVar: "n",
		Message:    fmt.Sprintf("%s signal net {n} has fewer than 2 connections (named but not wired through)", p.Name),
	})
}

// mustBindHeadFirst is the WS3-114 guard on every query a requirement compiler generates: no derived
// rule may OPEN with an unbound `reaches`, which walks from every net on the board before any filter
// applies.
//
// It panics rather than returning an error because a violation is an authoring mistake in engine
// code, not a runtime input, and the compilers run at package init — so a bad rule fails loudly the
// moment anything imports profiles, which is the same posture ruleDoc already takes for a missing
// requirement doc. The alternative, discovering it later, is what happened: two rules shipped in that
// shape and made `agni check` non-terminating on a real board while every fixture stayed green,
// because a profile fixture is far too small to show the cost.
//
// It is applied HERE, at the profile compilers, and deliberately not inside query.RuleFromQuery. A
// hand-authored query may legitimately lead with a small relation that omits the head variable when
// that is the cheaper plan; a generated one has no such excuse, because its author cannot see the
// board it will run against.
func mustBindHeadFirst(q query.Query) query.Query {
	if bad := query.GeneratorFirstRules(q); len(bad) > 0 {
		panic(fmt.Sprintf("profiles: generated rule(s) %v open with an unbound reaches, so the walk "+
			"starts from every net on the board and `agni check` will not finish on a real design "+
			"(WS3-114); reorder the body to lead with the guard the consuming rule already conjoins", bad))
	}
	return q
}
