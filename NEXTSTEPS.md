# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material goes in gitignored `HANDOFF*.md` notes.

Last pruned 2026-11-17, at `6b575fe6`.

## At a glance

**#1625 `mission_clients`** (build a tool in TypeScript, Rust, Java or C++ against a shared agni server)
is active, at 12 of 13 tickets closed. Its exercise is `make exercise-clients`, and at `1596f60d` all
five rows pass, against a server with no auth. The mission log on #1625 has each run.

- **#1627** (TLS and tokens), the mission's last ticket, is unblocked. Its three pushdowns closed and
  are tagged, as panyam/oneauth v0.3.3 and panyam/servicekit v0.2.3. agni still pins oneauth v0.3.2
  and servicekit v0.2.2. The ticket's last comment records what is decided and what is open.
- **#1664 `mission_board_rendering`** (see any board in 3D as its maker's tool shows it) is also active, at 11 of
  13. #1681 gave KiCad parts their STEP models' heights, and #1680 and #1682 closed #133. Its next
  tickets are #1652's mesh half and #1679 (IPC-2581 draws paste windows and unplated holes as copper,
  P3). Its exercise ran at `5069757f` and works headless, and #1682 merged since.
- **A release, then two pin bumps.** The viewer features since v0.7.0 reach a release binary only after
  the next agni release. Then panyam/agni-samples moves its CI's latest and its README off v0.7.0 and
  cuts a samples release. Then the engine bumps `hack/samples.pin`, fetches `remote/kicad-models.pin`
  from the `remote-boards` tarball (agni-samples#29), and drops `hack/kicad_models.pin`.
- **#1611 `mission_markup`** (mark places on a design, find them on the next revision) is filed and not
  active. #1612 (its exercise) and #1613 (the design of a mark and its stores) come first, then #1594.
- **#1176 `mission_shared_viewer_rooms`** is not active, and #1173 is `waiting` until someone runs agni
  serve for more than one person.
- This run: #1681 merged and its exercise posted on #1664, agni-samples#29 merged (KiCad models and a
  viewer section in its README), and #1680 and #1682 closed #133.

## Open, ranked

1. **Cut the agni release**, then the agni-samples and engine pin bumps above.
2. **#1627**: bump oneauth and servicekit to their new tags, then TLS and tokens on `agni serve`. Then a
   ticket for the gRPC clients to trust a CA, which closes #1625.
3. **#1652's mesh half**, then #1679, on #1664. KiCad 10's library ships no `.wrl`, so read the
   last comment on #1652 before planning it. Jetson's parts need agni to map `${ANT3DMDL}`, which has
   no ticket yet.
4. **Then #1611.** Make it active with `/retriage`, and start at #1612 and #1613.

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
  module, `TestCheckWorkbenchAssets` need a built web bundle, and `TestSampleBoard*` needs the gitignored samples corpus (`make samples`, or an ABSOLUTE symlink to the main checkout's `tools/samples`, since a relative one resolves from `tools/`). The Glasgow oracle tests also need `tools/samples/kicad-models`, which `make samples-oracle` fetches. `proto-check` needs `protoc-gen-es`, so run `make web-install` first. A worktree
  built under `GOTOOLCHAIN=go1.26.9` carries no VCS stamp, which fails the browser-cache test and rung
  12's capture in `tutorial-runs-check` (#957); the default Go 1.27 stamps a worktree and passes both.
- **The two `DesignHash` implementations differ on purpose** (see `OUT_OF_SCOPE.md`). Every caller now
  passes a resolved tier, so unifying them would change nothing a caller can observe.
