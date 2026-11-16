# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material goes in gitignored `HANDOFF*.md` notes.

Last pruned 2026-11-16, at `4caa3fe2`.

## At a glance

**#1625 `mission_clients`** (build a tool in TypeScript, Rust, Java or C++ against a shared agni server)
is active, at 12 of 13 tickets closed. Its exercise is `make exercise-clients`, and at `1596f60d` all
five rows pass, against a server with no auth. The mission log on #1625 has each run.

- **#1627** (TLS and tokens), the mission's last ticket, is parked. It waits on panyam/oneauth#462
  (cache validated API keys), panyam/oneauth#463 (refuse a call in its own protocol) and
  panyam/servicekit#60 (TLS in `ListenAndServeGraceful`), all three still open. Its comment records
  what is decided and what is open.
- **#1664 `mission_board_rendering`** (see any board in 3D as its maker's tool shows it) is also active, at 10 of
  12. Its next tickets are #1652 (3D models) and #133 (measuring the board render). Its exercise last ran
  at `4caa3fe2` and works, and the mission log on #1664 has each run.
- **#1611 `mission_markup`** (mark places on a design, find them on the next revision) is filed and not
  active. #1612 (its exercise) and #1613 (the design of a mark and its stores) come first, then #1594.
- **#1176 `mission_shared_viewer_rooms`** is not active, and #1173 is `waiting` until someone runs agni
  serve for more than one person.
- This run: #1675 and #1678 closed #1660. ODB++ jobs from Altium and Allegro now read the stack they
  state, `ir.Stackup` is gone, and `agni emit` writes the board's stack. The clay cross-probe walk's
  Board look step failed once on CI and passed locally, so watch it for a flake.

## Open, ranked

1. **#1664's next tickets**, #1652 then #133, while #1625 waits on its pushdowns.
2. **#1627** once its three pushdowns are tagged, then a ticket for the gRPC clients to trust a CA.
   That closes #1625.
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
- **Two failures in a fresh worktree are not yours.** `TestCheckWebAssets` and, in the `datasheet`
  module, `TestCheckWorkbenchAssets` need a built web bundle, and `TestSampleBoard*` needs the gitignored samples corpus (`make samples`, or an ABSOLUTE symlink to the main checkout's `tools/samples`, since a relative one resolves from `tools/`). A worktree
  built under `GOTOOLCHAIN=go1.26.9` carries no VCS stamp, which fails the browser-cache test and rung
  12's capture in `tutorial-runs-check` (#957); the default Go 1.27 stamps a worktree and passes both.
- **The two `DesignHash` implementations differ on purpose** (see `OUT_OF_SCOPE.md`). Every caller now
  passes a resolved tier, so unifying them would change nothing a caller can observe.
