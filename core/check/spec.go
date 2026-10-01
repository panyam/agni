package check

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// This file is the rule-as-value layer (WS3-003). A Spec is a rule body as plain data, a small
// AST of named primitives over declared facts, evaluated by the interpreter below. It binds into
// the same *Rule shape a Go closure does (the typed core stays C14's), its Reads and Primitives
// are DERIVED from the body, and every fact access goes through Model. A Call node invokes a
// registered SpecFunc for logic the AST should not express. The design is in
// docsite/content/architecture/rules-and-checks.md#a-rule-is-a-value.

// Term is a Spec expression that produces a value (string, int, or bool) for one entity.
// The Term set is closed by an unexported marker, which keeps the rule layer Datalog-class
// rather than Turing-complete (docsite/content/architecture/rules-and-checks.md) and keeps
// LLM-generated rules cheap to validate.
type Term interface{ isTerm() }

// Lit is a literal value: a string, an int, or a bool.
type Lit struct{ V any }

// Fact reads a named fact of the entity in scope. Names use the read vocabulary of
// docsite/content/architecture/web-app.md directly ("net.pin_count", "pin.electrical_type", "component.class", ...) so a Spec's
// derived Reads are its fact names verbatim; see specFacts for the full vocabulary and
// which entity scope each fact resolves against.
type Fact struct{ Name string }

// Var reads a Spec.Let binding, evaluated at most once per entity (memoized), so a value
// can be shared between the Where clause and the Message template.
type Var struct{ Name string }

// Call invokes a registered SpecFunc (the Go FFI escape hatch) with evaluated Args. The
// function also receives the entities in scope, so an entity-shaped helper (e.g.
// intentionally_unconnected) takes no explicit args.
type Call struct {
	Fn   string
	Args []Term
}

// CountOf counts the members of a collection ("net.connections") matching Where. It is the
// count primitive, and a nil Where counts every member.
type CountOf struct {
	Over  string
	Where Expr
}

func (Lit) isTerm()     {}
func (Fact) isTerm()    {}
func (Var) isTerm()     {}
func (Call) isTerm()    {}
func (CountOf) isTerm() {}

// Expr is a Spec predicate over one entity. Closed for the same reason as Term.
type Expr interface{ isExpr() }

// And is true when every member is (an empty And is true).
type And struct{ Xs []Expr }

// Or is true when any member is (an empty Or is false).
type Or struct{ Xs []Expr }

// Not negates its operand.
type Not struct{ X Expr }

// Cmp compares two terms, with "==" and "!=" on any value and "<", "<=", ">", ">=" on ints.
// Ordering a non-int is false, never a panic.
type Cmp struct {
	L  Term
	Op string
	R  Term
}

// In is true when the term's value is one of Set (string membership).
type In struct {
	T   Term
	Set []string
}

// Match is true when the term's string value matches Pattern, an unanchored RE2 compiled
// once at Validate time.
type Match struct {
	T       Term
	Pattern string
}

// ExistsIn is true when any member of a collection matches Where.
type ExistsIn struct {
	Over  string
	Where Expr
}

// IsTrue is true when the term evaluates to boolean true. It is how a bool Fact or a bool
// FFI Call is used as a predicate.
type IsTrue struct{ T Term }

func (And) isExpr()      {}
func (Or) isExpr()       {}
func (Not) isExpr()      {}
func (Cmp) isExpr()      {}
func (In) isExpr()       {}
func (Match) isExpr()    {}
func (ExistsIn) isExpr() {}
func (IsTrue) isExpr()   {}

// Spec is a rule body as a value: select the Over entity set, keep the entities matching
// Where, and report one finding per survivor. Let names intermediate terms usable in both
// Where (via Var) and Message. Message is a template; "{name}" interpolates a Let binding
// or a Fact by name, and "{name:q}" quotes the value like %q. Kind, the finding subject,
// and the provenance derive from the Over entity set (see specOvers). Name, Severity, the
// prose, and Tags stay on the Rule a Spec binds into, since the Spec is only the body.
type Spec struct {
	Over string
	Let  map[string]Term
	// Scope is which elements of Over this rule JUDGES. Nil means all of them (#397).
	//
	// Where is the VIOLATION condition, so an element the rule was never about fails it just as a
	// healthy one does. Once the interpreter states a considered set, "Where is false" reads as a
	// claim that the subject is fine, so Scope has to drop the others first. test-point-coverage
	// declares Over "nets" and is about RAILS. On the tutorial gateway design that is 4 rails among
	// 15 nets, and without Scope the rule would assert that 11 signal nets carry a test point.
	//
	// Scope is a per-element predicate in Where's environment, so it may quantify into a member
	// collection (ExistsIn over net.connections). It does NOT widen what a subject is. A spec still
	// ranges over one entity set (agni issue 370).
	Scope Expr
	// Where is the violation condition, meaningful only for elements inside Scope. Nil matches every
	// in-scope element.
	Where   Expr
	Message string
}

// SpecFunc is a Go function registered for Spec Call nodes, the FFI escape hatch for logic
// the AST should not express (multi-clause heuristics like intentionally_unconnected).
// Reads and Primitives declare what the function consumes in the rules' vocabulary, and a
// Spec's derived metadata includes them.
//
// Fn receives the Model, the entities in scope keyed by scope name ("net", "conn",
// "component", ...), and the evaluated Args. It returns a string, int, or bool.
type SpecFunc struct {
	Reads      []string
	Primitives []string
	Fn         func(m Model, ents map[string]any, args []any) any
}

// specFuncs is the Call registry. Built-ins live in spec_funcs.go; RegisterSpecFunc adds
// external ones.
var specFuncs = map[string]*SpecFunc{}

// RegisterSpecFunc registers fn for Spec Call nodes under name, replacing any existing
// registration. Registration is meant for package init time (an embedder wiring its own
// helpers before building rules); it is not synchronized for concurrent use.
func RegisterSpecFunc(name string, fn *SpecFunc) { specFuncs[name] = fn }

// --- the interpreter's vocabulary (entity sets, collections, facts) ---
//
// These tables are the spec language's LEXICON, a closed set of names defined here, so the
// AST needs no set-comprehension machinery and Validate and derivation stay decidable. They
// are private. An OPTIMIZED implementation of a name is NOT a vocabulary change, because
// every resolver delegates to a Model method, so a faster bnet.vias is a faster
// Model.BoardNets behind the same name. EXTERNAL vocabulary from an embedder or the param
// layer is future work, and would register the way RegisterSpecFunc does (OUT_OF_SCOPE.md).

// overDef describes an Over entity set: how to enumerate it, the finding shape its
// survivors report, and the reads its enumeration implies. bind and pin are optional.
// bind adds scope entries beyond the primary (the pins set binds the owning component too,
// so component facts resolve), and pin supplies Finding.Pin for sets whose subject is a
// (component, pin) pair.
type overDef struct {
	scope   string // the scope name entities bind under ("net", "component", ...)
	kind    string // Finding.Kind for this set
	reads   []string
	elems   func(m Model) []any
	subject func(e any) string
	prov    func(e any) *ir.Provenance
	bind    func(e any, ents map[string]any)
	pin     func(e any) string
	// netID supplies Finding.NetID for a net-subject set, so two survivors on same-named nets are
	// distinguishable. Nil for non-net sets.
	netID func(e any) string
}

var specOvers = map[string]overDef{
	"nets": {
		scope: "net", kind: KindNet,
		elems:   func(m Model) []any { return anySlice(m.Nets()) },
		subject: func(e any) string { return e.(*ir.Net).Name },
		prov:    func(e any) *ir.Provenance { return e.(*ir.Net).Prov },
		netID:   func(e any) string { return e.(*ir.Net).GetId() },
	},
	"components": {
		scope: "component", kind: KindComponent,
		elems:   func(m Model) []any { return anySlice(m.Components()) },
		subject: func(e any) string { return e.(*ir.Component).RefDes },
		prov:    func(e any) *ir.Provenance { return e.(*ir.Component).Prov },
	},
	"pins": {
		scope: "pin", kind: KindPin,
		elems:   func(m Model) []any { return anySlice(m.Pins()) },
		subject: func(e any) string { return e.(PinInst).Component.RefDes },
		prov:    func(e any) *ir.Provenance { return e.(PinInst).Component.Prov },
		bind:    func(e any, ents map[string]any) { ents["component"] = e.(PinInst).Component },
		pin:     func(e any) string { return e.(PinInst).Designator },
	},
	"dangling_endpoints": {
		scope: "endpoint", kind: KindEndpoint,
		reads:   []string{"wire.endpoint", "wire.junction"},
		elems:   func(m Model) []any { return anySlice(m.DanglingEndpoints()) },
		subject: func(e any) string { p := e.(*ir.DanglingEndpoint); return fmt.Sprintf("%d,%d", p.X, p.Y) },
		prov:    func(e any) *ir.Provenance { return e.(*ir.DanglingEndpoint).Prov },
	},
	"no_junction_endpoints": {
		scope: "endpoint", kind: KindEndpoint,
		reads:   []string{"wire.endpoint", "wire.junction"},
		elems:   func(m Model) []any { return anySlice(m.NoJunctionEndpoints()) },
		subject: func(e any) string { p := e.(*ir.DanglingEndpoint); return fmt.Sprintf("%d,%d", p.X, p.Y) },
		prov:    func(e any) *ir.Provenance { return e.(*ir.DanglingEndpoint).Prov },
	},
	"ref_des_collisions": {
		scope: "collision", kind: KindComponent,
		reads:   []string{"reader.ref_des_collision"},
		elems:   func(m Model) []any { return anySlice(m.RefDesCollisions()) },
		subject: func(e any) string { return e.(*ir.RefDesCollision).RefDes },
		prov: func(e any) *ir.Provenance {
			if c := e.(*ir.RefDesCollision); len(c.Instances) > 0 {
				return c.Instances[0]
			}
			return nil
		},
	},
	// Malformed-input diagnostic for pins claimed by more than one net, since a pin belongs
	// to at most one. Same shape as ref_des_collisions, a Model-collected list a thin rule
	// reports.
	"pin_net_conflicts": {
		scope: "pin_conflict", kind: KindPin,
		reads: []string{"pin.on_net", "reader.ref_des_collision"}, // collision is the suppression input

		elems:   func(m Model) []any { return anySlice(m.PinNetConflicts()) },
		subject: func(e any) string { return e.(PinNetConflict).RefDes },
		prov:    func(e any) *ir.Provenance { return e.(PinNetConflict).Prov },
		pin:     func(e any) string { return e.(PinNetConflict).Pin },
	},
	// Board-tier set (WS3-008), one entity per net's routed copper. Copper primitives have
	// no stable identity of their own, so geometric findings aggregate per net, keyed by
	// the net name a consumer can highlight.
	"board.nets": {
		scope: "bnet", kind: KindNet,
		reads:   []string{"board.copper"},
		elems:   func(m Model) []any { return anySlice(m.BoardNets()) },
		subject: func(e any) string { return e.(BoardNet).Net },
		prov:    func(e any) *ir.Provenance { return nil },
	},
}

// collDef describes a nested collection quantified by ExistsIn/CountOf: which scope its
// members bind under, and the read/primitive the quantification implies (walking a net's
// connections is the traverse primitive and the on_net read).
type collDef struct {
	scope      string
	reads      []string
	primitives []string
	elems      func(ev *evalEnv) []any
}

var specColls = map[string]collDef{
	"net.connections": {
		scope: "conn", reads: []string{"on_net"}, primitives: []string{"traverse"},
		elems: func(ev *evalEnv) []any { return anySlice(ev.ents["net"].(*ir.Net).Connections) },
	},
	// Board-tier collections (WS3-008) over a board net's copper, quantified within the
	// board.nets scope. They carry no traverse primitive, because the copper is the
	// entity's own body rather than a walk to another entity.
	"bnet.segments": {
		scope: "segment", reads: []string{"board.copper"},
		elems: func(ev *evalEnv) []any { return anySlice(ev.ents["bnet"].(BoardNet).Segments) },
	},
	"bnet.vias": {
		scope: "via", reads: []string{"board.copper"},
		elems: func(ev *evalEnv) []any { return anySlice(ev.ents["bnet"].(BoardNet).Vias) },
	},
}

// factDef describes one fact: the reads/primitives referencing it implies, and its
// resolver. A fact resolves against the entity scopes present in the env; facts shared by
// two scopes (component.class) prefer the innermost.
type factDef struct {
	reads      []string
	primitives []string
	get        func(ev *evalEnv) any
}

var specFacts = map[string]factDef{
	"net.names": { // the net's name, as pattern/pairing input
		reads: []string{"net.names"},
		get:   func(ev *evalEnv) any { return ev.ents["net"].(*ir.Net).Name },
	},
	"net.name_leaf": { // the name's leaf segment: "/amp1/SIG" -> "SIG", bare names unchanged.
		// Naming-convention patterns match the leaf by default, because hierarchy qualification
		// is the reader's scoping and not the author's spelling
		// (docsite/content/architecture/net-solving.md).
		reads: []string{"net.names"},
		get: func(ev *evalEnv) any {
			_, leaf := ScopeOf(ev.ents["net"].(*ir.Net).Name)
			return leaf
		},
	},
	"net.pin_count": { // how many connections the net has
		reads: []string{"net.pin_count"}, primitives: []string{"count"},
		get: func(ev *evalEnv) any { return len(ev.ents["net"].(*ir.Net).Connections) },
	},
	"net.attr.external":     netAttrFact("external"),
	"net.attr.global":       netAttrFact("global"),
	"net.attr.power_driven": netAttrFact("power_driven"),
	"pin.electrical_type": { // pin direction of the in-scope connection, or the in-scope pin
		reads: []string{"pin.electrical_type"}, primitives: []string{"pin-role"},
		get: func(ev *evalEnv) any {
			if c, ok := ev.ents["conn"].(*ir.Connection); ok {
				return DirString(ConnDir(ev.m, c)) // connection attr first, for virtual power pins (WS1-014)
			}
			p := ev.ents["pin"].(PinInst)
			return DirString(ev.m.PinDir(p.Component.RefDes, p.Designator))
		},
	},
	"conn.virtual": { // the in-scope connection's component is a virtual symbol (#PWR/#FLG)
		reads: []string{"on_net"},
		get: func(ev *evalEnv) any {
			return IsVirtualRef(ev.ents["conn"].(*ir.Connection).ComponentRef)
		},
	},
	"pin.declared": { // the in-scope connection's pin exists in its part type's pin list
		reads: []string{"pin.electrical_type"},
		get: func(ev *evalEnv) any {
			c := ev.ents["conn"].(*ir.Connection)
			return ev.m.PinDeclared(c.ComponentRef, c.PinRef)
		},
	},
	"pin.on_net": { // whether the in-scope pin appears in any net's connections
		reads: []string{"pin.on_net"}, primitives: []string{"traverse"},
		get: func(ev *evalEnv) any {
			p := ev.ents["pin"].(PinInst)
			return ev.m.PinConnected(p.Component.RefDes, p.Designator)
		},
	},
	"pin_conflict.nets": { // the claiming nets of the in-scope pin-net conflict, joined for messages
		reads: []string{"pin.on_net"},
		get: func(ev *evalEnv) any {
			return strings.Join(ev.ents["pin_conflict"].(PinNetConflict).Nets, ", ")
		},
	},
	"pin.role": { // derived semantic role (anode/cathode/power/ground) of the in-scope pin or connection
		reads: []string{"pin.role"}, primitives: []string{"pin-role"},
		get: func(ev *evalEnv) any {
			if c, ok := ev.ents["conn"].(*ir.Connection); ok {
				return string(ev.m.PinRole(c.ComponentRef, c.PinRef))
			}
			p := ev.ents["pin"].(PinInst)
			return string(ev.m.PinRole(p.Component.RefDes, p.Designator))
		},
	},
	"design.nc_channel": { // design-level: the source can express intentional no-connect
		reads: []string{"net.names", "pin.no_connect"},
		get:   func(ev *evalEnv) any { return ev.m.HasNoConnectChannel() },
	},
	"design.types_power_out": { // design-level: the source format classifies power-output pins (WS3-072 PR2)
		reads: []string{"pin.electrical_type"},
		get:   func(ev *evalEnv) any { return ev.m.FormatTypesPowerOut() },
	},
	"design.has_netclass": { // design-level: some net carries a tool-assigned net class (WS3-105)
		reads: []string{"net.netclass"},
		get:   func(ev *evalEnv) any { return ev.m.HasNetClasses() },
	},
	// Board-tier facts (WS3-008), in the sidecar's units (nm for the KiCad producer).
	"segment.width": {
		reads: []string{"board.copper"},
		get:   func(ev *evalEnv) any { return int(ev.ents["segment"].(BoardSeg).Width) },
	},
	"via.drill": {
		reads: []string{"board.copper"},
		get:   func(ev *evalEnv) any { return int(ev.ents["via"].(BoardVia).Drill) },
	},
	"via.annular": {
		reads: []string{"board.copper"},
		get:   func(ev *evalEnv) any { return int(ev.ents["via"].(BoardVia).Annular()) },
	},
	"component.class": { // device class of the in-scope connection's component, or the component itself
		reads: []string{"component.class"},
		get: func(ev *evalEnv) any {
			if c, ok := ev.ents["conn"].(*ir.Connection); ok {
				return string(ev.m.ComponentClass(c.ComponentRef))
			}
			return string(ev.m.ComponentClass(ev.ents["component"].(*ir.Component).RefDes))
		},
	},
	"component.ref_des": {
		get: func(ev *evalEnv) any { return ev.ents["component"].(*ir.Component).RefDes },
	},
	"on_net": { // whether the in-scope component appears on any net
		reads: []string{"on_net"}, primitives: []string{"traverse"},
		get: func(ev *evalEnv) any { return ev.m.IsConnected(ev.ents["component"].(*ir.Component).RefDes) },
	},
	"collision.instance_count": {
		reads: []string{"reader.ref_des_collision"},
		get:   func(ev *evalEnv) any { return len(ev.ents["collision"].(*ir.RefDesCollision).Instances) },
	},
}

// netAttrFact builds the factDef for one netgraph boolean attribute; all three share the
// net.attributes read.
func netAttrFact(key string) factDef {
	return factDef{
		reads: []string{"net.attributes"},
		get:   func(ev *evalEnv) any { return ev.ents["net"].(*ir.Net).Attributes[key] == "true" },
	}
}

// DirString maps a pin direction to the string vocabulary Specs compare against. Unmapped
// directions (tristate, ...) read as "unspecified" until a rule needs them. PASSIVE is
// mapped because unspecified-pin-with-driver reads "unspecified" as "the author declared
// nothing", and a passive pin declares something (otherwise every two-terminal part fires).
func DirString(d ir.PinDirection) string {
	switch d {
	case ir.PinDirection_PIN_DIRECTION_INPUT:
		return "input"
	case ir.PinDirection_PIN_DIRECTION_PASSIVE:
		return "passive"
	case ir.PinDirection_PIN_DIRECTION_OUTPUT:
		return "output"
	case ir.PinDirection_PIN_DIRECTION_INOUT:
		return "inout"
	case ir.PinDirection_PIN_DIRECTION_POWER_IN:
		return "power_in"
	case ir.PinDirection_PIN_DIRECTION_POWER_OUT:
		return "power_out"
	case ir.PinDirection_PIN_DIRECTION_NO_CONNECT:
		return "no_connect"
	}
	return "unspecified"
}

func anySlice[T any](xs []T) []any {
	out := make([]any, len(xs))
	for i, x := range xs {
		out[i] = x
	}
	return out
}

// --- the interpreter ---

// evalEnv is one entity's evaluation state: the entities in scope and the memoized Let
// values. A fresh env is built per Over entity; nested quantifiers push their member into
// ents for the duration of their Where.
type evalEnv struct {
	m    Model
	spec *Spec
	ents map[string]any
	lets map[string]any
}

// Eval runs the spec over a Model and returns findings with Kind/Subject/Message/Prov set
// (Rule and Severity are stamped by Run, same as a Go Eval). The spec must be valid; use
// Validate (or bind through Rule, which validates) before evaluating specs from an
// untrusted source.
func (s *Spec) Eval(m Model) []Finding {
	return VerdictsToFindings(s.Verdicts(m))
}

// Verdicts is the interpreter as a MAPPER, emitting one verdict per element of Over that the
// rule judges, passes included. It is what a spec-authored rule binds to Rule.Eval.
//
// Scope decides membership of the considered set and Where decides the outcome inside it. An
// out-of-scope element is not a pass, so it gets no verdict at all.
//
// A PASS NAMES THE CLAUSE THAT DECIDED IT. Where is a violation condition, so a pass refutes it
// and the witness names the conjunct that did. "The condition did not hold" would read the same
// on every passing subject, which docsite/content/build/evidence.md calls decoration.
func (s *Spec) Verdicts(m Model) []Verdict {
	over := specOvers[s.Over]
	out := []Verdict{}
	for _, e := range over.elems(m) {
		ents := map[string]any{over.scope: e}
		if over.bind != nil {
			over.bind(e, ents)
		}
		ev := &evalEnv{m: m, spec: s, ents: ents}
		if s.Scope != nil && !ev.expr(s.Scope) {
			continue // out of scope, so no verdict
		}
		// A spec quantifies over ONE entity set, so its subject tuple is always a 1-tuple. The AST
		// cannot express a relation between two entities, so the interpreter needs no notion of arity.
		subj := Entity{Kind: over.kind, Ref: over.subject(e)}
		if over.pin != nil {
			subj.Pin = over.pin(e)
		}
		if over.netID != nil {
			subj.NetID = over.netID(e)
		}
		v := Verdict{Subjects: []Entity{subj}}
		if s.Where == nil || ev.expr(s.Where) {
			v.Outcome = Fail
			f := Finding{Subject: subj, Message: ev.interpolate(s.Message), Prov: over.prov(e)}
			v.Finding = &f
			v.Witness = &Witness{Statement: ev.interpolate(s.Message)}
		} else {
			v.Outcome = Pass
			v.Witness = &Witness{Statement: "passes because " + ev.why(s.Where)}
		}
		out = append(out, v)
	}
	return out
}

// why names the clause that made a violation condition false, in the reader's terms.
//
// For an And it is the FIRST false conjunct, which refutes the whole condition. For an Or every
// branch is false, so all of them are named. Like renderExpr, it renders from the body rather than
// a stored sentence, so it cannot go stale.
func (ev *evalEnv) why(e Expr) string {
	switch x := e.(type) {
	case And:
		for _, sub := range x.Xs {
			if !ev.expr(sub) {
				return ev.why(sub)
			}
		}
	case Or:
		parts := make([]string, 0, len(x.Xs))
		for _, sub := range x.Xs {
			parts = append(parts, ev.why(sub))
		}
		return strings.Join(parts, ", and ")
	case Not:
		// A false Not means its operand HOLDS, so explain why THAT is true, with the value behind it.
		// The operand's syntax would read identically on every passing subject (agni issue 412).
		return ev.holds(x.X)
	case ExistsIn:
		// An absent match reads as an absence ("no connection is a no-connect") rather than a failed
		// test. The COUNT ties the sentence to this subject, so wiring a fifth pin changes it.
		return fmt.Sprintf("no %s where %s (%d examined)", x.Over, renderExpr(x.Where), ev.countMembers(x.Over))
	case Cmp:
		return ev.whyCmp(x)
	case Match:
		return ev.whyLeaf(x.T, "does not match /"+x.Pattern+"/")
	case In:
		return ev.whyLeaf(x.T, "is not one of ["+strings.Join(x.Set, ", ")+"]")
	case IsTrue:
		// Mirrors holdsTrue. A Call returns a bare bool, but its ARGUMENTS are terms this
		// interpreter can read, so a refusal names what was refused.
		if c, ok := x.T.(Call); ok {
			if len(c.Args) > 0 {
				return ev.argValues(c) + ", which " + c.Fn + " does not accept"
			}
			break
		}
		return ev.valueOf(x.T)
	}
	return renderExpr(e) + " does not hold"
}

// holds is why's mirror. It explains a TRUE expression, which a false Not needs because its operand
// is true. It uses the same value machinery as why, so both sides of a negation read the same way.
//
// The cases are the shapes the catalog nests under a Not. Any other shape falls through to the
// rule's syntax, which states no value.
func (ev *evalEnv) holds(e Expr) string {
	switch x := e.(type) {
	case Or:
		// The FIRST true disjunct is the reason. A naming rule's allow-list is an Or of patterns,
		// and the reader wants the one that matched.
		for _, sub := range x.Xs {
			if ev.expr(sub) {
				return ev.holds(sub)
			}
		}
	case And:
		parts := make([]string, 0, len(x.Xs))
		for _, sub := range x.Xs {
			parts = append(parts, ev.holds(sub))
		}
		return strings.Join(parts, ", and ")
	case Not:
		return ev.why(x.X) // a true Not is a false operand, which is why's job
	case Cmp:
		if x.Op == "==" || x.Op == "!=" {
			return ev.valueOf(x.L) // the value IS the relation; "is 2, which is == 2" says it twice
		}
		return ev.valueOf(x.L) + ", which is " + x.Op + " " + renderTerm(x.R)
	case Match:
		return ev.valueOf(x.T) + ", which matches /" + x.Pattern + "/"
	case In:
		return ev.valueOf(x.T) + ", which is one of [" + strings.Join(x.Set, ", ") + "]"
	case IsTrue:
		return ev.holdsTrue(x.T)
	case ExistsIn:
		if m := ev.firstMember(x.Over, x.Where); m != "" {
			return m + " is " + renderExpr(x.Where)
		}
	}
	return renderExpr(e) + " holds"
}

// holdsTrue explains a true term used as a predicate.
//
// A FACT reads as its value. A CALL's FFI returns a bare bool, but its ARGUMENTS are terms this
// interpreter can evaluate, so `ground_name(net.names)` names the value it accepted with no change
// to the SpecFunc contract. That covers every argument-carrying call in the catalog.
//
// An argument-LESS call (`intentionally_unconnected`, `tvs_reach`) takes the whole scope, so there
// is no value to print and the statement says so. Closing that needs a SpecFunc that returns what
// it observed, which cap-voltage needs too (OUT_OF_SCOPE.md).
func (ev *evalEnv) holdsTrue(t Term) string {
	c, ok := t.(Call)
	if !ok {
		return ev.valueOf(t)
	}
	if len(c.Args) == 0 {
		return c.Fn + " accepts this subject, which states no value it read"
	}
	return ev.argValues(c) + ", which " + c.Fn + " accepts"
}

// argValues renders a call's arguments as the values they evaluated to, which is the most a caller can
// say about an FFI that returns a bare bool.
func (ev *evalEnv) argValues(c Call) string {
	parts := make([]string, 0, len(c.Args))
	for _, a := range c.Args {
		parts = append(parts, ev.valueOf(a))
	}
	return strings.Join(parts, ", ")
}

// countMembers is how many members a collection has in the current scope, for a statement that has to
// say how much it looked at rather than only that it found nothing.
func (ev *evalEnv) countMembers(coll string) int {
	n := 0
	ev.eachMember(coll, func() bool { n++; return true })
	return n
}

// firstMember names the first member satisfying where, or "" when the collection gives no label to
// name one by. A true ExistsIn is explained by WHICH member matched.
func (ev *evalEnv) firstMember(coll string, where Expr) string {
	label := ""
	ev.eachMember(coll, func() bool {
		if where != nil && !ev.expr(where) {
			return true
		}
		label = ev.memberLabel(coll)
		return false
	})
	return label
}

// memberLabel names the in-scope member of a collection. It is per collection because identity
// depends on the kind (a connection is a ref-des and a pin, a via is a location). Empty for a
// collection with no label defined, which the caller reads as "cannot name one".
func (ev *evalEnv) memberLabel(coll string) string {
	if coll != "net.connections" {
		return ""
	}
	c, ok := ev.ents["conn"].(*ir.Connection)
	if !ok || c.GetComponentRef() == "" {
		return ""
	}
	if p := c.GetPinRef(); p != "" {
		return c.GetComponentRef() + "." + p
	}
	return c.GetComponentRef()
}

// whyCmp states a false comparison as the subject's ACTUAL value rather than the test applied to it
// (agni issue 391). renderExpr takes no evalEnv, so it can only print the rule's syntax
// ("claims >= 2") and drops the number that decided the pass. An ordering also names the
// threshold it did not reach, so a reader can see what would have made the rule fire.
func (ev *evalEnv) whyCmp(x Cmp) string {
	// "!=" came out false, so the two sides are EQUAL. The value is the whole reason and there is
	// no unmet threshold to report.
	if x.Op == "!=" {
		return ev.valueOf(x.L)
	}
	return ev.valueOf(x.L) + ", not " + renderComparand(x.Op, x.R)
}

// whyLeaf states a false membership or pattern test the same way: the value first, then the test it
// failed.
func (ev *evalEnv) whyLeaf(subject Term, suffix string) string {
	return ev.valueOf(subject) + ", so it " + suffix
}

// valueOf renders "<term> is <value>" for the value the subject actually supplied. The left operand
// is the one that gets read, because that is the subject's side of a comparison; the right side is
// the rule's own threshold.
func (ev *evalEnv) valueOf(t Term) string {
	v := ev.term(t)
	if s, ok := v.(string); ok && s == "" {
		return renderTerm(t) + " is empty" // reads as the absence it is; `is ""` reads as an artefact
	}
	return renderTerm(t) + " is " + renderTerm(Lit{V: v})
}

// renderComparand phrases the threshold half of a comparison that came out false. Equality drops the
// operator, since "not 5" already says it; an ordering keeps it, since "not >= 2" and "not 2" mean
// different things to a reader deciding whether the value is close to firing.
func renderComparand(op string, r Term) string {
	if op == "==" {
		return renderTerm(r)
	}
	return op + " " + renderTerm(r)
}

// Rule binds the spec into a *Rule: meta supplies the identity, severity, prose, and tags;
// the spec supplies Eval and the derived Reads and Primitives. It panics on an invalid
// spec, because binding happens at package init or registry-build time, where a bad spec
// is a programming error rather than an input error.
func (s *Spec) Rule(meta Rule) *Rule {
	if err := s.Validate(); err != nil {
		panic(fmt.Sprintf("check: invalid spec for rule %q: %v", meta.Name, err))
	}
	meta.Eval = s.Verdicts
	// Verdicts emits one per in-scope element of Over, passes included, so a spec states its
	// considered set. A nil Scope claims every element, so the author must narrow it when the rule
	// is about a subset.
	meta.StatesConsideredSet = true
	meta.Reads = s.DerivedReads()
	meta.Primitives = s.DerivedPrimitives()
	return &meta
}

func (ev *evalEnv) expr(e Expr) bool {
	switch x := e.(type) {
	case And:
		for _, sub := range x.Xs {
			if !ev.expr(sub) {
				return false
			}
		}
		return true
	case Or:
		return slices.ContainsFunc(x.Xs, ev.expr)
	case Not:
		return !ev.expr(x.X)
	case Cmp:
		return compare(ev.term(x.L), x.Op, ev.term(x.R))
	case In:
		v, _ := ev.term(x.T).(string)
		return slices.Contains(x.Set, v)
	case Match:
		v, _ := ev.term(x.T).(string)
		return compiledPattern(x.Pattern).MatchString(v)
	case ExistsIn:
		found := false
		ev.eachMember(x.Over, func() bool {
			if x.Where == nil || ev.expr(x.Where) {
				found = true
				return false
			}
			return true
		})
		return found
	case IsTrue:
		v, _ := ev.term(x.T).(bool)
		return v
	}
	return false
}

func (ev *evalEnv) term(t Term) any {
	switch x := t.(type) {
	case Lit:
		return x.V
	case Fact:
		return specFacts[x.Name].get(ev)
	case Var:
		if v, ok := ev.lets[x.Name]; ok {
			return v
		}
		v := ev.term(ev.spec.Let[x.Name])
		if ev.lets == nil {
			ev.lets = map[string]any{}
		}
		ev.lets[x.Name] = v
		return v
	case Call:
		fn := specFuncs[x.Fn]
		args := make([]any, len(x.Args))
		for i, a := range x.Args {
			args[i] = ev.term(a)
		}
		return fn.Fn(ev.m, ev.ents, args)
	case CountOf:
		n := 0
		ev.eachMember(x.Over, func() bool {
			if x.Where == nil || ev.expr(x.Where) {
				n++
			}
			return true
		})
		return n
	}
	return nil
}

// eachMember iterates a collection's members with each bound into scope, stopping early
// when f returns false. The scope entry is restored afterward so sibling quantifiers over
// the same collection do not see a stale member.
func (ev *evalEnv) eachMember(coll string, f func() bool) {
	c := specColls[coll]
	prev, had := ev.ents[c.scope]
	for _, e := range c.elems(ev) {
		ev.ents[c.scope] = e
		if !f() {
			break
		}
	}
	if had {
		ev.ents[c.scope] = prev
	} else {
		delete(ev.ents, c.scope)
	}
}

// compare implements Cmp: equality on any comparable value, ordering on ints only (a
// non-int operand makes an ordering false rather than panicking).
func compare(l any, op string, r any) bool {
	switch op {
	case "==":
		return l == r
	case "!=":
		return l != r
	}
	li, lok := l.(int)
	ri, rok := r.(int)
	if !lok || !rok {
		return false
	}
	switch op {
	case "<":
		return li < ri
	case "<=":
		return li <= ri
	case ">":
		return li > ri
	case ">=":
		return li >= ri
	}
	return false
}

// placeholderRe matches message-template placeholders: {name} or {name:q}.
var placeholderRe = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_.]*)(:q)?\}`)

// interpolate renders the message template for the current entity. A placeholder resolves
// as a Let binding first, then as a Fact, and ":q" quotes the value like %q.
func (ev *evalEnv) interpolate(msg string) string {
	return placeholderRe.ReplaceAllStringFunc(msg, func(ph string) string {
		parts := placeholderRe.FindStringSubmatch(ph)
		name, quote := parts[1], parts[2] == ":q"
		var v any
		if _, ok := ev.spec.Let[name]; ok {
			v = ev.term(Var{name})
		} else {
			v = ev.term(Fact{name})
		}
		s := formatValue(v)
		if quote {
			s = strconv.Quote(s)
		}
		return s
	})
}

func formatValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int:
		return strconv.Itoa(x)
	case bool:
		return strconv.FormatBool(x)
	}
	return fmt.Sprint(v)
}

// patternCache memoizes compiled Match regexps. It is a sync.Map because rule evaluation
// runs concurrently (the serve API checks designs in parallel requests). Compilation is on
// demand, so an unvalidated but well-formed spec still evaluates and a malformed pattern
// panics. Validate is the error-returning path for specs from untrusted sources.
var patternCache sync.Map

func compiledPattern(p string) *regexp.Regexp {
	if re, ok := patternCache.Load(p); ok {
		return re.(*regexp.Regexp)
	}
	re := regexp.MustCompile(p)
	patternCache.Store(p, re)
	return re
}

// --- validation and metadata derivation ---

// Validate checks that the Over set and every Fact, collection, Call target, Var binding,
// Cmp operator, Match pattern, and message placeholder resolve in the interpreter's
// vocabulary. A valid spec cannot fail at Eval time, which lets Eval return findings
// instead of errors, the same contract as a Go Eval closure.
func (s *Spec) Validate() error {
	if _, ok := specOvers[s.Over]; !ok {
		return fmt.Errorf("unknown entity set %q", s.Over)
	}
	var err error
	s.walk(func(n any) {
		if err != nil {
			return
		}
		switch x := n.(type) {
		case Fact:
			if _, ok := specFacts[x.Name]; !ok {
				err = fmt.Errorf("unknown fact %q", x.Name)
			}
		case Var:
			if _, ok := s.Let[x.Name]; !ok {
				err = fmt.Errorf("unbound var %q", x.Name)
			}
		case Call:
			if _, ok := specFuncs[x.Fn]; !ok {
				err = fmt.Errorf("unregistered func %q", x.Fn)
			}
		case CountOf:
			if _, ok := specColls[x.Over]; !ok {
				err = fmt.Errorf("unknown collection %q", x.Over)
			}
		case ExistsIn:
			if _, ok := specColls[x.Over]; !ok {
				err = fmt.Errorf("unknown collection %q", x.Over)
			}
		case Cmp:
			switch x.Op {
			case "==", "!=", "<", "<=", ">", ">=":
			default:
				err = fmt.Errorf("unknown comparison operator %q", x.Op)
			}
		case Match:
			re, cErr := regexp.Compile(x.Pattern)
			if cErr != nil {
				err = fmt.Errorf("bad pattern %q: %v", x.Pattern, cErr)
			} else {
				patternCache.Store(x.Pattern, re)
			}
		}
	})
	if err != nil {
		return err
	}
	for _, m := range placeholderRe.FindAllStringSubmatch(s.Message, -1) {
		name := m[1]
		if _, ok := s.Let[name]; ok {
			continue
		}
		if _, ok := specFacts[name]; !ok {
			return fmt.Errorf("message placeholder {%s} is neither a let binding nor a fact", name)
		}
	}
	return nil
}

// DerivedReads returns the facts the spec reads (the vocabulary of
// docsite/content/architecture/web-app.md), sorted. It is the union of the declared reads of
// its Over set, facts, collections, and called functions. Being computed from the body, a
// spec-built rule's Reads cannot say less or more than the rule does.
func (s *Spec) DerivedReads() []string {
	set := map[string]bool{}
	add := func(rs []string) {
		for _, r := range rs {
			set[r] = true
		}
	}
	add(specOvers[s.Over].reads)
	s.walk(func(n any) {
		switch x := n.(type) {
		case Fact:
			add(specFacts[x.Name].reads)
		case Call:
			add(specFuncs[x.Fn].Reads)
		case CountOf:
			add(specColls[x.Over].reads)
		case ExistsIn:
			add(specColls[x.Over].reads)
		}
	})
	for _, f := range s.messageFacts() {
		add(specFacts[f].reads)
	}
	return sortedKeys(set)
}

// messageFacts returns the facts the message template interpolates directly (placeholders
// that are not Let bindings). They are reads too, even when the Where clause never touches
// them.
func (s *Spec) messageFacts() []string {
	var out []string
	for _, m := range placeholderRe.FindAllStringSubmatch(s.Message, -1) {
		if _, isLet := s.Let[m[1]]; !isLet {
			if _, isFact := specFacts[m[1]]; isFact {
				out = append(out, m[1])
			}
		}
	}
	return out
}

// DerivedPrimitives returns the query primitives the spec composes (the vocabulary of
// docsite/content/architecture/rules-and-checks.md), sorted. Every spec is a selection, so "select" is always present; quantifiers add
// exists/count plus their collection's traversal, Match adds pattern, facts and called
// functions add what they declare.
func (s *Spec) DerivedPrimitives() []string {
	set := map[string]bool{"select": true}
	add := func(ps []string) {
		for _, p := range ps {
			set[p] = true
		}
	}
	s.walk(func(n any) {
		switch x := n.(type) {
		case Fact:
			add(specFacts[x.Name].primitives)
		case Call:
			add(specFuncs[x.Fn].Primitives)
		case Match:
			add([]string{"pattern"})
		case CountOf:
			add([]string{"count"})
			add(specColls[x.Over].primitives)
		case ExistsIn:
			add([]string{"exists"})
			add(specColls[x.Over].primitives)
		}
	})
	for _, f := range s.messageFacts() {
		add(specFacts[f].primitives)
	}
	return sortedKeys(set)
}

// walk visits every Expr and Term in the spec (every Let binding, Where, Scope, and all
// nested operands), calling f on each node. Message placeholders are resolved separately by
// Validate/interpolate since they are strings, not nodes.
func (s *Spec) walk(f func(n any)) {
	var expr func(Expr)
	var term func(Term)
	term = func(t Term) {
		if t == nil {
			return
		}
		f(t)
		switch x := t.(type) {
		case Call:
			for _, a := range x.Args {
				term(a)
			}
		case CountOf:
			expr(x.Where)
		}
	}
	expr = func(e Expr) {
		if e == nil {
			return
		}
		f(e)
		switch x := e.(type) {
		case And:
			for _, sub := range x.Xs {
				expr(sub)
			}
		case Or:
			for _, sub := range x.Xs {
				expr(sub)
			}
		case Not:
			expr(x.X)
		case Cmp:
			term(x.L)
			term(x.R)
		case In:
			term(x.T)
		case Match:
			term(x.T)
		case ExistsIn:
			expr(x.Where)
		case IsTrue:
			term(x.T)
		}
	}
	for _, t := range s.Let {
		term(t)
	}
	expr(s.Where)
	// DerivedReads feeds Available, so a fact consulted only in Scope must be walked or the rule
	// would run against a tier it needs. Moving a clause from Where to Scope changes what its
	// falsehood means, never what the rule declares.
	expr(s.Scope)
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// renderExpr describes a spec expression the way a reader of the rule would say it, for a witness.
//
// It is structural rather than a stored sentence per rule, so it cannot go stale when the body
// changes. It is terse because it appears inside a one-line statement beside the subject.
func renderExpr(e Expr) string {
	switch x := e.(type) {
	case And:
		return strings.Join(renderEach(x.Xs), " and ")
	case Or:
		return strings.Join(renderEach(x.Xs), " or ")
	case Not:
		return "not (" + renderExpr(x.X) + ")"
	case Cmp:
		return fmt.Sprintf("%s %s %s", renderTerm(x.L), x.Op, renderTerm(x.R))
	case In:
		return fmt.Sprintf("%s is one of [%s]", renderTerm(x.T), strings.Join(x.Set, ", "))
	case Match:
		return fmt.Sprintf("%s matches /%s/", renderTerm(x.T), x.Pattern)
	case ExistsIn:
		return fmt.Sprintf("some %s where %s", x.Over, renderExpr(x.Where))
	case IsTrue:
		return renderTerm(x.T)
	}
	return "the condition"
}

func renderEach(xs []Expr) []string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		out = append(out, renderExpr(x))
	}
	return out
}

// renderTerm describes a term. A Call renders as its function name applied to its arguments, which
// is what the rule author wrote and what the rule's doc page explains.
func renderTerm(t Term) string {
	switch x := t.(type) {
	case Lit:
		if str, ok := x.V.(string); ok {
			return strconv.Quote(str) // so an empty literal reads as "" rather than vanishing
		}
		return fmt.Sprintf("%v", x.V)
	case Fact:
		return x.Name
	case Var:
		return x.Name
	case Call:
		if len(x.Args) == 0 {
			return x.Fn
		}
		args := make([]string, 0, len(x.Args))
		for _, a := range x.Args {
			args = append(args, renderTerm(a))
		}
		return x.Fn + "(" + strings.Join(args, ", ") + ")"
	case CountOf:
		return "count of " + x.Over
	}
	return "a value"
}
