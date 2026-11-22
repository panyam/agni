# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material goes in gitignored `HANDOFF*.md` notes.

Last pruned 2026-11-21, at `0c1990ed`.

## At a glance

**#1799** (run agni inside an enterprise, against designs and packs in its own GitHub Enterprise
Server) is active from 2026-11-21, at 0 of 11. It has no exercise yet, and its first ticket, #1800,
writes `make exercise-enterprise` against the test suite's fake GitHub serving as a GHES host. A
compose bundle (#1802) and a Helm chart (#1803) both deploy it.

- **Three tickets wait on library releases.** #1809 (Git LFS) on goapplib#203, #1810 (TLS ended at
  a proxy) on oneauth#541, and #1811 (OIDC sign-in) on oneauth#542. agni moves to each release once
  it ships.
- **#1722** (linked sources) is no longer active, at 21 of 23. Its exercise passes at `0c1990ed`.
  #1729 (SharePoint) and #1730 (S3 and GCS) stay open there at P3.
- **Parked work lives in each mission's `## Later` list.** The 62 `waiting` issues were closed into
  those lists on 2026-11-21. Anything that fits no mission is on #1806, and
  `is:closed label:waiting reason:"not planned"` finds them all. A mission moves its Later lines
  before it closes.
- **Other missions, none active.** #1611 (markup, starts at #1612 and #1613), and four whose first
  ticket writes the exercise: #1708 (#1712), #1709 (#1713), #1710 (#1714) and #1711 (#1715, which
  needs a datasheet PDF agni-samples may carry). #1176 holds only Later lines.

## Open, ranked

1. **#1800**, `make exercise-enterprise`. Each step it can't pass yet names its ticket.
2. **Then #1802 and #1801**, the bundle and packs linked from a repo, since the exercise needs both.
   #1808 (bootstrap) comes with the bundle. Its scope is still open: the leaning is a file beside
   `agni.yaml`, not inside it, that also declares runtime mounts.

Later work is #370 then
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
  module, `TestCheckWorkbenchAssets` need a built web bundle, and `TestSampleBoard*` needs the gitignored samples corpus (`make samples`, or an ABSOLUTE symlink to the main checkout's `tools/samples`, since a relative one resolves from `tools/`). The Glasgow oracle tests also need `tools/samples/remote/kicad-models`, which `make samples-oracle` fetches. `proto-check` needs `protoc-gen-es`, so run `make web-install` first.
- **The two `DesignHash` implementations differ on purpose** (see `OUT_OF_SCOPE.md`). Every caller now
  passes a resolved tier, so unifying them would change nothing a caller can observe.
