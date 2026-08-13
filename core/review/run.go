package review

import (
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/datasheet/param"
)

// Outcome is how one checklist item resolved on a design. The states are honest along TWO axes
// (WS10-014): COVERAGE — does a mechanism exist (everything but NotAutomated is covered) — and
// RATIFICATION — is the data the mechanism ran on trustworthy. A not-automated item (no shipped rule)
// must never read as passed; a covered item that ran on unratified data is Provisional, not a clean
// pass/fail; an item blocked on a declared input reads NeedsDesignIntent, not not-automated; and an
// item a device-class fact determines does not apply reads ComputedNA, the same branch a human takes.
type Outcome string

const (
	Pass          Outcome = "pass"           // the item's rule(s) ran on ratified data and found nothing
	Fail          Outcome = "fail"           // the item's rule(s) fired on ratified (trustworthy) data
	NotApplicable Outcome = "not-applicable" // the item's rule(s) need a fact tier this design lacks
	NotAutomated  Outcome = "not-automated"  // no shipped rule/mechanism covers the item (or it has no binding)

	// Provisional: the item's rule(s) fired, but every finding ran on UNRATIFIED datasheet data
	// (method "mock" or confidence below the floor), so it is not a trustworthy fail yet. This set is
	// the HITL ratification worklist — a human enters the real value and, because the datasheet layer
	// is a cross-design shared resource, it ratifies every design using that MPN. Derived from
	// Finding.DatasheetProv (WS10-012); a single ratified or netlist finding among them makes it a Fail.
	Provisional Outcome = "provisional"
	// NeedsDesignIntent: the item binds an intent-contract rule (WS3-084, an `intent/`-namespaced rule)
	// but no design-intent declaration was supplied, so the rule is absent from the catalog. It is
	// COVERED (a mechanism exists) and blocked on a declared per-design input, distinct from a genuine
	// not-automated. Supplying --intent-path flips it to pass/fail.
	NeedsDesignIntent Outcome = "needs-design-intent"
	// ComputedNA: a device-class-gated item (applies_to_class) whose class matches no component on the
	// design, so the mechanism itself determined the check does not apply — the same branch a human
	// takes ("no crystals here → n/a"). Distinct from NotApplicable (a missing fact TIER) — here the
	// tier is present and the answer is a computed "does not apply".
	ComputedNA Outcome = "computed-n/a"
	// NeedsData: the item's mechanism exists and ran, but the specific data it joins against is not
	// present, so it could not actually evaluate (WS3-097). Today the one derivable case is a datasheet
	// inline query (param_symbol) whose symbol is seeded on no component: --params is supplied so the
	// param tier exists (Available does not gate), but the query matched nothing because the value is
	// unseeded, not because the design is clean — reading pass there would mean "deleting a seed makes
	// the review greener". It is COVERED (a mechanism exists, blocked on a supplyable value), so it
	// counts toward Covered() like NeedsDesignIntent, and it is the honest reading that lets an overlay
	// BIND a datasheet check before its seed lands and watch it flip to a real verdict as seeding arrives.
	NeedsData Outcome = "needs-data"
	// Inconclusive: the item's rule(s) ran with everything they needed, examined a specific subject,
	// and could not decide (agni issue 74). It is the only outcome on the RESULT side of a rule; every
	// other non-pass state above is a PRECONDITION, decided before or around the rule and design-wide.
	//
	// That distinction is why it is not folded into NeedsData. Half the cases it covers are not data
	// gaps at all: a netlist states reset polarity nowhere and no seeding will ever change that, while
	// an unclassified ORing controller resolves the moment its spec is seeded. Reporting the first as
	// needs-data would tell a reviewer to go supply something that does not exist, which is the same
	// class of error as a false pass, a verdict asserting something the engine does not know.
	//
	// It is COVERED (a mechanism exists and ran), so it counts toward Covered() like NeedsData and
	// NeedsDesignIntent, and it is NEVER a pass. Expect the pass count to DROP when rules start
	// emitting it: each item that moves was previously a silent pass on a question nothing answered,
	// which is the defect this whole family exists to remove (the same intended direction as WS3-099).
	//
	// The per-finding Message carries the remedy, so one outcome serves both kinds.
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
	// NeedsData. The Note says the same thing in a sentence; this is the form a consumer can act on
	// without parsing one, which is the whole difference between reporting a gap and closing it.
	Unmet []check.UnmetDependency
}

// AreaResult groups the item results of one review area.
type AreaResult struct {
	Area  Area
	Items []ItemResult
}

// Report is the per-design review result: the manifest and design names plus each area's item
// outcomes. It is the value a renderer (markdown today; JSON/web later) turns into a report.
type Report struct {
	Manifest string
	Design   string
	Areas    []AreaResult
}

// Presence is the review's evaluability verdict for an interface a profile item names: whether the
// profile's rules will GENUINELY evaluate on this design (WS3-090). It is not a bare "on the board"
// bool — a rule that could not evaluate must not score as a clean pass, and a host-bound interface
// whose host is annotated nowhere is a different case from one that is simply absent.
type Presence int

const (
	// IfaceAbsent: the interface is not on the design (its convention is not in use and no host is
	// declared). Its rules have nothing to fire on -> not-applicable.
	IfaceAbsent Presence = iota
	// IfaceHostUnsatisfied: the profile is host-bound but no component declares the host AND its signal
	// convention is not in use, so the intended (host) check cannot evaluate -> not-automated, not a
	// hollow pass. When the convention IS in use the verdict is IfacePresent instead, so a genuinely
	// checkable un-annotated design still runs.
	IfaceHostUnsatisfied
	// IfacePresent: the convention is in use AND its completeness anchor is matched (or a host is
	// declared); the rules genuinely evaluate -> run for pass/fail.
	IfacePresent
	// IfaceConventionUnmatched: the convention path's sibling of IfaceHostUnsatisfied (WS3-099). The
	// interface IS partly named to the profile — enough signals match to clear in_use — but its anchor
	// signal is absent, so the completeness rule has nothing to hang on and cannot evaluate. The
	// interface is neither absent (it is visibly there) nor checkable under this profile's naming; the
	// diagnosis a reviewer needs is "this looks like the interface but the naming does not match".
	//
	// Unlike the two verdicts above this does NOT stop the item running: a profile's secondary rules
	// (signal-dangling, missing-pullup) gate on in_use alone, so they do evaluate on the matched nets.
	// The item runs, a real finding still reads fail, and only a would-be PASS is replaced.
	IfaceConventionUnmatched
)

// PresenceFunc reports the review's evaluability verdict for an interface (by profile Name) and
// whether the name is a KNOWN interface. Known means a profile or a presence-only declaration of that
// name is loaded (built-in or via --profile-path). An UNKNOWN name (known=false) leaves the item
// running, so it can still read not-automated when nothing covers it. nil disables the gate (every
// item runs). It stays a function so `review` is decoupled from `profiles`; the service wires it
// (profiles.InUse / HostDeclared over every loaded profile of that name).
type PresenceFunc func(profileName string) (verdict Presence, known bool)

// ScopeFunc returns the set of net names belonging to a profile, so a scoped binding (WS3-058) can keep
// only the bound rule's findings on that interface's nets. nil disables the net side of scoping. Like
// PresenceFunc it keeps `review` decoupled from `profiles`; the CLI wires it (profiles.Nets).
type ScopeFunc func(profileName string) map[string]bool

// CompScopeFunc returns the set of component RefDes belonging to a profile, so a scoped binding can keep
// a COMPONENT-subject rule's findings for the interface's parts (a design-wide datasheet rail rule like
// rail-nominal-out-of-recommended emits component findings the net ScopeFunc alone drops — WS3-083). nil
// disables the component side of scoping. The CLI wires it (profiles.Components). A scoped item is
// unfiltered only when BOTH Scope and CompScope are nil; either present enables filtering.
type CompScopeFunc func(profileName string) map[string]bool

// RunParams carries everything Run needs. It is a struct (not a positional list) so a new capability —
// the component scope was the third scoping input added — extends the API by adding a field, without
// reshuffling every caller (WS3-083).
type RunParams struct {
	Model     check.Model
	Catalog   *check.Catalog
	Manifest  Manifest
	Design    string
	Present   PresenceFunc  // nil disables the interface-absence check (every item runs)
	Scope     ScopeFunc     // nil disables net scoping
	CompScope CompScopeFunc // nil disables component scoping
	// RatifiedFloor is the datasheet-confidence floor below which a finding's data is "unratified"
	// (WS10-014): a fail whose findings are all mock or below this confidence is Provisional, not Fail.
	// Zero means use DefaultRatifiedFloor — a floor of 0 would rate everything trustworthy, never the
	// intent. Config, not a literal (the CLI exposes --ratified-floor).
	RatifiedFloor float64
	// IntentRuleKnown reports whether an intent/-namespaced rule NAME is one the intent compiler can
	// produce (WS3-098). It distinguishes a real-but-undeclared intent rule (needs-design-intent) from a
	// not-yet-shipped intent rule name a manifest pre-bound (not-automated), which a bare intent/ prefix
	// test cannot. nil treats every intent/ name as known — the pre-WS3-098 behavior — so a caller that
	// does not wire it is unchanged. It stays a function so `review` is decoupled from the `intent`
	// package the way it is from `profiles`; the service wires intent.Emits.
	IntentRuleKnown func(ruleName string) bool
}

// DefaultRatifiedFloor is the confidence at or above which a datasheet value counts as ratified,
// matching WS10-012's "confidence < 0.9 → (verify)" report convention. A finding below it (or method
// "mock") makes an otherwise-failing item Provisional.
const DefaultRatifiedFloor = 0.9

func (p RunParams) ratifiedFloor() float64 {
	if p.RatifiedFloor <= 0 {
		return DefaultRatifiedFloor
	}
	return p.RatifiedFloor
}

// Run evaluates a review manifest against a design's Model, selecting each item's rules from the
// composed catalog (so overlay profiles added via --profile-path are in scope) and resolving each to
// an Outcome. Rules whose fact tier is absent are dropped as not-applicable rather than run; a profile
// item whose interface `present` says is absent is likewise not-applicable, not a silent pass.
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
	// A present: binding is a class-of-component presence assertion, resolved directly (not through the
	// catalog): pass if any component of the class is on the design, fail with one design-level finding
	// if none is. It is never not-applicable — the component-class tier exists on any netlist — so it is
	// handled before the interface-absence and catalog-resolution paths below.
	if pb := it.Binding.Present; pb != nil {
		return presentResult(m, it, pb)
	}
	// Interface absence takes precedence over every other outcome: a KNOWN interface that is absent on
	// this design is not-applicable whether or not a rule is shipped to check it. This is checked BEFORE
	// the not-automated shortcut so it covers two cases with one gate: a full profile whose bus is
	// absent (WS3-051), and a PRESENCE-ONLY interface declaration — a profile with signals but no
	// requirements, so it compiles to zero rules and exists only to answer "is this module on the
	// board?" (WS3-068). An UNKNOWN interface (no profile/declaration at all) is not absent-known, so it
	// falls through to the not-automated case below rather than reading as n/a. Several named interfaces
	// are not-applicable only when EVERY one is known-absent (any present, or unknown, keeps it running).
	ifaces := it.Binding.Scope.names()
	if it.Binding.Profile != "" {
		ifaces = []string{it.Binding.Profile}
	}
	// unmatched defers the WS3-099 verdict to the zero-findings tail: the convention is partly in use, so
	// the secondary rules still evaluate and a real finding must survive.
	unmatched := false
	if len(ifaces) > 0 && present != nil {
		runs, hostUnsatisfied := false, false
		for _, iface := range ifaces {
			v, known := present(iface)
			if !known || v == IfacePresent {
				runs = true // an unknown or genuinely-present interface keeps the item running
				break
			}
			switch v {
			case IfaceHostUnsatisfied:
				hostUnsatisfied = true
			case IfaceConventionUnmatched:
				unmatched = true
			}
		}
		// A convention-unmatched interface takes precedence over the other two non-running verdicts: it
		// is the most specific diagnosis (the naming IS partly there), and it runs the rules rather than
		// returning here, so its verdict is decided at the bottom.
		if !runs && !unmatched {
			// A rule that could not evaluate must not score PASS (WS3-090). A host-bound interface
			// annotated on no component reads not-automated (the intended check is blocked on the
			// annotation); an interface simply absent reads not-applicable.
			if hostUnsatisfied {
				return ItemResult{Item: it, Outcome: NotAutomated, Note: "host-bound interface declared on no component"}
			}
			return ItemResult{Item: it, Outcome: NotApplicable, Note: "interface not present on this design"}
		}
	}
	// Device-class computed-n/a (WS10-014): an item may declare the device classes it applies to
	// (applies_to_class). When no component on the design carries any of them, the mechanism itself
	// determines the check does not apply — the same branch a human takes ("no crystals here → n/a") —
	// which is honest and automatable via the device-class fact (WS10-013/015), NOT a dead end. Checked
	// before catalog resolution so it holds even for an item whose rule is not yet shipped.
	if classes := it.Binding.AppliesToClass; len(classes) > 0 && !anyComponentHasClass(m, classes) {
		return ItemResult{Item: it, Outcome: ComputedNA, Note: "no " + strings.Join(classes, "/") + " part on this design"}
	}
	rules := resolve(cat, it)
	if len(rules) == 0 {
		// An intent-bound item (WS3-084) with no declaration supplied resolves to zero rules because the
		// intent rule is absent from the catalog — but it is COVERED, blocked on a declared per-design
		// input, not genuinely un-mechanized. Report that distinctly so --intent-path is the obvious fix.
		if bindsIntent(it, p.IntentRuleKnown) {
			return ItemResult{Item: it, Outcome: NeedsDesignIntent, Note: "needs a design-intent declaration (--intent-path)"}
		}
		// The interface is present (or the item names no interface), but nothing shipped checks it: an
		// honest not-automated, not a silent pass.
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
	// A scoped binding keeps only findings for the named interface (the UNION when several are named),
	// so a per-interface ask reflects its bus, not the whole design's output: a net-subject finding on
	// one of the interface's nets (WS3-058), or a component-subject finding on one of its parts
	// (WS3-083, which lets a design-wide datasheet rail rule be scoped per interface). Filtering runs
	// when EITHER scope func is wired; a scoped item is unfiltered only when both are nil.
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
	// An inconclusive finding is a result the rule could not decide, not a defect, so it must not be
	// weighed as one: a subject the rule gave up on cannot make the item fail, and cannot make it pass
	// either. Split them before the fail branch so a rule may legitimately emit both at once (one
	// subject decided and wrong, another undecidable) and the real defect still wins.
	fs, undecided := splitInconclusive(fs)
	if len(fs) > 0 {
		// Data-trust axis (WS10-014): a fail every one of whose findings ran on UNRATIFIED datasheet data
		// (mock, or confidence below the floor) is Provisional — a HITL ratification worklist item, not a
		// trustworthy fail. A single ratified datasheet finding OR any netlist finding (no DatasheetProv,
		// so trustworthy by construction) makes it a real Fail.
		if allUnratified(fs, p.ratifiedFloor()) {
			return ItemResult{Item: it, Outcome: Provisional, Findings: fs}
		}
		return ItemResult{Item: it, Outcome: Fail, Findings: fs}
	}
	// Zero findings is only a genuine pass if the check could evaluate (WS3-097). A datasheet inline
	// query names the symbol it joins (param_symbol); when that symbol is seeded on no component, the
	// query matched nothing because the value is absent, not because the design is clean, so the honest
	// reading is needs-data, not pass. Only this datasheet case is gated: --params supplied means
	// Available did not gate, so an unseeded symbol is the silent gap. The general "all joined relations
	// empty" case is deliberately not chased (the datasheet join is the one that bites, the params tier
	// being sparse by nature).
	if syms := datasheetSymbols(it, avail); len(syms) > 0 && !check.SeedsAnySymbol(m, syms) {
		return ItemResult{
			Item:    it,
			Outcome: NeedsData,
			Note:    "no seeded datasheet value for " + strings.Join(syms, "/") + " on this design",
			Unmet:   check.UnseededSymbols(m, syms, it.Binding.AppliesToClass),
		}
	}
	// Same discipline for the unanchored interface (WS3-099): the secondary rules ran and found nothing,
	// but the completeness rule never evaluated, so this is not a clean bill of health. It reads
	// not-automated rather than a needs-* state because no shipped mechanism covers THIS design's naming
	// — scoring it covered would inflate the coverage axis, which is the defect this whole family fights.
	if unmatched {
		return ItemResult{Item: it, Outcome: NotAutomated, Note: "interface named but its completeness anchor signal is absent, so the convention check could not evaluate"}
	}
	// The rule ran, found no defect, and could not decide about at least one subject. That is not a
	// clean bill of health, so it does not read pass (agni issue 74).
	if len(undecided) > 0 {
		return ItemResult{Item: it, Outcome: Inconclusive, Findings: undecided,
			Note: "the check ran but could not decide for " + subjectList(undecided)}
	}
	return ItemResult{Item: it, Outcome: Pass}
}

// splitInconclusive partitions findings into real defects and the ones the rule could not decide.
// Both are returned so a caller can report the undecided subjects rather than only counting them: a
// reviewer needs to know WHICH net the check gave up on, and the finding's message says why.
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
		if s := f.Subject; s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return strings.Join(out, ", ")
}

// datasheetSymbols returns the datasheet symbols an item's check joins against, from BOTH places a
// binding can declare them: an inline query's param_symbol, and the ParamSymbols of each rule the item
// resolved to. Empty means the item's check has no declared datasheet dependency, so the WS3-097
// needs-data gate does not apply to it.
//
// Covering the rule side is what closes the hole for a RULE-bound datasheet item (WS3-095). Before
// this, only an inline query declared its symbol, so a design read WITH --params but with the
// particular part unseeded ran a datasheet rule that could join nothing, found nothing, and scored a
// pass. check.Available does not save it: that gates on the params TIER, which is present. Reading the
// symbols off the resolved rules gives every rule-bound datasheet item the same gate a query binding
// has always had, without the runner knowing anything about a specific rule.
//
// Duplicates are collapsed so the note reads once per symbol when several rules join on the same one.
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

// anyComponentHasClass reports whether any component on the design carries one of the given device
// classes (the applies_to_class computed-n/a gate). It reads the Model's class fact (HasClass), which
// includes the family tags (WS10-015), so applies_to_class: [clock] matches every clock source.
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

// bindsIntent reports whether an item's binding targets a design-intent rule the mechanism can
// actually produce (WS3-084/098): the name is `intent/`-namespaced AND known reports it is a rule the
// intent compiler emits. The name prefix alone is the wiring-free signal that a zero-rule resolution is
// a missing DECLARATION rather than a missing mechanism, but it over-matches a pre-bound NOT-YET-SHIPPED
// intent rule name (intent/power-sequence), which is a missing mechanism and must read not-automated,
// not needs-design-intent. known narrows the prefix to the compiler's actual name space; a nil known
// keeps the prefix-only behavior (every intent/ name treated as known) so an unwired caller is unchanged.
func bindsIntent(it Item, known func(ruleName string) bool) bool {
	if !strings.HasPrefix(it.Binding.Rule, "intent/") {
		return false
	}
	return known == nil || known(it.Binding.Rule)
}

// isUnratified reports whether a finding ran on untrustworthy datasheet data: it carries a datasheet
// citation whose method is "mock" or whose confidence is below the floor. A finding with NO datasheet
// citation is a netlist/structural finding — trustworthy by construction, so NOT unratified.
// A finding backed by SEVERAL datasheets (a connection-aware rule, WS3-028) is unratified when ANY of
// its citations fails the floor, because the conclusion rests on every value it joined and is only as
// trustworthy as the weakest one. A regulator-output-vs-abs-max finding whose abs-max was hand-read
// but whose output voltage came from a low-confidence extraction is exactly half-evidenced, and
// calling it a hard Fail would be the false-fail this axis exists to prevent.
//
// Note the quantifier here is the OPPOSITE of allUnratified's, deliberately. Across findings, one
// trustworthy finding among several makes the item a real Fail — they are independent claims and one
// standing up is enough. Within a finding, the citations are conjunctive evidence, so all of them
// have to stand up.
// A citation also fails the floor when a human verification exists but no longer applies to the
// revision the corpus holds (Stale), or cannot be checked against one (Unknown). This case CANNOT be
// caught by the confidence test above, and that is the whole reason it is written out separately:
// param.MarkVerified raises confidence to 1.0 to keep the older signal in step, and that 1.0 stays
// after the vendor revises the document. Judging on confidence alone would therefore treat a
// verification of a superseded revision as the most trustworthy evidence in the system, which
// inverts what the verification record was added to express. A value nobody ever verified is not
// affected: it has no verification record, reads as Unverified, and is judged on its confidence
// exactly as before.
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
// Provisional rather than Fail. Any trustworthy finding (ratified datasheet or netlist) among them is a
// real defect and keeps the item a Fail.
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
// class (Model.HasClass). Pass on the first match; fail with a single design-level finding when none
// is found. The finding carries a synthetic rule id ("present/<class>") and names the class as its
// subject, so both the markdown and JSON renderers show a readable "present/<class>: <class> (…)"
// line; it has no provenance because absence has no source site to cite.
//
// The membership test is HasClass, not ComponentClass ==, so present: matches on a family tag or a
// datasheet-enriched class, not only the most-specific keyword class (mirrors anyComponentHasClass).
// A component whose keyword class is `ic` but whose datasheet spec declares `efuse` (WS10-013) thus
// satisfies present: {class: efuse}, and present: {class: diode} matches an LED/TVS by family tag.
func presentResult(m check.Model, it Item, pb *PresentBinding) ItemResult {
	for _, c := range m.Components() {
		if m.HasClass(c.RefDes, check.ComponentClass(pb.Class)) {
			return ItemResult{Item: it, Outcome: Pass}
		}
	}
	return ItemResult{Item: it, Outcome: Fail, Findings: []check.Finding{{
		Rule:     "present/" + pb.Class,
		Severity: "warning",
		Kind:     check.KindComponent,
		Subject:  pb.Class,
		Message:  "no component of class " + pb.Class + " is present on the design",
	}}}
}

// filterToScope keeps a net-subject finding whose net is in nets, and a component-subject finding whose
// subject RefDes is in comps. A pin-subject finding is dropped — an interface scope has no pin→component
// map, and a scope has always dropped a finding kind it cannot place (WS3-058 dropped every non-net).
func filterToScope(fs []check.Finding, nets, comps map[string]bool) []check.Finding {
	kept := fs[:0]
	for _, f := range fs {
		switch f.Kind {
		case check.KindNet:
			if nets[f.Subject] {
				kept = append(kept, f)
			}
		case check.KindComponent:
			if comps[f.Subject] {
				kept = append(kept, f)
			}
		}
	}
	return kept
}

// The tag keys a profile binding selects on. They are declared here as plain strings rather than
// imported from stdlib/profiles because review is deliberately decoupled from that package — it
// reaches it only through the injected PresenceFunc/ScopeFunc, and core has no import of stdlib at
// all. stdlib/profiles is the AUTHORITY for both values (profiles.TagRequirement); what pins them
// together is not a shared constant but the end-to-end CLI test, which runs a real overlay profile
// through a real manifest and would fail on any drift.
const (
	profileTagName        = "profile"
	profileTagRequirement = "requirement"
)

// resolve turns an item's binding into the set of catalog rules it selects (or the compiled rule for
// an inline query). An empty result means nothing shipped covers the item. An interface profile whose
// bus is absent still resolves to rules here; runItem marks it not-applicable BEFORE running them via
// the PresenceFunc (WS3-051), so it does not read as a silent pass.
func resolve(cat *check.Catalog, it Item) []*check.Rule {
	b := it.Binding
	switch {
	case b.Query != nil:
		r, err := compileQuery(it)
		if err != nil {
			return nil // Load validated this already; be defensive
		}
		return []*check.Rule{r}
	case b.Rule != "":
		return cat.Filter(check.Facets{Names: []string{b.Rule}})
	case b.Profile != "":
		tags := map[string][]string{profileTagName: {b.Profile}}
		// A requirement selector narrows the profile's compiled set to the one rule that answers this
		// ask (WS3-115). Facets intersect distinct tag keys, so this is a conjunction: the rule must
		// belong to the profile AND come from that requirement. Empty adds no key, which is exactly the
		// union every manifest was written against.
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
