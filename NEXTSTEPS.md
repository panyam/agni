# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer: the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material stays in gitignored `HANDOFF*.md` notes.

Last pruned 2026-09-30, at `e04a731d` (PR 741).

## Open, ranked

1. **The datasheet workstream: #747, then #748, then #744, then #749.** The order and the storage
   design (blob store for raw files, an indexed metadata store derived from it and rebuildable) are the
   latest comment on #744. Start with #747: `LoadSet` reads only `.textproto`, so every PartSpec the
   workbench saves as `.partspec.json` is invisible to checks. #748 deletes `NewModelWithBoard` and
   `NewModelWithParams` outright, with no compatibility window. #744 builds on PR 743's asset groups.
2. **#729: named query sets answered over one read.** Unblocked by #731, which moved the engine to
   `panyam/jaala`. Its engine half (N queries over one fact base) belongs in jaala, the file format,
   `agni query --set`, the wire message and the example here. See the sequencing comment on the issue.
3. **#564: move the tutorial ladder onto the Jetson board.** Every blocker is closed. Start by
   deciding which view is the entry (`.kicad_sch` or `.kicad_pcb`) and which rungs need the big board.
   The per-rung plan is the latest comment on the issue.
4. **#724: restore rung 13's total.** #646 closed, so the CLI and the panel should now agree. Re-run
   both before editing anything. This is small and can ride with #564.
5. **#702: `PinsByName` compares pin names outside `core/ident`.** Decide first whether
   underscore-folding belongs in `core/ident` for pin names.
6. **#390, PR 1: the server caches check results.** Unblocked by PR 723 (`Overlay.Identity()`).
   **The key in the issue body is wrong.** Read the corrections comment before writing any code.
7. **#356: the run-wide highlight floods big boards.** Unblocked since #348.
8. **#485: review report links are built from findings, not verdicts.**
9. **#634: `agni query` has no viewer link and no `--server`.**
10. **Where the CLI and the server disagree: #736, #737, #734.** The Python client's cross-transport
    test declares #736 and #737 field by field and fails once each is fixed, so a fix also deletes its
    declaration in `clients/python/tests/test_cross_transport.py`. #734 (review json onto the `Review`
    proto) is what lets the Python CLI transport map `review`.

**Later:** #717 (show unexpanded EDIF hierarchy in the viewer and reports), #370 then #373, #380, #456,
#374, #716, #739 (read a Datalog-derived relation from Go), #742 (embed the viewer group, gzipped,
in release binaries). The query language roadmap lives on
panyam/jaala: modules (#3) and aggregation inside rules (#4) come first, since they retire the
self-join and pasted-preamble workarounds.

**Waiting on input:** #718 (EDIF descends into sub-cells). The only hierarchical netlist anywhere is the
synthetic `readers/edif/testdata/hier.edn`. Do not start it until there is a real multi-cell netlist
to test against.

**Gated:** #677 (only the "declare a NEW class" half, gated on the class registry), #667, #681, #523.

## Worth knowing before the next change

- **`reverse-blocking.undecided.kicad_sch` is the only fixture that reaches an inconclusive
  finding.** It is a barrel jack feeding an MCU through a P-FET, which `reverse-blocking-absent` cannot
  decide from a netlist. The browser suite mounts it as `conformance=`.
- **`severitySections` in `web/src/findings.ts` has no renderer, on purpose.** It is the parity oracle
  that pins the client against `GetCheckReport`. "Improving" it would hide the drift it exists to catch.
- **Two failures in a fresh worktree are not yours.** `TestCheckWebAssets` (and, after PR 743,
  `TestCheckDatasheetAssets`) needs a built web bundle, and `TestSampleBoard*` needs the gitignored
  samples corpus (`make samples`).
- **The two `DesignHash` implementations differ on purpose** (see `OUT_OF_SCOPE.md`). Every caller now
  passes a resolved tier, so unifying them buys symmetry nobody can observe.
