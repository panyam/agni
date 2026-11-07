# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material goes in gitignored `HANDOFF*.md` notes.

Last pruned 2026-11-07, at `e6914c1d`.

## At a glance

Three missions are active, each worked in its own clone. `MISSION=mission_<slug> queue.sh` (the
retriage skill's script) prints one mission's queue, and the order lives in GitHub labels and
blocked-by links rather than here. Log each exercise run on the mission issue.

- **#1321 `mission_cross_probe`** (find a schematic part on the board beside you, and back), 18 of
  20 tickets closed. Its exercise last passed at `19aeed5e`, and five PRs have merged since, so run
  `make exercise-cross-probe HEADED=0` (and with `WALK_DESIGN=clay`) before picking #1397 (the
  toolbar overflow and Jetson's 11 to 16 s board clicks), then #1401. The mission still has to pass
  beside a real board.
- **#1329 `mission_webgl`** (pan, zoom and click Jetson's whole board without lag), worked in the
  webgl clone, 1 of 11 closed. #1439 wrote `make exercise-webgl` (#1330), and nobody has logged a
  run of it on #1329 yet, so run it first. #1332 (click to pick on the WebGL canvas) is next ready.
- **#1353 `mission_board_formats`** (open an Altium or Allegro board with no export step), worked in
  the formats clone, 15 of 15 closed. `make exercise-allegro` and `make exercise-altium` pass at
  `67fbcf2f` and now cover all three steps, the viewer's pad click included. Its "Done when" line
  still wants someone to open a real board of their own in each format. Whether to close the mission before
  that run is the user's call. #1355, #1362, #1393, #1430 and #1437 are its `waiting` follow-ups.
- **#1194 `mission_pin_functions`** and **#1337 `mission_walk`** closed on 2026-11-06 with every
  ticket done and their exercises passing. Each still wants a person's run on a real board, and
  either reopens if that run finds something.
- **#1176 `mission_shared_viewer_rooms`**, 0 of 1 and not active. #1173 is `waiting` until someone
  runs agni serve for more than one person.
- **Candidate**: moving a house netlist audit off its spreadsheet. #1093 is its open ticket, and
  `guide/older-comparison-tools.md` its first page.
- **Candidate**: more tours of the viewer, each touring another workflow the way
  `web/src/reviewtour.ts` tours a review.
- This run: #1438 merged and closed #1383 and #1360. parasdr's exports now read the copper its
  `.brd` reads, an Allegro board's declared pairs and match groups reach `net.group` and
  `diff-pair-naming`, and the mission's exercise clicks a pad on each native board (CLAUDE.md, the ODX
  and Allegro bullets).

## Open, ranked

1. **Jetson's load**, off any mission: #1061's edit-session half. Measure with the exercise's
   `--timing`, `make wasm-bench` and `make wasm-layers` (pass `BOARD` and `DESIGN`, since a shell's own
   `DESIGN` overrides the Makefile's), and read CLAUDE.md on the bench's two misleading rows first.

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
