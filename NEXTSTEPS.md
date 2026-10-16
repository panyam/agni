# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material stays in gitignored `HANDOFF*.md` notes.

Last pruned 2026-10-15, at `4433e6de` (PR 1029).

## At a glance

Three missions are active, one per worktree, and a fourth is filed. `MISSION=mission_<slug> queue.sh` (the retriage
skill's script) prints one mission's queue, and the order lives in GitHub labels and blocked-by
links rather than here. Log each exercise run on the mission issue.

- **#844 `mission_real_board_tutorial`**, 7 of 7. #564 moved the ladder onto Jetson, and #116 was
  parked as `waiting` (Jetson's own schematic already carries `review --render`'s images). The
  exercise passed at `4433e6de`, so the mission closes once someone reads the rungs end to end by eye.
- **#845 `mission_browser_review`**, 4 of 16. The exercise is a manual walk with no logged run yet.
  #902 (a Properties panel and query tabs) is the viewer's next shape.
- **#851 `mission_public_demo`**, 44 of 64. #1002 (P1) is the only P1, and it needs clean deployed
  runs rather than code: two deployed walks at `4433e6de` opened every seed with the one failure
  #975, so 3 of the 10 clean runs #1002 asks for. Run checks on Jetson took 5.9 s at `18cfe8f6` and
  8.8 and 16.8 s at `4433e6de`, but the engine's share stayed near 6.4 s and the rest waited on a
  slow board fetch, so one walk is weak evidence for a timing claim. The jobs worker peaks at
  533 MB on a first visit and 419 MB on a reload (`make wasm-bench`, from 1.36 and 1.6 GB before PR
  1001). #1010 is the one regression the `18cfe8f6` walk showed (coverage and the first query rule
  about 750 ms slower each).
- **Seed triage on #851** (`hack/seed-triage/`, closed #857): no seed record holds a false positive.
  Two follow-ups are filed: #961 (the rules besides the pull-up walks that still credit a DNP part)
  and #967 (check the records against KiCad's ERC and DRC). #967's first run already disagrees:
  KiCad's DRC reports 357 `clearance` violations on Jetson where agni reports none against the
  board's declared 0.1 mm, most likely the netclass and `.kicad_dru` rules agni does not read.
- **#909 `mission_ask`**, filed, not active. #910 (P1, MCP on `agni serve`) is its first ticket.
- This run: PRs 1012, 1017, 1018, 1020, 1022, 1024, 1025 and 1029 merged (#564, #724, #998 and
  #453 closed). #453 put the docsite's embedded drawings in the gate (`docsite-images-check`).
  Filed #1028 (`waiting`, the faithful-sample golden's corner label off its canvas). The lazy fact
  base is still parked on `wip/lazy-fact-base` (#1007, `waiting`).

## Open, ranked

1. **Jetson on `mission_public_demo`.** #1010 first, since it is a measured regression. The
   deployed `--timing` walk at `4433e6de` is on the issue (coverage/A2B 1.5 s, spi_nor-signal-missing
   1.5 s, both engine time); measure locally with `--timing` before changing anything. Then #1005, the board
   read's remaining 412 MB of allocation, and #992 (the serve worker still receives the board's
   85 MB file), whose open question is the protocol: the page pushes files before a request and
   cannot tell whether a query needs the board, so the worker probably answers "needs the board's
   files" and the page retries. Measure with `make wasm-bench`, `make wasm-layers` and the
   exercise's `--timing`, and read CLAUDE.md on the bench's two misleading rows first.
2. **#978 (the static Compare picker is empty), #918 (Cancel on Run checks) and #897**, on
   `mission_public_demo`.
3. **#605, KiCad accepts `[hi:lo]` as a bus**, off any mission. It absorbed #758.
4. **#702, `PinsByName` compares pin names outside `core/ident`.** Decide first whether
   underscore-folding belongs in `core/ident` for pin names.
5. **#390 PR 1, the server caches check results.** **The key in the issue body is wrong.** Read the
   corrections comment before writing any code. #895 keeps the model and fact base, so a repeated
   check pays only rule evaluation; measure that with `--timing` before building a result cache.
6. **#910 on `mission_ask`**, MCP tools over the existing services. Every service is transport-neutral,
   so it is an adapter, not new analysis.
7. **#356, #485, #634, #736**, on `mission_browser_review`. The Python client's cross-transport
   test declares #736 field by field, so its fix also deletes that declaration in
   `clients/python/tests/test_cross_transport.py`.

Later work is #1007 (a fact store per tier, `waiting`), #717 (show unexpanded EDIF hierarchy in the viewer and reports), #370 then #373, #380,
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
  module, `TestCheckWorkbenchAssets` need a built web bundle, and `TestSampleBoard*` needs the gitignored samples corpus (`make samples`). A worktree's
  build also carries no VCS stamp here, which fails the browser-cache test and rung 12's capture in
  `tutorial-runs-check` (#957); the primary checkout passes both.
- **The two `DesignHash` implementations differ on purpose** (see `OUT_OF_SCOPE.md`). Every caller now
  passes a resolved tier, so unifying them buys symmetry nobody can observe.
