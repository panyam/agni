# agni

An EDA design engine: readers for several schematic and board formats feed one neutral,
protobuf-defined IR, and everything downstream (structural rule checks, revision diff,
schematic rendering, a browser viewer) is format-neutral over that IR.

Formats read today: EDIF 2.0.0 netlists (`.edn`) and schematics (`.eds`), KiCad
(`.kicad_sch`, `.kicad_pcb`, `.kicad_pro` project merge), IPC-2581 (`.xml`, `.cvg`),
xschem and gEDA gschem (`.sch`, sniffed). Adding a reader is one entry in
`formats/registry.go`.

## Quick start

Requires Go 1.26 and pnpm (the web viewer bundle).

```
cd web && pnpm install && cd ..   # once
make build                        # web bundle + go build ./...
make install                      # install the agni CLI to $GOBIN

agni stats  <file>               # component/section/net counts
agni check  <file>               # structural rule checks (--format json for tooling)
agni diff   <old> <new>          # structural diff between two revisions
agni render <file> -o out.svg    # schematic render (faithful or auto-layout)
agni validate <dir>              # reader-health smoke over a corpus folder
agni serve  web --mount d=path   # browser viewer + Connect API
```

`make testall` runs the full gate (vet, engine tests, example modules, web bundle, web
unit tests); CI runs exactly that.

## Where to go next

- [docs/GETTING_STARTED.md](docs/GETTING_STARTED.md) — build, symbol libraries for
  xschem/gEDA, installing the native EDA tools, golden comparisons.
- [docs/README.md](docs/README.md) — the engineering docs: IR and ingestion
  architecture, geometry and rendering, semantic diff, format primers.
- [examples/README.md](examples/README.md) — runnable walkthroughs, one per feature.
- [CONSTRAINTS.md](CONSTRAINTS.md) — the enforceable architectural rules; read before
  proposing changes.
