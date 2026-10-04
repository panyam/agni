# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material stays in gitignored `HANDOFF*.md` notes.

Last pruned 2026-10-04, at `a5eced9f` (PR 889).

## At a glance

Three missions are active, one per worktree. `MISSION=mission_<slug> queue.sh` (the retriage
skill's script) prints one mission's queue, and the order lives in GitHub labels and blocked-by
links rather than here. Log each exercise run on the mission issue.

- **#844 `mission_real_board_tutorial`**, 0 of 4. Next is #564. The exercise has not run yet, so run
  it before picking a ticket.
- **#845 `mission_browser_review`**, 1 of 11 (#829 closed with #859). Next by the queue is #356. The
  exercise is a manual walk with no logged run yet.
- **#851 `mission_public_demo`**, 6 of 16, in the `docs` clone. Exercise last ran at `a5eced9f`
  (PR 889): steps 2 to 5 pass (drop a folder, a zip, an EDIF pair; check, trace, query; nothing about
  the design on the network), and only step 1 fails, for want of the deployed landing page. #856
  part one is PR 890 (`feat/856-static-demo`); part two wires it into the docs Pages deploy.
- This run: #852, #853, #854, #863 and #887 closed through PRs 874, 881, 884, 886, 888 and 889.
  #857 is now blocked by #856, since there is nothing to triage until the seeds are chosen.

## Open, ranked

1. **#564, move the tutorial ladder onto the Jetson board, with #724 (rung 13's total).** Decide
   the entry view (`.kicad_sch` or `.kicad_pcb`) first; the per-rung plan is the latest comment on
   #564. Re-run rung 13 in the CLI and the panel before editing it.
2. **#856 part two on `mission_public_demo`**, after PR 890 merges: the docs workflow builds
   `make demo-site` into the Pages artifact under `/agni/demo/` and runs `web/browser/site.spec.ts`
   against it. Then #857 (triage the seeded boards' findings), #865, #868 and #127.
3. **#605, KiCad accepts `[hi:lo]` as a bus**, off any mission. It absorbed #758.
4. **#702, `PinsByName` compares pin names outside `core/ident`.** Decide first whether
   underscore-folding belongs in `core/ident` for pin names.
5. **#390 PR 1, the server caches check results.** **The key in the issue body is wrong.** Read the
   corrections comment before writing any code.
6. **#356, #485, #634, #736**, on `mission_browser_review`. The Python client's cross-transport
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
to test against.

Four items are gated, #677 (only the "declare a NEW class" half, gated on the class registry), #667,
#681 and #523.

## feat/856-static-demo

- **Last touched**: 2026-10-04 (`/workspace/repos/projects/Agni/docs`)
- **Ticket**: #856, PR 890 (part one, open).
- **Why**: the static demo, `agni site` and the viewer under a base path, so step 1 of #851's
  exercise can run against a page served as plain files.
- **Where it stopped**: PR 890 is green on `make testall`. Seeds are the tutorial gateway,
  RoyalBlue54L Feather and the Jetson AGX Thor baseboard (`make demo-site`).
- **Next action**: once PR 890 merges, branch part two from `origin/main` and add a step to
  `.github/workflows/docs.yml` that runs `agni site docsite/dist/demo --base /agni/demo/ --seed ...`
  before the artifact upload (`dist/` is served at `/agni/`), plus the site spec against that output.
- **Open questions**: the docs workflow does not fetch the samples corpus today, so part two either
  adds `make samples-oracle` to it or seeds only the tutorial there; the Jetson seed is a 92 MB
  download per visit, which may want a lighter stand-in once #855's server can answer it instead.

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
