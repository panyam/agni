package review

import (
	"context"
	"fmt"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	"github.com/panyam/agni/core/param"
)

// Outcome is how one checklist item resolved on a design, along TWO axes (WS10-014). COVERAGE asks
// whether a mechanism exists, and everything but NotAutomated is covered. RATIFICATION asks whether
// the data the mechanism ran on is trustworthy. If no shipped rule covers an item, the item must
// never read as passed.
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
	// ComputedNA marks a device-class-gated item (applies_to_class) when no component on the design
	// matches its class. NotApplicable means a fact TIER is missing. Here the tier is present, and the
	// mechanism computed that the check does not apply.
	ComputedNA Outcome = "computed-n/a"
	// NeedsData means the item's mechanism ran, but the specific data it joins against is absent, so it
	// could not evaluate (WS3-097). The case we can derive is a datasheet symbol seeded on no
	// component. --params supplied the tier, so Available does not gate. Nothing matched because the
	// value is unseeded, not because the design is clean. It is COVERED, like NeedsDesignIntent.
	NeedsData Outcome = "needs-data"
	// Inconclusive means the item's rule(s) ran with everything they needed, examined a specific
	// subject, and could not decide (agni issue 74). It is the only outcome on the RESULT side of a
	// rule. Every other non-pass state above is a design-wide PRECONDITION. It is not folded into
	// NeedsData, because some inconclusive cases are not data gaps. A netlist states reset polarity
	// nowhere, for example, and needs-data would send a reviewer to supply something that does not
	// exist.
	//
	// It is COVERED and NEVER a pass. The per-finding Message states the remedy.
	Inconclusive Outcome = "inconclusive"
	// CoveredExternally means another tool's imported report answers the item and shows no matching
	// violation (agni issue 152). It is COVERED and NEVER a pass. A vendor report lists violations
	// only. It has no coverage axis, so it cannot show the check ran on every subject. It is not
	// answered either, because agni checked nothing. The Note names the producer and its version.
	CoveredExternally Outcome = "covered-externally"
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

// Report is the per-design review result. It holds the manifest and design names and each area's
// item outcomes. The markdown, JSON and HTML renderers all read it.
type Report struct {
	Manifest string
	Design   string
	Areas    []AreaResult
}

// Presence is the review's evaluability verdict for an interface that a profile item names. It says
// whether the profile's rules will GENUINELY evaluate on this design (WS3-090). It is not a bare "on
// the board" bool, because a rule that could not evaluate must not score as a clean pass.
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

// PresenceFunc takes an interface's profile Name. It reports the review's evaluability verdict for
// that interface, and whether the name is a KNOWN interface. A name is known when a profile or a
// presence-only declaration of that name is loaded, built-in or via --profile-path. An UNKNOWN name
// leaves the item running, so it can still read not-automated when nothing covers it. nil disables the
// gate. It is a function so `review` does not import `profiles`, and the service wires it
// (profiles.InUse / HostDeclared).
type PresenceFunc func(profileName string) (verdict Presence, known bool)

// ScopeFunc returns the set of net names belonging to a profile, so a scoped binding (WS3-058) keeps
// only the bound rule's findings on that interface's nets. nil disables the net side of scoping. The
// service wires it (profiles.Nets).
type ScopeFunc func(profileName string) map[string]bool

// CompScopeFunc returns the set of component RefDes belonging to a profile. A scoped binding uses it
// to keep a COMPONENT-subject finding on the interface's parts. ScopeFunc alone drops such a finding
// (WS3-083). nil disables the component side. The service wires it (profiles.Components). A scoped
// item is unfiltered only when BOTH Scope and CompScope are nil.
type CompScopeFunc func(profileName string) map[string]bool

// RunParams holds everything Run needs. It is a struct, so a new capability adds a field rather than
// changing every caller (WS3-083).
type RunParams struct {
	Model     check.Model
	Catalog   *check.Catalog
	Manifest  Manifest
	Design    string
	Present   PresenceFunc  // nil disables the interface-absence check (every item runs)
	Scope     ScopeFunc     // nil disables net scoping
	CompScope CompScopeFunc // nil disables component scoping
	// RatifiedFloor is a datasheet-confidence floor (WS10-014). A finding's data below it is
	// "unratified". A fail whose findings are all mock or below the floor is Provisional. Zero means
	// DefaultRatifiedFloor, since a floor of 0 would rate everything trustworthy. The CLI exposes it
	// as --ratified-floor.
	RatifiedFloor float64
	// IntentRuleKnown reports whether the intent compiler can produce an intent/-namespaced rule NAME
	// (WS3-098). It separates a real-but-undeclared intent rule (needs-design-intent) from a
	// not-yet-shipped name that a manifest pre-bound (not-automated). A prefix test cannot tell the two
	// apart. nil treats every intent/ name as known. The service wires intent.Emits.
	IntentRuleKnown func(ruleName string) bool
	// Vocabulary is the relation vocabulary an inline query compiles against: the shipped one plus
	// the run's library (agni issue 779). Nil is the process default.
	Vocabulary *facts.Registry
	// External holds other tools' imported reports, which items with an ExternalBinding read (agni
	// issue 152). Nil means none was supplied, and every external item reads needs-data.
	External []ExternalReport
}

// ExternalReport is one imported report from another tool, in the engine's own finding form. The
// caller converts the agni.checks.results document, so this package reads no wire type.
type ExternalReport struct {
	Producer string // the document's meta.producer, matched against ExternalBinding.Producer
	Version  string // the document's meta.producer_version, named in the item's note
	Findings []check.Finding
	// Skipped maps each rule the tool did not run to its reason, from the document's skipped list.
	// KiCad lists the checks a design's rule settings ignore.
	Skipped map[string]string
}

// DefaultRatifiedFloor is the default ratification floor. A datasheet value with this confidence or
// higher counts as ratified. The value matches WS10-012's "confidence < 0.9 is (verify)" report
// convention. A finding below it, or with method "mock", makes an otherwise-failing item Provisional.
const DefaultRatifiedFloor = 0.9

func (p RunParams) ratifiedFloor() float64 {
	if p.RatifiedFloor <= 0 {
		return DefaultRatifiedFloor
	}
	return p.RatifiedFloor
}

// Run evaluates a review manifest against a design's Model. It selects each item's rules from the
// composed catalog, so overlay profiles from --profile-path are in scope. Then it resolves each item
// to an Outcome. A rule whose fact tier is absent does not run and reads not-applicable. So does a
// profile item when Present reports its interface absent.
//
// Run checks ctx after every item. When the caller has gone, it returns ctx's error (agni issue 795).
// Returning a report instead would read its unrun items as answered.
func Run(ctx context.Context, p RunParams) (Report, error) {
	rep := Report{Manifest: p.Manifest.Name, Design: p.Design}
	for _, a := range p.Manifest.Areas {
		ar := AreaResult{Area: a}
		for _, it := range a.Items {
			res := runItem(ctx, p, it)
			if err := ctx.Err(); err != nil {
				return Report{}, err
			}
			ar.Items = append(ar.Items, res)
		}
		rep.Areas = append(rep.Areas, ar)
	}
	return rep, nil
}

func runItem(ctx context.Context, p RunParams, it Item) ItemResult {
	m, cat, present := p.Model, p.Catalog, p.Present
	// The component-class tier exists on any netlist, so a present: binding is never not-applicable.
	// It resolves ahead of the paths below.
	if pb := it.Binding.Present; pb != nil {
		return presentResult(m, it, pb)
	}
	// Interface absence takes precedence over every other outcome, so a KNOWN absent interface is
	// not-applicable whether or not a rule checks it. This runs BEFORE the not-automated shortcut, so
	// one gate covers two cases. One is a full profile whose bus is absent (WS3-051). The other is a
	// PRESENCE-ONLY declaration, which compiles to zero rules (WS3-068). An UNKNOWN interface falls
	// through to not-automated. Several named interfaces are not-applicable only when EVERY one is
	// known-absent.
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
	// Device-class computed-n/a (WS10-014), read off the device-class fact (WS10-013/015). This runs
	// before catalog resolution, so it applies even to an item whose rule is not yet shipped.
	if classes := it.Binding.AppliesToClass; len(classes) > 0 && !anyComponentHasClass(m, classes) {
		return ItemResult{Item: it, Outcome: ComputedNA, Note: "no " + strings.Join(classes, "/") + " part on this design"}
	}
	if eb := it.Binding.External; eb != nil {
		return externalResult(it, eb, p.External)
	}
	rules := resolve(cat, it, p.Vocabulary)
	if len(rules) == 0 {
		// An intent-bound item (WS3-084) resolves to zero rules when no declaration was supplied, because
		// the intent rule is then absent from the catalog. It is COVERED, so it names --intent-path.
		if bindsIntent(it, p.IntentRuleKnown) {
			return ItemResult{Item: it, Outcome: NeedsDesignIntent, Note: "needs a design-intent declaration (an intent: section in the design's design.yaml, or --intent-path)"}
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
	fs, err := check.Run(ctx, m, avail)
	if err != nil {
		// Only a cancelled context fails check.Run, and Run returns that error after this item, so this
		// result is never reported.
		return ItemResult{Item: it, Outcome: Inconclusive, Note: err.Error()}
	}
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
	// gated. With --params supplied, check.Available did not gate, so a symbol seeded on no component
	// is the silent gap. The general "all joined relations empty" case is not chased, because the
	// params tier is the only one that is sparse by nature.
	if syms := datasheetSymbols(it, avail); len(syms) > 0 && !check.SeedsAnySymbol(m, syms) {
		return ItemResult{
			Item:    it,
			Outcome: NeedsData,
			Note:    "no seeded datasheet value for " + strings.Join(syms, "/") + " on this design",
			Unmet:   check.UnseededSymbols(m, syms, it.Binding.AppliesToClass),
		}
	}
	// The unanchored interface (WS3-099) ran its secondary rules clean, but the completeness rule never
	// evaluated. It reads not-automated rather than a needs-* state, because no shipped mechanism
	// covers THIS design's naming. Scoring it covered would inflate the coverage axis.
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

// externalResult answers an external item from the reports its producer supplied. Several documents
// from one producer pool their findings, so a team may import its DRC and a later re-run together.
func externalResult(it Item, eb *ExternalBinding, reports []ExternalReport) ItemResult {
	var versions, skipped []string
	var fs []check.Finding
	found := false
	for _, r := range reports {
		if r.Producer != eb.Producer {
			continue
		}
		found = true
		if r.Version != "" {
			versions = append(versions, r.Version)
		}
		if why, ok := r.Skipped[eb.Rule]; ok && eb.Rule != "" {
			skipped = append(skipped, why)
		}
		for _, f := range r.Findings {
			if eb.Rule == "" || f.Rule == eb.Rule {
				fs = append(fs, f)
			}
		}
	}
	if !found {
		return ItemResult{Item: it, Outcome: NeedsData,
			Note: "no report from " + eb.Producer + " was supplied (import one with `agni import-results`, then pass it with --external)"}
	}
	if len(fs) > 0 {
		return ItemResult{Item: it, Outcome: Fail, Findings: fs}
	}
	by := eb.Producer
	if len(versions) > 0 {
		by += " " + strings.Join(versions, ", ")
	}
	// The tool was told not to run this rule, so its silence covers nothing. Nothing checks the item.
	if len(skipped) > 0 {
		return ItemResult{Item: it, Outcome: NotAutomated, Note: by + " did not run " + eb.Rule + ": " + skipped[0]}
	}
	what := "no violation"
	if eb.Rule != "" {
		what = "no " + eb.Rule + " violation"
	}
	return ItemResult{Item: it, Outcome: CoveredExternally, Note: what + " in the report from " + by}
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
// RULE-bound item the same needs-data gate (WS3-097) as a query binding. The runner still knows no
// specific rule (WS3-095). Empty means the gate does not apply.
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

// bindsIntent reports whether an item binds a design-intent rule that the intent compiler can
// produce (WS3-084/098). The name must be `intent/`-namespaced AND known must accept it. The prefix
// alone over-matches a pre-bound NOT-YET-SHIPPED name (intent/power-sequence), and that name must read
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

// uncompiled builds the rule for an inline query that does not compile. The rule reports one
// inconclusive finding saying why, so the item reads as undecided rather than as not automated.
func uncompiled(it Item, err error) *check.Rule {
	name := "review/" + it.ID
	return &check.Rule{
		Name:     name,
		Severity: "warning",
		Summary:  it.Title,
		Eval: check.FailuresOnly(func(context.Context, check.Model) []check.Finding {
			return []check.Finding{{Inconclusive: true, Message: fmt.Sprintf("%s could not compile: %v", name, err)}}
		}),
	}
}
