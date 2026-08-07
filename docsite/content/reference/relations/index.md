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
| [`feedback(net)`](feedback/) | the net is a regulator feedback / sense node (must not be probed) |
| [`has_nc_channel(present)`](has_nc_channel/) | one row when the design can express intentional no-connect |
| [`has_netclass(present)`](has_netclass/) | one row when the design assigns net classes at all (absent it, a netclass-scoped rule selects nothing and reads clean) |
| [`net.bus_like(net)`](net.bus_like/) | a shared-distribution net (ground plane, global rail, or rail-scale fan-out) — the series-reach walk's stop predicate |
| [`net.external(net)`](net.external/) | the net may extend onto an unread sheet (read-gap marker) |
| [`net.ground(net)`](net.ground/) | the net is a ground rail (name-derived) |
| [`net.max_voltage(net, volts)`](net.max_voltage/) | a net's declared rail voltage |
| [`net.netclass(net, class)`](net.netclass/) | the tool-assigned net class a net belongs to (KiCad net_settings; not the derived semantic role) |
| [`net.nominal_voltage(net, volts)`](net.nominal_voltage/) | a rail's nominal voltage derived from its net name (3V3 -> 3.3) |
| [`net.pin_count(net, count)`](net.pin_count/) | the number of connections on a net |
| [`pin(ref_des, pin)`](pin/) | a part-type pin of a placed component |
| [`pin.net(ref_des, pin, net)`](pin.net/) | the net a pin is on (absent if unconnected) |
| [`pin.role(ref_des, pin, role)`](pin.role/) | a pin's derived role (power/ground/anode/cathode) |
| [`pin.type(ref_des, pin, etype)`](pin.type/) | a pin's electrical type (power_in, input, output, ...) |
| [`pin_net_conflict(ref_des, pin, net)`](pin_net_conflict/) | a pin the read placed on more than one net; one row per net (reader integrity diagnostic) |
| [`rail(net)`](rail/) | the net is a power or ground rail |
| [`ref_des_collision(ref_des)`](ref_des_collision/) | a reference designator used by more than one part (reader integrity diagnostic) |
| [`types_power_out(present)`](types_power_out/) | one row when the source format classifies power-output pins (EDIF/IPC do not, so a driver-absence check is unsound there) |

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
| [`param(mpn, symbol, max)`](param/) | a datasheet parameter's max value for a part (needs --params) |
| [`param.prov(mpn, symbol, doc, page, section)`](param.prov/) | the citation of a datasheet parameter — the SourceDoc title, page, and table/figure it was read from (needs --params) |
| [`param.range(mpn, symbol, kind, min, max)`](param.range/) | a datasheet parameter's two-sided limit with its kind (absolute_max / recommended_operating / characteristic; needs --params) |
| [`part.audience(mpn, who)`](part.audience/) | a team/license entitled to see a part's datasheet data (record-only, needs --params) |

## predicate

| Relation | Summary |
|---|---|
| `contains(string, substring)` | the string contains the substring |
| `glob(string, pattern)` | the whole string matches a shell-style glob (* any run, ? one char) |
| `match(string, regex)` | the string matches an (unanchored) regular expression |
| `prefix(string, prefix)` | the string starts with the prefix |
| [`reaches(from, net, hops?)`](reaches/) | transitive reachability through series pass elements (R/L/ferrite/fuse); the optional third argument binds the EXACT number of crossings, so a radius is written `reaches(?a,?b,?h), ?h <= 2` and not `reaches(?a,?b,2)`, which means exactly two |
| `suffix(string, suffix)` | the string ends with the suffix |

