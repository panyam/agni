# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material stays in gitignored `HANDOFF*.md` notes.

Last pruned 2026-10-05, at `70b1cfc4` (PR 925).

## At a glance

Three missions are active, one per worktree, and a fourth is filed. `MISSION=mission_<slug> queue.sh` (the retriage
skill's script) prints one mission's queue, and the order lives in GitHub labels and blocked-by
links rather than here. Log each exercise run on the mission issue.

- **#844 `mission_real_board_tutorial`**, 0 of 4. Next is #564. The exercise has not run yet, so run
  it before picking a ticket.
- **#845 `mission_browser_review`**, 1 of 14. The exercise is a manual walk with no logged run yet.
  #902 is the viewer's next shape: a Properties panel for any selection (it replaces Details) and a
  query panel holding named tabs that its Ask rows open. #745 (open a query set as tabs) and #717's
  viewer half fold into it; #904 (area selection) follows it and jaala#132 (bind a set of values).
- **#851 `mission_public_demo`**, 10 of 29. Exercise last ran at `70b1cfc4` (PR 925): `make
  demo-site` builds, the static site's spec passes, and a reload restores a parsed design (Run checks
  5.8 s first visit, 4.0 s after). The manual walk waits on #878. #857 (P1) is next by the queue, and
  #868 then #918 by the mission's last comment. Snappiness is #896, #897, #914 and #922; check #900
  and #901 against #911 (closed by PR 925) before starting either.
- **#909 `mission_ask`**, filed, not active: ask a design questions through an LLM and get answers
  that cite the rows. #910 (P1, MCP on `agni serve`) is its first ticket.
- This run: PRs 899, 912, 913, 915, 917, 920 and 921 merged, closing #895 and #916; agni follows
  jaala v0.1.23 (jaala#139, #140). Filed #909, #910, #914 and #922, and jaala#147 (Explain). #907
  (`tables_to_xlsx` was quadratic in its rows) closed through PR 908.

## Open, ranked

1. **#564, move the tutorial ladder onto the Jetson board, with #724 (rung 13's total).** Decide
   the entry view (`.kicad_sch` or `.kicad_pcb`) first; the per-rung plan is the latest comment on
   #564. Re-run rung 13 in the CLI and the panel before editing it.
2. **#857 on `mission_public_demo`** (triage the seeded boards' findings), then #865, #868 and #127.
   For snappiness, #896 (a starting-point query with a limit) and #897 (check one net or part) both
   build on #895's kept model; #914 (per-request timings) would have named the last two slow paths
   in one log line each.
3. **#605, KiCad accepts `[hi:lo]` as a bus**, off any mission. It absorbed #758.
4. **#702, `PinsByName` compares pin names outside `core/ident`.** Decide first whether
   underscore-folding belongs in `core/ident` for pin names.
5. **#390 PR 1, the server caches check results.** **The key in the issue body is wrong.** Read the
   corrections comment before writing any code. #895 now keeps the model and fact base, so a repeated
   check pays only rule evaluation; measure that before building a result cache on top.
6. **#910 on `mission_ask`**, MCP tools over the existing services. Every service is transport-neutral,
   so it is an adapter, not new analysis.
7. **#356, #485, #634, #736**, on `mission_browser_review`. The Python client's cross-transport
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
