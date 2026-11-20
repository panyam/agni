# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material goes in gitignored `HANDOFF*.md` notes.

Last pruned 2026-11-20, at `b4cd9d6e`.

## At a glance

**#1722** (link a design to where the team keeps it, and follow its history) is active, at 14 of 20.
Its exercise is `make exercise-linked-sources`, which now ends with a browser walk of the designs
page's link form, so it needs `make ui`. PR #1782 merged on 2026-11-20 and closed #1727. The
exercise passes at `b4cd9d6e`. What remains is leftovers (#1774, #1778, #1781) and two P3 sources.

- **#1611 `mission_markup`** is filed and not active. #1612 and #1613 come first.
- **Four missions filed on 2026-11-17, none active.** Each one's first ticket writes its exercise.
  #1708 (#1712), #1709 (#1713), #1710 (#1714), and #1711 (#1715, which needs a datasheet PDF that
  agni-samples may carry before anything else can run).
- **#1176 `mission_shared_viewer_rooms`** is not active, and #1173 is `waiting`.

## Open, ranked

1. **The leftovers of #1722**, in a worktree of the `~/projects/Agni/main` clone. #1774 (from PR
   #1773), #1778 (from PR #1779) and #1781 (from PR #1782). #1729 (SharePoint) and #1730 (S3 and
   GCS) are P3.
2. **#1780** (CI uses the runner's Chrome) is P2 and serves no mission.
3. **Then #1611.** Make it active with `/retriage`, and start at #1612 and #1613.

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
- **Some failures in a fresh worktree are not yours.** `TestCheckWebAssets` and, in the `datasheet`
  module, `TestCheckWorkbenchAssets` need a built web bundle, and `TestSampleBoard*` needs the gitignored samples corpus (`make samples`, or an ABSOLUTE symlink to the main checkout's `tools/samples`, since a relative one resolves from `tools/`). The Glasgow oracle tests also need `tools/samples/remote/kicad-models`, which `make samples-oracle` fetches. `proto-check` needs `protoc-gen-es`, so run `make web-install` first. A worktree
  built under `GOTOOLCHAIN=go1.26.9` carries no VCS stamp, which fails the browser-cache test and rung
  12's capture in `tutorial-runs-check` (#957); the default Go 1.27 stamps a worktree and passes both.
- **The two `DesignHash` implementations differ on purpose** (see `OUT_OF_SCOPE.md`). Every caller now
  passes a resolved tier, so unifying them would change nothing a caller can observe.
