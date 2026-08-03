---
title: "EDIF netlist primer"
description: "The EDIF 2.0.0 netlist format Agni ingests: syntax, structure, gotchas, and the mapping into the IR."
---

Reference for studying the format. Pairs with `edif/reader.go` and
`protos/agni/v1/ir/ir.proto`.

## What EDIF is

EDIF = Electronic Design Interchange Format. A vendor-neutral text format for exchanging
electronic designs (netlists, schematics, and more), standardized as EIA-548. Versions: 2.0.0
(1988, by far the most common for netlist/schematic interchange), 3.0.0, 4.0.0. The files Agni
targets are 2.0.0.

It is open and sanctioned, so reading it is a legally clean ingestion path. It is an EXPORT
format. The authoritative design lives in the native tool database, so EDIF can already be lossy
relative to that (see [Ingestion and IR](../../architecture/ingestion-and-ir/)).

The reference files here are exported by Siemens/Mentor xDX Designer, `edifVersion 2 0 0`,
`edifLevel 0`. Two views:

- `.edn` netlist (10MB): connectivity. This is what the netlist reader parses.
- `.eds` schematic (62MB): adds geometry/graphics. This feeds the geometry sidecar.

### Versions, and why we target 2.0.0 (with readiness for later)

2.0.0 is what tools actually export. 3.0.0 (1993) and 4.0.0 (1996) added features but saw poor
adoption and are rare in the wild. The design split keeps that cheap. The S-expression parser
(`sexpr.go`) is version-agnostic, while the extractor (`reader.go`) is keyed to the 2.0.0 netlist
schema. The reader detects the `(edifVersion X Y Z)` header, records it, and rejects non-2.x
files rather than silently mis-parsing. Supporting 3.0.0 or 4.0.0 later is an additive extractor
over the same parser and IR, not a rewrite.

## Syntax in 60 seconds

- **S-expressions.** Everything is `(keyword args...)`, nested with parentheses, LISP-like,
  ASCII.
- **Atoms:** keywords (`net`, `instance`), identifiers, integers, and `"quoted strings"`.
- **Identifiers** must start with a letter or `&` and contain letters/digits/underscore, up to
  255 chars. They cannot start with a digit or contain spaces or dashes.
- **The rename trick.** To carry a name that is not a valid identifier (e.g. `2525913-1`,
  `Sync Exclude`, `$17I12086`), EDIF writes `(rename &validid "Display")`. The `&validid` is the
  machine identifier used in all cross-references, and the quoted string is the human name. This
  is why `&25259130551` and `&04417I12086` appear everywhere.
- **Escapes:** non-ASCII in strings via `%int%`, comments via `(comment "...")`.

## Document structure

```
(edif <name>
  (edifVersion 2 0 0)
  (edifLevel 0)
  (keywordMap (keywordLevel 0))
  (status (written (timestamp ...) (author ...) (program ...)))
  (library <LibName> ...)          ; one or more: part-type definitions
  (design  <DesignName> (cellRef <top> (libraryRef <lib>))))
```

The actual placed components and nets live inside the top cell's netlist view `(contents ...)`.
The reader just collects every `(instance ...)` and `(net ...)` node in the tree, since they only
occur there.

### Libraries and cells (part TYPES)

```
(library Connector
  (technology (numberDefinition (gridMap 0 0)) (figureGroup GRAPHICS))
  (cell (rename &25259130551 "2525913-1")            ; a part type
    (cellType GENERIC)
    (view (rename &2525913_18pin "2525913_18pin")
      (viewType NETLIST)
      (interface
        (designator "J?")                            ; ref-des PREFIX for this part
        (port (rename &1 "1") (direction INPUT) (designator "1"))   ; a pin
        ...))))
```

- **cell** = component/part TYPE, not a placed instance.
- **cellType GENERIC**, **viewType NETLIST** (vs SCHEMATIC/GRAPHIC).
- **interface** = external pins (`port`), each with a `direction` and a pin `designator` (pin
  number). The interface's `designator "J?"` is the ref-des prefix.

### Instances (placed components)

```
(instance (rename &04417I12086 "$17I12086")          ; internal id + display
  (viewRef TP (cellRef TP_1mm (libraryRef Misc)))     ; which part TYPE this is
  (portInstance &1 (designator "1"))
  (designator "TP9224")                                ; the REF-DES
  (property Description (string "Test Point 1mm") (owner "Siemens")))
```

- **instance** = a placed component. Its `(rename &id ...)` gives the internal id used by net
  cross-references.
- **viewRef -> cellRef -> libraryRef** points at the part type in a library.
- **designator "TP9224"** = the reference designator (ref-des).
- **property** = attributes (Value, Description, Status, DXDB_LIBNAME, ...). Value forms:
  `(string "..")`, `(integer N)`, `(boolean (true))`.

### Nets (connectivity)

```
(net <name>
  (joined
    (portRef &1 (instanceRef &04432I179))   ; pin "1" of that instance
    (portRef K  (instanceRef &04432I158))   ; pin "K" of that instance
    ...))
```

- **net** = an electrical node. `(joined ...)` lists the pins tied together.
- **portRef PIN (instanceRef INST)** = pin PIN on instance INST. INST is the instance's internal
  `&id`, so to get the ref-des you resolve INST to the instance whose `(rename &INST ...)` matches,
  then read its `(designator ...)`.

## Gotchas that bit us / will bite you

1. **`designator` is overloaded.** Component ref-des (in instance), pin number (in
   port/portInstance), and ref-des PREFIX (in cell interface) all use `designator`.
2. **Cross-references use the `&` internal id, not the display name.**
   `instanceRef`/`portRef`/`viewRef`/`cellRef`/`libraryRef` reference the machine id. Human names
   come from the paired `(rename ...)`.
3. **Multi-section components: ref-des is NOT unique per instance.** One physical component is
   often several `(instance ...)` nodes sharing a ref-des, each a section/bank of pins (connector
   J1906 into A/B/C banks, multi-gate ICs, relays with coil plus contacts). Sections may share a
   cell (homogeneous) or use different cells (heterogeneous). Group by ref-des to reason about a
   physical part. This is why a naive "duplicate ref-des" check is wrong.
4. **NETLIST vs SCHEMATIC views.** The `.edn` has only connectivity. Geometry (symbol shapes,
   coordinates, routing) lives in the `.eds` SCHEMATIC view, out of scope for the netlist reader
   (see the geometry sidecar).
5. **`edifLevel 0`** = static, no parameters/expressions. Simpler to parse.

## How we map EDIF -> the IR

See `edif/reader.go` and `protos/agni/v1/ir/ir.proto`.

| EDIF construct | IR |
|---|---|
| `(library NAME (cell ...))` | `Library{ name, cells }` |
| `(cell (rename &id "N") (cellType T) ... ports)` | `Cell{ name, cell_type, designator_prefix, ports }` |
| `(port (rename &id "N") (direction D) (designator P))` | `Port{ name, direction, designator }` |
| `(instance ... (cellRef C (libraryRef L)) (designator R) (property ...))` | `ComponentInstance{ ref_des=R, cell_ref=C, library_ref=L, attributes }` |
| `(net N (joined (portRef PIN (instanceRef INST))...))` | `Net{ name=N, connections=[PortRef{ instance_ref=refdes(INST), port_ref=PIN }] }` |
| `(rename &id "display")` | `Provenance{ source_id=&id }` + display used as the name |

**Fidelity:** lossy-bounded (netlist subset). The reader keeps components, part refs, properties,
pins, and connectivity. It drops graphics, technology, status and timestamps, and most non-netlist
metadata.

## Reading a real file

Point these at your own `.edn` export (real design data stays outside the repo). Useful greps:

- Instances: `grep -c '(instance ' file`
- Nets: `grep -c '(joined' file` (each joined = one net's connectivity)
- A component's sections: `grep -n '(designator "J1")' file`

## Further reading

- EDIF 2.0.0 specification (EIA-548).
- Wikipedia "EDIF" for history and version differences.
