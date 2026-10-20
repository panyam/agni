# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material stays in gitignored `HANDOFF*.md` notes.

Last pruned 2026-10-20, at `986b09b0` (PR 1110).

## At a glance

Two missions are active, one per worktree, and a third is filed. `MISSION=mission_<slug> queue.sh`
(the retriage skill's script) prints one mission's queue, and the order lives in GitHub labels and
blocked-by links rather than here. Log each exercise run on the mission issue.

- **#851 `mission_public_demo`**, 67 of 74, at its done-when on the deployed page (walk of 2026-10-20 at
  v0.3.2: 94 ok, none failed). Left are the seed false positives #1014, #1015, #1016 and #1009, then
  closing it. #855 (a public server) is parked until a seed outgrows the browser.
- **Possible next mission**: moving a house netlist audit off its spreadsheet. #820, #821 and #1093 are
  its tickets, and `guide/older-comparison-tools.md` its first page. Not yet a mission.
- **#845 `mission_browser_review`**, 16 of 19, three P3s (#759, #774, #904) and a checklist for one
  sample board.
- **#909 `mission_ask`**, filed, not active. #910 (P1, MCP on `agni serve`) is its first ticket.
- This run: retriage after PRs 1108 and 1110 (#1082, #1097, #1098, #1005 closed). #855 and #866 parked; #1061, #924, #877, #922, #957, #1107 moved off #851; #1089 to #1096 folded into #820, #821 and #1093 or closed as supported.

## Open, ranked

1. **The seed false positives on `mission_public_demo`**, #1014 (a PTVS part read as a plain diode),
   #1015 (bulk-cap stops at a 0 Ohm link) and #1016 (a powered connector read as a supply entry), then
   #1009 and closing #851.
2. **Jetson's load**, now off the mission: #1061's edit-session half. Measure with the exercise's
   `--timing`, `make wasm-bench` and `make wasm-layers` (pass `BOARD` and `DESIGN`, since a shell's own
   `DESIGN` overrides the Makefile's), and read CLAUDE.md on the bench's two misleading rows first.
3. **#605, KiCad accepts `[hi:lo]` as a bus**, off any mission. It absorbed #758.
4. **#702, `PinsByName` compares pin names outside `core/ident`.** Decide first whether
   underscore-folding belongs in `core/ident` for pin names.
5. **#910 on `mission_ask`**, MCP tools over the existing services. Every service is transport-neutral,
   so it is an adapter, not new analysis.
6. **#845's three P3s** (#759, #774, #904), and a checklist for one sample board.

Later work is #1007 (a fact store per tier, `waiting`), #1027 (edit sessions, `waiting`, after #1061), #370 then #373, #380,
#456, #374, #716, #739 (read a Datalog-derived relation from Go) and #742 (embed the viewer group,
gzipped, in release binaries). The query language roadmap lives on panyam/jaala. Modules (jaala#3)
and aggregation inside rules (jaala#4) have both landed, and agni tracks the latest jaala, so a
derived question goes in `stdlib/lib` before it becomes a Go relation.

#799 (the PartSpec contract module) waits for a second implementer or consumer. The first release
since #744 publishes `agnids` for the first time, and `RELEASING.md`'s checklist now carries the
anonymous-pull check that release owes.

`OUT_OF_SCOPE.md` has five rows waiting on a decision about the ledger itself, not on code: the
`core/svg` move, the `.edn` sniffing row (its WS6-008 ticket was wrong), the WS9-035 row that agni
issue 541 half-settled, the `Witness.Statement` row that asks for a ticket, and the `agni version`
row whose trigger is "never".

Issue #718 (EDIF descends into sub-cells) is waiting on input. The only hierarchical netlist anywhere is the
synthetic `readers/edif/testdata/hier.edn`. Do not start it until there is a real multi-cell netlist
to test against. #893 was a duplicate; its extra points are a comment on #718.

Four items are gated, #677 (only the "declare a NEW class" half, gated on the class registry), #667,
#681 and #523.

## Worth knowing before the next change

- **`reverse-blocking.undecided.kicad_sch` is the only fixture that reaches an inconclusive
  finding.** It is a barrel jack feeding an MCU through a P-FET, which `reverse-blocking-absent` cannot
  decide from a netlist. The browser suite mounts it as `conformance=`.
- **`severitySections` in `web/src/findings.ts` has no renderer, on purpose.** It is the parity oracle
  that pins the client against `GetCheckReport`. "Improving" it would hide the drift it exists to catch.
- **Two failures in a fresh worktree are not yours.** `TestCheckWebAssets` and, in the `datasheet`
  module, `TestCheckWorkbenchAssets` need a built web bundle, and `TestSampleBoard*` needs the gitignored samples corpus (`make samples`). A worktree
  built under `GOTOOLCHAIN=go1.26.4` carries no VCS stamp, which fails the browser-cache test and rung
  12's capture in `tutorial-runs-check` (#957); the default Go 1.27 stamps a worktree and passes both.
- **The two `DesignHash` implementations differ on purpose** (see `OUT_OF_SCOPE.md`). Every caller now
  passes a resolved tier, so unifying them buys symmetry nobody can observe.
