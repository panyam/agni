# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material goes in gitignored `HANDOFF*.md` notes.

Last pruned 2026-11-05, at `344c03c3`.

## At a glance

Four missions are active. `MISSION=mission_<slug> queue.sh` (the retriage skill's script) prints one
mission's queue, and the order lives in GitHub labels and blocked-by links rather than here. Log each
exercise run on the mission issue.

- **#1321 `mission_cross_probe`** (find a schematic part on the board beside you, and back), 7 of 12
  closed. `make exercise-cross-probe` last ran at `be1510d0` and passes four of its five steps on
  Jetson. Step 3 (turn the board over) waits on #1327, which another session has in PR #1378.
  #1382 needs a decision first. The only public EDIF design with its board is Clay R6, whose board
  is an Allegro `.brd` rather than ODB++. Either reword the mission's done-when to "an EDIF design on
  its Cadence board", or wait for an ODB++ export. Leaning to the first, which waits on #1350.
- **#1194 `mission_pin_functions`**, 7 of 8 closed. #1342 is its last ticket, on branch
  `fix/trial-site-pinmap-pin-1342` with #1374, and its fix makes `agni trial` count KiCad designs.
  After it merges, re-run `make exercise-pin-functions`. The mission then waits on a team's
  clean-room run (`guide/trying-agni.md`), whose `agni trial` report becomes its next tickets.
- **#1353** (open an Altium or Allegro board with no export step), 2 of 4 closed. #1349 (`.PcbDoc`)
  and #1350 (`.brd`) are its readers. agni-samples now carries Allegro boards and Cadence exports to
  check them against.
- **#1264** (how agni reads your board's names), 11 of 11 closed. It closes once an audit confirms the
  lexicon holds every built-in rule's name heuristics, and once its exercise's output is right by eye.
- **#1176 `mission_shared_viewer_rooms`**, not active. #1173 is `waiting` on a multi-user deployment.
- **Candidates**, not missions yet: moving a house netlist audit off its spreadsheet (#1093), and more
  viewer tours in the shape of `web/src/reviewtour.ts`.

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
