package review

import (
	"fmt"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	"github.com/panyam/agni/core/param"
)

// Outcome is how one checklist item resolved on a design, along TWO axes (WS10-014). COVERAGE asks
// whether a mechanism exists, and everything but NotAutomated is covered. RATIFICATION asks whether
// the data the mechanism ran on is trustworthy. An item no shipped rule covers must never read as
// passed.
type Outcome string

const (
	Pass          Outcome = "pass"           // the item's rule(s) ran on ratified data and found nothing
	Fail          Outcome = "fail"           // the item's rule(s) fired on ratified (trustworthy) data
	NotApplicable Outcome = "not-applicable" // the item's rule(s) need a fact tier this design lacks
	NotAutomated  Outcome = "not-automated"  // no shipped rule/mechanism covers the item (or it has no binding)

	// Provisional means the item's rule(s) fired, but every finding ran on UNRATIFIED datasheet data
	// (method "mock" or confidence below the floor), so it is not a trustworthy fail yet. This set is
	// the HITL ratification worklist, derived from Finding.DatasheetProv (WS10-012). A single ratified
	// or netlist finding among them makes it a Fail.
	Provisional Outcome = "provisional"
	// NeedsDesignIntent means the item binds an `intent/`-namespaced rule (WS3-084) but no design-intent
	// declaration was supplied, so the rule is absent from the catalog. It is COVERED and blocked on a
	// per-design input. Supplying --intent-path flips it to pass/fail.
	NeedsDesignIntent Outcome = "needs-design-intent"
	// ComputedNA is a device-class-gated item (applies_to_class) whose class matches no component on
	// the design. Unlike NotApplicable (a missing fact TIER), the tier is present and the mechanism
	// computed that the check does not apply.
	ComputedNA Outcome = "computed-n/a"
	// NeedsData means the item's mechanism ran, but the specific data it joins against is absent, so it
	// could not evaluate (WS3-097). The derivable case is a datasheet symbol seeded on no component:
	// --params supplied the tier, so Available does not gate, and nothing matched because the value is
	// unseeded rather than because the design is clean. It is COVERED, like NeedsDesignIntent.
	NeedsData Outcome = "needs-data"
	// Inconclusive means the item's rule(s) ran with everything they needed, examined a specific
	// subject, and could not decide (agni issue 74). It is the only outcome on the RESULT side of a
	// rule. Every other non-pass state above is a design-wide PRECONDITION. It is not folded into
	// NeedsData because some of what it covers is not a data gap (a netlist states reset polarity
	// nowhere), and needs-data would send a reviewer to supply something that does not exist.
	//
	// It is COVERED and NEVER a pass. The per-finding Message carries the remedy.
	Inconclusive Outcome = "inconclusive"
)

// ItemResult is one item's outcome, with the findings that made it fail (or the reason it did not
// apply).
type ItemResult struct {
	Item     Item
	Outcome  Outcome
	Findings []check.Finding
	Note     string // the not-applicable reason (from check.Available), when Outcome is NotApplicable
	// Unmet names the datasheet facts this item needed and did not find, set only when Outcome is
	// NeedsData. It is the machine-readable form of the Note.
	Unmet []check.UnmetDependency
}

// AreaResult groups the item results of one review area.
type AreaResult struct {
	Area  Area
	Items []ItemResult
}

// Report is the per-design review result, carrying the manifest and design names plus each area's
// item outcomes. The markdown, JSON and HTML renderers all read it.
type Report struct {
	Manifest string
	Design   string
	Areas    []AreaResult
}

// Presence is the review's evaluability verdict for an interface a profile item names, saying whether
// the profile's rules will GENUINELY evaluate on this design (WS3-090). It is not a bare "on the
// board" bool, because a rule that could not evaluate must not score as a clean pass.
type Presence int

const (
	// IfaceAbsent means the interface is not on the design (its convention is not in use and no host
	// is declared). Its rules have nothing to fire on, so the item is not-applicable.
	IfaceAbsent Presence = iota
	// IfaceHostUnsatisfied means the profile is host-bound, no component declares the host, AND its
	// signal convention is not in use, so the host check cannot evaluate and the item is not-automated.
	// When the convention IS in use the verdict is IfacePresent instead, so a checkable un-annotated
	// design still runs.
	IfaceHostUnsatisfied
	// IfacePresent means the convention is in use AND its completeness anchor is matched (or a host is
	// declared), so the rules run for pass/fail.
	IfacePresent
	// IfaceConventionUnmatched is the convention path's sibling of IfaceHostUnsatisfied (WS3-099).
	// Enough signals match to clear in_use, but the anchor signal is absent, so the completeness rule
	// cannot evaluate. The diagnosis is "this looks like the interface but the naming does not match".
	//
	// Unlike the two verdicts above this does NOT stop the item running, because a profile's secondary
	// rules (signal-dangling, missing-pullup) gate on in_use alone. A real finding still reads fail,
	// and only a would-be PASS is replaced.
	IfaceConventionUnmatched
)

// PresenceFunc reports the review's evaluability verdict for an interface (by profile Name) and
// whether the name is a KNOWN interface, meaning a profile or presence-only declaration of that name
// is loaded (built-in or via --profile-path). An UNKNOWN name leaves the item running, so it can still
// read not-automated when nothing covers it. nil disables the gate. It is a function so `review` does
// not import `profiles`, and the service wires it (profiles.InUse / HostDeclared).
type PresenceFunc func(profileName string) (verdict Presence, known bool)

// ScopeFunc returns the set of net names belonging to a profile, so a scoped binding (WS3-058) keeps
// only the bound rule's findings on that interface's nets. nil disables the net side of scoping. The
// service wires it (profiles.Nets).
type ScopeFunc func(profileName string) map[string]bool

// CompScopeFunc returns the set of component RefDes belonging to a profile, so a scoped binding keeps
// a COMPONENT-subject finding on the interface's parts, which ScopeFunc alone drops (WS3-083). nil
// disables the component side. The service wires it (profiles.Components). A scoped item is
// unfiltered only when BOTH Scope and CompScope are nil.
type CompScopeFunc func(profileName string) map[string]bool

// RunParams carries everything Run needs, as a struct so a new capability adds a field rather than
// changing every caller (WS3-083).
type RunParams struct {
	Model     check.Model
	Catalog   *check.Catalog
	Manifest  Manifest
	Design    string
	Present   PresenceFunc  // nil disables the interface-absence check (every item runs)
	Scope     ScopeFunc     // nil disables net scoping
	CompScope CompScopeFunc // nil disables component scoping
	// RatifiedFloor is the datasheet-confidence floor below which a finding's data is "unratified"
	// (WS10-014), so a fail whose findings are all mock or below it is Provisional. Zero means
	// DefaultRatifiedFloor, since a floor of 0 would rate everything trustworthy. The CLI exposes it
	// as --ratified-floor.
	RatifiedFloor float64
	// IntentRuleKnown reports whether an intent/-namespaced rule NAME is one the intent compiler can
	// produce (WS3-098). It separates a real-but-undeclared intent rule (needs-design-intent) from a
	// not-yet-shipped name a manifest pre-bound (not-automated), which a prefix test cannot. nil treats
	// every intent/ name as known. The service wires intent.Emits.
	IntentRuleKnown func(ruleName string) bool
	// Vocabulary is the relation vocabulary an inline query compiles against: the shipped one plus
	// the run's library (agni issue 779). Nil is the process default.
	Vocabulary *facts.Registry
}

// DefaultRatifiedFloor is the confidence at or above which a datasheet value counts as ratified,
// matching WS10-012's "confidence < 0.9 is (verify)" report convention. A finding below it (or method
// "mock") makes an otherwise-failing item Provisional.
const DefaultRatifiedFloor = 0.9

func (p RunParams) ratifiedFloor() float64 {
	if p.RatifiedFloor <= 0 {
		return DefaultRatifiedFloor
	}
	return p.RatifiedFloor
}

// Run evaluates a review manifest against a design's Model, selecting each item's rules from the
// composed catalog (so overlay profiles from --profile-path are in scope) and resolving each to an
// Outcome. Rules whose fact tier is absent are not-applicable rather than run, and so is a profile
// item whose interface Present reports absent.
func Run(p RunParams) Report {
	rep := Report{Manifest: p.Manifest.Name, Design: p.Design}
	for _, a := range p.Manifest.Areas {
		ar := AreaResult{Area: a}
		for _, it := range a.Items {
			ar.Items = append(ar.Items, runItem(p, it))
		}
		rep.Areas = append(rep.Areas, ar)
	}
	return rep
}

func runItem(p RunParams, it Item) ItemResult {
	m, cat, present := p.Model, p.Catalog, p.Present
	// A present: binding is never not-applicable (the component-class tier exists on any netlist), so it
	// resolves ahead of the paths below.
	if pb := it.Binding.Present; pb != nil {
		return presentResult(m, it, pb)
	}
	// Interface absence takes precedence over every other outcome, so a KNOWN absent interface is
	// not-applicable whether or not a rule checks it. Checked BEFORE the not-automated shortcut so one
	// gate covers a full profile whose bus is absent (WS3-051) and a PRESENCE-ONLY declaration, which
	// compiles to zero rules (WS3-068). An UNKNOWN interface falls through to not-automated. Several
	// named interfaces are not-applicable only when EVERY one is known-absent.
	ifaces := it.Binding.Scope.names()
	if it.Binding.Profile != "" {
		ifaces = []string{it.Binding.Profile}
	}
	// unmatched defers the WS3-099 verdict to the zero-findings tail, because the secondary rules still
	// evaluate on a partly-used convention and a real finding must survive.
	unmatched := false
	if len(ifaces) > 0 && present != nil {
		runs, hostUnsatisfied := false, false
		for _, iface := range ifaces {
			v, known := present(iface)
			if !known || v == IfacePresent {
				runs = true
				break
			}
			switch v {
			case IfaceHostUnsatisfied:
				hostUnsatisfied = true
			case IfaceConventionUnmatched:
				unmatched = true
			}
		}
		// A convention-unmatched interface outranks the other two non-running verdicts. It runs the
		// rules, so its verdict is decided at the bottom.
		if !runs && !unmatched {
			// A rule that could not evaluate must not score PASS (WS3-090).
			if hostUnsatisfied {
				return ItemResult{Item: it, Outcome: NotAutomated, Note: "host-bound interface declared on no component"}
			}
			return ItemResult{Item: it, Outcome: NotApplicable, Note: "interface not present on this design"}
		}
	}
	// Device-class computed-n/a (WS10-014), read off the device-class fact (WS10-013/015). Checked
	// before catalog resolution so it holds even for an item whose rule is not yet shipped.
	if classes := it.Binding.AppliesToClass; len(classes) > 0 && !anyComponentHasClass(m, classes) {
		return ItemResult{Item: it, Outcome: ComputedNA, Note: "no " + strings.Join(classes, "/") + " part on this design"}
	}
	rules := resolve(cat, it, p.Vocabulary)
	if len(rules) == 0 {
		// An intent-bound item (WS3-084) resolves to zero rules when no declaration was supplied, because
		// the intent rule is then absent from the catalog. It is COVERED, so it names --intent-path.
		if bindsIntent(it, p.IntentRuleKnown) {
			return ItemResult{Item: it, Outcome: NeedsDesignIntent, Note: "needs a design-intent declaration (--intent-path)"}
		}
		// The interface is present (or none is named) but nothing shipped checks it.
		return ItemResult{Item: it, Outcome: NotAutomated}
	}
	var avail []*check.Rule
	var reason string
	for _, r := range rules {
		if ok, why := check.Available(r, m); ok {
			avail = append(avail, r)
		} else {
			reason = why
		}
	}
	if len(avail) == 0 {
		return ItemResult{Item: it, Outcome: NotApplicable, Note: reason}
	}
	fs := check.Run(m, avail)
	// A scoped binding keeps only findings for the named interfaces (their UNION), meaning a net-subject
	// finding on one of their nets (WS3-058) or a component-subject finding on one of their parts
	// (WS3-083).
	if names := it.Binding.Scope.names(); len(names) > 0 && (p.Scope != nil || p.CompScope != nil) {
		nets := map[string]bool{}
		comps := map[string]bool{}
		for _, nm := range names {
			if p.Scope != nil {
				for n := range p.Scope(nm) {
					nets[n] = true
				}
			}
			if p.CompScope != nil {
				for c := range p.CompScope(nm) {
					comps[c] = true
				}
			}
		}
		fs = filterToScope(fs, nets, comps)
	}
	// A subject the rule gave up on can make the item neither fail nor pass. Split before the fail
	// branch so a rule may emit both kinds at once and the real defect still wins.
	fs, undecided := splitInconclusive(fs)
	if len(fs) > 0 {
		// Data-trust axis (WS10-014). A fail whose findings ALL ran on unratified datasheet data is
		// Provisional. One ratified datasheet finding, or any netlist finding (no DatasheetProv), makes
		// it a real Fail.
		if allUnratified(fs, p.ratifiedFloor()) {
			return ItemResult{Item: it, Outcome: Provisional, Findings: fs}
		}
		return ItemResult{Item: it, Outcome: Fail, Findings: fs}
	}
	// Zero findings is only a pass if the check could evaluate (WS3-097). Only the datasheet case is
	// gated, since with --params supplied check.Available did not gate and a symbol seeded on no
	// component is the silent gap. The general "all joined relations empty" case is not chased,
	// because the params tier is the one sparse by nature.
	if syms := datasheetSymbols(it, avail); len(syms) > 0 && !check.SeedsAnySymbol(m, syms) {
		return ItemResult{
			Item:    it,
			Outcome: NeedsData,
			Note:    "no seeded datasheet value for " + strings.Join(syms, "/") + " on this design",
			Unmet:   check.UnseededSymbols(m, syms, it.Binding.AppliesToClass),
		}
	}
	// The unanchored interface (WS3-099) ran its secondary rules clean, but the completeness rule never
	// evaluated. It reads not-automated rather than a needs-* state because no shipped mechanism covers
	// THIS design's naming, and scoring it covered would inflate the coverage axis.
	if unmatched {
		return ItemResult{Item: it, Outcome: NotAutomated, Note: "interface named but its completeness anchor signal is absent, so the convention check could not evaluate"}
	}
	// No defect, but at least one undecided subject, so it does not read pass (agni issue 74).
	if len(undecided) > 0 {
		return ItemResult{Item: it, Outcome: Inconclusive, Findings: undecided,
			Note: "the check ran but could not decide for " + subjectList(undecided)}
	}
	return ItemResult{Item: it, Outcome: Pass}
}

// splitInconclusive partitions findings into real defects and the ones the rule could not decide, so
// a caller can report WHICH subjects the check gave up on.
func splitInconclusive(fs []check.Finding) (defects, undecided []check.Finding) {
	for _, f := range fs {
		if f.Inconclusive {
			undecided = append(undecided, f)
		} else {
			defects = append(defects, f)
		}
	}
	return defects, undecided
}

// subjectList names the undecided subjects for the item note, deduplicated and order-preserving so a
// rule that reports several findings on one net does not repeat it.
func subjectList(fs []check.Finding) string {
	var out []string
	seen := map[string]bool{}
	for _, f := range fs {
		if s := check.EntityRef(f.Subject); s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return strings.Join(out, ", ")
}

// datasheetSymbols returns the datasheet symbols an item's check joins against, deduplicated, from an
// inline query's param_symbol and the ParamSymbols of each resolved rule. Reading the rules gives a
// RULE-bound item the same needs-data gate (WS3-097) a query binding has without the runner knowing
// any specific rule (WS3-095). Empty means the gate does not apply.
func datasheetSymbols(it Item, rules []*check.Rule) []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	if q := it.Binding.Query; q != nil {
		add(q.ParamSymbol)
	}
	for _, r := range rules {
		for _, s := range r.ParamSymbols {
			add(s)
		}
	}
	return out
}

// anyComponentHasClass reports whether any component carries one of the given device classes (the
// applies_to_class gate). HasClass includes the family tags (WS10-015), so applies_to_class [clock]
// matches every clock source.
func anyComponentHasClass(m check.Model, classes []string) bool {
	for _, c := range m.Components() {
		for _, cl := range classes {
			if m.HasClass(c.RefDes, check.ComponentClass(cl)) {
				return true
			}
		}
	}
	return false
}

// bindsIntent reports whether an item binds a design-intent rule the intent compiler can produce
// (WS3-084/098), meaning the name is `intent/`-namespaced AND known accepts it. The prefix alone
// over-matches a pre-bound NOT-YET-SHIPPED name (intent/power-sequence), which must read
// not-automated. A nil known treats every intent/ name as known.
func bindsIntent(it Item, known func(ruleName string) bool) bool {
	if !strings.HasPrefix(it.Binding.Rule, "intent/") {
		return false
	}
	return known == nil || known(it.Binding.Rule)
}

// isUnratified reports whether a finding carries a datasheet citation whose method is "mock" or whose
// confidence is below the floor. A finding with NO datasheet citation is a netlist finding and is
// NOT unratified. A finding citing SEVERAL datasheets (WS3-028) is unratified when ANY citation fails,
// because the citations are conjunctive evidence. That quantifier is the OPPOSITE of allUnratified's,
// where findings are independent claims.
//
// A citation also fails when its human verification is Stale or Unknown against the revision the
// corpus holds. Confidence cannot catch this, because param.MarkVerified sets 1.0 and that survives a
// vendor revision. An unverified value reads Unverified and is judged on confidence alone. See
// docsite/content/architecture/datasheet-layer.md#verification-and-why-it-expires.
func isUnratified(f check.Finding, floor float64) bool {
	for _, dp := range f.DatasheetProv {
		if dp == nil {
			continue
		}
		if dp.Method == "mock" || dp.Confidence < floor {
			return true
		}
		switch param.VerificationState(dp.Verification) {
		case param.Stale, param.Unknown:
			return true
		}
	}
	return false
}

// allUnratified reports whether a non-empty finding set is ENTIRELY unratified, so the item is
// Provisional rather than Fail. One trustworthy finding keeps the item a Fail.
func allUnratified(fs []check.Finding, floor float64) bool {
	if len(fs) == 0 {
		return false
	}
	for _, f := range fs {
		if !isUnratified(f, floor) {
			return false
		}
	}
	return true
}

// presentResult resolves a present: binding by scanning the design for any component of the bound
// class. It passes on the first match and otherwise fails with one design-level finding whose rule id
// is "present/<class>" and whose subject is the class. That finding has no provenance, since absence
// has no source site.
//
// The test is HasClass rather than ComponentClass ==, so a family tag or a datasheet-enriched class
// matches too. A keyword-`ic` part whose spec declares `efuse` (WS10-013) satisfies {class: efuse}.
func presentResult(m check.Model, it Item, pb *PresentBinding) ItemResult {
	for _, c := range m.Components() {
		if m.HasClass(c.RefDes, check.ComponentClass(pb.Class)) {
			return ItemResult{Item: it, Outcome: Pass}
		}
	}
	return ItemResult{Item: it, Outcome: Fail, Findings: []check.Finding{{Subject: check.Entity{Kind: check.KindComponent, Ref: pb.Class}, Rule: "present/" + pb.Class, Severity: "warning", Message: "no component of class " + pb.Class + " is present on the design"}}}
}

// filterToScope keeps a net-subject finding whose net is in nets, and a component-subject finding whose
// subject RefDes is in comps. Any other kind, pins included, is dropped, because an interface scope
// cannot place it (WS3-058).
func filterToScope(fs []check.Finding, nets, comps map[string]bool) []check.Finding {
	kept := fs[:0]
	for _, f := range fs {
		switch f.Subject.Kind {
		case check.KindNet:
			if nets[f.Subject.Ref] {
				kept = append(kept, f)
			}
		case check.KindComponent:
			if comps[f.Subject.Ref] {
				kept = append(kept, f)
			}
		}
	}
	return kept
}

// The tag keys a profile binding selects on. They are plain strings because core imports nothing from
// stdlib. stdlib/profiles is the AUTHORITY for both values (profiles.TagRequirement), and only the
// end-to-end CLI test catches a drift between them.
const (
	profileTagName        = "profile"
	profileTagRequirement = "requirement"
)

// resolve turns an item's binding into the catalog rules it selects (or the compiled rule for an
// inline query). Empty means nothing shipped covers the item. An absent interface still resolves to
// rules here, and runItem marks it not-applicable before running them (WS3-051).
func resolve(cat *check.Catalog, it Item, vocab *facts.Registry) []*check.Rule {
	b := it.Binding
	switch {
	case b.Query != nil:
		r, err := compileQuery(it, vocab)
		if err != nil {
			// Validate refuses this before a run, so reaching here means a caller ran without it.
			// Returning no rule would read as "nothing automates this item", hiding the error, so the
			// item gets a rule that reports it could not decide.
			return []*check.Rule{uncompiled(it, err)}
		}
		return []*check.Rule{r}
	case b.Rule != "":
		return cat.Filter(check.Facets{Names: []string{b.Rule}})
	case b.Profile != "":
		tags := map[string][]string{profileTagName: {b.Profile}}
		// A requirement selector narrows the profile's rules to the one answering this ask (WS3-115).
		// Facets intersect distinct tag keys, so the rule must belong to the profile AND come from that
		// requirement. Empty adds no key, leaving the union.
		if b.Requirement != "" {
			tags[profileTagRequirement] = []string{b.Requirement}
		}
		return cat.Filter(check.Facets{Tags: tags})
	case b.Tag != "":
		k, v, ok := strings.Cut(b.Tag, "=")
		if !ok {
			return nil
		}
		return cat.Filter(check.Facets{Tags: map[string][]string{k: {v}}})
	}
	return nil
}

// uncompiled is the rule an inline query that does not compile resolves to: one inconclusive finding
// saying why, so the item reads as undecided rather than as not automated.
func uncompiled(it Item, err error) *check.Rule {
	name := "review/" + it.ID
	return &check.Rule{
		Name:     name,
		Severity: "warning",
		Summary:  it.Title,
		Eval: check.FailuresOnly(func(check.Model) []check.Finding {
			return []check.Finding{{Inconclusive: true, Message: fmt.Sprintf("%s could not compile: %v", name, err)}}
		}),
	}
}
