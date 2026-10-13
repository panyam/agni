# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material stays in gitignored `HANDOFF*.md` notes.

Last pruned 2026-10-13, at `d642633b` (PR 983).

## At a glance

Three missions are active, one per worktree, and a fourth is filed. `MISSION=mission_<slug> queue.sh` (the retriage
skill's script) prints one mission's queue, and the order lives in GitHub labels and blocked-by
links rather than here. Log each exercise run on the mission issue.

- **#844 `mission_real_board_tutorial`**, 0 of 4. Next is #564. The exercise has not run yet, so run
  it before picking a ticket.
- **#845 `mission_browser_review`**, 4 of 16. The exercise is a manual walk with no logged run yet.
  #902 (a Properties panel and query tabs) is the viewer's next shape.
- **#851 `mission_public_demo`**, 35 of 53, no P1 left. Exercise last ran at `d642633b` (PR 983)
  against the deployed page: 67 steps ok, the one failure is #975 (the companion pair's busiest net).
  Since #974 (PR 982) the browser keeps its serve worker between requests, so a Jetson query
  answers in about 125 ms and a trace in about 210 ms where each took seconds. Next ready: #978
  (the static Compare picker is empty), then #981 (what the worker's 548 MB on Jetson is made of,
  deciding between a jaala fact store and reading the board lazily). Jetson's Run checks still
  takes about 18 to 22 s deployed (#968). #949 is `waiting`; the board read is Jetson's largest
  cost, so consider un-parking it with #981's numbers. The EDIF seed is `waiting` on a licensed
  pair (#928), and a hosted server on #880.
- **Seed triage on #851** (`hack/seed-triage/`, closed #857): no seed record holds a false positive.
  Two follow-ups are filed: #961 (the rules besides the pull-up walks that still credit a DNP part)
  and #967 (check the records against KiCad's ERC and DRC). #967's first run already disagrees:
  KiCad's DRC reports 357 `clearance` violations on Jetson where agni reports none against the
  board's declared 0.1 mm, most likely the netclass and `.kicad_dru` rules agni does not read.
- **#909 `mission_ask`**, filed, not active. #910 (P1, MCP on `agni serve`) is its first ticket.
- This run: PRs 977, 979, 980, 982 and 983 merged (#865, #898, #956, #974, and jaala v0.1.38 then
  v0.1.44). Filed #978, #981 and panyam/jaala#216 (a computed number misses `=`, which
  `guide/querying.md` now warns about).

## Open, ranked

1. **#564, move the tutorial ladder onto the Jetson board, with #724 (rung 13's total).** Decide
   the entry view (`.kicad_sch` or `.kicad_pcb`) first; the per-rung plan is the latest comment on
   #564. Re-run rung 13 in the CLI and the panel before editing it.
2. **Jetson's Run checks on `mission_public_demo`.** #968 (the 10 s outside `CheckDesign`) first,
   since it is the largest unmeasured part; then the board read (#949 if its geometry dominates,
   measured with `--timing`) and the interface-profile rules, whose slowest are
   `emmc-signal-missing` and `spi_nor-signal-missing`. #964 (the overlay recomposed per request) is
   the gateway's biggest cost. Measure with `--timing` or `?timing` before changing anything.
3. **#978, then #981, on `mission_public_demo`**, then #918 (Cancel on Run checks) and #897.
4. **#605, KiCad accepts `[hi:lo]` as a bus**, off any mission. It absorbed #758.
5. **#702, `PinsByName` compares pin names outside `core/ident`.** Decide first whether
   underscore-folding belongs in `core/ident` for pin names.
6. **#390 PR 1, the server caches check results.** **The key in the issue body is wrong.** Read the
   corrections comment before writing any code. #895 keeps the model and fact base, so a repeated
   check pays only rule evaluation; measure that with `--timing` before building a result cache.
7. **#910 on `mission_ask`**, MCP tools over the existing services. Every service is transport-neutral,
   so it is an adapter, not new analysis.
8. **#356, #485, #634, #736**, on `mission_browser_review`. The Python client's cross-transport
   test declares #736 field by field, so its fix also deletes that declaration in
   `clients/python/tests/test_cross_transport.py`.

Later work is #717 (show unexpanded EDIF hierarchy in the viewer and reports), #370 then #373, #380,
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
  module, `TestCheckWorkbenchAssets` need a built web bundle, and `TestSampleBoard*` needs the gitignored samples corpus (`make samples`).
- **The two `DesignHash` implementations differ on purpose** (see `OUT_OF_SCOPE.md`). Every caller now
  passes a resolved tier, so unifying them buys symmetry nobody can observe.
