# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material stays in gitignored `HANDOFF*.md` notes.

Last pruned 2026-10-20, at `e719bbce` (PR 1083).

## At a glance

Two missions are active, one per worktree, and a third is filed. `MISSION=mission_<slug> queue.sh`
(the retriage skill's script) prints one mission's queue, and the order lives in GitHub labels and
blocked-by links rather than here. Log each exercise run on the mission issue.

- **#851 `mission_public_demo`**, 60 of 74. Since v0.2.5 the demo reads a design on a Load page
  before the viewer opens it (#1072), checks a selected net on its own (#897), cancels a run (#918),
  and keeps the board out of the serve worker (#992). Every seed record now says whether KiCad's ERC
  and DRC agree (#967). The last walk on the PR 1083 branch exited 0, 94 rows ok.
- **Seed triage on #851** (`hack/seed-triage/`): false positives #1014, #1015, #1016 and #1082 (four
  verdicts the KiCad comparison and the DNP work found, the KiCad reader's `tri_state` among them).
  Jetson's 357 KiCad clearance violations are its `.kicad_dru` rules, on #142.
- **#845 `mission_browser_review`**, 16 of 19, three P3s (#759, #774, #904) and a checklist for one
  sample board.
- **#909 `mission_ask`**, filed, not active. #910 (P1, MCP on `agni serve`) is its first ticket.
- This run: PR 1083 merged (#961, #967, jaala v0.1.84); dropped #978, #918, #897 and #992 (closed).

## Open, ranked

1. **The seed false positives, #1082 first**, on `mission_public_demo`. Its `tri_state` item and its
   GND-driver item each move a seed verdict, and the gateway fixture item would make KiCad an oracle
   for the tutorial's copper faults (it restamps every tutorial capture). Then #1014, #1015, #1016.
2. **Jetson's load**, #1005 (the board read's remaining allocation) and the rest of #1061. Measure
   with the exercise's `--timing`, `make wasm-bench` and `make wasm-layers`, and read CLAUDE.md on
   the bench's two misleading rows first.
3. **#605, KiCad accepts `[hi:lo]` as a bus**, off any mission. It absorbed #758.
4. **#702, `PinsByName` compares pin names outside `core/ident`.** Decide first whether
   underscore-folding belongs in `core/ident` for pin names.
5. **#910 on `mission_ask`**, MCP tools over the existing services. Every service is transport-neutral,
   so it is an adapter, not new analysis.
6. **#845's three P3s** (#759, #774, #904), and a checklist for one sample board.

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
  module, `TestCheckWorkbenchAssets` need a built web bundle, and `TestSampleBoard*` needs the gitignored samples corpus (`make samples`). A worktree
  built under `GOTOOLCHAIN=go1.26.4` carries no VCS stamp, which fails the browser-cache test and rung
  12's capture in `tutorial-runs-check` (#957); the default Go 1.27 stamps a worktree and passes both.
- **The two `DesignHash` implementations differ on purpose** (see `OUT_OF_SCOPE.md`). Every caller now
  passes a resolved tier, so unifying them buys symmetry nobody can observe.
