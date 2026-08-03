// Package model is the design read-surface CONTRACT: the Model interface a rule or query evaluates
// against, plus the value types that interface exposes. It imports only the generated IR/geom/param
// protos — never the check implementation, its rules, or the param logic package — so a consumer
// (an auto-layout, a diff, an overlay) can depend on the contract without pulling the analysis
// engine into its build. The default implementation lives in package check (check.NewModel), which
// imports this package and satisfies Model; check re-exports these names as aliases for
// backward-compatibility, so check.Model / check.ComponentClass and this package's names are the
// same types.
package model

import (
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// Model is the query interface a rule evaluates against: the entity sets it selects over and
// the derived facts it reads. A Run depends only on this interface, never on how the facts are
// computed, so an alternate implementation (lazy, cached, a columnar fact base, a
// datasheet-backed part-class source) can be substituted without touching any rule.
//
// The generic combinators (Select/Exists/Count in package check) are not methods: Go generics
// cannot be interface methods. They operate on the slices the interface returns, which keeps the
// split clean: the Model supplies facts, the combinators are the primitives over them.
type Model interface {
	// select: the entity sets a rule quantifies over.
	Nets() []*ir.Net
	Components() []*ir.Component
	// SourceFormat is the reader label of the design's source (e.g. "kicad-pcb", "edif-2.0.0"),
	// "" when unknown. The coarse catalog-availability gate reads it — a board.* rule is only
	// listed available for a board-carrying format (check.Available).
	SourceFormat() string
	// HasParams reports whether a datasheet parameter tier is attached (a ParamProvider was
	// supplied to NewModelWithParams), independent of whether any specific part is seeded. The
	// catalog-availability gate reads it so a datasheet rule is applicable exactly when params
	// were supplied — the per-run analogue of SourceFormat's per-format board gate.
	HasParams() bool
	// HasBoard reports whether a board-geometry tier is attached (a non-nil BoardGeometry was
	// supplied to NewModelWithBoard/NewModelWithParams), independent of the design's source
	// format. The catalog-availability gate reads it so a geometric rule is applicable when a
	// board was supplied even for a netlist entry that carries no sidecar of its own — the case
	// of `agni review --board-path`, where the netlist SourceFormat is not a board format but a
	// separate board export is attached. A board file with no routed copper still reports true
	// (checked, clean), the same posture as HasParams for an unseeded corpus.
	HasBoard() bool
	// reader-emitted input diagnostics (docs/19): wire endpoints on nothing, and ref-des collisions.
	// Empty for sources that carry none; a thin rule reports each.
	DanglingEndpoints() []*ir.DanglingEndpoint
	NoJunctionEndpoints() []*ir.DanglingEndpoint
	RefDesCollisions() []*ir.RefDesCollision
	// UnmodeledBuses are bus constructs a reader detected but does not yet expand into member nets
	// (WS1-034). Empty for sources with no bus, or once Phase 2 models them; the bus-not-modeled
	// integrity rule reports each so a bussed design is flagged, not silently mis-read.
	UnmodeledBuses() []*ir.BusNotModeled
	// traverse / pin-role: a pin's electrical direction, or the unspecified zero value when the
	// source carries no part-type pin data (so direction-based rules do not fire).
	PinDir(refDes, pin string) ir.PinDirection
	// Whether the component's part type declares this pin at all — even with an explicitly
	// unspecified electrical type. False for pins known only from net connections (a board
	// footprint's pads, a partially-read hierarchy): there the unspecified PinDir means "the
	// read never saw the symbol", not "the author declared nothing", and a rule keying on
	// declared-but-untyped pins (unspecified-pin-with-driver) must not read a read gap as an
	// authoring gap.
	PinDeclared(refDes, pin string) bool
	// on_net: whether a ref_des appears on at least one net (section-aware).
	IsConnected(refDes string) bool
	// membership: whether a ref_des is known to the design at all — a listed component OR a
	// connection ref (a netlist read may carry connections without a component list). Unlike
	// IsConnected, it is true for a listed-but-unconnected part.
	HasComponent(refDes string) bool
	// pair: whether a net with the given name exists (case-insensitive).
	HasNetName(name string) bool
	// whether a net is a power/ground rail — asserted-driven or global, or a ground/power-rail
	// name. Such rails are distributed by power-symbol taps, not drawn wires (WS9-039). Unknown
	// net -> false.
	IsPowerRail(name string) bool
	// pair: how many nets carry EXACTLY this name (case-sensitive, unlike HasNetName's
	// pairing lookup). More than one means the design states the same name for
	// electrically distinct nets — impossible on connect-by-name formats (the solver
	// merges them), real on formats with explicit net lists (EDIF), and either an
	// authoring slip or a reader gap (duplicate-net-name reads this).
	NetNameCount(name string) int
	// pins: every part-type pin of every placed component (empty for sources that
	// carry no part-type pin data, so pin-level rules do not fire), and per-pin net
	// membership. PinConnected is the per-pin analogue of IsConnected: whether this
	// specific (ref_des, designator) appears in any net's connections.
	Pins() []PinInst
	PinConnected(refDes, pin string) bool
	// pin-role (WS1-009): the pin's semantic role, DERIVED from its declared name within
	// the component's device-class context (anode/cathode only for the diode family;
	// power/ground rail names on any class) — a projection like component.class, never an
	// IR field, because no source format states polarity as data (the WS1-009 audit).
	// RoleUnknown when the name carries no recognized convention; rules skip, never guess.
	PinRole(refDes, pin string) PinRole
	// The name of the net this pin appears on ("" when unconnected). Pins-to-net is
	// many-to-one by definition (a net IS the equivalence class of joined pins), so a
	// pin in several nets' connection lists is malformed input; PinNetName then reports
	// the first net in design order, and the conflict itself surfaces through
	// PinNetConflicts + the pin-net-conflict rule — the arbitrary pick is safe because
	// the condition that makes it arbitrary is separately reported.
	PinNetName(refDes, pin string) string
	// The malformed-input diagnostics PinNetName's contract leans on: every pin that
	// appears in more than one net's connections, with the full net list. Detected from
	// the normalized IR (not a reader InputDiagnostic: no reader-only information is
	// involved), collected once at model build.
	PinNetConflicts() []PinNetConflict
	// no-connect channel: whether the source can express "intentionally unconnected" at
	// all — any NO_CONNECT-typed pin or any no-connect-marker net name. Where the channel
	// is absent (a bare EDIF netlist), an unwired pin carries no NC flag because none
	// exists, not because the designer missed it, so per-pin absence rules must not fire.
	HasNoConnectChannel() bool
	// power-output typing channel (WS3-072 PR2): whether the source format classifies
	// power-OUTPUT pins at all. EDIF's port grammar carries only INPUT/OUTPUT/INOUT (no
	// power_out) and IPC-2581 is a board format with no pin electrical types, so on those a
	// power rail's driver reads as a plain input. A rule that infers "unpowered" from the
	// ABSENCE of a typed driver (power-input-not-driven) must not fire where this is false —
	// the source is under-typed, not undriven. The symmetric power_in side is stamped
	// (StampPowerInPins) so decoupling / input-protection work; the power_out stamp is PR3.
	FormatTypesPowerOut() bool
	// reach (WS3-011): the bounded series-walk neighborhood of a net — nets reachable by
	// crossing two-net pass elements (R/L/ferrite/fuse), rails excluded — and the
	// on-path class predicate over it. Protection rules are reachability questions: a
	// series element splits the net, so "a fuse sits between connector and regulator" is
	// invisible to any per-net quantifier.
	Reach(start *ir.Net, hops int) Reach
	Between(from, to *ir.Net, class ComponentClass, hops int) bool
	// board tier (WS3-008): each net's routed copper from the board-geometry sidecar.
	// Empty when the model was built without a board (NewModel), so geometric rules are
	// silent on netlist-only designs; see NewModelWithBoard.
	BoardNets() []BoardNet
	// component.class: the MOST-SPECIFIC device class from the normalized device_classes set
	// (WS3-071), stamped once at ingestion by the classify pass and refined by part-type data;
	// ClassUnknown when the design carries no usable signal, so class-quantified rules skip
	// rather than misfire.
	ComponentClass(refDes string) ComponentClass
	// HasClass reports whether a component carries a class in its device_classes SET, so a rule
	// asks family membership (HasClass(ref, ClassDiode) is true for a diode, an LED, or a TVS)
	// rather than equality against the single most-specific class. This is the WS3-071 replacement
	// for the isDiodeFamily Go helper. An unknown ref-des or class is false (absent-tolerant).
	HasClass(refDes string, class ComponentClass) bool
	// Classes returns the full device_classes set for a component (specific class plus family
	// tags), or nil for an unknown/unclassified ref-des. The set backing component.class(ref, class).
	Classes(refDes string) []ComponentClass
	// params tier (WS10-003): the design-side part identity (BomLine mpn, else the MPN
	// attribute, else "") and the seeded datasheet spec joined to it. Nil/"" when the
	// model was built without a seeded set (NewModel, NewModelWithBoard) or the part is
	// unseeded, so datasheet-backed rules skip rather than false-pass; see
	// NewModelWithParams and params.go.
	ComponentMPN(refDes string) string
	PartSpec(refDes string) *parampb.PartSpec
}
