package facts

// RelationInfo describes one queryable relation or predicate for discovery surfaces such as the
// web query panel's relation picker. It carries the name, the argument labels a template inserts
// as `?arg`, a one-line summary, and a Kind for grouping. It is metadata only, and the schema
// (layout) and the projectors (rows) stay authoritative.
type RelationInfo struct {
	Name    string
	Args    []string
	Summary string
	Kind    string // "netlist" | "board" | "datasheet" | "predicate" | "extension"
	// Detail is the relation's reference markdown (WS14-005), or "" when the relation has no doc
	// yet. A discovery surface shows Summary in a list and Detail on demand. It is not part of the
	// arity-vs-schema assertion.
	Detail string
	// ArgKinds declares what an argument DENOTES, keyed by its label in Args. Present only for the
	// arguments that name something a reader can act on; absent means a scalar, which is most of them.
	//
	// Args is PROSE, so a surface turning an answer cell into a clickable entity reads this rather
	// than matching label strings (agni issue 548).
	ArgKinds map[string]ArgKind
}

// ArgKind is what one relation argument denotes. The zero value is a scalar, so a relation declares
// only the arguments that are more than that.
//
// Two of the three forms are RELATIONAL, so this is a struct and not a string. A polymorphic column
// takes its kind from another column's VALUE (`entity(name, kind)` yields a component, a net and a
// bus in one answer set), and a pin is only a design pin when the relation also says which
// component it belongs to (`pin.net(ref_des, pin, net)` does, while `param.pin(mpn, pin, ...)`
// names a pin of a part TYPE, which is not on a canvas).
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
	// ValidOptions is the CLOSED set of values this argument may take, when it has one. A query naming
	// a constant outside it is rejected rather than answered with no rows (agni 696).
	//
	// Not called "domain", which already means a rule's CONSIDERED SET (query.Domain).
	//
	// Most columns must leave this empty, since a net name, a part number, an attribute key and a
	// regex are all open and a closed set on one would reject legitimate questions. It is for the few
	// arguments whose values are a vocabulary the engine defines, such as a net role or a pin's
	// electrical type.
	//
	// Strings rather than a typed enum because this package imports nothing. A registration site
	// computes the list from whatever generated enum owns the vocabulary, so it cannot drift.
	ValidOptions []string
}

// Relation kinds. A picker groups by these in KindOrder. KindPredicate is here rather than with any
// one engine because the grouping belongs to the catalog a reader browses, not to the engine that
// computes the predicate.
const (
	KindNetlist   = "netlist"
	KindBoard     = "board"
	KindDatasheet = "datasheet"
	KindPredicate = "predicate"
	KindExtension = "extension"
)

// KindOrder is the display order of the kind groups, most-common first.
var KindOrder = []string{KindNetlist, KindBoard, KindDatasheet, KindPredicate, KindExtension}
