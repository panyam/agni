// Package relations is the standard EDB relation catalog: the built-in "data providers" that
// project a check.Model (and a seeded datasheet library) into the query engine's fact base —
// netlist, board, and datasheet relations. It moved out of package check (issue 10) so the built-in
// relations register through the same public seam an overlay uses (facts.RegisterBuiltinFacts), the
// symmetric twin of how the built-in RULES moved to stdlib/rules/builtin in issue 4 phase 2b. The
// core engine (packages check and query) owns no relations; blank-importing this package installs
// the catalog, and a binary that omits the import runs the query engine with only overlay relations.
package relations

import (
	"github.com/panyam/agni/core/facts"
)

// init installs the built-in relations with the query engine by import side effect, the way an
// overlay calls facts.RegisterRelation. It runs before an importing package's own var initializers,
// so any binary that blank-imports this package has the relations available before it builds a Base.
func init() {
	facts.RegisterBuiltinFacts(facts.BuiltinFacts{
		Schema:  builtinSchema,
		Catalog: builtinCatalog,
		Model:   Facts,
		SpecLib: SpecLibFacts,
		Doc:     RelationDoc,
	})
}

// builtinSchema is each built-in relation's positional argument layout over FactRow, so a flat tuple
// is queried as reln(arg0, arg1, ...). It is the data query/schema.go used to hold as a literal;
// registering it here keeps the relation shapes with the projectors that fill them. Relations the
// evaluator computes rather than looks up (reaches) are NOT here.
var builtinSchema = map[string][]facts.Field{
	RelNetMaxVoltage:     {facts.FieldSubject, facts.FieldNum},                                                                   // net.max_voltage(net, volts)
	RelNetNominalVoltage: {facts.FieldSubject, facts.FieldNum},                                                                   // net.nominal_voltage(net, volts)
	RelNetSignalLevel:    {facts.FieldSubject, facts.FieldNum},                                                                   // net.signal_level(net, volts)
	RelComponentMPN:      {facts.FieldSubject, facts.FieldValue},                                                                 // component.mpn(ref, mpn)
	RelParam:             {facts.FieldSubject, facts.FieldObject, facts.FieldNum},                                                // param(mpn, symbol, max)
	RelParamRange:        {facts.FieldSubject, facts.FieldObject, facts.FieldValue, facts.FieldMin, facts.FieldNum},              // param.range(mpn, symbol, kind, min, max)
	RelParamTyp:          {facts.FieldSubject, facts.FieldObject, facts.FieldNum},                                                // param.typ(mpn, symbol, typ)
	RelParamProv:         {facts.FieldSubject, facts.FieldObject, facts.FieldValue, facts.FieldQualifier, facts.FieldConditions}, // param.prov(mpn, symbol, doc, page, section). page is a STRING slot, see RelParamProv
	RelParamUnit:         {facts.FieldSubject, facts.FieldObject, facts.FieldValue},                                              // param.unit(mpn, symbol, unit)
	RelParamPin:          {facts.FieldSubject, facts.FieldObject, facts.FieldValue, facts.FieldQualifier},                        // param.pin(mpn, pin, name, function)
	// param.pin_range is the widest relation the fact base carries, and the one FieldQualifier was
	// added for: mpn/pin/symbol fill Subject/Object/Value, leaving the limit kind nowhere to go.
	// Conditions is NOT reused for it — every param relation carries the test conditions there as
	// unbound metadata, and spending that slot would strip the trust context from exactly the rows a
	// pin-rating rule compares against.
	RelParamPinRange: {facts.FieldSubject, facts.FieldObject, facts.FieldValue, facts.FieldQualifier, facts.FieldMin, facts.FieldNum}, // param.pin_range(mpn, pin, symbol, kind, min, max)
	// param.pin_relation borrows pin_range's shape for a different fact: Object and Value are the
	// two PIN ids rather than a pin and a symbol, and Min/Num bound the difference between them
	// rather than one terminal's own quantity.
	RelParamPinRelation: {facts.FieldSubject, facts.FieldObject, facts.FieldValue, facts.FieldQualifier, facts.FieldMin, facts.FieldNum}, // param.pin_relation(mpn, subject_pin, reference_pin, modality, min, max)
	RelPartAudience:     {facts.FieldSubject, facts.FieldObject},                                                                         // part.audience(mpn, who)
	RelComponentOnNet:   {facts.FieldSubject, facts.FieldObject},                                                                         // component-on-net(ref, net)
	// Pin tier (WS3-038) — pin-granular relations, queryable with no evaluator change.
	RelPin:           {facts.FieldSubject, facts.FieldObject},                   // pin(ref, pin)
	RelPinRole:       {facts.FieldSubject, facts.FieldObject, facts.FieldValue}, // pin.role(ref, pin, role)
	RelPinType:       {facts.FieldSubject, facts.FieldObject, facts.FieldValue}, // pin.type(ref, pin, etype)
	RelPinNet:        {facts.FieldSubject, facts.FieldObject, facts.FieldValue}, // pin.net(ref, pin, net)
	RelNetPinCount:   {facts.FieldSubject, facts.FieldNum},                      // net.pin_count(net, count)
	RelHasNCChannel:  {facts.FieldSubject},                                      // has_nc_channel(present)
	RelTypesPowerOut: {facts.FieldSubject},                                      // types_power_out(present)
	RelRail:          {facts.FieldSubject},                                      // rail(net)
	RelFeedback:      {facts.FieldSubject},                                      // feedback(net)
	RelComponentAttr: {facts.FieldSubject, facts.FieldObject, facts.FieldValue}, // component.attr(ref, key, value)
	// Device-class and net-attribute relations (WS3-074). component.class emits one row per class
	// tag in the device_classes SET (WS3-071), so a family tag answers too.
	RelComponentClass:        {facts.FieldSubject, facts.FieldValue},                    // component.class(ref, class)
	RelNetGround:             {facts.FieldSubject},                                      // net.ground(net)
	RelNetExternal:           {facts.FieldSubject},                                      // net.external(net)
	RelEsdRated:              {facts.FieldSubject},                                      // component.esd_rated(ref) — WS3-076, datasheet tier
	RelComponentDeviceClass:  {facts.FieldSubject, facts.FieldValue},                    // component.device_class(ref, class) — WS10-013, datasheet tier
	RelBus:                   {facts.FieldSubject, facts.FieldValue},                    // bus(label, kind) — reader-detected unmodeled bus (WS1-034)
	RelEntity:                {facts.FieldSubject, facts.FieldValue},                    // entity(name, kind) — the enumeration relation, what a name search ranges over
	RelUnresolvedSymbol:      {facts.FieldSubject, facts.FieldValue},                    // unresolved_symbol(ref_des, symref) — a placement that lost its pins (WS1-052)
	RelRefDesCollision:       {facts.FieldSubject},                                      // ref_des_collision(ref_des) — WS3-081
	RelPinNetConflict:        {facts.FieldSubject, facts.FieldObject, facts.FieldValue}, // pin_net_conflict(ref_des, pin, net) — WS3-081
	RelNetBusLike:            {facts.FieldSubject},                                      // net.bus_like(net) — WS3-080
	RelExternalSignalNet:     {facts.FieldSubject},                                      // external_signal_net(net) — WS3-061
	RelNetBias:               {facts.FieldSubject, facts.FieldValue},                    // net.bias(net, level) — WS3-088
	RelNetACCoupled:          {facts.FieldSubject},                                      // net.ac_coupled(net) — WS3-088
	RelNetNetClass:           {facts.FieldSubject, facts.FieldValue},                    // net.netclass(net, class) — WS3-105
	RelHasNetClass:           {facts.FieldSubject},                                      // has_netclass(present) — WS3-105
	RelNetClassClearance:     {facts.FieldSubject, facts.FieldNum},                      // netclass.clearance(class, mm) — WS3-111
	RelNetClassTrackWidth:    {facts.FieldSubject, facts.FieldNum},                      // netclass.track_width(class, mm) — WS3-111
	RelNetClassViaDiameter:   {facts.FieldSubject, facts.FieldNum},                      // netclass.via_diameter(class, mm) — WS3-111
	RelNetClassViaDrill:      {facts.FieldSubject, facts.FieldNum},                      // netclass.via_drill(class, mm) — WS3-111
	RelHasNetClassDefs:       {facts.FieldSubject},                                      // has_netclass_defs(present) — WS3-111
	RelNetDeclaredTrackWidth: {facts.FieldSubject, facts.FieldNum},                      // net.declared_track_width(net, mm) — WS3-111
	RelNetDeclaredViaDrill:   {facts.FieldSubject, facts.FieldNum},                      // net.declared_via_drill(net, mm) — WS3-111
	// Board tier — queryable with no evaluator change (tier-generality).
	RelBoardTrackWidth: {facts.FieldSubject, facts.FieldNum},    // board.track_width(net, mm)
	RelBoardViaDrill:   {facts.FieldSubject, facts.FieldNum},    // board.via_drill(net, mm)
	RelBoardLayer:      {facts.FieldSubject, facts.FieldObject}, // board.layer(net, layer)
}

// builtinCatalog is the human-facing metadata for the built-in relations — the picker's name, arg
// labels, one-line summary, and kind. Arg counts are asserted against builtinSchema in the tests
// (TestCatalogMatchesSchema), so a relation added to the schema without a catalog entry — or with a
// mismatched arity — fails CI rather than shipping an undiscoverable relation. The engine's computed
// predicates (reaches, the string filters) stay in query (builtinPredicates); this is relations only.
var builtinCatalog = []facts.RelationInfo{
	{Name: "component.mpn", Args: []string{"ref_des", "mpn"}, Summary: "the design-side part identity (manufacturer part number)", Kind: facts.KindNetlist},
	{Name: "component-on-net", Args: []string{"ref_des", "net"}, Summary: "a component sits on a net", Kind: facts.KindNetlist},
	{Name: "entity", Args: []string{"name", "kind"}, Summary: "a thing exists in the design under this name, with kind one of component/net/bus. The relation to start a name search from, since every other one ranges over an association and so misses whatever it does not reach (a part with no connections, a net with nothing on it)", Kind: facts.KindNetlist},
	{Name: "net.max_voltage", Args: []string{"net", "volts"}, Summary: "a net's declared rail voltage", Kind: facts.KindNetlist},
	{Name: "net.nominal_voltage", Args: []string{"net", "volts"}, Summary: "a RAIL's nominal voltage derived from its net name (3V3 -> 3.3). Rails only; a non-rail net's name-derived level is net.signal_level", Kind: facts.KindNetlist},
	{Name: "net.signal_level", Args: []string{"net", "volts"}, Summary: "the signalling level a NON-RAIL net's name declares, the other half of net.nominal_voltage. A house convention that encodes a level into a signal net's name lands here rather than being read as a rail nominal", Kind: facts.KindNetlist},
	{Name: "board.layer", Args: []string{"net", "layer"}, Summary: "a net appears on a board copper layer", Kind: facts.KindBoard},
	{Name: "board.track_width", Args: []string{"net", "mm"}, Summary: "a copper track's width on a net (millimetres)", Kind: facts.KindBoard},
	{Name: "board.via_drill", Args: []string{"net", "mm"}, Summary: "a via's drill diameter on a net (millimetres)", Kind: facts.KindBoard},
	{Name: "pin", Args: []string{"ref_des", "pin"}, Summary: "a part-type pin of a placed component", Kind: facts.KindNetlist},
	{Name: "pin.role", Args: []string{"ref_des", "pin", "role"}, Summary: "a pin's derived role (power/ground/anode/cathode)", Kind: facts.KindNetlist},
	{Name: "pin.type", Args: []string{"ref_des", "pin", "etype"}, Summary: "a pin's electrical type (power_in, input, output, ...)", Kind: facts.KindNetlist},
	{Name: "pin.net", Args: []string{"ref_des", "pin", "net"}, Summary: "the net a pin is on (absent if unconnected)", Kind: facts.KindNetlist},
	{Name: "net.pin_count", Args: []string{"net", "count"}, Summary: "the number of connections on a net", Kind: facts.KindNetlist},
	{Name: "has_nc_channel", Args: []string{"present"}, Summary: "one row when the design can express intentional no-connect", Kind: facts.KindNetlist},
	{Name: "types_power_out", Args: []string{"present"}, Summary: "one row when the source format classifies power-output pins (EDIF/IPC do not, so a driver-absence check is unsound there)", Kind: facts.KindNetlist},
	{Name: "rail", Args: []string{"net"}, Summary: "the net is a power or ground rail", Kind: facts.KindNetlist},
	{Name: "feedback", Args: []string{"net"}, Summary: "the net is a regulator feedback / sense node (must not be probed)", Kind: facts.KindNetlist},
	{Name: "component.attr", Args: []string{"ref_des", "key", "value"}, Summary: "a component-level attribute (e.g. interface, MPN)", Kind: facts.KindNetlist},
	{Name: "component.class", Args: []string{"ref_des", "class"}, Summary: "a device class the part is in (a family tag too, e.g. a TVS is both tvs and diode)", Kind: facts.KindNetlist},
	{Name: "component.esd_rated", Args: []string{"ref_des"}, Summary: "the part carries a datasheet ESD rating at or above the credit floor (needs --params)", Kind: facts.KindDatasheet},
	{Name: "component.device_class", Args: []string{"ref_des", "class"}, Summary: "the device class the part's datasheet declares (authoritative over the ref-des/keyword class; needs --params)", Kind: facts.KindDatasheet},
	{Name: "net.ground", Args: []string{"net"}, Summary: "the net is a ground rail (name-derived)", Kind: facts.KindNetlist},
	{Name: "net.external", Args: []string{"net"}, Summary: "the net may extend onto an unread sheet (read-gap marker)", Kind: facts.KindNetlist},
	{Name: "bus", Args: []string{"label", "kind"}, Summary: "a reader-detected bus not yet expanded into member nets (WS1-034)", Kind: facts.KindNetlist},
	{Name: "unresolved_symbol", Args: []string{"ref_des", "symref"}, Summary: "a placement whose symbol did not resolve, so it carries no pins (WS1-052)", Kind: facts.KindNetlist},
	{Name: "ref_des_collision", Args: []string{"ref_des"}, Summary: "a reference designator used by more than one part (reader integrity diagnostic)", Kind: facts.KindNetlist},
	{Name: "pin_net_conflict", Args: []string{"ref_des", "pin", "net"}, Summary: "a pin the read placed on more than one net; one row per net (reader integrity diagnostic)", Kind: facts.KindNetlist},
	{Name: "net.bus_like", Args: []string{"net"}, Summary: "a shared-distribution net (ground plane, global rail, or rail-scale fan-out), the series-reach walk's stop predicate", Kind: facts.KindNetlist},
	{Name: "external_signal_net", Args: []string{"net"}, Summary: "a connector-facing signal net (not a rail, ground, no-connect, or power path), the scope the ESD rules share", Kind: facts.KindNetlist},
	{Name: "net.bias", Args: []string{"net", "level"}, Summary: "a bias resistor holds the net at a rail (high) or ground (low); absent when unbiased or held by a divider", Kind: facts.KindNetlist},
	{Name: "net.ac_coupled", Args: []string{"net"}, Summary: "a SERIES capacitor carries the net (a decoupling cap to ground/rail does not count)", Kind: facts.KindNetlist},
	{Name: "net.netclass", Args: []string{"net", "class"}, Summary: "the tool-assigned net class a net belongs to (KiCad net_settings; not the derived semantic role)", Kind: facts.KindNetlist},
	{Name: "has_netclass", Args: []string{"present"}, Summary: "one row when the design assigns net classes at all (absent it, a netclass-scoped rule selects nothing and reads clean)", Kind: facts.KindNetlist},
	{Name: "netclass.clearance", Args: []string{"class", "mm"}, Summary: "the clearance a net class declares its nets should route at (millimetres)", Kind: facts.KindNetlist},
	{Name: "netclass.track_width", Args: []string{"class", "mm"}, Summary: "the track width a net class declares its nets should route at (millimetres)", Kind: facts.KindNetlist},
	{Name: "netclass.via_diameter", Args: []string{"class", "mm"}, Summary: "the via diameter a net class declares (millimetres)", Kind: facts.KindNetlist},
	{Name: "netclass.via_drill", Args: []string{"class", "mm"}, Summary: "the via drill a net class declares (millimetres)", Kind: facts.KindNetlist},
	{Name: "has_netclass_defs", Args: []string{"present"}, Summary: "one row when the design declares net-class definitions at all (absent it, a declared-vs-actual rule has no limit to compare against and reads clean)", Kind: facts.KindNetlist},
	{Name: "net.declared_track_width", Args: []string{"net", "mm"}, Summary: "the track width a net SHOULD route at, cascaded across its classes by priority (join this, not the per-class rows)", Kind: facts.KindNetlist},
	{Name: "net.declared_via_drill", Args: []string{"net", "mm"}, Summary: "the via drill a net SHOULD route at, cascaded across its classes by priority (join this, not the per-class rows)", Kind: facts.KindNetlist},
	{Name: "param", Args: []string{"mpn", "symbol", "max"}, Summary: "a datasheet parameter's max value for a part, in its SI base unit (needs --params)", Kind: facts.KindDatasheet},
	{Name: "param.range", Args: []string{"mpn", "symbol", "kind", "min", "max"}, Summary: "a datasheet parameter's two-sided limit with its kind, both bounds in the SI base unit (absolute_max / recommended_operating / characteristic; needs --params)", Kind: facts.KindDatasheet},
	{Name: "param.typ", Args: []string{"mpn", "symbol", "typ"}, Summary: "a datasheet parameter's TYPICAL value in the SI base unit, the third member of the min/typ/max triple. A typical value is what the part usually does, never a guaranteed limit, so it is its own relation rather than a column on param.range (needs --params)", Kind: facts.KindDatasheet},
	{Name: "param.prov", Args: []string{"mpn", "symbol", "doc", "page", "section"}, Summary: "the citation of a datasheet parameter: the SourceDoc title, page, and table/figure it was read from. The page is a locator and binds as a string, not a number (needs --params)", Kind: facts.KindDatasheet},
	{Name: "param.unit", Args: []string{"mpn", "symbol", "unit"}, Summary: "the unit a datasheet parameter is PRINTED in; param and param.range carry their numbers in SI base units, so join this to see the vendor's own spelling (needs --params)", Kind: facts.KindDatasheet},
	{Name: "param.pin", Args: []string{"mpn", "pin", "name", "function"}, Summary: "a pin the part's datasheet declares, keyed by its spec-local id, with the printed name and its function (power_input / ground / bidirectional / no_connect / ...; needs --params)", Kind: facts.KindDatasheet},
	{Name: "param.pin_range", Args: []string{"mpn", "pin", "symbol", "kind", "min", "max"}, Summary: "a datasheet limit bound to ONE pin, both bounds in the SI base unit, the per-terminal counterpart to param.range, so a part with several supply pins answers per pin instead of once (needs --params)", Kind: facts.KindDatasheet},
	{Name: "param.pin_relation", Args: []string{"mpn", "subject_pin", "reference_pin", "modality", "min", "max"}, Summary: "a datasheet constraint BETWEEN two pins of one part: bounds on (subject - reference) in the SI base unit, with the vendor's modality (required/recommended). The pin order is load-bearing, so swapping the two inverts the requirement (needs --params)", Kind: facts.KindDatasheet},
	{Name: "part.audience", Args: []string{"mpn", "who"}, Summary: "a team/license entitled to see a part's datasheet data (record-only, needs --params)", Kind: facts.KindDatasheet},
}
