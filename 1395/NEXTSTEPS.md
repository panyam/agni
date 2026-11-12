# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material goes in gitignored `HANDOFF*.md` notes.

Last pruned 2026-11-05, at `243f13a6`.

## At a glance

Four missions are active. `MISSION=mission_<slug> queue.sh` (the retriage skill's script) prints one
mission's queue, and the order lives in GitHub labels and blocked-by links rather than here. Log each
exercise run on the mission issue.

- **#1321 `mission_cross_probe`** (find a schematic part on the board beside you, and back), 10 of 12
  tickets closed. `make exercise-cross-probe HEADED=0 WALK_DESIGN=jetson` works at `781731a7`, every
  step passing on Jetson, with the two board clicks at 12 to 17 s (#1329's ground). #1367 (add a
  board to an open design) is ready next. #1382 walks the mission on Clay R6, an OrCAD EDIF design on
  its Allegro board, and waits on #1388 (Allegro 16.6 boards). The mission still has to pass beside a
  real board.
- **#1353 `mission_board_formats`** (open an Altium or Allegro board with no export step), 4 of 5
  closed. agni reads Allegro 17.2 boards (#1365), holds their copper to Allegro's films (#1380), reads
  Altium `.PcbDoc` (#1349) and Allegro's ODB++X export (#1379). #1388, Allegro 16.6, is next, and it
  also unblocks #1382. `make exercise-allegro` is the Allegro half. #1383 holds the IPC-2581 board
  read's missing widths, pad nets and drills, which make parasdr's two exports `check` differently.
- **#1264** (see, explain and change how agni reads your board's names), 11 of 11 tickets closed,
  exercise last run at `014819c5` (works on the MNT Reform motherboard). Nothing is ready. It closes
  once an audit confirms that the lexicon holds every built-in rule's name heuristics, and once the
  exercise's output is right by eye. If the audit finds a heuristic outside the lexicon, that
  heuristic becomes a ticket here.
- **#1194 `mission_pin_functions`** (sign off a board's pin map against the netlist and the
  datasheet), 8 of 8 tickets closed, exercise last run at `aba62b48` (works on the fixtures), with
  many PRs merged since, so re-run `make exercise-pin-functions` before a team starts. It waits on a
  team's clean-room run on their own board. The team runs the three commands in
  `guide/trying-agni.md`'s "Signing off a pin map in a clean room" and sends back the `agni trial`
  report. Their findings become its next tickets. #1198 left one choice open. Should
  `service.ShippedRules` name the fixed `intent/io-map-*` rules? The trial now counts them under
  project rules.
- **#1176 `mission_shared_viewer_rooms`** (several people on one server, each driving their own tabs),
  0 of 1 and not active. #1173 holds the multi-user half and is `waiting` until someone runs agni
  serve for more than one person. The mission has no exercise yet; its first PR writes one.
- **Candidate**: moving a house netlist audit off its spreadsheet. #1093 is its open ticket, and
  `guide/older-comparison-tools.md` its first page. Its reproducer still names its rail `RAIL_3V3`,
  and the default vocabulary does not read that name.
- **Candidate**: more tours of the viewer. Each would tour another workflow the way
  `web/src/reviewtour.ts` tours a review. Work on each starts with the viewer events, since the
  tour advances on them.
- This run: ODB++X read as ODB++ and `agni.yaml`'s relative paths anchored beside the file (#1379,
  samples pin v0.1.7), and #1344's orientation and KiCad-text suspects checked against Lepton,
  kicad-cli and the EDIF spec (#1390). #1389 holds the rest of #1344: schematic text about 28%
  small, the worksheet frame, and an EDIF field angle waiting on a real EDIF export.

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
