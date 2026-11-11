# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material goes in gitignored `HANDOFF*.md` notes.

Last pruned 2026-11-10, at `9783fb9d`.

## At a glance

One mission is active. `MISSION=mission_<slug> queue.sh` (the retriage skill's script) prints one
mission's queue, and the order lives in GitHub labels and blocked-by links rather than here. Log each
exercise run on the mission issue.

- **#1499 `mission_board_open`** (open Jetson's board in the browser in under 1.5 s, and reopen it in
  under 0.5 s). Next ready is #1500, which writes `make exercise-board-open`. #1501 (draw the board
  before the check model) and #1502 (cache the packed board, send it as binary) follow at P1. #1484,
  #1503 and #1504 are P2. A profile at `9783fb9d` measured a cold open at about 4.0 s in wasm and 1.9 s
  against a server. #1499's body gives the stages.
- **#1321 `mission_cross_probe`** and **#1353 `mission_board_formats`** closed on 2026-11-10. Each
  reopens if a run on a real board finds something. #1353's `waiting` follow-ups keep its label.
- **#1329 `mission_webgl`** closes with #1505, which opens a page in WebGL on a hardware GPU.
- **#1176 `mission_shared_viewer_rooms`**, 0 of 1 and not active. #1173 is `waiting` until someone
  runs agni serve for more than one person.
- **#1442 `mission_demo_journeys`** has no tickets yet. Its first PR adds `make demo-video`.
- **Candidate**: moving a house netlist audit off its spreadsheet. #1093 is its open ticket, and
  `guide/older-comparison-tools.md` its first page.
- This run: a retriage on 2026-11-10 closed 21 issues, filed #1497 and #1498 for their leftovers,
  and moved 13 labels. Each change has a comment on its issue.

## Open, ranked

1. **Jetson's board open**, the #1499 mission above. #1061's load-once work closed, and its edit-session
   half moved to #1027.

Later work is #1007 (a fact store per tier, `waiting`), #1027 (edit sessions, `waiting`), #370 then
#373, #380, #456, #374, #716 and #739 (read a Datalog-derived relation from Go). The query language
roadmap lives on panyam/jaala. Modules (jaala#3) and aggregation inside rules (jaala#4) have both shipped, and agni tracks the latest jaala, so a
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
