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
