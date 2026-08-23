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
	"fmt"
	"sort"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Finding is one rule violation. Prov locates it in the source (nil when the source carries
// no provenance for the subject).
//
// The subject travels as an Entity rather than as loose kind/ref/pin strings, so a consumer can
// group and highlight by entity type without re-guessing from the string, and so a rule builds it
// through a constructor that knows what each kind requires (see NetFinding/CompFinding).
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
	// Subject is the ONE entity a reader has to change to fix this, and singular is the point.
	//
	// A Verdict names every entity the rule quantified over, because that tuple is its IDENTITY and
	// an incomplete one collides. A Finding answers a different question, "what do I edit", and an
	// answer with three entities in it is not one a reader can act on: they would have to pick, and
	// the rule's author knows which one better than they do. Everything else the sentence names goes
	// in Context, typed and clickable and with equal structural standing for a consumer that wants it.
	//
	// The tie between the two is checked rather than assumed: this entity's Ref must be one of the
	// refs in the verdict's subject tuple, so the thing a reader is told to change is always one of
	// the things the rule looked at.
	Subject Entity
	Message string
	Prov    *ir.Provenance
	// Context are the entities this finding's message NAMES but is not ABOUT (agni issue 349).
	//
	// A Finding carries one Subject, so a rule whose sentence involves two entities could highlight
	// only one of them, and it was not always the one the sentence named. crystal-load-caps reads
	// "crystal terminal net XOUT1 has no load capacitor" with the CRYSTAL as its subject, so clicking
	// it sent the reader to a part while the message talked about a net; and since a crystal has two
	// terminals and both sit inside the highlighted symbol, the drawing could not say which one was at
	// fault. The net was known and was thrown away when the Finding was built.
	//
	// A rule sets it where its message names a real design entity other than the subject. It is not a
	// place for values, thresholds, or units: those belong in the message, and a consumer renders these
	// as clickable entities.
	//
	// Empty is the common case and always will be. See ContextSubject for what a Role means and why
	// order matters.
	Context []ContextSubject
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

// ContextSubject is one entity a finding's message names but is not about, with the part it plays.
//
// It repeats the subject's identity fields rather than reusing Finding, because a context entity has
// no severity, no message and no provenance of its own: it is a REFERENCE to something already in
// the design, not a second finding.
//
// Role is a short lower-case noun naming the part this entity plays in the rule's sentence
// ("terminal", "rail", "source"). It is the author's vocabulary rather than a closed set, because the
// useful name is rule-specific and an enum would collapse distinctions the message depends on.
//
// A Role is NOT unique within a finding. Two entities can play the same part, which is the
// i2c-address-collision shape ("A and B both strap to address N"), so a consumer must treat Context
// as a list rather than a map. Order is the author's and matches the order the message names them, so
// a panel rendering chips reads in the same order as the sentence above it.
type ContextSubject struct {
	Entity
	Role string
}

// Ctx pairs an entity with the part it plays, which is the only way a ContextSubject is ever built.
// The constructor exists so the two halves cannot be assembled out of loose strings: an entity comes
// from one of the constructors below, which is what keeps a net's NetID and a pin's designator from
// being forgotten at a call site that only had the name to hand.
func Ctx(e Entity, role string) ContextSubject { return ContextSubject{Entity: e, Role: role} }

// Entity names ONE design entity, and it is the single element type every subject and every context
// list is built from.
//
// ONE TYPE RATHER THAN TWO SHAPES, which is what makes a subject and a context entry
// interchangeable to a consumer that only wants to point at something. Before this they were
// different structs that happened to carry the same four fields, so `subjectsToSpecs` in the viewer
// had to satisfy both structurally and a third producer could have drifted from either.
//
// The fields are IDENTITY only. No severity, no message, no provenance: an entity is a REFERENCE to
// something already in the design, never a second finding about it.
type Entity struct {
	Kind string // KindNet | KindComponent | KindPin | KindEndpoint | KindBus | KindSymbol
	Ref  string // net name (KindNet), ref_des (KindComponent | KindPin), or the kind's own spelling
	Pin  string // pin designator, set only when Kind == KindPin
	// NetID is the per-instance net identity (ir.Net.id) for a net entity, so two nets sharing a
	// name are distinguishable and each locates to ITS wires. Empty for every other kind, and for a
	// net reached by name only; a consumer then joins by Ref.
	NetID string
}

// The entity constructors. A rule builds an entity through one of these rather than by filling the
// struct, because the struct's correctness is per-kind and invisible at a call site: a net entity
// wants its NetID, a pin entity wants both halves of its key, and a component entity must leave both
// empty. Several rules set a net subject and forgot the NetID for exactly that reason.

// NetEntity names a net, carrying the per-instance id so a duplicate name stays distinguishable.
func NetEntity(n *ir.Net) Entity {
	return Entity{Kind: KindNet, Ref: n.GetName(), NetID: n.GetId()}
}

// NetNameEntity names a net the caller holds only by NAME, so it carries no per-instance id. It is
// the honest constructor for a rule that resolved a rail through a name lookup and never had the
// net itself; a consumer joins it by name and may match either of two same-named nets.
func NetNameEntity(name string) Entity { return Entity{Kind: KindNet, Ref: name} }

// ComponentEntity names a placed part by its reference designator.
func ComponentEntity(refDes string) Entity { return Entity{Kind: KindComponent, Ref: refDes} }

// PinEntity names one terminal of one part. Both halves are required: a designator alone is not a
// pin, and a pin number alone belongs to no part.
func PinEntity(refDes, pin string) Entity {
	return Entity{Kind: KindPin, Ref: refDes, Pin: pin}
}

// BusEntity names a source bus construct by its display label, which is also its geometry join key.
func BusEntity(label string) Entity { return Entity{Kind: KindBus, Ref: label} }

// SymbolEntity names a symbol REFERENCE as the source spelled it, which names a file rather than
// anything placed in the design.
func SymbolEntity(ref string) Entity { return Entity{Kind: KindSymbol, Ref: ref} }

// EndpointEntity names a geometric location in the sheet frame, for the wire-end diagnostics that
// have no captured entity to point at.
func EndpointEntity(x, y int64) Entity {
	return Entity{Kind: KindEndpoint, Ref: fmt.Sprintf("%d,%d", x, y)}
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
	// VerifiedRevision is the document identity as printed when the verification was performed, "" if
	// nothing was ever verified. It exists so a stale citation can name the revision that WAS checked
	// and not only the one the corpus now holds: Doc above is resolved from the CURRENT SourceDoc, so
	// after a re-seed it names the new revision, and the old one survives nowhere else. Reporting
	// "verified against SCES650K, corpus now holds SCES650L" is a re-confirm task someone can act on;
	// reporting two hashes is not. Display only, never compared.
	VerifiedRevision string
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
// Impact is what goes wrong when the rule is violated, Remedy is what to do about it, and Detail
// is the long-form markdown (what it means, why engineers want it, a diagram, the query
// structure). Category and Tier
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
	Name     string // stable identifier, e.g. "single-pin-net"
	Severity string // "error" | "warning" | "info"
	Summary  string // one-line description for listings
	Impact   string // what goes wrong when violated
	// Remedy is what to DO about a violation, in the imperative, as a hardware engineer would say it
	// to another one. Impact says why the finding matters; without this, a reader who accepts that it
	// matters still has to know the fix already.
	//
	// It is deliberately generic over the rule, not over the subject: this is the fix for the RULE,
	// so it names the class of change ("add a bulk capacitor at the rail's entry") and never a
	// specific designator or a computed value. Where the real fix needs a number the engine cannot
	// derive (the resistance an I2C pull-up should be), say what to size it from and stop. Inventing
	// a plausible value here would be the same silent-authority problem the verdict work exists to
	// remove, one layer up.
	Remedy     string
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
	// Eval MAPS every subject the rule was applied to onto a verdict, rather than filtering the
	// design down to what failed. A pass carries the proof it rests on, a subject the rule could not
	// judge says so, and a violation carries the Finding it always did.
	//
	// The findings contract is the PROJECTION of this (VerdictsToFindings), taken by Run rather than
	// restated per rule. That is what keeps a rule's two answers from drifting: there is one body, so
	// "the findings disagree with the verdicts" is not a state a rule can be in.
	Eval func(Model) []Verdict
	// SubjectShape declares the KINDS of a verdict's subject tuple, in the rule's own order, for a rule
	// whose subject is a relation between entities. Empty means the ordinary case: one subject, of
	// whatever kind the rule's own enumeration yields.
	//
	// It exists so a verdict id stays a QUESTION SOMEONE CAN POSE. A reader worried about a terminal can
	// type `pin-exceeds-abs-max:(pin:U12.7)` without running the check first, and that only works while
	// they know what goes in the parentheses. For a 1-tuple the shape is obvious; for
	// `regulator-output-exceeds-abs-max:(component:U1,net:+5V,component:U5)` it is not, and nothing in
	// the output would otherwise say that the source comes before the rail.
	//
	// It is also the honest place to see that a rule's arity is FIXED. A rule emitting a 2-tuple on one
	// design and a 3-tuple on another cannot be indexed by anything, and TestSubjectShapeHolds fails it.
	SubjectShape []string
	// StatesConsideredSet reports whether Eval's verdicts are the rule's full CONSIDERED SET, or only
	// the subjects that failed.
	//
	// It exists because the signature alone cannot say this. A rule still on the pre-verdicts shape
	// returns Fail verdicts and nothing else (see FailuresOnly), which is structurally
	// indistinguishable from a rule that examined exactly those subjects and found them all wanting.
	// Reading the second as the first is a coverage claim the run has not earned, and it is the same
	// silence-reads-as-data mistake one level up from the one verdicts exist to remove. RunVerdicts
	// filters on this, so an unconverted rule contributes nothing rather than contributing a lie.
	//
	// It fails in the SAFE direction. Forgetting it on a genuinely converted rule under-reports that
	// rule; there is no way to set it and be believed while reporting only failures, because setting
	// it is a deliberate claim by the author.
	//
	// MIGRATION-ONLY. When the last rule converts (agni issue 391) every rule sets it, the filter in
	// RunVerdicts becomes a tautology, and the field deletes itself.
	StatesConsideredSet bool
}

// Findings is the rule's verdicts projected onto the findings contract: the violations alone, which
// is what `check` and every consumer of its output read.
//
// It is a projection rather than a second body, so a rule cannot report findings that disagree with
// its verdicts. That used to be a parity test's job over two hand-written functions; now the shape
// makes the disagreement unrepresentable.
func (r *Rule) Findings(m Model) []Finding { return VerdictsToFindings(r.Eval(m)) }

// FailuresOnly adapts a pre-verdicts rule body to the Eval signature, turning each Finding into a
// Fail verdict and claiming nothing about the subjects that did not fail.
//
// It is deliberately conspicuous at the call site. A rule reading `Eval: check.FailuresOnly(...)` is
// telling a reader it has not been converted yet, and `grep -c FailuresOnly` is the remaining work in
// agni issue 391. Converting a rule means deleting the wrapper, writing the map, and setting
// StatesConsideredSet.
//
// The witness is deliberately absent rather than invented. A Fail carries its Finding, which is the
// evidence this shape has always had; manufacturing a Witness by restating the message would be the
// decoration build/evidence.md warns about, and would make an unconverted rule look converted.
func FailuresOnly(eval func(Model) []Finding) func(Model) []Verdict {
	return func(m Model) []Verdict {
		fs := eval(m)
		out := make([]Verdict, 0, len(fs))
		for _, f := range fs {
			outcome := Fail
			if f.Inconclusive {
				outcome = Inconclusive
			}
			out = append(out, Verdict{
				Outcome:  outcome,
				Subjects: []Entity{f.Subject},
				Context:  f.Context,
				Finding:  &f,
			})
		}
		return out
	}
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

	// CapRefDesCollisions: the READER detects ref-des collisions for this design's format. Unlike
	// the capabilities above it is a property of the reader rather than of the format's grammar,
	// and it is declared per read (ir.InputDiagnostics.supplied) rather than inferred here, because
	// only the reader knows whether it looked. A rule whose entire subject is a reader diagnostic
	// finds nothing when nobody looked, and "found nothing" is what a clean pass looks like: on
	// EDIF, IPC-2581, gEDA and xschem, duplicate-ref-des read as passing for as long as it existed
	// (agni issue 309). The queryable twin is supplies_ref_des_collisions.
	CapRefDesCollisions Capability = "ref_des_collisions"

	// CapNetClassDefs: the design declares net-class DEFINITIONS — what a class's nets should route
	// at (clearance, track width, via sizes), WS3-111. Deliberately SEPARATE from CapNetClass, because
	// net_settings carries membership and definitions in independent blocks: a project can assign nets
	// to a class it never defines. A declared-vs-actual rule needs the LIMIT, so gating it on the
	// membership capability would let a project with assignments and no definitions run the rule over
	// zero comparisons and report a clean pass. The queryable twin is has_netclass_defs.
	CapNetClassDefs Capability = "netclass_defs"

	// CapJunctionTaps: the READER examines wire-end-on-wire-body taps and records BOTH halves, the
	// ones something joins and the ones nothing does. A reader-property capability like
	// CapRefDesCollisions, declared per read rather than inferred, and for the same reason: only the
	// KiCad reader looks at wire geometry at all, so on every other format wire-no-junction found
	// nothing and "found nothing" is what a clean pass looks like.
	//
	// It gates on the JOINED half rather than on the diagnostic as a whole, because a reader could
	// record the silent taps without recording the joined ones, which is what the KiCad reader did
	// until agni issue 420 and is a coverage claim a considered set must not make.
	CapJunctionTaps Capability = "junction_taps"
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
		for _, f := range r.Findings(m) {
			f.Rule, f.Severity = r.Name, r.Severity
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Rule != out[j].Rule {
			return out[i].Rule < out[j].Rule
		}
		return EntityRef(out[i].Subject) < EntityRef(out[j].Subject)
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
		return Finding{Subject: SymbolEntity(subject), Inconclusive: true, Message: msg, Prov: unresolved[0].GetProv()}, true
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

// RunVerdicts collects the considered set across every rule that states one, stamping Rule from the
// rule's own name so a verdict carries its identity the way Run stamps a finding's.
//
// Rules that do not state a considered set contribute nothing, and the caller cannot tell that apart
// from a rule that considered no subjects. That is a real limit of this seam while the catalog is
// part converted, and it is why the return is a verdict list rather than anything shaped like a
// coverage report: a coverage claim over a part-converted catalog would be wrong in the direction
// that matters, reporting more assurance than the run has.
//
// The filter is StatesConsideredSet rather than anything read off the verdicts themselves. Every rule
// now returns verdicts, so an unconverted one yields a list of Fail verdicts that is structurally
// identical to a considered set whose every subject failed. Only the author knows which it is, which
// is why the claim is a declaration rather than an inference.
//
// Ordering matches Run: rule name, then subject, so a verdict table and a findings table read down
// the same axis.
func RunVerdicts(m Model, rules []*Rule) []Verdict {
	var out []Verdict
	for _, r := range rules {
		if !r.StatesConsideredSet {
			continue
		}
		for _, v := range r.Eval(m) {
			v.Rule = r.Name
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Rule != out[j].Rule {
			return out[i].Rule < out[j].Rule
		}
		return SubjectRefs(out[i]) < SubjectRefs(out[j])
	})
	return out
}

// Report maps a selection to findings (the report step every rule ends with). Subject and
// Prov come from the selected entity via mk.
func Report[T any](xs []T, mk func(T) Finding) []Finding {
	out := make([]Finding, 0, len(xs))
	for _, x := range xs {
		out = append(out, mk(x))
	}
	return out
}

// NetFinding and CompFinding are the two subject constructors rules use with Report. They build the
// subject through an Entity constructor, so the entity type and a net's per-instance id travel with
// the finding rather than depending on the call site remembering both.
func NetFinding(msg string) func(*ir.Net) Finding {
	return func(n *ir.Net) Finding {
		return Finding{Subject: NetEntity(n), Message: msg, Prov: n.Prov}
	}
}

// CompFinding is the component-subject counterpart of NetFinding: a Report constructor that
// stamps KindComponent so the finding's subject is the component ref-des.
func CompFinding(msg string) func(*ir.Component) Finding {
	return func(c *ir.Component) Finding {
		return Finding{Subject: ComponentEntity(c.RefDes), Message: msg, Prov: c.Prov}
	}
}
