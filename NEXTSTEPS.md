# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material goes in gitignored `HANDOFF*.md` notes.

Last pruned 2026-11-17, at `28bc4cf0`.

## At a glance

**#1691 `mission_design_store`** (keep projects and designs on an agni server, and come back to every
one) is active, at 0 of 7. The mission body states the model: a store port with a filesystem
adapter first, editable designs, a computed `series` in place of `revisions:`, and reviews under the
project. Its exercise (`make exercise-design-store`) does not exist yet, and #1696 writes it.

- **#1625 `mission_clients`** is active, at 12 of 13. Its last ticket is #1627 (TLS and tokens). The
  pushdowns are tagged as panyam/oneauth v0.3.3 and panyam/servicekit v0.2.3, and agni still pins
  v0.3.2 and v0.2.2. The ticket's last comment records what is decided and what is open.
- **#1664 `mission_board_rendering`** closed on 2026-11-17, at 15 of 15, with its exercise passing at
  `76ef182e`. Run headed, the cross-probe walks still time out at "two views" on the main Mac.
- **Four missions filed on 2026-11-17 by a retriage, none active.** Each one's first ticket writes its
  exercise. #1708 `mission_check_verdicts` (#1712), #1709 `mission_rule_authoring` (#1713), #1710
  `mission_format_fidelity` (#1714), and #1711 `mission_datasheets` (#1715, which needs a datasheet
  PDF that agni-samples may carry before anything else can run).
- **#1611 `mission_markup`** is filed and not active. #1612 and #1613 come first, then #1594.
- **#1176 `mission_shared_viewer_rooms`** is not active, and #1173 is `waiting`.

## Open, ranked

1. **#1692 and #1693**, in parallel. #1692 amends C23 and drops `revisions:` (a breaking proto change),
   and #1693 builds the read-write store port. #1694, #1695, #1696 and #1697 wait on #1693's port.
2. **#1627**: bump oneauth and servicekit to their new tags, then TLS and tokens on `agni serve`. Then a
   ticket for the gRPC clients to trust a CA, which closes #1625.
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
