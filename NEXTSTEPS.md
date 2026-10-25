# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material stays in gitignored `HANDOFF*.md` notes.

Last pruned 2026-10-25, at `3b79a527`.

## At a glance

One mission is active. `MISSION=mission_<slug> queue.sh`
(the retriage skill's script) prints one mission's queue, and the order lives in GitHub labels and
blocked-by links rather than here. Log each exercise run on the mission issue.

- **#845 `mission_browser_review`**, 28 of 28. `make exercise-browser-review` last passed at
  `b553cf49`, and two PRs (#1174, #1175) have merged since without it running again,
  though `browser-test` runs the same walk on gateway. It closes when the user agrees.
- **#1157 `mission_viewer_page_api`** closed on 2026-10-25 at its done-when (`make
  exercise-viewer-page`, 8 of 8 at `3b79a527`). The exercise stays as the regression walk.
- **#1176 `mission_shared_viewer_rooms`** (several people on one server, each driving their own tabs),
  0 of 1 and not active. Its ticket #1173 is `waiting` until someone runs agni serve for more than one
  person, and the mission has no exercise yet; its first PR writes one.
- **#909 `mission_ask`** closed on 2026-10-22 at its done-when (Jetson 7 of 7, gateway 9 of 9 at
  `c88fb204`). `make exercise-mcp-ask` remains, and no ticket is left on it.
- **#851 `mission_public_demo`** closed on 2026-10-20. #855, #923 and #949 stay `waiting`, off any
  mission. `make exercise-public-demo` remains.
- **Possible next mission**: moving a house netlist audit off its spreadsheet. #820, #821 and #1093 are
  its tickets, and `guide/older-comparison-tools.md` its first page. Not yet a mission.
- This run: #1175 merged (a tab answers only the room bridge, and an agent hears its Ask box),
  #1157 closed, and #1173 moved to the new mission #1176.

## Open, ranked

1. **Decide the strap band's unreadable value.** A declared band whose resistor's value does not read
   (`DNP`, a typo) passes with "the design does not contradict it", because
   `TestStrapValueUnreadableIsSilent` chose silence over a finding; inconclusive is the alternative.
   #1115 fixed the Jetson case by reading library names. File it once decided.
2. **Jetson's load**, off any mission: #1061's edit-session half. Measure with the exercise's
   `--timing`, `make wasm-bench` and `make wasm-layers` (pass `BOARD` and `DESIGN`, since a shell's own
   `DESIGN` overrides the Makefile's), and read CLAUDE.md on the bench's two misleading rows first.

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

Four items are gated, #677 (only the "declare a NEW class" half, gated on the class registry), #667,
#681 and #523.

## Worth knowing before the next change

- **`reverse-blocking.undecided.kicad_sch` is the only fixture that reaches an inconclusive
  finding.** It is a barrel jack feeding an MCU through a P-FET, which `reverse-blocking-absent` cannot
  decide from a netlist. The browser suite mounts it as `conformance=`.
- **`severitySections` in `web/src/findings.ts` has no renderer, on purpose.** It is the parity oracle
  that pins the client against `GetCheckReport`. "Improving" it would hide the drift it exists to catch.
- **Two failures in a fresh worktree are not yours.** `TestCheckWebAssets` and, in the `datasheet`
  module, `TestCheckWorkbenchAssets` need a built web bundle, and `TestSampleBoard*` needs the gitignored samples corpus (`make samples`, or an ABSOLUTE symlink to the main checkout's `tools/samples`, since a relative one resolves from `tools/`). A worktree
  built under `GOTOOLCHAIN=go1.26.9` carries no VCS stamp, which fails the browser-cache test and rung
  12's capture in `tutorial-runs-check` (#957); the default Go 1.27 stamps a worktree and passes both.
- **The two `DesignHash` implementations differ on purpose** (see `OUT_OF_SCOPE.md`). Every caller now
  passes a resolved tier, so unifying them buys symmetry nobody can observe.
