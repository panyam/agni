// Package check runs rule checks over a netlist IR Design. Pure IR operations, no I/O
// (CONSTRAINTS C1).
//
// A rule is a Rule value whose Eval is written against the query primitives in query.go
// (Select/Exists/Count over a shared Model) rather than raw loops over the IR. The built-in rules
// and their Spec twins (spec.go) live in stdlib/rules/builtin and install through RegisterBuiltins.
// See docsite/content/architecture/rules-and-checks.md#the-evaluation-model.
package check

import (
	"fmt"
	"sort"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Finding is one rule violation. Prov locates it in the source, nil when the source carries no
// provenance for the subject. The subject is an Entity, built through NetFinding/CompFinding or an
// Entity constructor, so a consumer can group and highlight by kind without parsing a string.
type Finding struct {
	Severity string // "error" | "warning" | "info"
	// Inconclusive marks a subject the rule examined and could not decide, rather than a defect
	// (agni issue 74). It is a per-subject result and not one of the review's design-wide needs-*
	// preconditions, because some cases have nothing to supply. A netlist states reset polarity
	// nowhere, while an unclassified controller resolves once its spec is seeded, and both are
	// inconclusive. The remedy goes in Message, as for a defect.
	//
	// Set it only where the rule can NAME the thing it could not resolve. See
	// docsite/content/build/check-rule.md#five-outcomes-and-the-three-that-are-not-a-pass.
	Inconclusive bool
	Rule         string
	// Subject is the ONE entity a reader has to change to fix this. A Verdict's subject tuple names
	// everything the rule quantified over; a Finding picks the one to edit and puts the rest in
	// Context. Its Ref must be one of the verdict's subject refs, and that is checked. See
	// docsite/content/build/check-rule.md#a-tuple-in-the-verdict-one-entity-in-the-finding.
	Subject Entity
	Message string
	Prov    *ir.Provenance
	// Context are the entities this finding's message NAMES but is not ABOUT (agni issue 349). A
	// crystal-load-caps finding names the terminal net XOUT1 while its subject is the crystal, and
	// without Context the drawing could not say which terminal was at fault.
	//
	// Only real design entities go here. Values, thresholds and units stay in Message. Usually empty.
	// See ContextSubject for Role and ordering.
	Context []ContextSubject
	// DatasheetProv is the datasheet side of a finding's provenance, one citation per value the
	// conclusion rests on, so a consumer can show document, page and section without parsing Message.
	// Empty for a finding no seeded datasheet value backs. A slice because a connection-aware rule
	// reads more than one part's datasheet (WS3-028).
	//
	// The review ratifies a finding only when EVERY citation clears the confidence floor
	// (review.isUnratified), since one weak input weakens the whole conclusion. Across findings the
	// quantifier flips, and one trustworthy finding makes the item a Fail.
	DatasheetProv []*DatasheetCitation
}

// ContextSubject is one entity a finding's message names but is not about, with the part it plays.
//
// Role is a short lower-case noun from the rule author's vocabulary ("terminal", "rail", "source"),
// not a closed enum. A Role can repeat within a finding (i2c-address-collision's "A and B both strap
// to address N"), so treat Context as a list rather than a map. Order matches the order the message
// names them.
type ContextSubject struct {
	Entity
	Role string
}

// Ctx pairs an entity with its role. Build the entity with one of the constructors below so a net's
// NetID or a pin's designator is not dropped at the call site.
func Ctx(e Entity, role string) ContextSubject { return ContextSubject{Entity: e, Role: role} }

// Entity names ONE design entity. Subjects and context lists share this type, so a consumer that only
// wants to point at something treats them alike. It carries identity only, never severity, message
// or provenance.
type Entity struct {
	Kind string // KindNet | KindComponent | KindPin | KindEndpoint | KindBus | KindSymbol
	Ref  string // net name (KindNet), ref_des (KindComponent | KindPin), or the kind's own spelling
	Pin  string // pin designator, set only when Kind == KindPin
	// NetID is the per-instance net identity (ir.Net.id) for a net entity, so two nets sharing a
	// name are distinguishable and each locates to ITS wires. Empty for every other kind, and for a
	// net reached by name only; a consumer then joins by Ref.
	NetID string
}

// The entity constructors. Build an Entity through one of these rather than a struct literal,
// because each kind needs different fields. A net wants its NetID, a pin wants both halves of its
// key, and a component leaves both empty.

// NetEntity names a net, carrying the per-instance id so a duplicate name stays distinguishable.
func NetEntity(n *ir.Net) Entity {
	return Entity{Kind: KindNet, Ref: n.GetName(), NetID: n.GetId()}
}

// NetNameEntity names a net the caller holds only by NAME, so it carries no per-instance id. Use it
// for a rule that resolved a rail through a name lookup and never had the net itself. A consumer
// joins it by name and may match either of two same-named nets.
func NetNameEntity(name string) Entity { return Entity{Kind: KindNet, Ref: name} }

// ComponentEntity names a placed part by its reference designator.
func ComponentEntity(refDes string) Entity { return Entity{Kind: KindComponent, Ref: refDes} }

// PinEntity names one terminal of one part. Both halves are required.
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

// DatasheetCitation records which document, page and section a datasheet-backed value came from, and
// how it was extracted. It is the typed twin of the Citation string the built-in datasheet rules put
// in a message, so a renderer can show it in columns and surface a low Confidence as "verify before
// trusting". The design side of the provenance stays in Finding.Prov.
type DatasheetCitation struct {
	Doc        string  // the SourceDoc title (vendor doc number + revision), resolved from DocRef; "" if unresolved
	DocRef     string  // the SourceDoc id the value cites (the stable join key into PartSpec.docs)
	Page       int32   // 1-based page in the document
	Section    string  // the table or figure the value was read from
	Method     string  // how the value was extracted ("hand", "derive/v0", "mock", ...)
	Confidence float64 // extraction confidence in (0, 1]; a low value flags "verify before trusting"
	// Verification says whether a person has checked this value and whether that still holds for the
	// revision the corpus now has (param.Unverified/Verified/Stale/Unknown). "" for a citation with no
	// parameter behind it (a pin declaration, a relation bound).
	//
	// Separate from Confidence, which is fixed at extraction. A verification goes stale when the
	// document moves, and a stale one still looks fully trustworthy by every other signal.
	Verification string
	// VerifiedRevision is the document identity as printed when the value was verified, "" if it never
	// was. Doc is resolved from the CURRENT SourceDoc, so after a re-seed this is the only place the
	// checked revision survives, which lets a stale citation say "verified against SCES650K, corpus now
	// holds SCES650L". Display only, never compared.
	VerifiedRevision string
}

// Finding subject kinds, the values of Entity.Kind.
const (
	KindNet       = "net"
	KindComponent = "component"
	KindPin       = "pin"
	// KindEndpoint is a geometric location rather than a captured entity. Its Ref is a dangling wire
	// endpoint's "x,y" in the sheet geometry frame.
	KindEndpoint = "endpoint"
	// KindBus is a source bus construct, not a net. Its Ref is the display label and its geometry
	// join key is the construct's uuid (Finding.Prov.NativeId), sent to the client as
	// Subject.bus_id so a bus-not-modeled finding highlights its own drawn bus (WS7-042b).
	KindBus = "bus"
	// KindSymbol is a symbol REFERENCE that failed to resolve (WS1-052), naming an absent file rather
	// than anything placed. Its Ref is the reference as the source spelled it (`res.sym`,
	// `Library:Symbol`). Not KindComponent, because one missing file is one finding however many parts
	// it cost pins; the affected ref-des live in Message and the unresolved_symbol relation.
	KindSymbol = "symbol"
	// KindSignal is a REQUIREMENT SLOT, the role a profile asks for ("STB", "CANL"), with Ref the
	// signal name as the profile spells it. It is the one kind that may name something the design
	// lacks. An interface rule's verdict subject is (host, role), so a host missing four signals gets
	// four distinct verdict ids rather than one id on four rows. It never highlights, since the
	// finding's own subject is the component.
	KindSignal = "signal"
)

// Rule is a named, self-describing check over the IR. Summary, Impact, Remedy and Detail are its
// documentation, so a rule is authored in one file and listed without a separate catalog.
//
// Only the fields the engine acts on are typed: Name, Severity, Reads and Eval. Reads gates
// availability by fact tier (TierOf matches on the name), so spell each fact as the fact vocabulary
// does. Everything classificatory lives in Tags, an open key -> value bag the catalog groups and
// filters on (TreeBy, Filter), so an operator's or embedder's rule can add its own axes. See
// docsite/content/architecture/rules-and-checks.md#a-rules-library-in-go.
//
// Run stamps Rule and Severity from the rule's metadata, so a rule body never sets them.
type Rule struct {
	Name     string // stable identifier, e.g. "single-pin-net"
	Severity string // "error" | "warning" | "info"
	Summary  string // one-line description for listings
	Impact   string // what goes wrong when violated
	// Remedy is what to DO about a violation, in the imperative, as one hardware engineer would say it
	// to another. It is the fix for the RULE rather than the subject, so it names the class of change
	// ("add a bulk capacitor at the rail's entry") and never a designator or a computed value. Where
	// the fix needs a number the engine cannot derive (an I2C pull-up's resistance), say what to size
	// it from and stop, since an invented value would read as the engine's authority.
	Remedy     string
	Detail     string   // long-form markdown: meaning, rationale, diagram, query structure
	Primitives []string // query primitives Eval composes
	Reads      []string // facts the rule reads, in the fact vocabulary (net.pin_count, on_net, param(...))
	// OptionalReads is the subset of Reads a rule consults only to EXEMPT findings, so their absence
	// does not make it inapplicable. Available's tier gate skips them. esd-protection, for example,
	// credits an IC's ESD rating when a datasheet is attached and still runs without --params, unlike
	// supply-exceeds-abs-max, whose finding REQUIRES the datasheet. Still listed in Reads for twin
	// parity and docs.
	OptionalReads []string
	// RequiresCapability lists the source-format capabilities a rule needs to evaluate SOUNDLY
	// (WS3-096). Where the format cannot express a construct the rule reads, the rule finds nothing
	// and looks like a clean pass, so Available reports not-applicable instead. Unlike Reads, a
	// capability is a property of the source format, always answerable from the Model. Every entry
	// is required; a capability that only NARROWS a rule is handled in its own Eval (as
	// power-input-not-driven and unconnected-pin do).
	RequiresCapability []Capability
	// ParamSymbols lists the datasheet SYMBOLS a rule joins on (the vendor spellings, e.g. the
	// output-current alias set), the rule-bound twin of an inline query's param_symbol (WS3-097). A
	// symbol seeded on no component yields zero findings, which looks like a design within its
	// limits, so a review runner reads this to render needs-data. Reads/Available do not cover it,
	// since they gate only on the params tier being attached.
	//
	// Declare it only where a finding REQUIRES the symbol. A rule that consults a value only to
	// exempt findings leaves it out, as with OptionalReads.
	ParamSymbols []string
	Tags         map[string]string // open classification (category, tier, distribution, ...); see index.go Key*
	// Eval MAPS every subject the rule was applied to onto a verdict. A pass carries its proof, an
	// undecidable subject says so, and a violation carries its Finding. Findings are the projection
	// of this (VerdictsToFindings, taken by Run), so a rule's findings cannot disagree with its
	// verdicts.
	Eval func(Model) []Verdict
	// SubjectShape declares the KINDS of a verdict's subject tuple, in the rule's order, for a rule
	// whose subject is a relation between entities. Empty means one subject of whatever kind the rule
	// enumerates. It tells a reader how to spell a verdict id without running the check, e.g.
	// `regulator-output-exceeds-abs-max:(component:U1,net:+5V,component:U5)`, where nothing else says
	// the source comes before the rail (see verdict.go for the id form).
	//
	// The arity is FIXED. TestSubjectShapeHolds fails a rule that emits different tuple sizes.
	SubjectShape []string
	// StatesConsideredSet reports whether Eval's verdicts are the rule's full CONSIDERED SET or only
	// its failures. The signature cannot say which. A pre-verdicts rule wrapped in FailuresOnly
	// returns only Fail verdicts, indistinguishable from a rule that examined exactly those subjects
	// and found them all wanting. RunVerdicts filters on it, so an unconverted rule contributes
	// nothing. Forgetting it under-reports a converted rule and can never over-report one.
	//
	// MIGRATION-ONLY. Once every rule converts (agni issue 391) the filter in RunVerdicts is a
	// tautology and the field can go.
	StatesConsideredSet bool
}

// Findings is the rule's verdicts projected onto the findings contract, the violations alone, which
// is what `check` and every consumer of its output read. It is a projection rather than a second
// body, so the two cannot disagree.
func (r *Rule) Findings(m Model) []Finding { return VerdictsToFindings(r.Eval(m)) }

// FailuresOnly adapts a pre-verdicts rule body to the Eval signature, turning each Finding into a
// Fail (or Inconclusive) verdict and claiming nothing about the subjects that did not fail. A built-in
// rule using it has not been converted (agni issue 391). Converting means deleting the wrapper,
// writing the map, and setting StatesConsideredSet. core/query's RuleFromQuery also uses it.
//
// It invents no Witness. The Finding is the evidence, and a Witness restating the message would make
// an unconverted rule look converted.
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
// infers a defect from the ABSENCE of a construct the format cannot express would false-fire there,
// so it declares the capability and Available reports not-applicable instead of a silent pass. The
// value is a stable contract string matching the design-level fact the same gate reads. Which formats
// lack each one is tabled in
// docsite/content/architecture/rules-and-checks.md#source-format-capabilities.
type Capability string

const (
	// CapTypesPowerOut means the source format classifies power-OUTPUT pins. Without it a rail's
	// driver reads as a plain input and power-input-not-driven cannot conclude "unpowered". The
	// queryable twin is types_power_out / the design.types_power_out fact.
	CapTypesPowerOut Capability = "types_power_out"
	// CapNoConnectChannel means the design can express an intentional no-connect (a NO_CONNECT-typed
	// pin or an nc-marker net name). Without it unconnected-pin cannot tell a deliberate open pin from
	// a forgotten one. The queryable twin is has_nc_channel / design.nc_channel.
	CapNoConnectChannel Capability = "nc_channel"
	// CapNetClass means the design carries tool-assigned net-class membership (WS3-105). A rule scoped
	// by net class selects nothing without it and reports clean. It depends on the design's CONTENT,
	// not its format grammar. The queryable twin is has_netclass / the design.has_netclass fact.
	CapNetClass Capability = "netclass"

	// CapRefDesCollisions means the READER checked this design for ref-des collisions. It is declared
	// per read (ir.InputDiagnostics.supplied) because only the reader knows whether it looked, and
	// duplicate-ref-des read as passing on formats where nothing looked (agni issue 309). The
	// queryable twin is supplies_ref_des_collisions.
	CapRefDesCollisions Capability = "ref_des_collisions"

	// CapNetClassDefs means the design declares net-class DEFINITIONS, the clearance, track width and
	// via sizes a class's nets should route at (WS3-111). Separate from CapNetClass because a project
	// can assign nets to a class it never defines, and a declared-vs-actual rule gated on membership
	// would then run over zero comparisons and pass. The queryable twin is has_netclass_defs.
	CapNetClassDefs Capability = "netclass_defs"

	// CapJunctionTaps means the READER examined wire-end-on-wire-body taps and recorded both the joined
	// and the silent ones, declared per read like CapRefDesCollisions. Only the KiCad reader looks at
	// wire geometry, so elsewhere wire-no-junction would find nothing and read as a pass. It gates on
	// the JOINED half, since a reader recording only the silent taps (the KiCad reader until agni issue
	// 420) has not examined the considered set.
	CapJunctionTaps Capability = "junction_taps"
)

// Run evaluates rules over a Model and returns findings sorted by rule, then subject. The caller
// picks the rules (the full set, a subset, diff gates); RunDesign is the shortcut for the standard
// checks over a design. On a design with an unresolved symbol, a connectivity rule emits one
// Inconclusive finding instead of evaluating (see unresolvedSymbolGate).
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

// unresolvedSymbolGate returns a per-rule gate reporting whether a rule cannot be DECIDED because the
// read lost pins (WS1-052), with the Inconclusive finding to emit instead.
//
// The gate is DESIGN-WIDE, so one unresolved symbol gates every connectivity rule, as WS1-013 does
// for dangling endpoints in the readers. A per-subject gate needs a finding-to-refdes mapping that
// does not exist yet, and without one it would claim the unflagged parts are unaffected.
//
// Inconclusive rather than not-applicable, because a --symbol-path fixes an unresolved symbol while a
// missing capability is permanent for the format.
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
	// Short because every gated rule repeats it. symbol-unresolved carries the affected placements
	// and the fix.
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
// only net names, component classes or datasheet params keeps evaluating when a symbol is lost.
func readsConnectivity(r *Rule) bool {
	for _, fact := range r.Reads {
		if TierOf(fact) == TierConnectivity {
			return true
		}
	}
	return false
}

// RunDesign runs the installed built-in rules over NewModel(d). The built-ins install by importing
// stdlib/rules/builtin; without that import the set is empty and RunDesign returns no findings.
func RunDesign(d *ir.Design) []Finding { return Run(NewModel(d), builtinRules) }

// RunVerdicts collects verdicts from every rule that sets StatesConsideredSet, stamping Rule from the
// rule's name, and orders them as Run does (rule, then subject).
//
// A rule that does not state a considered set contributes nothing, and a caller cannot tell that
// apart from a rule that considered no subjects. So this returns a verdict list rather than a
// coverage report, which over a part-converted catalog would overstate the run. See
// StatesConsideredSet for why the filter is a declaration.
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

// NetFinding is the Report constructor for a net subject. It builds the subject with NetEntity, so
// the net's per-instance id travels with the finding.
func NetFinding(msg string) func(*ir.Net) Finding {
	return func(n *ir.Net) Finding {
		return Finding{Subject: NetEntity(n), Message: msg, Prov: n.Prov}
	}
}

// CompFinding is the component-subject counterpart of NetFinding.
func CompFinding(msg string) func(*ir.Component) Finding {
	return func(c *ir.Component) Finding {
		return Finding{Subject: ComponentEntity(c.RefDes), Message: msg, Prov: c.Prov}
	}
}
