# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material goes in gitignored `HANDOFF*.md` notes.

Last pruned 2026-11-12, at `495f316c`.

## At a glance

One mission is active. `MISSION=mission_<slug> queue.sh` (the retriage skill's script) prints one
mission's queue, and the order lives in GitHub labels and blocked-by links rather than here. Log each
exercise run on the mission issue.

- **#1499 `mission_board_open`** (open Jetson's board in the browser in under 1.5 s, and reopen it in
  under 0.5 s), 11 of 14 closed. The last exercise, at `a8cda0fe`, has both reloads passing (server
  306 ms, wasm 377 ms) and both cold opens failing (server 2.3 s, wasm 2.2 s). The latest log comment
  on #1499 has the timelines. PRs 1544 to 1568 merged since, for the demo work, so rerun
  `make exercise-board-open RUNS=5` before trusting those numbers.
- Next ready: #1534 (P1, a server's request work lands on an efficiency core on Apple Silicon about
  half the time). #1484 and #1504 are P2.
- **#1176 `mission_shared_viewer_rooms`** is not active, and #1173 is `waiting` until someone runs agni
  serve for more than one person. **#1442 `mission_demo_journeys`** is done and closed.
- This run: v0.6.10 to v0.6.14 released, with the demo mission, the dock's drawers, Jetson revs 1.1.2
  and 1.1.3 to compare, and KiCad 10 reading. #1531, #1503 and #1442 dropped as closed.

## Open, ranked

1. **Jetson's board open**, the #1499 mission above.

Later work is #1007 (a fact store per tier, `waiting`), #1027 (edit sessions, `waiting`), #370 then
#373, #380, #456, #374, #716 and #739 (read a Datalog-derived relation from Go). The query language
roadmap lives on panyam/jaala. Modules (jaala#3) and aggregation inside rules (jaala#4) have both shipped, and agni tracks the latest jaala, so a
derived question goes in `stdlib/lib` before it becomes a Go relation.

#799 (the PartSpec contract module) waits for a second implementer or consumer.

`OUT_OF_SCOPE.md` has five rows waiting on a decision about the ledger itself, not on code: the
`core/svg` move, the `.edn` sniffing row (its WS6-008 ticket was wrong), the WS9-035 row (agni
issue 541 half-settled it), the `Witness.Statement` row that asks for a ticket, and the
`agni version` row whose trigger is "never".

Two items are gated, #677 (only the "declare a NEW class" half, gated on the class registry) and
#681.

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
