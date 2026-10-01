// Package profiles turns a declarative interface definition into check rules (WS3-034). An interface
// profile (SPI-NOR, eMMC, CAN, ...) names its required signals and support needs. Compile generates a
// datalog program per requirement and wraps each with query.RuleFromQuery (WS3-038), so adding an
// interface is a data value rather than new code, and one mechanism replaces ~130 near-identical
// "verify signal X connected" review items (docsite/content/architecture/rules-and-checks.md). The
// generated datalog uses only the merged pin/net relations (component.net, the string/pattern
// predicates, net.reaches, net.rail, net.pin_count).
//
// A signal is matched by NET NAME through one of the matcher forms in matcher.go (affix, glob, or
// regex), and the completeness check anchors on a designated always-present signal. A declared host
// (WS3-042) is the complementary path. Without one, a wholly-absent interface is silent, since nothing
// declares it should exist.
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

// Profile is one interface definition. Name appears in rule names, messages and the "profile" tag.
// The Signal flagged Anchor is the always-present one the CONVENTION completeness check hangs on when
// no host is declared.
//
// Host binding (WS3-042). When a component DECLARES this interface via an attribute
// (component.attr(?ref, HostAttrKey, HostAttrVal), e.g. interface=SPI_NOR), an ADDITIONAL completeness
// check anchors on it ("this component is the flash, so its bus must have CS/SCLK/IO0-3"). That check
// can flag a WHOLLY-ABSENT bus (a host wired to none of its signals), which the convention path
// cannot. It runs alongside the convention path, so a design that declares no host still gets the
// convention and confidence-gate check.
//
// TWO host forms, either or both, and a component matching either is a host (WS3-044). The ATTRIBUTE
// form is the design stating its own intent, so it is authoritative and needs nothing seeded. The
// CLASS form infers from the datasheet (a part IS an SPI-NOR flash because its spec says device_class
// is spi_nor_flash), so host binding works on a design that carries MPNs but no interface annotation.
//
// Identifying a host by MPN prefix stays rejected, since a hardcoded part-family list is unverified
// and rots. The class form keys on component.device_class rather than component.class because the
// latter also carries keyword evidence from the ref-des and description, which would bring that guess
// back in.
type Profile struct {
	Name    string
	Signals []Signal

	HostAttrKey string // a declared component attribute key (e.g. "interface"); "" = no host binding
	HostAttrVal string // ... and its value (e.g. "SPI_NOR")

	// HostClass binds the host by the DATASHEET's declared device class (e.g. "crystal"), matched
	// against component.device_class. "" = no class binding. It needs a seeded param set. Without
	// --params no component has a spec, so no host is found and the host path stays silent rather
	// than guessing.
	//
	// Both sides go through classify.NormalizeDeviceClass, which folds only what the WS10-015
	// vocabulary KNOWS. For those, case and vendor aliases both fold, so a datasheet saying "XTAL" or
	// "Crystal" matches a profile declaring "crystal". A class the vocabulary does NOT recognize
	// passes through unchanged INCLUDING ITS CASE, so "LDO" and "ldo" do not match. Two classes in the
	// seeded corpus ("ldo", "mcu") are in that position, so a profile binding one of them must spell
	// it as the datasheet does. Widening the vocabulary is WS10-004.
	HostClass string

	// Requirements is the ordered list of checks this profile declares (WS3-045). Each names a
	// registered requirement-type compiler and carries its params, and Compile iterates them
	// uniformly, so a new interface or a new requirement type like CAN's termination is a declaration
	// rather than engine code.
	Requirements []Requirement
}

// Requirement is one declared check in a Profile, a registered compiler Type (signal-missing,
// missing-pullup, signal-dangling, host-incomplete, termination, esd) plus its Params (empty for
// most; termination names the two bridged net suffixes). Adding a requirement TYPE is registering one
// compiler, and adding a requirement to a profile is one slice entry.
type Requirement struct {
	Type   string
	Params map[string]string
}

// TagRequirement is the tag key Compile stamps on every generated rule, valued with the Type of the
// Requirement that produced it. The "profile" tag says which interface a rule belongs to and this one
// says which requirement of that interface it answers, so a consumer can select one requirement's
// rule instead of the profile's whole compiled set (WS3-115).
//
// Do not match on the rule NAME instead. It encodes the type only by convention (termination compiles
// to "<profile>-termination-missing", esd to "<profile>-esd-missing"), so it drifts when a compiler
// picks a different suffix.
//
// The type is unique within a profile, since two requirements of one type compile to the same rule
// name and catalog composition rejects a duplicate composed name.
const TagRequirement = "requirement"

// TagProfile is the tag key every generated rule carries, valued with the profile's Name. WS9-041
// groups coverage by it, and catalog supersession selects a whole interface family with it (WS3-056).
const TagProfile = "profile"

// BuiltinSourceName is the catalog source name the built-in profiles register under, so their rules
// compose as "profile/<rule>". Supersession SELECTS on it, so an overlay replacing the built-in
// reading of an interface does not reach rules that share the interface tag from some other source.
const BuiltinSourceName = "profile"

// requirementCompiler turns one declared Requirement on a Profile into a check rule, or nil when the
// requirement does not apply to this profile (no host, no pull-up signal). Most emit datalog through
// the query builder (pullupRule is the exception). The check LOGIC stays Go, a closed vocabulary, and
// only the COMPOSITION is data.
type requirementCompiler func(Profile, Requirement) *check.Rule

// requirementValidator reports why a requirement's declared params cannot produce the check its type
// promises. It sees only the params, not the Profile, so a validator cannot grow into a second
// compiler. Most requirement types take no params and register none.
type requirementValidator func(params map[string]string) error

// requirementEntry pairs a requirement type's compiler with its optional param validator, which stops
// an incomplete declaration reaching the compiler.
type requirementEntry struct {
	compile  requirementCompiler
	validate requirementValidator // nil when the type takes no params
}

// requirementRegistry maps a requirement Type to its compiler and optional param validator. The
// built-ins are a package var, initialized before register.go's init calls Compile, so there is no
// ordering hazard. An overlay adds its own through RegisterRequirement or
// RegisterRequirementWithValidator. termination is the only built-in taking params, so it is the only
// one with a validator.
var requirementRegistry = map[string]requirementEntry{
	"signal-missing":  {compile: func(p Profile, _ Requirement) *check.Rule { return p.signalMissingRule() }},
	"host-incomplete": {compile: func(p Profile, _ Requirement) *check.Rule { return p.hostIncompleteRule() }},
	"missing-pullup":  {compile: func(p Profile, _ Requirement) *check.Rule { return p.pullupRule() }},
	"signal-dangling": {compile: func(p Profile, _ Requirement) *check.Rule { return p.danglingRule() }},
	"termination":     {compile: terminationRule, validate: validateTermination},
	"esd":             {compile: esdRule},
}

// RegisterRequirement adds a requirement-type compiler under name, overwriting any existing entry.
// It is the extension hook for an out-of-module overlay to ship its own requirement type, like
// check.RegisterSource and formats.Register.
//
// A type registered this way declares no params, and any params a profile gives it reach the compiler
// unchecked. Use RegisterRequirementWithValidator when the type needs params, so an incomplete
// declaration is an error at load rather than a surprise inside Compile.
func RegisterRequirement(name string, c func(Profile, Requirement) *check.Rule) {
	requirementRegistry[name] = requirementEntry{compile: c}
}

// RegisterRequirementWithValidator registers a requirement-type compiler together with a validator for
// its params (WS3-047). The validator runs at LOAD time for a YAML-authored profile and at Compile time
// for a Go-literal one. Its error text is shown to the profile author verbatim, so it should name the
// params it wanted. It is separate from RegisterRequirement to keep that public signature unchanged.
func RegisterRequirementWithValidator(name string, c func(Profile, Requirement) *check.Rule, v func(params map[string]string) error) {
	requirementRegistry[name] = requirementEntry{compile: c, validate: v}
}

// HasHost reports whether this profile is host-bound (WS3-042), by attribute or by device class. A
// host-bound profile whose host is found nowhere cannot evaluate its host path, which the review gate
// treats distinctly from an absent interface (WS3-090).
func (p Profile) HasHost() bool { return p.HostAttrKey != "" || p.HostClass != "" }

// IsHost reports whether component c is a host of this profile, meaning it carries the declared
// attribute or its seeded datasheet declares the bound device class. This is the ONE definition of "host", read by
// the review scope (Nets/Components) and HostDeclared, with hostRules as its datalog twin.
//
// Keep them in step, because a disagreement is invisible. A host the datalog anchors on but the
// scope does not recognise produces findings that HostDeclared reports as unevaluable, so a
// review shows an interface both failing and not-automated (WS3-044).
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

// reqSignalMissing is the convention completeness requirement, the one built-in whose compiler can
// only hang on a declared anchor, hence validateAnchorDeclared.
const reqSignalMissing = "signal-missing"

// validateAnchorDeclared rejects a profile that declares the convention completeness requirement but
// gives signalMissingRule nothing to compile, either no anchor signal or an anchor with no OTHER signal
// to report missing. Either way the requirement compiles to NOTHING, silently, and paired with one that
// does compile (signal-dangling) the item scores a PASS for a check that never existed (WS3-099).
//
// Scoped to this one requirement type because an overlay-registered compiler owns its own
// applicability and may legitimately return nil (no host, no pull-up signal).
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

// anchorSuffix returns the net-name suffix of the profile's anchor signal, or "" when none is flagged
// or the anchor is glob/regex-matched.
func (p Profile) anchorSuffix() string {
	for _, s := range p.Signals {
		if s.Anchor {
			return s.Suffix
		}
	}
	return ""
}

// hostRules derives host(?ref), the datalog twin of IsHost. It emits one Def per declared host form
// sharing the head, which is how datalog spells a union.
//
// The class clause reads component.device_class, the DATASHEET-declared class, not component.class
// (see the Profile doc). It is empty without a seeded param set, so the clause contributes nothing.
func (p Profile) hostRules() []query.Rule {
	var out []query.Rule
	if p.HostAttrKey != "" {
		out = append(out, query.Def(query.Rel("host", query.V("ref")),
			query.Pos(query.Rel("component.attr", query.V("ref"), query.Str(p.HostAttrKey), query.Str(p.HostAttrVal)))))
	}
	if p.HostClass != "" {
		// The relation projects the canonical key (WS3-044), so the literal must be normalized too or
		// it never matches.
		cl := string(classify.NormalizeDeviceClass(p.HostClass))
		out = append(out, query.Def(query.Rel("host", query.V("ref")),
			query.Pos(query.Rel("component.device_class", query.V("ref"), query.Str(cl)))))
	}
	return out
}

// Signal is one interface member, identified by how its net is NAMED. A signal declares exactly one
// matcher form (WS3-057), all evaluated in matcher.go:
//
//   - Suffix, optionally narrowed by Prefix (both must match). The default, with the prefix telling
//     apart buses that share a suffix (prefix "PCIE_" + suffix "_TXP", so a UWB serdes _TXP cannot
//     anchor a PCIe check).
//   - Glob, a whole-name shell-style pattern ("ETH_SW*_A_H"), for naming where the identity is the
//     PREFIX and the suffix is generic.
//   - Regex, an unanchored RE2 pattern for multi-instance naming a glob cannot express.
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

// Compile turns a Profile into its check rules. Each declared Requirement is looked up in the
// requirement registry and its compiler run, dropping the ones that return nil (no host, no pull-up
// signal). Applicability lives in each compiler, with no per-requirement special-casing here
// (WS3-045). The rules carry a "profile" tag (Profile.Name) so a consumer can group them by interface.
//
// It panics on an invalid Go-literal profile, since Parse/Load reject the same cases for YAML.
func Compile(p Profile) []*check.Rule {
	// A matcher-less or over-broad signal compiles to a rule that selects every net, which anchors
	// completeness anywhere and reports noise.
	for _, s := range p.Signals {
		if err := validateSignalMatcher(s); err != nil {
			panic(fmt.Sprintf("profiles: profile %q: %v", p.Name, err))
		}
	}
	// A completeness requirement that would compile to nothing (WS3-099).
	if err := validateAnchorDeclared(p); err != nil {
		panic("profiles: " + err.Error())
	}
	// Params that cannot produce the check their type promises (WS3-047). A Go literal never passes
	// Load, so this repeats Load's check rather than replacing it.
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

// stampRequirement records on a compiled rule which Requirement produced it (TagRequirement). It runs
// in Compile rather than in each compiler because the compilers are an open set (an overlay registers
// its own), and Compile is the one place that knows both the rule and its requirement.
//
// The tag map is REPLACED with a copy rather than written in place. An out-of-module compiler may
// hand back a package-level map shared by every rule it emits, and writing through that would make
// the last requirement's type win for all of them.
func stampRequirement(r *check.Rule, reqType string) *check.Rule {
	tags := make(map[string]string, len(r.Tags)+1)
	maps.Copy(tags, r.Tags)
	tags[TagRequirement] = reqType
	r.Tags = tags
	return r
}

func (p Profile) lname() string { return strings.ToLower(strings.ReplaceAll(p.Name, "-", "_")) }

// presenceRules are the IDB rules every requirement shares, has_signal("X") for each present signal
// and the CONFIDENCE gate in_use, true only when TWO DISTINCT signals of the interface are present.
// One name-matched signal is not evidence the interface exists. A real corpus has many `_CS` nets
// whose buses are named nothing like SPI-NOR, and firing "missing SCLK" on each is noise. A declared
// host (WS3-042) avoids the guess entirely.
func (p Profile) presenceRules() []query.Rule {
	var rules []query.Rule
	for _, s := range p.Signals {
		body := append([]query.Literal{query.Pos(query.Rel("component.net", query.V("r"), query.V("n")))},
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

// signalMissingRule (convention path) fires when the interface is in use (its anchor net exists and
// the in_use confidence gate holds) but a required signal net is absent. It covers un-annotated
// designs, and a design that declares a host gets hostIncompleteRule instead.
func (p Profile) signalMissingRule() *check.Rule {
	anchorSig := p.anchorSignal()
	if anchorSig == nil {
		// Unreachable for a validated profile (validateAnchorDeclared). A hand-built Profile that
		// bypasses validation gets no rule rather than a panic.
		return nil
	}
	rules := p.presenceRules()
	// On a design that DECLARES a host, the host path covers it, so the convention path stands down
	// rather than reporting a signal twice (net + component).
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
		// The anchor net is matched by the anchor signal's FULL matcher (suffix + optional prefix), so a
		// prefix-named interface anchors only on its own nets and not a foreign same-suffix serdes.
		body := append([]query.Literal{query.Pos(query.Rel("component.net", query.V("r"), query.V("a")))},
			netMatch(query.V("a"), *anchorSig)...)
		scope := append(append([]query.Literal{}, body...), query.Pos(query.Rel("in_use", query.V("iu"))))
		scope = append(scope, guard...)
		body = append(body,
			query.Pos(query.Rel("in_use", query.V("iu"))),
			query.Neg(query.Rel("has_signal", query.Str(s.Name))))
		body = append(body, guard...)
		rules = append(rules, query.Def(query.Rel("missing", query.V("a"), query.Str(s.Name)), body...))
		// The considered set is the same body with the has_signal test dropped and the guard kept.
		//
		// The domain is DECLARED, not derived, because the body carries TWO negated literals.
		// `not has_signal(S)` is the condition, and the host guard `not any_host("y")` is part of the
		// scope. "The body minus its negation" cannot tell them apart, and dropping the guard would
		// count every host-annotated design as considered here and pass it twice.
		rules = append(rules, query.Def(query.Rel("sig_scope", query.V("a"), query.Str(s.Name)), scope...))
	}
	if n == 0 {
		return nil
	}
	q := query.Build(rules,
		[]query.Literal{query.Pos(query.Rel("missing", query.V("a"), query.V("sig")))},
		query.V("a"), query.V("sig"))
	domain := query.Build(rules,
		[]query.Literal{query.Pos(query.Rel("sig_scope", query.V("a"), query.V("sig")))},
		query.V("a"), query.V("sig"))
	fq := p.missingFindingQuery("-signal-missing", q, check.KindNet, "a",
		fmt.Sprintf("%s interface (anchored at net {a}) is missing required signal {sig}", p.Name))
	fq.TupleVars = []query.TupleVar{{Var: "sig", Kind: check.KindSignal}}
	fq.Domain = &query.Domain{
		Query:   mustBindHeadFirst(domain),
		Witness: fmt.Sprintf("%s interface (anchored at net {a}) carries required signal {sig}", p.Name),
	}
	return query.MustRuleFromQuery(fq)
}

// hostIncompleteRule (host path, WS3-042) anchors completeness on a component that DECLARES the
// interface. present_X(?h) holds when the host connects to a matching net, and missing(?h,"X") fires
// per signal the host lacks, so a host wired to none of its bus fires for every signal. It reports one
// finding per host per missing signal.
func (p Profile) hostIncompleteRule() *check.Rule {
	if !p.HasHost() {
		return nil // requirement declared but the profile binds no host, so nothing to anchor on
	}
	rules := p.hostRules()
	for _, s := range p.Signals {
		present := "present_" + s.Name
		presentBody := append([]query.Literal{
			query.Pos(query.Rel("host", query.V("h"))),
			query.Pos(query.Rel("component.net", query.V("h"), query.V("n"))),
		}, netMatch(query.V("n"), s)...)
		rules = append(rules,
			query.Def(query.Rel(present, query.V("h")), presentBody...),
			query.Def(query.Rel("missing", query.V("h"), query.Str(s.Name)),
				query.Pos(query.Rel("host", query.V("h"))),
				query.Neg(query.Rel(present, query.V("h")))),
			// The considered set is every (declared host, required signal) pair. No in_use gate, since
			// a declared host IS the evidence the convention path has to infer.
			query.Def(query.Rel("host_scope", query.V("h"), query.Str(s.Name)),
				query.Pos(query.Rel("host", query.V("h")))))
	}
	q := query.Build(rules,
		[]query.Literal{query.Pos(query.Rel("missing", query.V("h"), query.V("sig")))},
		query.V("h"), query.V("sig"))
	domain := query.Build(rules,
		[]query.Literal{query.Pos(query.Rel("host_scope", query.V("h"), query.V("sig")))},
		query.V("h"), query.V("sig"))
	fq := p.missingFindingQuery("-host-incomplete", q, check.KindComponent, "h",
		fmt.Sprintf("%s host {h} declares the interface but is missing required signal {sig}", p.Name))
	fq.TupleVars = []query.TupleVar{{Var: "sig", Kind: check.KindSignal}}
	fq.Domain = &query.Domain{
		Query:   mustBindHeadFirst(domain),
		Witness: fmt.Sprintf("%s host {h} is wired to required signal {sig}", p.Name),
	}
	return query.MustRuleFromQuery(fq)
}

func (p Profile) missingFindingQuery(nameSuffix string, q query.Query, kind, subjectVar, msg string) query.FindingQuery {
	return query.FindingQuery{
		Rule: check.Rule{
			Name:     p.lname() + nameSuffix,
			Severity: "error",
			Summary:  fmt.Sprintf("A required %s signal is absent.", p.Name),
			Impact:   fmt.Sprintf("An %s bus that has some of its signals but not all is a wiring omission: the interface will not work, and it reads at bring-up as a dead peripheral rather than a capture slip.", p.Name),
			Remedy:   requirementRemedy("signal-missing"),
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
//
// IT IS THE ONLY REQUIREMENT THAT COMPILES TO GO RATHER THAN TO A QUERY. It asks whether a bounded
// walk lands on a rail, which datalog could only approximate. Calling check.PullUpVerdict gives it
// the same witness and context entities as the built-in i2c-pull-up, so a pass names the PATH it
// walked ("SCL reaches rail +3V3 through R7") rather than the net's own name (agni issue 516).
// coverage.go's reachesRail calls the same predicate.
//
// C30 allows a requirement type to produce a Go body. The cost is that Reads and Primitives are
// hand-declared here rather than derived from a query body, so they can drift from what the rule does.
func (p Profile) pullupRule() *check.Rule {
	var pullups []Signal
	for _, s := range p.Signals {
		if s.PullUp {
			pullups = append(pullups, s)
		}
	}
	if len(pullups) == 0 {
		return nil
	}
	name := p.Name
	return &check.Rule{
		Name:     p.lname() + "-missing-pullup",
		Severity: "warning",
		Summary:  fmt.Sprintf("A %s signal that needs a pull-up reaches no rail.", name),
		Impact:   "An open-drain or chip-select line with no pull-up floats to an undefined level between drives, so the device can select or clock spuriously at power-up.",
		Remedy:   requirementRemedy("missing-pullup"),
		Tags:     p.tags(),
		Detail:   ruleDoc("missing-pullup"),
		// Hand-declared to match what the datalog form derived and what i2c-pull-up declares for the
		// same walk.
		Reads:               []string{"net.names", "on_net", "component.class"},
		Primitives:          []string{"select", "pattern", "traverse", "exists", "reach"},
		Eval:                p.pullupVerdicts(pullups),
		StatesConsideredSet: true,
	}
}

// pullupVerdicts decides every net this profile declared as needing a pull-up, on a bus in use. The
// considered set is those nets whether or not they are pulled, so a bus absent from the findings
// reached a rail rather than going unexamined.
//
// GATED ON InUse, as the datalog form was, so a profile whose signals are not on this board
// contributes no verdicts.
func (p Profile) pullupVerdicts(pullups []Signal) func(check.Model) []check.Verdict {
	rule := p.lname() + "-missing-pullup"
	name := p.Name
	return func(m check.Model) []check.Verdict {
		if !InUse(m, p) {
			return nil
		}
		var out []check.Verdict
		for _, n := range m.Nets() {
			// A net with NO connections is not a subject, mirroring the datalog form's component.net.
			// On a read whose symbols did not resolve the net NAMES survive and the connections do not,
			// and matching by name alone turned that into four confident findings about buses whose
			// pins the reader never saw. matchSignalNet applies the same condition for coverage.
			if !anySignalMatches(n.GetName(), pullups) || len(n.GetConnections()) == 0 {
				continue
			}
			outcome, w, ctx := check.PullUpVerdict(m, n)
			v := check.Verdict{
				Subjects: []check.Entity{{Kind: check.KindNet, Ref: n.GetName(), NetID: n.GetId()}},
				Rule:     rule, Outcome: outcome, Witness: w, Context: ctx,
			}
			if outcome == check.Fail {
				f := check.NetFinding(fmt.Sprintf("%s signal net %s needs a pull-up but reaches no rail", name, n.GetName()))(n)
				v.Finding = &f
			}
			out = append(out, v)
		}
		return out
	}
}

// anySignalMatches reports whether a net name satisfies any of these signals' full matchers, through
// the same netMatchesSignal the InUse gate applies. The WHOLE matcher keeps a prefix-discriminated
// profile from claiming a foreign net that shares a bare suffix.
func anySignalMatches(net string, signals []Signal) bool {
	for _, s := range signals {
		if netMatchesSignal(net, s) {
			return true
		}
	}
	return false
}

// danglingRule fires when a signal net exists by name but carries fewer than two connections (present
// in the netlist but not actually wired through to both ends of the bus).
func (p Profile) danglingRule() *check.Rule {
	rules := p.presenceRules()
	for _, s := range p.Signals {
		body := append([]query.Literal{query.Pos(query.Rel("component.net", query.V("r"), query.V("n")))},
			netMatch(query.V("n"), s)...)
		rules = append(rules, query.Def(query.Rel("sig_net", query.V("n")), body...))
	}
	rules = append(rules, query.Def(query.Rel("dangling", query.V("n")),
		query.Pos(query.Rel("sig_net", query.V("n"))),
		query.Pos(query.Rel("in_use", query.V("iu"))),
		query.Pos(query.Rel("net.pin_count", query.V("n"), query.V("c"))),
		query.Cmp(query.V("c"), "<", query.Num(2))),
		// The considered set is every signal net of this profile on a bus in use, whatever its pin
		// count. It cannot be derived from the goal, because `dangling` ends in a comparison rather
		// than a negated literal, so "the body minus its negation" is the body itself and would claim
		// the rule considered only the nets it faulted.
		query.Def(query.Rel("dangling_scope", query.V("n")),
			query.Pos(query.Rel("sig_net", query.V("n"))),
			query.Pos(query.Rel("in_use", query.V("iu")))))
	q := query.Build(rules,
		[]query.Literal{query.Pos(query.Rel("dangling", query.V("n")))}, query.V("n"))
	danglingDomain := query.Build(rules,
		[]query.Literal{query.Pos(query.Rel("dangling_scope", query.V("n")))}, query.V("n"))
	return query.MustRuleFromQuery(query.FindingQuery{
		Rule: check.Rule{
			Name:     p.lname() + "-signal-dangling",
			Severity: "warning",
			Summary:  fmt.Sprintf("A %s signal net has fewer than two connections.", p.Name),
			Impact:   "A signal that is named but wired to only one pin is a half-made connection: the net exists, so presence checks pass, but the far end of the bus is not actually reached.",
			Remedy:   requirementRemedy("signal-dangling"),
			Tags:     p.tags(),
			Detail:   ruleDoc("signal-dangling"),
		},
		Query:      mustBindHeadFirst(q),
		Kind:       check.KindNet,
		SubjectVar: "n",
		Message:    fmt.Sprintf("%s signal net {n} has fewer than 2 connections (named but not wired through)", p.Name),
		Domain: &query.Domain{
			Query:   mustBindHeadFirst(danglingDomain),
			Witness: fmt.Sprintf("%s signal net {n} is wired to at least two pins", p.Name),
		},
	})
}

// mustBindHeadFirst guards every query a requirement compiler generates. No derived rule may OPEN with
// an unbound `net.reaches`, which walks from every net on the board before any filter applies (WS3-114),
// and none may be non-injective (WS3-127).
//
// It panics because a violation is an authoring mistake in engine code and the compilers run at
// package init, so a bad rule fails the moment anything imports profiles. Two rules once shipped in
// that shape and made `agni check` non-terminating on a real board while every fixture stayed green.
//
// It is applied here and not in query.RuleFromQuery. A hand-authored query may lead with a small
// relation that omits the head variable when that is the cheaper plan, while a generated query's
// author cannot see the board it will run against.
func mustBindHeadFirst(q query.Query) query.Query {
	if bad := query.NonInjectiveRules(q); len(bad) > 0 {
		panic(fmt.Sprintf("profiles: generated rule(s) %v put two variables in ONE argument position "+
			"of one relation with nothing separating them, so datalog's homomorphic matching lets a "+
			"single node satisfy both (WS3-127); a presence rule in that shape reports an interface "+
			"in use on half the evidence. Add the disequality the body means", bad))
	}
	if bad := query.GeneratorFirstRules(q); len(bad) > 0 {
		panic(fmt.Sprintf("profiles: generated rule(s) %v open with an unbound reaches, so the walk "+
			"starts from every net on the board and `agni check` will not finish on a real design "+
			"(WS3-114); reorder the body to lead with the guard the consuming rule already conjoins", bad))
	}
	return q
}
