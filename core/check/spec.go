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

// This file is the rule-as-value layer (WS3-003, docs/19 "A rule is a value"): a Spec is a
// rule body expressed as plain data — a small AST of named primitives over declared facts —
// evaluated by the tiny interpreter below. Nothing downstream changes: a Spec binds into the
// same *Rule shape (Eval closure) every consumer already takes, and the typed core of Rule
// stays exactly C14's. The payoff over a Go closure is threefold: the body is inspectable
// data (diffable, serializable, the Phase-2 DSL's compile target — WS3-007 becomes a parser
// that produces a Spec value), the Reads and Primitives metadata are DERIVED from the body
// instead of hand-maintained (so they cannot drift from what the rule actually does), and
// every fact access goes through Model by construction (so a future indexed fact base,
// WS3-004, lands in one place). Go remains the escape hatch: a Call node invokes a
// registered SpecFunc by name for logic the AST cannot or should not express.

// Term is a Spec expression that produces a value (string, int, or bool) for one entity.
// The Term set is deliberately closed (unexported marker): the bounded vocabulary is what
// keeps the rule layer Datalog-class rather than Turing-complete (docs/19), and what makes
// LLM-generated rules cheap to validate.
type Term interface{ isTerm() }

// Lit is a literal value: a string, an int, or a bool.
type Lit struct{ V any }

// Fact reads a named fact of the entity in scope. Names use the docs/15 read vocabulary
// directly ("net.pin_count", "pin.electrical_type", "component.class", ...) so a Spec's
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

// CountOf counts the members of a collection ("net.connections") matching Where — the
// count/aggregate primitive. A nil Where counts every member.
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

// Cmp compares two terms: "==" and "!=" on any value, "<", "<=", ">", ">=" on ints — the
// arithmetic-compare primitive. Ordering a non-int is false, never a panic, matching the
// absent-tolerant posture of the rest of check.
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

// Match is true when the term's string value matches Pattern (unanchored RE2, compiled
// once at Validate time) — the pattern primitive.
type Match struct {
	T       Term
	Pattern string
}

// ExistsIn is true when any member of a collection matches Where — the exists quantifier.
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
// and the provenance derive from the Over entity set (see specOvers); Name, Severity, the
// prose, and Tags stay on the Rule a Spec binds into — the Spec is only the body.
type Spec struct {
	Over    string
	Let     map[string]Term
	Where   Expr // nil selects every entity
	Message string
}

// SpecFunc is a Go function registered for Spec Call nodes — the FFI escape hatch for
// logic the AST should not express (multi-clause heuristics like
// intentionally_unconnected). Reads and Primitives declare what the function consumes in
// the same vocabulary rules use, so derivation stays honest through the FFI boundary: a
// Spec's derived metadata includes what its called functions declare.
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

// --- entity sets, collections, facts: the interpreter's vocabulary ---
//
// These tables are the spec language's LEXICON: a closed set of names whose meaning is
// defined here, so the AST needs no set-comprehension machinery and Validate/derivation
// stay decidable. They are deliberately private, and the two pressures to open them have
// different answers. An OPTIMIZED implementation of a name is NOT a vocabulary change:
// every resolver is a thin delegate to a Model method, so a faster bnet.vias is a faster
// Model.BoardNets behind the same name (one name = one meaning; WS3-004's indexed fact
// base swaps in there). EXTERNAL vocabulary (an embedder's or the param layer's own
// facts/sets) is real future work but rides the provider story: registration mirroring
// RegisterSpecFunc (declared reads/primitives, a namespace prefix Available can gate on),
// designed with WS3-004/006 when the first consumer arrives (OUT_OF_SCOPE.md).

// overDef describes an Over entity set: how to enumerate it, the finding shape its
// survivors report, and the reads its enumeration implies. bind and pin are optional:
// bind adds extra scope entries beyond the primary (the pins set binds the owning
// component too, so component facts resolve), and pin supplies Finding.Pin for sets
// whose subject is a (component, pin) pair.
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
	// distinguishable (the duplicate-net-name case). Optional: nil for non-net sets.
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
		reads:   []string{"ref_des_collision"},
		elems:   func(m Model) []any { return anySlice(m.RefDesCollisions()) },
		subject: func(e any) string { return e.(*ir.RefDesCollision).RefDes },
		prov: func(e any) *ir.Provenance {
			if c := e.(*ir.RefDesCollision); len(c.Instances) > 0 {
				return c.Instances[0]
			}
			return nil
		},
	},
	// Malformed-input diagnostics: pins claimed by more than one net (the pins-to-net
	// invariant is many-to-one). Same shape as ref_des_collisions: a Model-collected
	// list a thin rule reports.
	"pin_net_conflicts": {
		scope: "pin_conflict", kind: KindPin,
		reads: []string{"pin.on_net", "ref_des_collision"}, // collision is the suppression input

		elems:   func(m Model) []any { return anySlice(m.PinNetConflicts()) },
		subject: func(e any) string { return e.(PinNetConflict).RefDes },
		prov:    func(e any) *ir.Provenance { return e.(PinNetConflict).Prov },
		pin:     func(e any) string { return e.(PinNetConflict).Pin },
	},
	// Board-tier set (WS3-008): one entity per net's routed copper, so geometric
	// findings aggregate per net — copper primitives have no stable identity of their
	// own, and the net name is the join key a consumer can highlight.
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
	// Board-tier collections (WS3-008): a board net's copper, quantified within the
	// board.nets scope. No traverse primitive — the copper is the entity's own body,
	// not a walk to another entity.
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
		// Naming-convention patterns match the leaf by default: hierarchy qualification
		// (docs/22) is the reader's scoping, not the author's spelling.
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
				return DirString(ConnDir(ev.m, c)) // connection attr first: virtual power pins (WS1-014)
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
		reads: []string{"ref_des_collision"},
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
// mapped: unspecified-pin-with-driver keys on "unspecified" meaning "the author declared
// nothing", and a passive pin declares something (any two-terminal part would fire otherwise).
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
	over := specOvers[s.Over]
	out := []Finding{} // non-nil like Report, so a Go Eval and its twin are DeepEqual on empty
	for _, e := range over.elems(m) {
		ents := map[string]any{over.scope: e}
		if over.bind != nil {
			over.bind(e, ents)
		}
		ev := &evalEnv{m: m, spec: s, ents: ents}
		if s.Where != nil && !ev.expr(s.Where) {
			continue
		}
		f := Finding{
			Kind:    over.kind,
			Subject: over.subject(e),
			Message: ev.interpolate(s.Message),
			Prov:    over.prov(e),
		}
		if over.pin != nil {
			f.Pin = over.pin(e)
		}
		if over.netID != nil {
			f.NetID = over.netID(e)
		}
		out = append(out, f)
	}
	return out
}

// Rule binds the spec into a *Rule: meta supplies the identity, severity, prose, and tags;
// the spec supplies Eval and the derived Reads and Primitives. It panics on an invalid
// spec — binding happens at package init / registry-build time, where a bad spec is a
// programming error, not an input error.
func (s *Spec) Rule(meta Rule) *Rule {
	if err := s.Validate(); err != nil {
		panic(fmt.Sprintf("check: invalid spec for rule %q: %v", meta.Name, err))
	}
	meta.Eval = s.Eval
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

// interpolate renders the message template for the current entity: a placeholder resolves
// as a Let binding first, then as a Fact; ":q" quotes the value like %q.
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
// runs concurrently (the serve API checks designs in parallel requests); compilation is
// on-demand so an unvalidated-but-well-formed spec still evaluates, while a malformed
// pattern panics — Validate is the error-returning path for specs from untrusted sources.
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

// Validate checks the spec is closed over the interpreter's vocabulary: the Over set, every
// Fact, collection, Call target, Var binding, Cmp operator, Match pattern, and message
// placeholder must resolve. A valid spec cannot fail at Eval time, which is what lets Eval
// return findings instead of errors (the same contract as a Go Eval closure).
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

// DerivedReads returns the facts the spec reads (docs/15 vocabulary), sorted: the union of
// its Over set's, facts', collections', and called functions' declared reads. This is what
// keeps a spec-built rule's Reads honest — it is computed from the body, so it cannot say
// less (or more) than the rule does.
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
// that are not Let bindings) — they are reads too, even when the Where clause never touches
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

// DerivedPrimitives returns the query primitives the spec composes (docs/19 vocabulary),
// sorted. Every spec is a selection, so "select" is always present; quantifiers add
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

// walk visits every Expr and Term in the spec (Where, every Let binding, and all nested
// operands), calling f on each node. Message placeholders are resolved separately by
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
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
