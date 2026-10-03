# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material stays in gitignored `HANDOFF*.md` notes.

Last pruned 2026-10-03, at `016973d3` (PR 850).

## At a glance

Four missions are active, one per worktree. `MISSION=mission_<slug> queue.sh` (the retriage
skill's script) prints one mission's queue, and the order lives in GitHub labels and blocked-by
links rather than here. Log each exercise run on the mission issue.

- **#843 `mission_revision_audit`**, 3 of 9 tickets closed. Exercise `make exercise-revision-audit`
  last ran at `e5fa07df`, and PRs 841 and 850 have merged since: the query and findings tabs build,
  and Diff, Review and Verdicts are listed as missing until #822. Next ready is #822, #825 and #848,
  all P1.
- **#844 `mission_real_board_tutorial`**, 0 of 4. Next is #564. The exercise has not run yet.
- **#845 `mission_browser_review`**, 0 of 11. Next is #829. The exercise is a manual walk with no
  logged run yet.
- **#851 `mission_public_demo`**, active in the `docs` clone. The engine runs as wasm in
  the browser for a dropped KiCad or EDIF design, and a read-only server answers seeded boards too
  large for it. 0 of 9; start with #178 (revive the parked wasm build, #567), which unblocks #852.

## Open, ranked

1. **#848, declare a design's revisions each with its own board.** Found by the revision-audit
   exercise: `gateway-rev-b.edn` is an undeclared sibling, so it reads with no board and 13 of rev
   A's 28 findings (copper-clearance, track-width) silently do not run. Needs a
   `gateway-rev-b.kicad_pcb` fixture carrying R4 and R5, which restamps the tutorial captures.
2. **#822, the `agni.tables` diff, review and verdict sheets.** #818 landed (PR 841), so the diff
   sheet can use the `equal` kind. Update the Diff line in `revision_audit.py`'s `plan()` from
   `missing` to the real builder, which still cites #818.
3. **#825, a docsite capture whose command fails renders as an empty block and the gate stays
   green.** Its comment adds the `capture: none` case. #823's checked summary capture leans on it.
4. **#829, the viewer's checklist picker misses checklists a project inherits through `extends`.**
   `agni review` resolves the chain and `ResolveDesign` does not. The issue has two fix shapes.
5. **#564, move the tutorial ladder onto the Jetson board, with #724 (rung 13's total).** Decide
   the entry view (`.kicad_sch` or `.kicad_pcb`) first; the per-rung plan is the latest comment on
   #564. Re-run rung 13 in the CLI and the panel before editing it.
6. **#605, KiCad accepts `[hi:lo]` as a bus**, off any mission. It absorbed #758.
7. **#702, `PinsByName` compares pin names outside `core/ident`.** Decide first whether
   underscore-folding belongs in `core/ident` for pin names.
8. **#390 PR 1, the server caches check results.** **The key in the issue body is wrong.** Read the
   corrections comment before writing any code.
9. **#356, #485, #634, #736**, on `mission_browser_review`. The Python client's cross-transport
   test declares #736 field by field, so its fix also deletes that declaration in
   `clients/python/tests/test_cross_transport.py`.

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
