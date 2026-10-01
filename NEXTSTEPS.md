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

1. **#564, move the tutorial ladder onto the Jetson board.** Every blocker is closed. Start by
   deciding which view is the entry (`.kicad_sch` or `.kicad_pcb`) and which rungs need the big board.
   The per-rung plan is the latest comment on the issue.
2. **#724, restore rung 13's total.** #646 closed, so the CLI and the panel should now agree. Re-run
   both before editing anything. This is small and can ride with #564.
3. **#702, `PinsByName` compares pin names outside `core/ident`.** Decide first whether
   underscore-folding belongs in `core/ident` for pin names.
4. **#390 PR 1, the server caches check results.** Unblocked by PR 723 (`Overlay.Identity()`).
   **The key in the issue body is wrong.** Read the corrections comment before writing any code.
5. **#356, the run-wide highlight floods big boards.** Unblocked since #348.
6. **#485, review report links are built from findings, not verdicts.**
7. **#634, `agni query` has no viewer link and no `--server`.**

Later work is #717 (show unexpanded EDIF hierarchy in the viewer and reports), #370 then #373, #380, #456,
#374, #716 and #739 (read a Datalog-derived relation from Go). The query language roadmap lives on
panyam/jaala, where modules (#3) and aggregation inside rules (#4) come first, since they retire the
self-join and pasted-preamble workarounds.

Issues #755 to #769 came out of reading every comment for PR #754. Four look like real bugs and are
small: #755 (`CheckService.fallback` is never assigned), #756 (a nil check after the call it
guards), #757 (`TraceDesign` uses `check.NewModel`) and #758 (KiCad accepts `[hi:lo]` as a bus).
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
- **Two failures in a fresh worktree are not yours.** `TestCheckWebAssets` needs a built web bundle, and
  `TestSampleBoard*` needs the gitignored samples corpus (`make samples`).
- **The two `DesignHash` implementations differ on purpose** (see `OUT_OF_SCOPE.md`). Every caller now
  passes a resolved tier, so unifying them buys symmetry nobody can observe.
