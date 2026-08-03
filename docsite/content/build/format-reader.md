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

The EDIF reader's signature is exactly this (`edif/reader.go`):

```go
func Read(r io.Reader, sourceFile string) (*ir.Design, error)
```

`sourceFile` is the file name, used only for provenance and error messages. The reader does not
open it. It reads bytes from `r` and returns the design. Keeping the parser `io.Reader`-pure is
what lets the same reader run in the CLI, in the server, and in tests over an in-memory fixture.

## The registry entry

Every format the engine reads is one entry in the registry, a `formats.Format` value keyed by
extension (`formats/formats.go`):

```go
type Format struct {
    Ext      string  // lowercase extension including the dot, e.g. ".myfmt"
    Name     string  // the file-tree / UI label
    Design   func(l *Loader, path string) (*ir.Design, error)              // netlist; nil if none
    Geometry func(l *Loader, path string) (*geom.SchematicGeometry, error) // faithful schematic; nil if none
    Board    func(l *Loader, path string) (*geom.BoardGeometry, error)     // board layout; nil if none
}
```

The three reader fields are capabilities. A nil field means the format lacks that capability. A
plain netlist format sets only `Design`. A schematic format that also carries drawable geometry
sets `Design` and `Geometry`. A board format sets `Board`. `.eds` is dual-capability (`Design`
plus `Geometry`), and `.kicad_pcb` and the IPC-2581 extensions show the `Board` case.

For a built-in reader, add the registration to `formats/registry.go`'s `init`. The wiring for a
netlist-only format is one adapter that opens the file and hands the bytes to your pure `Read`:

```go
Register(&Format{Ext: ".myfmt", Name: "myfmt", Design: readMyFmt})

func readMyFmt(_ *Loader, path string) (*ir.Design, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()
    return myfmt.Read(f, path)
}
```

`Register` validates the entry and panics on a malformed one or a duplicate extension, the same
way the standard library's `image.RegisterFormat` and `sql.Register` do. Those are programming
errors surfaced at process start, not runtime conditions. The `Ext` has to be lowercase and start
with a dot, `Name` has to be non-empty, and at least one of the three capability funcs has to be
set.

Once registered, the extension resolves through every derived surface: the CLI reader dispatch,
the file-tree label, and the supported-extensions error text all read this one table. There is no
second table to update.

## One format, several extensions

The same format often appears under several conventional suffixes. Register a loop over the
aliases sharing one reader func. EDIF does this (`formats/registry.go`):

```go
for _, ext := range []string{".edn", ".edf", ".edif"} {
    Register(&Format{Ext: ext, Name: "edif", Design: readEDIF})
}
```

Extension matching is case-insensitive, so `.EDF` resolves through the `.edf` entry with no extra
registration.

A reader existing is not the same as its files being visible. For years the EDIF corpus was
invisible in the file tree and rejected by the CLI because only the nonstandard `.edn` was wired,
even though the parser handled `.edf` fine. When a folder of files shows nothing, check that the
extension is registered before suspecting the reader.

## Ambiguous extensions get sniffed

Some extensions are shared across formats. `.sch` is xschem, gEDA gschem, or legacy KiCad, and
`.xml` might be IPC-2581 or something else. The registry entry names the format optimistically and
the adapter sniffs the file header before committing to a reader. IPC-2581 peeks the first bytes
for its root element (`formats/registry.go`):

```go
func readIPC2581(_ *Loader, path string) (*ir.Design, error) {
    f, err := os.Open(path)
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

## Multi-file resolution stays in `formats`, not the reader

A format that references other files (KiCad sub-sheets, xschem or gEDA symbol libraries) still
does not open a second file inside the reader. The `Loader` builds an opener closure and passes it
in, so the reader receives already-resolved bytes or a resolver it can call, and the file I/O stays
in one place.

The `Loader` carries the configuration a reader needs beyond the file itself, today the
`--symbol-path` search directories (`formats/loader.go`):

```go
type Loader struct {
    SymbolPaths []string
}
```

For KiCad, the adapter reads the root schematic and hands the reader an opener that resolves each
child sheet's `Sheetfile` against the root's directory, plus a separate opener for external symbol
libraries:

```go
d, _, err := kicad.ReadSchematicHierarchyNetsWithSymbols(
    path, content, sheetOpener(path), l.kicadSymOpener(path))
```

`sheetOpener` and `kicadSymOpener` are built by the `Loader` and injected. The reader calls them
but never touches the filesystem directly. If your format has this shape, write the parser to take
an opener closure and build that closure in the registry adapter, following `sheetOpener` and
`symbolOpener` in `formats/loader.go`.

Any diagnostic that depends on the referenced files has to account for a failed resolution. An
unresolved symbol drops that symbol's pins, so wire ends meant to land on it read as dangling.
Gate any such check on full resolution rather than reporting the phantom findings.

## What the Loader does after your reader returns

`Loader.ReadDesign` picks the reader by extension, runs it, and then applies a few format-neutral
normalization passes so every reader's output is consistent (`formats/loader.go`):

```go
d, err := f.Design(l, path)
// ...
netgraph.StampNetIDs(d)      // deterministic per-instance net ids
classify.Stamp(d)            // component device_classes
classify.StampNetRoles(d)    // net roles (rail / ground / feedback) from the naming lexicon
classify.StampPowerInPins(d) // fill POWER_IN on under-typed supply pins
```

These run for every reader, so a new reader does not have to reproduce them. Build a faithful IR
(components, nets, pins, provenance) and the shared passes fill in the derived facts. If your
format under-types a construct the way EDIF types every supply pin as a plain input, the stamp
passes normalize it after the fact rather than pushing that concern into your parser.

## Reconcile against the IR, not against your format

Do not add IR fields a second format would not populate. The IR is a cross-format representation,
so a field only one reader can fill tends to be a modeling mistake. When you reach for a new field,
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

A reader lands with a runnable example under `examples/`, one directory per feature over the
shared `common` reuse package. Each example is its own Go module so the demo code stays out of the
engine's `go.mod`, with the narration in a sidecar `walkthrough.md` rather than in Go string
literals. `examples/CONVENTIONS.md` has the layout and `examples/read-and-stats/` is the reference
to copy.
