# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material goes in gitignored `HANDOFF*.md` notes.

Last pruned 2026-11-05, at `b0340480`.

## At a glance

Three missions are active. `MISSION=mission_<slug> queue.sh` (the retriage skill's script) prints one
mission's queue, and the order lives in GitHub labels and blocked-by links rather than here. Log each
exercise run on the mission issue.

- **#1321 `mission_cross_probe`** (find a schematic part on the board beside you, and back), 15 of 18
  tickets closed. `make exercise-cross-probe HEADED=0` passes every step at `b0340480` with
  `WALK_DESIGN=jetson` and with `WALK_DESIGN=clay`, Clay R6's Capture `.DSN` on its Allegro board
  (#1418), and the gate walks the cut and Clay. #1397 (the toolbar overflow and Jetson's 9 to 13 s
  board clicks) is next, then #1406 (OrCAD net names that match the board's) and #1401. #1419 is
  waiting. The mission still has to pass beside a real board.
- **#1353 `mission_board_formats`** (open an Altium or Allegro board with no export step), 9 of 10
  closed, released in v0.5.0. Its closing run at `1b7426a4` passed on every sample board, and
  agni-samples' `make serve` mounts `allegro/` and `altium/` (agni-samples #20). It closes once
  someone opens a real board of their own in each format. #1355 holds the Allegro releases not read
  yet: 17.4, 17.5, and pc069's magic `0x00150200`, which reads as "not an Allegro board".
- **#1194 `mission_pin_functions`** (sign off a board's pin map against the netlist and the
  datasheet), 8 of 8 tickets closed, exercise last run at `aba62b48`, with many PRs merged since, so
  re-run `make exercise-pin-functions` before a team starts. It waits on a team's clean-room run on
  their own board (`guide/trying-agni.md`, "Signing off a pin map in a clean room").
- **#1337 `mission_walk`** (click one part and walk outwards), not active, 3 of 4 closed.
  `make exercise-walk HEADED=0 WALK_DESIGN=jetson` passes every row at `76979309` but the goal steps,
  which wait on #1340, the next ticket. Its last log notes the zoom to a small start is tight.
- **#1176 `mission_shared_viewer_rooms`**, 0 of 1 and not active. #1173 is `waiting` until someone
  runs agni serve for more than one person.
- **Candidate**: moving a house netlist audit off its spreadsheet. #1093 is its open ticket, and
  `guide/older-comparison-tools.md` its first page.
- **Candidate**: more tours of the viewer, each touring another workflow the way
  `web/src/reviewtour.ts` tours a review.
- This run: released v0.6.0 (the OrCAD Capture reader, #1405 and #1412, board layers, padstacks and
  pours) and agni-samples v0.1.12 and v0.1.13, whose Clay R6 now reads from `CLAYR6.DSN` with its
  board. #1388, #1399, #1400 and #1382 closed. KiCad 26003 asks KiCad's OrCAD importer to follow a
  block kept in another `.DSN`.

## Open, ranked

1. **Jetson's load**, off any mission: #1061's edit-session half. Measure with the exercise's
   `--timing`, `make wasm-bench` and `make wasm-layers` (pass `BOARD` and `DESIGN`, since a shell's own
   `DESIGN` overrides the Makefile's), and read CLAUDE.md on the bench's two misleading rows first.
2. **#1180**. A Shift-drag that starts under the canvas control bar selects nothing. The review
   tour's select-a-block step now sends every reader into that case.

Later work is #1007 (a fact store per tier, `waiting`), #1027 (edit sessions, `waiting`, after #1061), #370 then #373, #380,
#456, #374, #716, #739 (read a Datalog-derived relation from Go) and #742 (embed the viewer group,
gzipped, in release binaries). The query language roadmap lives on panyam/jaala. Modules (jaala#3)
and aggregation inside rules (jaala#4) have both shipped, and agni tracks the latest jaala, so a
derived question goes in `stdlib/lib` before it becomes a Go relation.

#799 (the PartSpec contract module) waits for a second implementer or consumer. The first release
since #744 publishes `agnids` for the first time, and `RELEASING.md`'s checklist now carries the
anonymous-pull check for that release.

`OUT_OF_SCOPE.md` has five rows waiting on a decision about the ledger itself, not on code: the
`core/svg` move, the `.edn` sniffing row (its WS6-008 ticket was wrong), the WS9-035 row (agni
issue 541 half-settled it), the `Witness.Statement` row that asks for a ticket, and the
`agni version` row whose trigger is "never".

Three items are gated, #677 (only the "declare a NEW class" half, gated on the class registry), #681
and #523.

## Worth knowing before the next change

- **`reverse-blocking.undecided.kicad_sch` is the only fixture that reaches an inconclusive
  finding.** It is a barrel jack feeding an MCU through a P-FET, and `reverse-blocking-absent` cannot
  decide that circuit from a netlist. The browser suite mounts it as `conformance=`.
- **`severitySections` in `web/src/findings.ts` has no renderer, on purpose.** It is a parity oracle.
  It pins the client against `GetCheckReport`. It exists to catch drift, and "improving" it would
  hide that drift.
- **Two failures in a fresh worktree are not yours.** `TestCheckWebAssets` and, in the `datasheet`
  module, `TestCheckWorkbenchAssets` need a built web bundle, and `TestSampleBoard*` needs the gitignored samples corpus (`make samples`, or an ABSOLUTE symlink to the main checkout's `tools/samples`, since a relative one resolves from `tools/`). A worktree
  built under `GOTOOLCHAIN=go1.26.9` carries no VCS stamp, which fails the browser-cache test and rung
  12's capture in `tutorial-runs-check` (#957); the default Go 1.27 stamps a worktree and passes both.
- **The two `DesignHash` implementations differ on purpose** (see `OUT_OF_SCOPE.md`). Every caller now
  passes a resolved tier, so unifying them would change nothing a caller can observe.
