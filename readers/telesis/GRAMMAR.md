# Telesis netlist grammar (ingested subset)

What the `telesis` package parses out of a flat Telesis netlist (`.tel`), the format the
Mentor/Siemens schematic flow emits, and how each construct maps to the neutral IR
(`agni.v1.ir`). This is the subset we read, not the full format, and it was learned from real
exports rather than from a specification. Same posture as the KiCad, xschem and gEDA grammars.

The format carries connectivity and properties and no geometry at all, so this is a Design-only
reader. Layout arrives as a separate companion (CONSTRAINTS C21).

## 1. Sections (read.go `parse`)

The file is line-oriented. A line whose first non-blank character is `$` opens a section, and
the first word on it is the marker. Everything up to the next marker belongs to that section.

```
$PACKAGES        'PART' ! 'MPN' ; U1 U2 U3,        part identity, grouped by part type
$A_PROPERTIES    'Capacitance' '100nF' ; C1 C2,    component attributes, grouped by VALUE
$NETS            'SOME_NET' ; U1.14 R7.2,          connectivity
$PINS                                              observed empty in every real export
$A_PROPERTIES    'Pin Type' 'IN' ; U1.14 U2.3,     pin attributes, grouped by VALUE
$END                                               stops the read
```

`$PACKAGES`, `$A_PROPERTIES` and `$NETS` are read. `$END` stops the read, and anything after it
is ignored. Any other section that holds content is skipped whole and its marker is recorded in
the design attribute `telesis.unparsed_sections`, comma-separated in file order. `$PINS` is the
case in hand, since nothing is known about its body. A section that is present and empty records
nothing.

The sniff (`IsTelesis`) wants a line-initial `$NETS` or `$PACKAGES`. A file with neither
packages nor nets is an error.

## 2. Entries (read.go `splitEntryLine`, `foldEntries`)

Every section body is a run of entries. An entry is a HEAD of single-quoted fields, a `;`, and a
TARGET run of whitespace-separated tokens. A line that ends in `,` continues onto the next line,
and the comma is the only continuation marker, so a target run can span any number of lines. One
property entry can carry over a thousand targets.

The head comes in five shapes, each found in a real export:

```
'PART' ! 'MPN' ; targets                  the common package entry
! 'MPN' ; targets                         a part the library names only by MPN
'PART' ! 'MPN' ! 'value' ! '10%' ; ...    extra positional fields after the MPN
'Name' 'Value' ; targets                  a property entry, no bang
'NET_NAME' ; targets                      a net entry, one field
```

Quoting is respected while scanning for `;` and for `!`, so a field containing either character
does not end the head early. A leading `!` records an empty first field, so fields keep their
positions. A line that neither opens an entry nor continues one is skipped, and it also ends any
continuation in progress.

## 3. Three rules that decide what an entry means

- **The `!` marks a package.** A package entry and a property entry are both a quoted head, a
  `;` and targets, and only the package has the `!`. A non-bang entry inside `$PACKAGES` is
  ignored, because reading it as a package would invent a component named after a property.
- **The target shape decides component versus pin.** `$A_PROPERTIES` appears twice. The first
  block's targets are ref-des and the second block's are `REFDES.PIN`. The reader routes each
  target by its shape (`^[A-Za-z][A-Za-z0-9_]*\.[A-Za-z0-9_]+$`) rather than by which block it is
  in, so a file that orders the blocks differently, or carries only one, reads the same. A ref-des
  never contains a dot. The pin half is permissive on purpose, since `U1.14`, the BGA form `U7.L1`
  and the connector-shell form `J1.A` are all legal.
- **A property section is an inverted index.** Each entry is (name, value) with many targets, so
  every component sharing a value rides on one entry. The IR is component-major, so reading
  properties is a transpose and most of the reader's work.

## 4. IR mapping

| Telesis | IR |
|------|----|
| `$PACKAGES` entry head field 1 (`PART`) | `PartType.name` (`kind = "telesis"`), in library `"telesis"` |
| `$PACKAGES` entry head field 2 (`MPN`) | `PartType.mpn`. `classify.StampMPN` later copies it onto each component |
| `! 'MPN'` with no part name | `PartType.name` = the MPN, plus attribute `name_from_mpn = "true"` |
| head fields 3 and on | `PartType.attributes["field_3"]`, `["field_4"]`, ... by 1-based position, uninterpreted |
| a package target | `Component.ref_des`, one `ComponentSection` naming the part type |
| a second entry with the same `PART` and a different `MPN` | its own `PartType`, named `PART:MPN` with attribute `package = PART`, so each part keeps the MPN its entry states (parts sharing a footprint). A repeat of an `MPN` already seen for that `PART` joins the existing part type |
| a ref-des under a second package entry | `InputDiagnostics.ref_des_collisions`. The first declaration wins |
| `$NETS` entry head | `Net.name`. A repeated name is ignored after the first |
| a net name starting with `$` (`$1N0001`) | `Net.attributes["generated_name"] = "true"`. Kept, never dropped |
| a `REFDES.PIN` net target | `Connection{component_ref, pin_ref}`. A target without a dot is skipped |
| property with a ref-des target | `Component.attributes[name] = value` |
| `'Pin Type'` on `REFDES.PIN` | `Pin.direction`, raw spelling in `Pin.attributes["direction_raw"]` |
| `'PinLabel'` on `REFDES.PIN` | `Pin.name` (first writer wins) |
| any other property on `REFDES.PIN` | `Pin.attributes[name] = value` |

`InputDiagnostics.supplied` is `["ref_des_collisions"]`, the one diagnostic this format can
answer.

Pin facts arrive per INSTANCE while the IR hangs pins off the shared part type. The reader folds
every instance's pins onto the part type's pin list, sorted naturally (`2` before `10`, `A1` before
`B1`). Where two instances disagree on `Pin Type`, the first wins and the pin carries
`direction_conflict = "<first>|<second>"`. A difference in case alone is not a conflict.

`Pin Type` is the only place the format states direction. It maps case-insensitively:

| `Pin Type` | `PinDirection` |
|------|----|
| `IN` | `INPUT` |
| `OUT` | `OUTPUT` |
| `BI` | `INOUT` |
| `POWER`, `GROUND` | `POWER_IN` |
| `TERMINAL`, `ANALOG` | `PASSIVE` |
| anything else | `UNSPECIFIED`, with the spelling kept in `direction_raw` |

## 5. Encoding

The format declares no encoding. A file that is valid UTF-8 is read as UTF-8, and anything else is
read as latin-1, which real exports carry and which cannot fail. `testdata/latin1.tel` covers it.

Fidelity is lossy-bounded (CONSTRAINTS C6). The file's ordering and its grouping are dropped,
because a property is stored on each component it named and the inverted-index entry it arrived
on is not recoverable. There is no geometry to lose.
