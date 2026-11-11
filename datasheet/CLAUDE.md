# datasheet

This file covers the datasheet tooling, the workbench's drafts, the corpus index, and how the engine reaches a corpus.

- **`datasheet/` is a Go module of its own**, `github.com/panyam/agni/datasheet` (agni 744). It holds `doc`, `derive`, `docindex`, `candidate`, the transport-neutral `dsservice`, its Connect adapter `dsserver`, and the `agnids` binary in `datasheet/cmd/agnids`. It depends on the root and never the reverse (C34, C18). Its tests run in `make datasheet-test`, since `go test ./...` from the root never reaches a nested module.

## Tooling

- `make setup` builds the docling venv for the datasheet tooling. Then `make pdf2doc`, `make pdf2doc-all`, `make datasheets-status` and `make dsserve` work over `DATASHEET_DIR`.
- The tools live in `datasheet/tools/`, so those targets `cd datasheet` first.
- The venv is found by lookup, first a repo-local `.venv` and then the parent directory's. Worktrees sharing a root share one env, so each does not carry gigabytes of torch.

## Reading a spec

- **`agni params <mpn>` prints the RECORD that a query answer cannot reach.** It needs no design, because a spec library is not one.
- The datalog relations carry what a query can BIND. `agni params` reads the rest off the `PartSpec`. That covers the conditions a value holds under, the pin bindings, the full provenance and the verification state.
- `--params <dir>` names a corpus. `--design <path>` lets a design's PROJECT supply one, layered over `--params` per MPN through `Overlay.SpecsOver`. `--format json` emits the bare `PartSpec`.
- **Staleness is decided on the content hash and NEVER on the printed revision.** Say someone verified a parameter, and the corpus then moved to a later revision. The parameter reports `stale` and names BOTH revisions, and the two strings are for the reader. `DECISIONS.md` records this under "A document revision is recorded for the reader, and never compared".

## Drafts and publishing

- **A workbench draft is a DRAFT that no check reads.** It is saved unvalidated on purpose, keyed by MPN and citing its datasheets, at `<corpus>/drafts/<MPN>.draft.json` under `agnids serve --corpus`.
- `PublishDraft` or `agnids publish <mpn> --corpus <dir>` validates a draft into `<mpn>.textproto` (agni 749).
- **Never read drafts in `LoadSet`.** A half-finished transcription would fail the whole all-or-nothing load.
- The workbench saves nothing until an MPN is confirmed.
- **Publishing and the corpus index belong to the datasheet service** (`datasheet/corpus`). Publishing writes `corpus.index.json` (MPN, file, hash, a generation) beside the specs.
- `agnids index [--check]` rebuilds or verifies the index, and `agnids serve --corpus` refreshes it at start.
- All of them build from `param.LoadCorpus`. `LoadSet` makes the same walk, so the index and every eager load agree.
- The files are the source of truth, and no check reads the index.

## A shared corpus

- **The engine reaches a SHARED corpus only through the contract's `PartSpecService`** (`protos/agni/v1/param/service.proto`).
- `agnids serve --corpus` answers it from the index. `agni serve --params-url` reads it through `param.Remote`.
- `Lookup` returns no error, so a remote provider is a `param.Prefetcher`. `service.BuildModel` fetches the design's MPNs through `Prefetch`, and an unreachable corpus then returns `ErrUnavailable` rather than reading every part as unseeded.
- A new model-building path that skips `Prefetch` gets no datasheet tier.
