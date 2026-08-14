// Package model is the design read-surface CONTRACT: the Model interface a rule or query evaluates
// against, plus the value types that interface exposes. It imports only the generated IR/geom/param
// protos, never the check implementation, its rules, or the param logic package, so a consumer (an
// auto-layout, a diff, an overlay) can depend on the contract without pulling the analysis engine
// into its build. The default implementation is check.NewModel, and check re-exports these names as
// aliases, so check.Model / check.ComponentClass and this package's names are the same types.
package model

import (
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// Model is the query interface a rule evaluates against: the entity sets it selects over and the
// derived facts it reads. A Run depends only on this interface, never on how the facts are computed,
// so an alternate implementation (lazy, cached, a columnar fact base, a datasheet-backed part-class
// source) can be substituted without touching any rule.
//
// The generic combinators (Select/Exists/Count in package check) are not methods because Go generics
// cannot be interface methods. They operate on the slices the interface returns.
type Model interface {
	// select: the entity sets a rule quantifies over.
	Nets() []*ir.Net
	Components() []*ir.Component
	// SourceFormat is the reader label of the design's source (e.g. "kicad-pcb", "edif-2.0.0"),
	// "" when unknown. The coarse catalog-availability gate reads it, so a board.* rule is only
	// listed available for a board-carrying format (check.Available).
	SourceFormat() string
	// HasParams reports whether a datasheet parameter tier is attached (a ParamProvider was
	// supplied to NewModelWithParams), independent of whether any specific part is seeded. The
	// catalog-availability gate reads it, so a datasheet rule is applicable exactly when params
	// were supplied.
	HasParams() bool
	// HasBoard reports whether a board-geometry tier is attached (a non-nil BoardGeometry was
	// supplied to NewModelWithBoard/NewModelWithParams), independent of the design's source
	// format. That is what makes a geometric rule applicable under `agni review --board-path`,
	// where the netlist SourceFormat is not a board format but a separate board export is
	// attached. A board file with no routed copper still reports true (checked, clean).
	HasBoard() bool
	// reader-emitted input diagnostics (docs/19): wire endpoints on nothing, and ref-des collisions.
	// Empty for sources that carry none; a thin rule reports each.
	DanglingEndpoints() []*ir.DanglingEndpoint
	NoJunctionEndpoints() []*ir.DanglingEndpoint
	RefDesCollisions() []*ir.RefDesCollision
	// UnmodeledBuses are bus constructs a reader detected but does not expand into member nets
	// (WS1-034). Empty for sources with no bus; the bus-not-modeled integrity rule reports each so
	// a bussed design is flagged, not silently mis-read.
	UnmodeledBuses() []*ir.BusNotModeled
	// UnresolvedSymbols are symbol references the reader could not open or parse (WS1-052), each
	// with the placements that lost their pins. Non-empty means the netlist is INCOMPLETE by an
	// unknown amount: those parts have no pins, so a rule reading connectivity cannot tell them
	// from a design where the connections were never drawn. Rules that read pin or connectivity
	// facts are gated to inconclusive while this is non-empty (check.Run).
	UnresolvedSymbols() []*ir.UnresolvedSymbol
	// traverse / pin-role: a pin's electrical direction, or the unspecified zero value when the
	// source carries no part-type pin data (so direction-based rules do not fire).
	PinDir(refDes, pin string) ir.PinDirection
	// PinDeclared reports whether the component's part type declares this pin at all, even with an
	// explicitly unspecified electrical type. False for pins known only from net connections (a
	// board footprint's pads, a partially-read hierarchy), where the unspecified PinDir means "the
	// read never saw the symbol" rather than "the author declared nothing". A rule keying on
	// declared-but-untyped pins (unspecified-pin-with-driver) must not read a read gap as an
	// authoring gap.
	PinDeclared(refDes, pin string) bool
	// traverse / param-join: the pin's NAME as its part type declares it ("VCCA", "GND"), or "" when
	// the source carries no part-type pin data. Distinct from the designator, which is the pin's
	// position in one package and therefore changes when the same die ships in a different body.
	// The name is the die-relative channel, so it is what a datasheet join leads with; see
	// param.ResolvePin for the precedence and why the designator only breaks ties.
	PinName(refDes, pin string) string
	// on_net: whether a ref_des appears on at least one net (section-aware).
	IsConnected(refDes string) bool
	// membership: whether a ref_des is known to the design at all, either a listed component or a
	// connection ref (a netlist read may carry connections without a component list). Unlike
	// IsConnected, it is true for a listed-but-unconnected part.
	HasComponent(refDes string) bool
	// pair: whether a net with the given name exists (case-insensitive).
	HasNetName(name string) bool
	// whether a net is a power/ground rail: asserted-driven or global, or a ground/power-rail
	// name. Such rails are distributed by power-symbol taps, not drawn wires (WS9-039). Unknown
	// net -> false.
	IsPowerRail(name string) bool
	// role: whether a net carries the ground / rail naming role, reading the STAMPED role fact
	// (ir.Net.roles, filled at ingestion) and falling back to this model's naming lexicon only for a
	// net that skipped the loader (WS3-106). They take the net rather than its name because names
	// are not unique (see NetNameCount), so a by-name lookup can answer about a different net.
	// IsRailNet is the role question alone, deliberately NARROWER than IsPowerRail, which also
	// answers true for a driven-or-global net and for grounds. A rule asking "is this a rail"
	// wants IsRailNet.
	IsGroundNet(n *ir.Net) bool
	IsRailNet(n *ir.Net) bool
	// naming lexicon: does a bare NAME match a role vocabulary, for the callers that hold no net to
	// read a stamped role from (the spec-language name FFIs over a literal, and pin-name role
	// derivation). Reads the lexicon this model's design was READ with (WS3-106), so a project's
	// conventions reach a name match without a package global.
	IsPowerRailName(name string) bool
	IsGroundName(name string) bool
	IsFeedbackName(name string) bool
	// pair: how many nets carry EXACTLY this name (case-sensitive, unlike HasNetName's
	// pairing lookup). More than one means the design states the same name for electrically
	// distinct nets, which is impossible on connect-by-name formats (the solver merges them)
	// and real on formats with explicit net lists (EDIF), so it is either an authoring slip
	// or a reader gap (duplicate-net-name reads this).
	NetNameCount(name string) int
	// pins: every part-type pin of every placed component (empty for sources that
	// carry no part-type pin data, so pin-level rules do not fire), and per-pin net
	// membership. PinConnected is the per-pin analogue of IsConnected: whether this
	// specific (ref_des, designator) appears in any net's connections.
	Pins() []PinInst
	PinConnected(refDes, pin string) bool
	// pin-role (WS1-009): the pin's semantic role, DERIVED from its declared name within
	// the component's device-class context (anode/cathode only for the diode family;
	// power/ground rail names on any class). It is a projection like component.class, never an
	// IR field, because no source format states polarity as data. RoleUnknown when the name
	// carries no recognized convention; rules skip, never guess.
	PinRole(refDes, pin string) PinRole
	// PinNetName is the name of the net this pin appears on ("" when unconnected). Pins-to-net
	// is many-to-one by definition (a net IS the equivalence class of joined pins), so a pin in
	// several nets' connection lists is malformed input; PinNetName then reports the first net
	// in design order. The arbitrary pick is safe because the conflict itself surfaces through
	// PinNetConflicts and the pin-net-conflict rule.
	PinNetName(refDes, pin string) string
	// PinNetConflicts are the malformed-input diagnostics PinNetName's contract leans on: every
	// pin that appears in more than one net's connections, with the full net list. Detected
	// from the normalized IR and collected once at model build, not a reader InputDiagnostic,
	// since no reader-only information is involved.
	PinNetConflicts() []PinNetConflict
	// no-connect channel: whether the source can express "intentionally unconnected" at
	// all, meaning any NO_CONNECT-typed pin or any no-connect-marker net name. Where the channel
	// is absent (a bare EDIF netlist), an unwired pin carries no NC flag because none
	// exists, not because the designer missed it, so per-pin absence rules must not fire.
	HasNoConnectChannel() bool
	// power-output typing channel (WS3-072 PR2): whether the source format classifies
	// power-OUTPUT pins at all. EDIF's port grammar carries only INPUT/OUTPUT/INOUT (no
	// power_out) and IPC-2581 is a board format with no pin electrical types, so on those a
	// power rail's driver reads as a plain input. A rule that infers "unpowered" from the
	// ABSENCE of a typed driver (power-input-not-driven) must not fire where this is false,
	// because the source is under-typed rather than undriven.
	FormatTypesPowerOut() bool
	// net-class channel (WS3-105): whether the design carries any tool-assigned net-class
	// membership at all. Only a KiCad project supplies it (net_settings in the .kicad_pro);
	// an EDIF or IPC-2581 read, a bare .kicad_sch, and a project that declares no classes all
	// leave every net_class empty. A rule SCOPED by net class evaluates over nothing there and
	// reports clean, which a review cannot tell from a pass, so such a rule declares
	// CapNetClass and reads not-applicable instead.
	HasNetClasses() bool
	// net-class DEFINITIONS (WS3-111): the per-class routing constraints the project declares
	// (clearance, track width, via sizes), as ir.Constraint nodes of kind "netclass". Separate
	// from HasNetClasses because membership and definitions are two independent blocks of
	// net_settings, so a project can assign nets to a class it never defines, or define classes
	// it assigns nothing to.
	NetClassDefs() []*ir.Constraint
	// reach (WS3-011): the bounded series-walk neighborhood of a net, meaning nets reachable by
	// crossing two-net pass elements (R/L/ferrite/fuse) with rails excluded, plus the
	// on-path class predicate over it. Protection rules are reachability questions: a
	// series element splits the net, so "a fuse sits between connector and regulator" is
	// invisible to any per-net quantifier.
	Reach(start *ir.Net, hops int) Reach
	Between(from, to *ir.Net, class ComponentClass, hops int) bool
	// board tier (WS3-008): each net's routed copper from the board-geometry sidecar.
	// Empty when the model was built without a board (NewModel); see NewModelWithBoard.
	BoardNets() []BoardNet
	// component.class: the MOST-SPECIFIC device class from the normalized device_classes set
	// (WS3-071), stamped once at ingestion by the classify pass and refined by part-type data.
	// ClassUnknown when the design carries no usable signal, so class-quantified rules skip
	// rather than misfire.
	ComponentClass(refDes string) ComponentClass
	// HasClass reports whether a component carries a class in its device_classes SET, so a rule
	// asks family membership (HasClass(ref, ClassDiode) is true for a diode, an LED, or a TVS)
	// rather than equality against the single most-specific class (WS3-071). An unknown ref-des
	// or class is false.
	HasClass(refDes string, class ComponentClass) bool
	// Classes returns the full device_classes set for a component (specific class plus family
	// tags), or nil for an unknown/unclassified ref-des. The set backing component.class(ref, class).
	Classes(refDes string) []ComponentClass
	// params tier (WS10-003): the design-side part identity (BomLine mpn, else the MPN
	// attribute, else "") and the seeded datasheet spec joined to it. Nil/"" when the model was
	// built without a seeded set (NewModel, NewModelWithBoard) or the part is unseeded, so
	// datasheet-backed rules skip rather than false-pass; see NewModelWithParams and params.go.
	ComponentMPN(refDes string) string
	PartSpec(refDes string) *parampb.PartSpec
}
