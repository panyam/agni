# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material stays in gitignored `HANDOFF*.md` notes.

Last pruned 2026-10-01, at `f2a44ce9` (PR 754).

## Open, ranked

1. **The datasheet workstream: #744, then #749.** The order and the storage design (blob store for
   raw files, an indexed metadata store derived from it and rebuildable, draft and published as states)
   are the latest comments on #744. #744 starts with its dependency constraint (the engine never
   imports `doc`, `derive`, `docindex` or `candidate`), before any move. `agni params promote` (#209's
   CLI half) and the single `check.NewModel` (#748) have landed.
2. **#564, move the tutorial ladder onto the Jetson board.** Every blocker is closed. Start by
   deciding which view is the entry (`.kicad_sch` or `.kicad_pcb`) and which rungs need the big board.
   The per-rung plan is the latest comment on the issue.
3. **#724, restore rung 13's total.** #646 closed, so the CLI and the panel should now agree. Re-run
   both before editing anything. This is small and can ride with #564.
4. **#702, `PinsByName` compares pin names outside `core/ident`.** Decide first whether
   underscore-folding belongs in `core/ident` for pin names.
5. **#390 PR 1, the server caches check results.** Unblocked by PR 723 (`Overlay.Identity()`).
   **The key in the issue body is wrong.** Read the corrections comment before writing any code.
6. **#356, the run-wide highlight floods big boards.** Unblocked since #348.
7. **#485, review report links are built from findings, not verdicts.**
8. **#634, `agni query` has no viewer link and no `--server`.**
9. **Where the CLI and the server disagree: #736, #737, #734.** The Python client's cross-transport
   test declares #736 and #737 field by field and fails once each is fixed, so a fix also deletes its
   declaration in `clients/python/tests/test_cross_transport.py`. #734 (review json onto the `Review`
   proto) is what lets the Python CLI transport map `review`.

Later work is #717 (show unexpanded EDIF hierarchy in the viewer and reports), #370 then #373, #380, #456,
#374, #716, #739 (read a Datalog-derived relation from Go) and #742 (embed the viewer group, gzipped,
in release binaries). The query language roadmap lives on
panyam/jaala, where modules (#3) and aggregation inside rules (#4) come first, since they retire the
self-join and pasted-preamble workarounds.

Issues #755 to #769 came out of reading every comment for PR #754. Three look like real bugs and are
small: #755 (`CheckService.fallback` is never assigned), #756 (a nil check after the call it
guards) and #758 (KiCad accepts `[hi:lo]` as a bus). #757 closed with #748.
The rest are strings, proto comments, examples and rule semantics.

`OUT_OF_SCOPE.md` has five rows waiting on a decision about the ledger itself, not on code: the
`core/svg` move, the `.edn` sniffing row (its WS6-008 ticket was wrong), the WS9-035 row that agni
issue 541 half-settled, the `Witness.Statement` row that asks for a ticket, and the `agni version`
row whose trigger is "never".

Issue #718 (EDIF descends into sub-cells) is waiting on input. The only hierarchical netlist anywhere is the
synthetic `readers/edif/testdata/hier.edn`. Do not start it until there is a real multi-cell netlist
to test against.

Four items are gated, #677 (only the "declare a NEW class" half, gated on the class registry), #667,
#681 and #523.

## Worth knowing before the next change

- **`reverse-blocking.undecided.kicad_sch` is the only fixture that reaches an inconclusive
  finding.** It is a barrel jack feeding an MCU through a P-FET, which `reverse-blocking-absent` cannot
  decide from a netlist. The browser suite mounts it as `conformance=`.
- **`severitySections` in `web/src/findings.ts` has no renderer, on purpose.** It is the parity oracle
  that pins the client against `GetCheckReport`. "Improving" it would hide the drift it exists to catch.
- **Two failures in a fresh worktree are not yours.** `TestCheckWebAssets` and `TestCheckDatasheetAssets`
  need a built web bundle, and `TestSampleBoard*` needs the gitignored samples corpus (`make samples`).
- **The two `DesignHash` implementations differ on purpose** (see `OUT_OF_SCOPE.md`). Every caller now
  passes a resolved tier, so unifying them buys symmetry nobody can observe.
