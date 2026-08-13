// Package check runs rule checks over a netlist IR Design. Pure IR operations, no I/O
// (CONSTRAINTS C1). This is the Phase-1 rules library (docs/19): rules are individually
// registered Rule values evaluated over the IR, the shape a declarative DSL would later
// compile down to.
//
// Structure: one file per rule (rule_*.go), each a Rule value carrying its documentation and
// an Eval written in terms of the query primitives in query.go (Select/Exists/Count over a
// shared Model), never raw loops over the IR. index.go is the registry. Each rule also
// carries a declarative twin — the same body as a Spec value (spec.go), the rules-as-data
// form a Phase-2 DSL compiles to — kept identical to the Go Eval by the parity tests; see
// Specs in index.go for the contract between the two forms.
package check

import (
	"sort"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Finding is one rule violation. Prov locates it in the source (nil when the source carries
// no provenance for the subject).
//
// Kind names what Subject refers to (a net, a component, or a pin) so a consumer can group and
// highlight by entity type without re-guessing from the string: Subject is a net name when Kind is
// KindNet and a ref_des when KindComponent or KindPin, and Pin holds the pin designator only when
// Kind is KindPin. A rule states its subject kind at construction (see NetFinding/CompFinding).
type Finding struct {
	Severity string // "error" | "warning" | "info"
	// Inconclusive marks a finding as a RESULT the rule could not decide, rather than a defect it
	// found. The rule ran, it had everything it needed, it examined this subject, and it could not
	// conclude (agni issue 74).
	//
	// WHY THIS IS NOT ONE OF THE REVIEW'S needs-* OUTCOMES. Those are PRECONDITIONS, decided before or
	// around the rule and always design-wide: NotApplicable (the design lacks a fact tier),
	// NeedsDesignIntent (no declaration supplied), NeedsData (nothing on the design seeds this symbol),
	// ComputedNA (no component of the applicable class). This is a per-SUBJECT result on the other side
	// of the rule, so folding it into a precondition would mean telling a reviewer to go supply
	// something in the cases where nothing is missing. A netlist states reset polarity nowhere and no
	// amount of seeding will change that, while an unclassified controller resolves the moment its
	// spec is seeded; both are inconclusive, and only the second is a data gap.
	//
	// The REMEDY belongs in Message, exactly as it does for a defect. That is why one flag suffices
	// where the precondition axis needed four outcomes: "seed a spec for U7" and "a netlist cannot
	// state this, verify by hand" are both inconclusive results with different next steps, and next
	// steps already live in the message.
	//
	// A rule must only set it where it can NAME the specific thing it could not resolve. Emitting it
	// for everything hard converts a coverage problem into a reporting problem.
	Inconclusive bool
	Rule         string
	Kind         string // KindNet | KindComponent | KindPin
	Subject      string // net name (KindNet) or ref_des (KindComponent | KindPin)
	Pin          string // pin designator, set only when Kind == KindPin
	Message      string
	Prov         *ir.Provenance
	// NetID is the per-instance net identity (ir.Net.id) for a net subject, so two findings on
	// nets that share a Subject name are distinguishable and each locates to ITS wires. Empty for a
	// component/pin subject or a pinless net; consumers then join by Subject (the name).
	NetID string
	// DatasheetProv is the datasheet side of a datasheet-backed finding's provenance, so a consumer
	// (review report, web checks panel) can show which document, page, and section a limit came from
	// without parsing it out of Message. Empty for a finding not backed by a seeded datasheet value.
	//
	// A SLICE because a connection-aware rule rests on more than one part's datasheet (WS3-028): a
	// regulator's output voltage against a downstream part's absolute maximum takes a value from each.
	// The review's data-trust gate reads this to decide whether a finding is trustworthy enough to
	// fail an item, so it has to see every value the conclusion rests on — with one slot, a finding
	// half of whose evidence was a low-confidence extraction would still rate as a hard Fail.
	//
	// A finding is ratified only when EVERY citation clears the floor (see review.isUnratified): the
	// values inside one finding are conjunctive evidence, so the finding is only as trustworthy as its
	// weakest input. That is the opposite quantifier from the one ACROSS findings, where a single
	// trustworthy finding among several makes the item a real Fail — different question, different
	// answer, and both deliberate.
	DatasheetProv []*DatasheetCitation
}

// DatasheetCitation is the structured datasheet provenance of a finding: which document, page, and
// section a datasheet-backed value came from, plus how it was extracted. It is the typed twin of the
// Citation string the built-in datasheet rules embed in a message, so a renderer can column it and,
// crucially, surface Confidence — a low confidence flags a value a reviewer should verify before
// trusting it, rather than burying that in prose. The design side of the dual provenance stays in
// Finding.Prov.
type DatasheetCitation struct {
	Doc        string  // the SourceDoc title (vendor doc number + revision), resolved from DocRef; "" if unresolved
	DocRef     string  // the SourceDoc id the value cites (the stable join key into PartSpec.docs)
	Page       int32   // 1-based page in the document
	Section    string  // the table or figure the value was read from
	Method     string  // how the value was extracted ("hand", "derive/v0", "mock", ...)
	Confidence float64 // extraction confidence in (0, 1]; a low value flags "verify before trusting"
	// Verification is whether a person has stood behind this value AND whether that still holds for
	// the revision the corpus now has: param.Unverified/Verified/Stale/Unknown, "" for a citation with
	// no parameter behind it (a pin declaration, a relation bound).
	//
	// It is separate from Confidence because the two answer different questions and only one of them
	// can go out of date. Confidence describes an extraction and is fixed the moment the value is
	// produced; verification describes a person's agreement with a specific revision, and the document
	// can move afterwards. A stale verification is the case worth naming: it reads as maximally
	// trustworthy by every older signal precisely because someone did check it once.
	Verification string
}

// Finding subject kinds: what a Finding.Subject refers to.
const (
	KindNet       = "net"
	KindComponent = "component"
	KindPin       = "pin"
	// KindEndpoint is a geometric location rather than a captured entity: a dangling wire endpoint,
	// whose Subject is its "x,y" in the sheet geometry frame (there is no net/component/pin to name).
	KindEndpoint = "endpoint"
	// KindBus is a source bus construct, not a net: its Subject is the display label and its
	// geometry join key is the construct's uuid (Finding.Prov.NativeId), carried to the client as
	// Subject.bus_id so a bus-not-modeled finding highlights its own drawn bus (WS7-042b), never a net.
	KindBus = "bus"
	// KindSymbol is a symbol REFERENCE that failed to resolve (WS1-052), not a placed entity: its
	// Subject is the reference as the source spelled it (`res.sym`, `Library:Symbol`), which names
	// a file that is absent rather than anything in the design. Distinct from KindComponent
	// because the affected ref-des set is a PROPERTY of the finding (Message and the
	// unresolved_symbol relation), not its subject — one missing file is one finding however many
	// parts it cost pins, and a consumer that joined Subject to a component would find nothing.
	KindSymbol = "symbol"
)

// Rule is a named, self-describing check over the IR.
//
// The documentation fields are first-class so a rule is authored in one file and can be
// listed, explained, and grouped without a separate catalog: Summary is the one-line form,
// Impact is what goes wrong when the rule is violated, and Detail is the long-form markdown
// (what it means, why engineers want it, a diagram, the query structure). Category and Tier
// classify it for grouping (index.go Tree) and for the expressiveness tier (docs/19). Primitives
// names the query primitives Eval composes, which documents the rule and seeds later coverage
// analysis.
//
// Eval reads the shared Model and returns findings with Subject/Message/Prov set; Run stamps
// Rule and Severity from the rule's metadata, so a rule states its identity once.
//
// Only the fields the engine acts on are typed: Name (identity/lookup), Severity (sort + gate),
// Reads (the fact dependencies Available derives from), and Eval. Reads names the facts (docs/15
// vocabulary) the rule consumes; it doubles as the incremental capture WS3-004 indexes into the
// fact schema, so keep it in the doc-15 vocabulary verbatim.
//
// Everything classificatory lives in Tags, an open key -> value bag the catalog groups and filters
// on (see TreeBy and Filter). Agni's own rules populate the well-known keys (index.go Key*), but
// Tags is deliberately not a closed schema: rules provided by an operator, a DSL author, or an
// integrator embedding Agni as a library (rules that never live in this check/ folder) add their
// own keys, and the catalog UI pivots by whatever keys are present. Nothing in the engine or the
// IR depends on a Tag, so a new axis needs no core change.
type Rule struct {
	Name       string   // stable identifier, e.g. "single-pin-net"
	Severity   string   // "error" | "warning" | "info"
	Summary    string   // one-line description for listings
	Impact     string   // what goes wrong when violated
	Detail     string   // long-form markdown: meaning, rationale, diagram, query structure
	Primitives []string // query primitives Eval composes (docs/19)
	Reads      []string // facts the rule reads, docs/15 vocabulary (net.pin_count, on_net, param(...))
	// OptionalReads is the subset of Reads a rule consults opportunistically: their absence
	// does not make the rule inapplicable. Available's tier-gate skips them, so a netlist rule
	// that only EXEMPTS findings using a datasheet fact (esd-protection crediting an IC's ESD
	// rating) still runs and reports over a design read with no --params, instead of reading as
	// not-applicable the way a rule whose finding REQUIRES the datasheet does (supply-exceeds-abs-max).
	// Still declared in Reads (twin parity, docs); this only annotates which reads are non-gating.
	OptionalReads []string
	// RequiresCapability lists the source-format capabilities a rule needs to evaluate SOUNDLY.
	// It is a third gating axis alongside the param and board tiers (WS3-096): a rule that reads a
	// construct the format cannot express produces no findings, which is indistinguishable from a
	// clean pass, so Available gates it to not-applicable where the capability is absent. Distinct
	// from Reads (a fact TIER that a --params/board injection supplies): a capability is a property
	// of the design's source format, always answerable from the Model. Every entry here is REQUIRED
	// by construction (the field name says so), which sidesteps the OptionalReads two-valued
	// problem — a capability that merely NARROWS a rule is not declared here; the rule handles that
	// case in its own Eval (as power-input-not-driven and unconnected-pin still do internally).
	RequiresCapability []Capability
	// ParamSymbols lists the datasheet SYMBOLS a rule joins on (the vendor spellings, e.g. the
	// output-current alias set). It closes the same silent-pass hole an inline query's param_symbol
	// closes (WS3-097), for a rule-bound review item: a datasheet rule whose symbol is seeded on no
	// component produces zero findings, which is indistinguishable from a design that is within its
	// limits. A review runner reads this to render needs-data instead. Reads/Available do NOT cover
	// it. Those gate on the params TIER being attached at all, so a run WITH --params but without this
	// particular symbol seeded sails through to a pass.
	//
	// Declare it only where a finding REQUIRES the symbol. A rule that merely consults a value to
	// exempt findings leaves it out, the same distinction OptionalReads draws inside Reads.
	ParamSymbols []string
	Tags         map[string]string // open classification (category, tier, distribution, ...); see index.go Key*
	Eval         func(Model) []Finding
}

// Capability names a source-format ability a rule needs to evaluate soundly (WS3-096). A rule that
// infers a defect from the ABSENCE of a construct the format cannot express (an undriven power net,
// an unwired pin) would false-fire on that format, so it gates itself internally AND declares the
// capability here, letting Available report not-applicable instead of a silent pass. The value is a
// stable contract string, matching the design-level fact / queryable twin the same gate reads.
type Capability string

const (
	// CapTypesPowerOut: the source format classifies power-OUTPUT pins. EDIF (INPUT/OUTPUT/INOUT
	// only) and IPC-2581 (a board format with no pin electrical types) do not, so a rail's driver
	// reads as a plain input and a driver-absence rule (power-input-not-driven) cannot conclude
	// "unpowered". The queryable twin is types_power_out / the design.types_power_out fact.
	CapTypesPowerOut Capability = "types_power_out"
	// CapNoConnectChannel: the design can express intentional no-connect (a NO_CONNECT-typed pin or
	// an nc-marker net name). Without it a per-pin absence rule (unconnected-pin) cannot tell a
	// deliberate open pin from a forgotten one. The queryable twin is has_nc_channel / design.nc_channel.
	CapNoConnectChannel Capability = "nc_channel"
	// CapNetClass: the design carries tool-assigned net-class membership (WS3-105). A rule SCOPED
	// by net class ("every HV net must ...") selects nothing where the field is empty, and a rule
	// that finds nothing to check reports clean — indistinguishable from a pass. Only a KiCad
	// project supplies the field, so an EDIF or IPC-2581 read, a bare .kicad_sch, and a project
	// that declares no classes all need this gate. Unlike the two above it is a property of the
	// design's CONTENT rather than its format grammar, which is the honest reading: a scoped rule
	// has nothing to say either way. The queryable twin is has_netclass / the design.has_netclass fact.
	CapNetClass Capability = "netclass"

	// CapNetClassDefs: the design declares net-class DEFINITIONS — what a class's nets should route
	// at (clearance, track width, via sizes), WS3-111. Deliberately SEPARATE from CapNetClass, because
	// net_settings carries membership and definitions in independent blocks: a project can assign nets
	// to a class it never defines. A declared-vs-actual rule needs the LIMIT, so gating it on the
	// membership capability would let a project with assignments and no definitions run the rule over
	// zero comparisons and report a clean pass. The queryable twin is has_netclass_defs.
	CapNetClassDefs Capability = "netclass_defs"
)

// Run evaluates rules over a Model (the query interface) and returns findings sorted by rule
// then subject. Both the Model and the rule set are supplied by the caller: Run depends only on
// the Model interface, not on how facts are computed, and the caller chooses which rules run
// (the full set, a subset, or diff-gates). Use RunDesign for the common "standard checks over a
// design" case.
func Run(m Model, rules []*Rule) []Finding {
	var out []Finding
	gate := unresolvedSymbolGate(m)
	for _, r := range rules {
		if f, gated := gate(r); gated {
			f.Rule, f.Severity = r.Name, r.Severity
			out = append(out, f)
			continue
		}
		for _, f := range r.Eval(m) {
			f.Rule, f.Severity = r.Name, r.Severity
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Rule != out[j].Rule {
			return out[i].Rule < out[j].Rule
		}
		return out[i].Subject < out[j].Subject
	})
	return out
}

// unresolvedSymbolGate returns a per-rule gate that reports whether a rule cannot be DECIDED
// because the read lost pins (WS1-052), along with the inconclusive finding to emit instead.
//
// The gate is DESIGN-WIDE, not per-subject: any unresolved symbol gates every connectivity rule
// for the whole design. That is deliberately the conservative reading, and it matches the gate
// WS1-013 already applies to dangling endpoints in the readers ("one unresolved placement
// suppresses the whole design's dangles"). A per-subject gate needs a finding-to-refdes mapping
// that does not exist yet; until then, narrowing the gate would mean asserting that the parts we
// did NOT flag are unaffected, which the reader cannot support.
//
// Inconclusive rather than not-applicable, which is the distinction Available draws: a missing
// capability is a permanent property of the source FORMAT, while an unresolved symbol is a
// property of THIS read that a --symbol-path away is fixable.
func unresolvedSymbolGate(m Model) func(*Rule) (Finding, bool) {
	var unresolved []*ir.UnresolvedSymbol
	if m != nil {
		unresolved = m.UnresolvedSymbols()
	}
	if len(unresolved) == 0 {
		return func(*Rule) (Finding, bool) { return Finding{}, false }
	}
	refs := make([]string, 0, len(unresolved))
	for _, u := range unresolved {
		refs = append(refs, u.GetSymref())
	}
	// Terse on purpose. Every gated rule emits this, so a full remediation paragraph here would be
	// repeated once per rule and bury the one finding that explains the cause. symbol-unresolved
	// carries the affected placements and the fix; this only says why THIS rule has no verdict.
	subject := strings.Join(refs, ", ")
	msg := "cannot decide: pins are unknown while " + subject + " is unresolved (see symbol-unresolved)"
	if len(refs) > 1 {
		msg = "cannot decide: pins are unknown while " + subject + " are unresolved (see symbol-unresolved)"
	}
	return func(r *Rule) (Finding, bool) {
		if !readsConnectivity(r) {
			return Finding{}, false
		}
		return Finding{Kind: KindSymbol, Subject: subject, Inconclusive: true, Message: msg, Prov: unresolved[0].GetProv()}, true
	}
}

// readsConnectivity reports whether a rule's conclusions depend on pins existing. A rule that reads
// only net names, component classes or datasheet params is unaffected by a lost symbol and keeps
// evaluating, so the gate costs nothing where it buys nothing.
func readsConnectivity(r *Rule) bool {
	for _, fact := range r.Reads {
		if TierOf(fact) == TierConnectivity {
			return true
		}
	}
	return false
}

// RunDesign evaluates the built-in rule set over the default Model for d: the common entry point
// when a caller just has a design and wants the standard checks. It builds the shared Model
// once (NewModel) and hands it, with the installed built-ins, to Run. The built-ins are installed
// by importing stdlib/rules/builtin; without that import the set is empty and RunDesign returns
// no findings.
func RunDesign(d *ir.Design) []Finding { return Run(NewModel(d), builtinRules) }

// Report maps a selection to findings (the report step every rule ends with). Subject and
// Prov come from the selected entity via mk.
func Report[T any](xs []T, mk func(T) Finding) []Finding {
	out := make([]Finding, 0, len(xs))
	for _, x := range xs {
		out = append(out, mk(x))
	}
	return out
}

// NetFinding and CompFinding are the two subject constructors rules use with Report. They stamp
// Kind so the subject's entity type travels with the finding.
func NetFinding(msg string) func(*ir.Net) Finding {
	return func(n *ir.Net) Finding {
		return Finding{Kind: KindNet, Subject: n.Name, NetID: n.GetId(), Message: msg, Prov: n.Prov}
	}
}

// CompFinding is the component-subject counterpart of NetFinding: a Report constructor that
// stamps KindComponent so the finding's subject is the component ref-des.
func CompFinding(msg string) func(*ir.Component) Finding {
	return func(c *ir.Component) Finding {
		return Finding{Kind: KindComponent, Subject: c.RefDes, Message: msg, Prov: c.Prov}
	}
}
