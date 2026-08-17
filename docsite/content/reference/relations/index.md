---
title: "Relations catalog"
description: "Every query relation the fact base exposes, grouped by kind."
---

The relations a datalog query joins over. Each documented relation links to its full reference: the hardware it describes, its Go projector, and example queries. See the [querying guide](../../guide/querying/) for how to compose them. This page is generated from the shipped fact base.

## netlist

| Relation | Summary |
|---|---|
| [`bus(label, kind)`](bus/) | a reader-detected bus not yet expanded into member nets (WS1-034) |
| [`component-on-net(ref_des, net)`](component-on-net/) | a component sits on a net |
| [`component.attr(ref_des, key, value)`](component.attr/) | a component-level attribute (e.g. interface, MPN) |
| [`component.class(ref_des, class)`](component.class/) | a device class the part is in (a family tag too, e.g. a TVS is both tvs and diode) |
| [`component.mpn(ref_des, mpn)`](component.mpn/) | the design-side part identity (manufacturer part number) |
| [`external_signal_net(net)`](external_signal_net/) | a connector-facing signal net (not a rail, ground, no-connect, or power path), the scope the ESD rules share |
| [`feedback(net)`](feedback/) | the net is a regulator feedback / sense node (must not be probed) |
| [`has_nc_channel(present)`](has_nc_channel/) | one row when the design can express intentional no-connect |
| [`has_netclass(present)`](has_netclass/) | one row when the design assigns net classes at all (absent it, a netclass-scoped rule selects nothing and reads clean) |
| [`has_netclass_defs(present)`](has_netclass_defs/) | one row when the design declares net-class definitions at all (absent it, a declared-vs-actual rule has no limit to compare against and reads clean) |
| [`net.ac_coupled(net)`](net.ac_coupled/) | a SERIES capacitor carries the net (a decoupling cap to ground/rail does not count) |
| [`net.bias(net, level)`](net.bias/) | a bias resistor holds the net at a rail (high) or ground (low); absent when unbiased or held by a divider |
| [`net.bus_like(net)`](net.bus_like/) | a shared-distribution net (ground plane, global rail, or rail-scale fan-out), the series-reach walk's stop predicate |
| [`net.declared_track_width(net, mm)`](net.declared_track_width/) | the track width a net SHOULD route at, cascaded across its classes by priority (join this, not the per-class rows) |
| [`net.declared_via_drill(net, mm)`](net.declared_via_drill/) | the via drill a net SHOULD route at, cascaded across its classes by priority (join this, not the per-class rows) |
| [`net.external(net)`](net.external/) | the net may extend onto an unread sheet (read-gap marker) |
| [`net.ground(net)`](net.ground/) | the net is a ground rail (name-derived) |
| [`net.max_voltage(net, volts)`](net.max_voltage/) | a net's declared rail voltage |
| [`net.netclass(net, class)`](net.netclass/) | the tool-assigned net class a net belongs to (KiCad net_settings; not the derived semantic role) |
| [`net.nominal_voltage(net, volts)`](net.nominal_voltage/) | a RAIL's nominal voltage derived from its net name (3V3 -> 3.3). Rails only; a non-rail net's name-derived level is net.signal_level |
| [`net.pin_count(net, count)`](net.pin_count/) | the number of connections on a net |
| [`net.signal_level(net, volts)`](net.signal_level/) | the signalling level a NON-RAIL net's name declares, the other half of net.nominal_voltage. A house convention that encodes a level into a signal net's name lands here rather than being read as a rail nominal |
| [`netclass.clearance(class, mm)`](netclass.clearance/) | the clearance a net class declares its nets should route at (millimetres) |
| [`netclass.track_width(class, mm)`](netclass.track_width/) | the track width a net class declares its nets should route at (millimetres) |
| [`netclass.via_diameter(class, mm)`](netclass.via_diameter/) | the via diameter a net class declares (millimetres) |
| [`netclass.via_drill(class, mm)`](netclass.via_drill/) | the via drill a net class declares (millimetres) |
| [`pin(ref_des, pin)`](pin/) | a part-type pin of a placed component |
| [`pin.net(ref_des, pin, net)`](pin.net/) | the net a pin is on (absent if unconnected) |
| [`pin.role(ref_des, pin, role)`](pin.role/) | a pin's derived role (power/ground/anode/cathode) |
| [`pin.type(ref_des, pin, etype)`](pin.type/) | a pin's electrical type (power_in, input, output, ...) |
| [`pin_net_conflict(ref_des, pin, net)`](pin_net_conflict/) | a pin the read placed on more than one net; one row per net (reader integrity diagnostic) |
| [`rail(net)`](rail/) | the net is a power or ground rail |
| [`ref_des_collision(ref_des)`](ref_des_collision/) | a reference designator used by more than one part (reader integrity diagnostic) |
| [`types_power_out(present)`](types_power_out/) | one row when the source format classifies power-output pins (EDIF/IPC do not, so a driver-absence check is unsound there) |
| [`unresolved_symbol(ref_des, symref)`](unresolved_symbol/) | a placement whose symbol did not resolve, so it carries no pins (WS1-052) |

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
| [`param(mpn, symbol, max)`](param/) | a datasheet parameter's max value for a part, in its SI base unit (needs --params) |
| [`param.pin(mpn, pin, name, function)`](param.pin/) | a pin the part's datasheet declares, keyed by its spec-local id, with the printed name and its function (power_input / ground / bidirectional / no_connect / ...; needs --params) |
| [`param.pin_range(mpn, pin, symbol, kind, min, max)`](param.pin_range/) | a datasheet limit bound to ONE pin, both bounds in the SI base unit, the per-terminal counterpart to param.range, so a part with several supply pins answers per pin instead of once (needs --params) |
| [`param.pin_relation(mpn, subject_pin, reference_pin, modality, min, max)`](param.pin_relation/) | a datasheet constraint BETWEEN two pins of one part: bounds on (subject - reference) in the SI base unit, with the vendor's modality (required/recommended). The pin order is load-bearing, so swapping the two inverts the requirement (needs --params) |
| [`param.prov(mpn, symbol, doc, page, section)`](param.prov/) | the citation of a datasheet parameter: the SourceDoc title, page, and table/figure it was read from (needs --params) |
| [`param.range(mpn, symbol, kind, min, max)`](param.range/) | a datasheet parameter's two-sided limit with its kind, both bounds in the SI base unit (absolute_max / recommended_operating / characteristic; needs --params) |
| [`param.unit(mpn, symbol, unit)`](param.unit/) | the unit a datasheet parameter is PRINTED in; param and param.range carry their numbers in SI base units, so join this to see the vendor's own spelling (needs --params) |
| [`part.audience(mpn, who)`](part.audience/) | a team/license entitled to see a part's datasheet data (record-only, needs --params) |

## predicate

| Relation | Summary |
|---|---|
| `absent(value)` | the field carried no value at all, which is different from an empty string and from zero (a datasheet row stating only a maximum leaves its minimum absent); `not absent(?x)` reads "this row states one" |
| `contains(string, substring)` | the string contains the substring |
| `glob(string, pattern)` | the whole string matches a shell-style glob (* any run, ? one char) |
| `match(string, regex)` | the string matches an (unanchored) regular expression |
| `prefix(string, prefix)` | the string starts with the prefix |
| [`reaches(from, net, hops?)`](reaches/) | transitive reachability through series pass elements (R/L/ferrite/fuse); the optional third argument binds the EXACT number of crossings, so a radius is written `reaches(?a,?b,?h), ?h <= 2` and not `reaches(?a,?b,2)`, which means exactly two |
| `suffix(string, suffix)` | the string ends with the suffix |

