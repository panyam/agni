---
title: "Adding a format reader"
description: "Wire a new EDA file format into the neutral IR with one package and one registry entry."
---

Agni reads many EDA formats into one neutral IR, the way Pandoc reads many document formats into
one AST. Adding a format means writing a parser that produces that IR and registering it. The
IR itself and the reasoning behind the design are covered in
[Ingestion and IR](../../architecture/ingestion-and-ir/). This page is the mechanical how-to,
grounded in the code as it stands.

## What a reader is

A reader is its own Go package that exposes a pure parse function over an `io.Reader`:

```go
package myfmt

import (
    "io"
    ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func Read(r io.Reader, sourceFile string) (*ir.Design, error) {
    // parse r, build and return an *ir.Design
}
```

The EDIF reader's signature is exactly this (`readers/edif/reader.go`):

```go
func Read(r io.Reader, sourceFile string) (*ir.Design, error)
```

`sourceFile` is the file name, used only for provenance and error messages. The reader does not
open it. It reads bytes from `r` and returns the design. An `io.Reader`-pure parser runs
unchanged in the CLI, in the server, and in tests over an in-memory fixture.

## The registry entry

Each format has one entry in the registry, a `formats.Format` value keyed by extension
(`readers/formats/formats.go`):

```go
type Format struct {
    Ext      string  // lowercase extension including the dot, e.g. ".myfmt"
    Name     string  // the file-tree / UI label
    Design   func(l *Loader, path string) (*ir.Design, error)              // netlist; nil if none
    Geometry func(l *Loader, path string) (*geom.SchematicGeometry, error) // faithful schematic; nil if none
    Board    func(l *Loader, path string) (*geom.BoardGeometry, error)     // board layout; nil if none
    DrawnNetlist bool // the netlist counts what the drawing draws, so its counts say nothing
}
```

The three reader fields are capabilities. A nil field means the format lacks that capability. A
plain netlist format sets only `Design`. A schematic format that also carries drawable geometry
sets `Design` and `Geometry`. A board format sets `Board`. `.eds` is dual-capability (`Design`
plus `Geometry`), and `.kicad_pcb` and the IPC-2581 extensions show the `Board` case.

Set `DrawnNetlist` only when the format's netlist counts the drawing rather than the design.
`.eds` sets it, because an EDIF schematic export nets per-sheet segments that nothing joins. A
design read from such a file then reports no part or net counts, in the viewer or in `agni
trial`. A KiCad, xschem or gEDA schematic is its design's netlist and leaves it false, so it
reports its counts (agni issue 1342).

For a built-in reader, add the registration to `readers/formats/registry.go`'s `init`. A
netlist-only format needs one adapter that opens the file and hands the bytes to your pure `Read`:

```go
Register(&Format{Ext: ".myfmt", Name: "myfmt", Design: readMyFmt})

func readMyFmt(l *Loader, path string) (*ir.Design, error) {
    f, err := l.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()
    return myfmt.Read(f, path)
}
```

Read the bytes through `l.Open` (or `l.ReadFile`), never `os` directly. The `Loader` decides
what a path resolves against: the host filesystem by default, or an in-memory `fs.FS` when the host
has no filesystem at all, such as a WASM build or an embedder holding designs in memory. A reader
that opens its own files works on a server and fails everywhere else, and the failure looks like a
missing design rather than a missing capability.

`Register` validates the entry and panics on a malformed one or a duplicate extension, the same
way the standard library's `image.RegisterFormat` and `sql.Register` do. Those are programming
errors surfaced at process start, not runtime conditions. The `Ext` has to be lowercase and start
with a dot, `Name` has to be non-empty, and at least one of the three capability funcs has to be
set.

Once registered, the extension resolves in the CLI reader dispatch, the file-tree label, and the
supported-extensions error text. All three read this one table, and there is no second table to
update.

## One format, several extensions

The same format often appears under several conventional suffixes. Register a loop over the
aliases sharing one reader func. EDIF does this (`readers/formats/registry.go`):

```go
for _, ext := range []string{".edn", ".edf", ".edif"} {
    Register(&Format{Ext: ext, Name: "edif", Design: readEDIF})
}
```

Extension matching is case-insensitive, so `.EDF` resolves through the `.edf` entry with no extra
registration.

A reader can exist while its files are still invisible. For years the EDIF corpus was
invisible in the file tree and rejected by the CLI because only the nonstandard `.edn` was wired,
even though the parser handled `.edf` fine. When a folder of files shows nothing, check that the
extension is registered before suspecting the reader.

## Ambiguous extensions get sniffed

Some extensions are shared across formats. `.sch` is xschem, gEDA gschem, or legacy KiCad, and
`.xml` might be IPC-2581 or something else. The registry entry names the format optimistically and
the adapter sniffs the file header before committing to a reader. IPC-2581 peeks the first bytes
for its root element (`readers/formats/registry.go`):

```go
func readIPC2581(l *Loader, path string) (*ir.Design, error) {
    f, err := l.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()
    br := bufio.NewReader(f)
    head, _ := br.Peek(1024)
    if !bytes.Contains(head, []byte("IPC-2581")) {
        return nil, fmt.Errorf("%q: not an IPC-2581 file (no IPC-2581 root element)", path)
    }
    return ipc2581.Read(br, path)
}
```

The `.sch` adapter sniffs the same way, dispatching to the xschem or gEDA reader by header.

## Multi-file resolution belongs in `formats`, not the reader

{{ includeFile "figures/reader-boundary.svg" }}

Some formats reference other files, such as KiCad sub-sheets and xschem or gEDA symbol libraries.
Their readers still do not open a second file. The `Loader` builds an opener closure and passes it
in, so the reader receives already-resolved bytes or a resolver it can call, and all file I/O
happens in one place.

The `Loader` also carries configuration a reader needs beyond the file itself. It holds the
`--symbol-path` search directories, the naming vocabulary that stamps each read, and the file
namespace those paths resolve in (abridged from `readers/formats/loader.go`):

```go
type Loader struct {
    SymbolPaths []string
    Lexicon     *classify.Lexicon
    FS          fs.FS // nil = the host filesystem
}
```

Build sibling names with `l.Sibling(designPath, ref)` rather than `filepath.Join`. An `fs.FS`
namespace is always slash-separated and the host filesystem uses the platform's separator, so a
reader that joins paths itself resolves siblings correctly on unix and silently fails to find them
on Windows under an FS. `Sibling` keeps that distinction in one place.

For KiCad, the adapter reads the root schematic and hands the reader an opener that resolves each
child sheet's `Sheetfile` against the root's directory, plus a separate opener for external symbol
libraries:

```go
d, _, err := kicad.ReadSchematicHierarchyNetsWithSymbols(
    path, content, l.sheetOpener(path), l.kicadSymOpener(path))
```

`sheetOpener` and `kicadSymOpener` are built by the `Loader` and injected. The reader calls them
but never touches the filesystem directly. If your format has this shape, write the parser to take
an opener closure and build that closure in the registry adapter, following `sheetOpener` in
`readers/formats/registry.go` and `symbolOpener` in `readers/formats/loader.go`.

Any diagnostic that depends on the referenced files has to account for a failed resolution. An
unresolved symbol drops that symbol's pins, so wire ends meant to end on it read as dangling.
Gate any such check on full resolution rather than reporting the phantom findings.

## A format that is a directory

ODB++ is the one format whose design is a directory tree rather than a file (agni issue 1250). Its
reader (`readers/odb`) takes an `fs.FS` rooted at the job instead of an `io.Reader`, so it is as
runtime-agnostic as a single-file reader. The registry adapter builds that `fs.FS` over the
`Loader`, so every file the job opens is still reached through `l.Open` and recorded for the cache,
and an archive (`.tgz`, `.zip`) is unpacked in memory into the same shape.

The registry is keyed by extension and a directory has none, so a job is entered by one file inside
it, `matrix/matrix`. `lowerExt` files that path under a key of its own. Two hosts then need the
job's real extent rather than the entry's folder. `ListDesignFiles` gathers the files a browser
engine fetches, and the proposer turns a dropped folder into designs. `formats.DesignRoot` answers
the question for both, and a second directory format would answer through the same function.

## What the Loader does after your reader returns

`Loader.ReadDesign` picks the reader by extension, runs it, and then applies a few format-neutral
normalization passes so every reader's output is consistent (`readers/formats/loader.go`). It is two
halves. `ParseDesign` runs the passes that read no configuration, and `DeriveDesign` the ones that read
a project's vocabulary or datasheet corpus:

```go
// ParseDesign
d, err := f.Design(l, path)
// ...
relocateSources(d, l.sourceName()) // locators name the mount path, not the host path
netgraph.StampNetIDs(d)            // deterministic per-instance net ids
classify.StampMPN(d)               // fill ir.Component.mpn from the aliases or the part type

// DeriveDesign(d, lex, deviceClassFor)
lex.Stamp(d)                       // component device_classes
lex.StampNetRoles(d)               // net roles (ir.Role) from the naming lexicon
lex.StampValues(d)                 // component values as machine-comparable quantities
lex.StampPowerInPins(d)            // fill POWER_IN on under-typed supply pins
classify.StampClassesFromSpecs(d, deviceClassFor) // device classes a datasheet states
```

A server and the browser engine keep the parse and derive it again for each overlay (agni issue
1061), so a new pass goes in the half that matches what it reads. Suppose a pass that reads a
vocabulary sits in `ParseDesign`. Its output would be kept under one project's conventions and
served under another's.

These run for every reader, so a new reader does not have to reproduce them. Build a faithful IR
(components, nets, pins, provenance) and the shared passes fill in the derived facts. If your
format under-types a construct the way EDIF types every supply pin as a plain input, the stamp
passes normalize it after the fact rather than pushing that concern into your parser.

### Getting the part number wrong fails quietly

Record the manufacturer part number wherever your grammar states it, under whatever key the format
spells it, and on the part type if the format puts it there. `StampMPN` fills the typed
`ir.Component.mpn` field, trying the component's own attributes under each alias first and then the
component's part type.

Do not write your own promotion. Both halves of this used to live inside the EDIF reader, so EDIF
designs resolved part numbers and no other format did. Telesis recorded the number on the part type,
every consumer read the component, and `component.mpn` came back empty for every component of every
`.tel` design. That silently disabled the whole datasheet tier on that format (agni issue 519).
Nothing failed, because when a parameter rule finds no part number, it cannot tell "no datasheet
seeded" from "this format never delivers one".

If your format spells the key in some new way, add the spelling to `classify.MPNAliases` rather than
normalizing it in your reader. The vocabulary is data in one place. Readers share it
to find the fact in their own grammar, and a project extends it with the lexicon's `attribute.mpn`
without touching either. Keep a format's other fields where the attribute keys can
reach them, on the component section if classification should not read them as part text.

## Reconcile against the IR, not against your format

Do not add an IR field unless a second format would populate it. The IR is a cross-format
representation, so a field only one reader can fill tends to be a modeling mistake. When you reach for a new field,
check whether another format could carry the same concept. If it could not, the fact probably
belongs in provenance or a derived pass, not the core IR.

## Verify against real files

Unit tests use tiny hand-authored fixtures in the package's `testdata/` directory, loaded with an
`os.ReadFile` helper rather than inline raw-string constants. Those pin the parser's behavior. They
are not enough on their own. Verify a reader against the real corpus by running `agni stats` and
`agni diff` over real exports, because rotation and mirror errors and tool-dialect quirks do not
show up in a small fixture. For a geometry reader, render a real file to PNG and look at it. For a
board reader, cross-check the DRC counts against `kicad-cli pcb drc` where a KiCad equivalent
exists. See [Native verification](../native-verification/) for using a format's own tool as an
oracle.

## Ship an example

A reader ships with a runnable example under `examples/`, one directory per feature over the
shared `common` reuse package. Each example is its own Go module, so the demo code is kept out of
the engine's `go.mod`, with the narration in a sidecar `walkthrough.md` rather than in Go string
literals. `examples/CONVENTIONS.md` has the layout and `examples/read-and-stats/` is the reference
to copy.
