# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material stays in gitignored `HANDOFF*.md` notes.

Last pruned 2026-10-03, at `396cfe12` (PR 838).

## Open, ranked

1. **#825, a docsite capture whose command fails renders as an empty block and the gate stays
   green.** Rung 7 shipped this way. Its comment adds the `capture: none` case, which should demand
   exit 0 too. Small, and every later docs or `demofeature` PR leans on captures being right.
2. **#829, the viewer's checklist picker misses checklists a project inherits through `extends`.**
   `agni review` resolves the chain and `ResolveDesign` does not. The issue has two fix shapes.
3. **Three small real bugs: #755 (`CheckService.fallback` is never assigned), #756 (a nil check
   after the call it guards), #758 (KiCad accepts `[hi:lo]` as a bus).** #755 matters most, and
   PR 838's coverage rpc now passes that unassigned fallback like every other rpc.
4. **#564, move the tutorial ladder onto the Jetson board, with #724 (rung 13's total).** Every
   blocker is closed. Decide the entry view (`.kicad_sch` or `.kicad_pcb`) first; the per-rung plan
   is the latest comment on #564. Re-run rung 13 in the CLI and the panel before editing it.
5. **The `demofeature` track, #818 to #823.** #817 is done (PR 832). #818 (unchanged nets in a
   diff) comes before the #822/#823 audit workbook it feeds. Each PR adds a docsite capture.
6. **#702, `PinsByName` compares pin names outside `core/ident`.** Decide first whether
   underscore-folding belongs in `core/ident` for pin names.
7. **#390 PR 1, the server caches check results.** **The key in the issue body is wrong.** Read the
   corrections comment before writing any code.
8. **#356 (the run-wide highlight floods big boards), #485 (review report links built from findings,
   not verdicts), #634 (`agni query` has no viewer link), #736 (`GetLayoutReport` skips
   `TierURIs`).** The Python client's cross-transport test declares #736 field by field, so its fix
   also deletes that declaration in `clients/python/tests/test_cross_transport.py`.

Later work is #717 (show unexpanded EDIF hierarchy in the viewer and reports), #370 then #373, #380,
#456, #374, #716, #739 (read a Datalog-derived relation from Go) and #742 (embed the viewer group,
gzipped, in release binaries). The query language roadmap lives on panyam/jaala, where modules (#3)
and aggregation inside rules (#4) come first, since they retire the self-join and pasted-preamble
workarounds.

#799 (the PartSpec contract module) waits for a second implementer or consumer. The first release
since #744 publishes `agnids` for the first time, and `RELEASING.md`'s checklist now carries the
anonymous-pull check that release owes.

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
- **Two failures in a fresh worktree are not yours.** `TestCheckWebAssets` and, in the `datasheet`
  module, `TestCheckWorkbenchAssets` need a built web bundle, and `TestSampleBoard*` needs the gitignored samples corpus (`make samples`).
- **The two `DesignHash` implementations differ on purpose** (see `OUT_OF_SCOPE.md`). Every caller now
  passes a resolved tier, so unifying them buys symmetry nobody can observe.
