---
title: "Relations catalog"
description: "Every query relation the fact base exposes, grouped by kind."
---

The relations a datalog query joins over. Each documented relation links to its full reference: the hardware it describes, its Go projector, and example queries. See the [querying guide](../../guide/querying/) for how to compose them. This page is generated from the shipped fact base.

## netlist

| Relation | Summary |
|---|---|
| [`bus(label, kind)`](bus/) | a reader-detected bus not yet expanded into member nets (WS1-034) |
| [`component.attr(ref_des, key, value)`](component.attr/) | a component-level attribute (e.g. interface, MPN) |
| [`component.class(ref_des, class)`](component.class/) | a device class the part is in (a family tag too, e.g. a TVS is both tvs and diode) |
| [`component.mpn(ref_des, mpn)`](component.mpn/) | the design-side part identity (manufacturer part number) |
| [`component.net(ref_des, net)`](component.net/) | a component sits on a net |
| [`component.net_count(ref_des, count)`](component.net_count/) | the number of distinct nets a component touches (0 for a part wired to nothing) |
| [`component.pin(ref_des, pin)`](component.pin/) | a part-type pin of a placed component |
| [`design.has_nc_channel(present)`](design.has_nc_channel/) | one row when the design can express intentional no-connect |
| [`design.has_netclass(present)`](design.has_netclass/) | one row when the design assigns net classes at all (absent it, a netclass-scoped rule selects nothing and reads clean) |
| [`design.has_netclass_defs(present)`](design.has_netclass_defs/) | one row when the design declares net-class definitions at all (absent it, a declared-vs-actual rule has no limit to compare against and reads clean) |
| [`design.types_power_out(present)`](design.types_power_out/) | one row when the source format classifies power-output pins (EDIF/IPC do not, so a driver-absence check is unsound there) |
| [`entity(name, kind)`](entity/) | a thing exists in the design under this name, with kind one of component/net/bus. The relation to start a name search from, since every other one ranges over an association and so misses whatever it does not reach (a part with no connections, a net with nothing on it) |
| [`net.ac_coupled(net)`](net.ac_coupled/) | a SERIES capacitor carries the net (a decoupling cap to ground/rail does not count) |
| [`net.attr(net, key, value)`](net.attr/) | a net-level attribute DECLARED by the source file (external, global, power_driven), the twin of component.attr; a role the engine derived is net.role |
| [`net.bias(net, level)`](net.bias/) | a bias resistor holds the net at a rail (high) or ground (low); absent when unbiased or held by a divider |
| [`net.bus_like(net)`](net.bus_like/) | a shared-distribution net (ground plane, global rail, or rail-scale fan-out), the series-reach walk's stop predicate |
| [`net.connector_signal(net)`](net.connector_signal/) | a connector-facing signal net (not a rail, ground, no-connect, or power path), the scope the ESD rules share |
| [`net.declared_track_width(net, mm)`](net.declared_track_width/) | the track width a net SHOULD route at, cascaded across its classes by priority (join this, not the per-class rows) |
| [`net.declared_via_drill(net, mm)`](net.declared_via_drill/) | the via drill a net SHOULD route at, cascaded across its classes by priority (join this, not the per-class rows) |
| [`net.external(net)`](net.external/) | the net may extend onto an unread sheet (read-gap marker) |
| [`net.feedback(net)`](net.feedback/) | the net is a regulator feedback / sense node (must not be probed) |
| [`net.ground(net)`](net.ground/) | the net is a ground rail (name-derived) |
| [`net.max_voltage(net, volts)`](net.max_voltage/) | a net's declared rail voltage |
| [`net.netclass(net, class)`](net.netclass/) | the tool-assigned net class a net belongs to (KiCad net_settings; not the derived semantic role) |
| [`net.nominal_voltage(net, volts)`](net.nominal_voltage/) | a RAIL's nominal voltage derived from its net name (3V3 -> 3.3). Rails only; a non-rail net's name-derived level is net.signal_level, and a regulator internal (_FB, _SW, _BOOT) is on neither because the number in its name is another net's voltage |
| [`net.pin_count(net, count)`](net.pin_count/) | the number of connections on a net |
| [`net.rail(net)`](net.rail/) | the net is a power or ground rail |
| [`net.role(net, role)`](net.role/) | a role the net carries, derived from its name by the lexicon (rail, ground, feedback, switching, control, gate_drive); one row per role, the net-side twin of component.class |
| [`net.signal_level(net, volts)`](net.signal_level/) | the signalling level a NON-RAIL net's name declares, the other half of net.nominal_voltage. A house convention that encodes a level into a signal net's name lands here rather than being read as a rail nominal; a regulator internal is on neither relation |
| [`net.switching(net)`](net.switching/) | the net is a regulator power-stage node, the switch node or its bootstrap (must not be probed); the twin of feedback |
| [`netclass.clearance(class, mm)`](netclass.clearance/) | the clearance a net class declares its nets should route at (millimetres) |
| [`netclass.track_width(class, mm)`](netclass.track_width/) | the track width a net class declares its nets should route at (millimetres) |
| [`netclass.via_diameter(class, mm)`](netclass.via_diameter/) | the via diameter a net class declares (millimetres) |
| [`netclass.via_drill(class, mm)`](netclass.via_drill/) | the via drill a net class declares (millimetres) |
| [`pin.name(ref_des, pin, name)`](pin.name/) | the part type's functional name for a pin ("SDA", "PTC11"), the spelling a datasheet and a firmware header use, against the package designator every other pin relation is keyed on; absent when the part type declares none |
| [`pin.net(ref_des, pin, net)`](pin.net/) | the net a pin is on (absent if unconnected) |
| [`pin.role(ref_des, pin, role)`](pin.role/) | a pin's derived role (power/ground/anode/cathode) |
| [`pin.type(ref_des, pin, etype)`](pin.type/) | a pin's electrical type (power_in, input, output, ...) |
| [`reader.pin_net_conflict(ref_des, pin, net)`](reader.pin_net_conflict/) | a pin the read placed on more than one net; one row per net (reader integrity diagnostic) |
| [`reader.ref_des_collision(ref_des)`](reader.ref_des_collision/) | a reference designator used by more than one part (reader integrity diagnostic) |
| [`reader.unresolved_symbol(ref_des, symref)`](reader.unresolved_symbol/) | a placement whose symbol did not resolve, so it carries no pins (WS1-052) |

## board

| Relation | Summary |
|---|---|
| [`board.layer(net, layer)`](board.layer/) | a net appears on a board copper layer |
| [`board.track_width(net, mm)`](board.track_width/) | a copper track's width on a net (millimetres) |
| [`board.via_drill(net, mm)`](board.via_drill/) | a via's drill diameter on a net (millimetres) |

## datasheet

| Relation | Summary |
|---|---|
| [`component.device_class(ref_des, class)`](component.device_class/) | the device class the part's datasheet declares (authoritative over the ref-des/keyword class; needs --params) |
| [`component.esd_rated(ref_des)`](component.esd_rated/) | the part carries a datasheet ESD rating at or above the credit floor (needs --params) |
| [`param.max(mpn, symbol, max)`](param.max/) | a datasheet parameter's max value for a part, in its SI base unit (needs --params) |
| [`param.pin(mpn, pin, name, function)`](param.pin/) | a pin the part's datasheet declares, keyed by its spec-local id, with the printed name and its function (power_input / ground / bidirectional / no_connect / ...; needs --params) |
| [`param.pin_range(mpn, pin, symbol, kind, min, max)`](param.pin_range/) | a datasheet limit bound to ONE pin, both bounds in the SI base unit, the per-terminal counterpart to param.range, so a part with several supply pins answers per pin instead of once (needs --params) |
| [`param.pin_relation(mpn, subject_pin, reference_pin, modality, min, max)`](param.pin_relation/) | a datasheet constraint BETWEEN two pins of one part: bounds on (subject - reference) in the SI base unit, with the vendor's modality (required/recommended). The pin order is load-bearing, so swapping the two inverts the requirement (needs --params) |
| [`param.prov(mpn, symbol, doc, page, section)`](param.prov/) | the citation of a datasheet parameter: the SourceDoc title, page, and table/figure it was read from. The page is a locator and binds as a string, not a number (needs --params) |
| [`param.range(mpn, symbol, kind, min, max)`](param.range/) | a datasheet parameter's two-sided limit with its kind, both bounds in the SI base unit (absolute_max / recommended_operating / characteristic; needs --params) |
| [`param.typ(mpn, symbol, typ)`](param.typ/) | a datasheet parameter's TYPICAL value in the SI base unit, the third member of the min/typ/max triple. A typical value is what the part usually does, never a guaranteed limit, so it is its own relation rather than a column on param.range (needs --params) |
| [`param.unit(mpn, symbol, unit)`](param.unit/) | the unit a datasheet parameter is PRINTED in; param and param.range carry their numbers in SI base units, so join this to see the vendor's own spelling (needs --params) |
| [`part.audience(mpn, who)`](part.audience/) | a team/license entitled to see a part's datasheet data (record-only, needs --params) |

## predicate

| Relation | Summary |
|---|---|
| `absent(value)` | reports whether the field carried no value at all, which is different from an empty string and from zero; `not absent(?x)` reads "this field is stated" |
| [`net.reaches(from, net, hops?)`](net.reaches/) | transitive reachability through series pass elements (R/L/ferrite/fuse); the optional third argument binds the EXACT number of crossings, so a radius is written `net.reaches(?a,?b,?h), ?h <= 2` and not `net.reaches(?a,?b,2)`, which means exactly two |
| [`net.route(from, net, path)`](net.route/) | the same walk as `net.reaches`, with the route it found bound as a readable value (`VBUS -> [R5] -> VBUS_F -> [L1] -> VDD_3V3`), so a connectivity answer carries the evidence for itself; one route per pair, and a route never ends on a rail because the walk refuses one |
| `str.contains(string, substring)` | reports whether a string contains a substring |
| `str.glob(string, pattern)` | reports whether the whole string matches a shell-style glob (`*` any run, `?` one character) |
| `str.match(string, regex)` | reports whether the string matches an unanchored regular expression |
| `str.prefix(string, prefix)` | reports whether a string starts with a prefix |
| `str.suffix(string, suffix)` | reports whether a string ends with a suffix |

## derived

| Relation | Summary |
|---|---|
| `component.probed_both(r)` | A two-terminal part with a test point on both of its nets, so it can be measured in circuit. |
| `component.probed_one(r, probed, unprobed)` | A two-terminal part with a test point on exactly one of its nets, naming the probed net and then the other, which is where a missing test point would go. |
| `component.two_terminal(r, a, b)` | A part on exactly two nets, with the nets in name order (?a < ?b) so each part answers once. The parts an in-circuit test measures across two nodes: resistors, capacitors, inductors, diodes. |
| `net.has_test_point(n)` | A net at least one test point sits on, so a probe can land on it during bring-up or in-circuit test. |

