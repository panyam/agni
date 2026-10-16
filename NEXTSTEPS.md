# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material stays in gitignored `HANDOFF*.md` notes.

Last pruned 2026-10-16, at `202c5dc7` (PR 1041).

## At a glance

Two missions are active, one per worktree, and a third is filed. `MISSION=mission_<slug> queue.sh`
(the retriage skill's script) prints one mission's queue, and the order lives in GitHub labels and
blocked-by links rather than here. Log each exercise run on the mission issue.

- **#845 `mission_browser_review`**, 9 of 17. The walk at `bec675b9` completed every step on the
  tutorial project for the first time: picker, runs, a query set opened as tabs, clicks into the
  drawing, and Save report matching `agni review --format html`. Done-when also wants one sample
  board, and no sample board carries a checklist yet, so that run needs one first. #390's first PR (1041, the server
  keeps each rule's results) merged; the next step is its second PR, dropping the viewer's
  `findingCache` and `verdictCache`, then #485 and #634.
- **#851 `mission_public_demo`**, 46 of 68. #1002 closed after ten clean deployed runs. jaala v0.1.59
  made Jetson's Run checks about three times slower through a planner change (#1037), fixed by
  v0.1.60: the local walk at `28a6f080` took 4.9 s, against about 28 s at v0.1.59 and 9.6 to 11.8 s at
  v0.1.53. #1010 is still the open measured regression.
- **Seed triage on #851** (`hack/seed-triage/`): the Jetson record holds three false positives, each
  with its ticket (#1014, #1015, #1016). #961 and #967 are filed; #967's first run found KiCad's DRC
  reporting 357 `clearance` violations on Jetson where agni reports none.
- **#909 `mission_ask`**, filed, not active. #910 (P1, MCP on `agni serve`) is its first ticket.
- This run: v0.2.2 tagged and released; PR 1041 merged (#390 PR 1). The #845 walk at `202c5dc7`
  ran through picker, both checklist runs, a finding click and Run checks (28 findings, highlight 0)
  and was stopped before the query-set step, so Save report was not re-checked there.

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
5. **#390 PR 2, the viewer stops caching check answers.** PR 1041 made a rule already run on a kept
   model cost a round trip and no evaluation, so `findingCache` and `verdictCache` in
   `web/src/viewer.ts` can go. PR 3 (split the presenter) follows.
6. **#910 on `mission_ask`**, MCP tools over the existing services. Every service is transport-neutral,
   so it is an adapter, not new analysis.
7. **#485 and #634**, on `mission_browser_review`, after #390.

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
