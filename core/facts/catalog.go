package facts

// RelationInfo describes one queryable relation or predicate for discovery surfaces (the web query
// panel's relation picker today; a CLI listing or an LSP later): its name, the argument labels a
// template inserts as `?arg`, a one-line summary, and a Kind for grouping. It is metadata only —
// the authoritative behavior stays in the schema (layout) and the projectors (rows); this is the
// human-facing catalog over them.
type RelationInfo struct {
	Name    string
	Args    []string
	Summary string
	Kind    string // "netlist" | "board" | "datasheet" | "predicate" | "overlay"
	// Detail is the relation's rich reference markdown (WS14-005), or "" when the relation has no doc
	// yet (the staged backfill: not every relation is documented on day one). It is the deep-dive
	// behind Summary — a discovery surface shows Summary in a list and Detail on demand. It is not
	// part of the arity-vs-schema assertion.
	Detail string
	// ArgKinds declares what an argument DENOTES, keyed by its label in Args. Present only for the
	// arguments that name something a reader can act on; absent means a scalar, which is most of them.
	//
	// It exists because Args is PROSE. A surface turning an answer cell into a clickable entity used
	// to decide by matching those label strings, so "ref_des" meant a component and "pin" meant a
	// design pin, which put a type system inside a naming convention and needed a hand-written guard
	// every time the convention misfired (agni issue 548). Nothing kept a label and a column's meaning
	// in agreement, and nothing failed when they drifted.
	ArgKinds map[string]ArgKind
}

// ArgKind is what one relation argument denotes. The zero value is a scalar, so a relation declares
// only the arguments that are more than that.
//
// Two of the three forms are RELATIONAL rather than a flat kind, which is the reason this is a struct
// and not a string. A polymorphic column takes its kind from another column's VALUE (`entity(name,
// kind)` yields a component, a net and a bus in one answer set), and a pin is only a design pin when
// the relation also says which component it belongs to (`pin.net(ref_des, pin, net)` does;
// `param.pin(mpn, pin, ...)` names a pin of a part TYPE, which is not a thing on a canvas).
type ArgKind struct {
	// Entity is the fixed kind this argument names, in check's vocabulary (KindNet, KindComponent,
	// KindBus, KindPin). Empty when the kind comes from another argument.
	Entity string
	// KindArg is the argument whose per-row VALUE gives this one's entity kind. Set only for a
	// polymorphic column, and then Entity is empty.
	KindArg string
	// OwnerArg is the argument naming the component this one belongs to. Set only on a pin, where a
	// pin without its component cannot be located.
	OwnerArg string
}

// Relation kinds. A picker groups by these and orders the groups netlist → board → datasheet →
// predicate → overlay (KindOrder). KindPredicate is here rather than with any one engine because the
// grouping is a property of the catalog a reader browses, not of the engine that computes the
// predicate.
const (
	KindNetlist   = "netlist"
	KindBoard     = "board"
	KindDatasheet = "datasheet"
	KindPredicate = "predicate"
	KindOverlay   = "overlay"
)

// KindOrder is the display order of the kind groups, most-common first.
var KindOrder = []string{KindNetlist, KindBoard, KindDatasheet, KindPredicate, KindOverlay}
