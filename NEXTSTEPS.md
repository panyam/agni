# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material goes in gitignored `HANDOFF*.md` notes.

Last pruned 2026-11-22, at `dd30166a`.

## At a glance

**#1799** (run agni inside an enterprise, against designs and packs in its own GitHub Enterprise
Server) is active from 2026-11-21, at 6 of 12. Its exercise is `make exercise-enterprise`, against
a fake GHES (`tools/fakegithub --seed`), and CI runs it in its own job. At `b99c4a81` all 20 steps
pass with no waits, and `make exercise-enterprise-bundle` passes the same 20 against
`deploy/enterprise/compose.yaml` (PR 1832). Run `make image` before the bundle run until #1803
makes the target build it. The Helm chart (#1803) comes next and puts agni behind an ingress with
`--trusted-proxy`.

- **agni is on goapplib v0.7.30, oneauth v0.3.19, servicekit v0.2.5 and ci-actions v0.1.4.**
  `--trusted-proxy` reads through servicekit's `TrustedProxies` (PR 1829). #1811 (OIDC) needs only
  agni wiring. #1826 adopts what v0.7.30 adds past the bump: the fake GHES with App limits, the
  seeded fake, sealed server secrets, and bucket syncs that skip held files.
- **`--store` takes `file://`, `sqlite://`, `postgres://` and `datastore://`** (PRs 1822 and 1825),
  and two servers can share one store. `--uploads` takes `s3://` and `gs://` with a local
  `--uploads-cache` (PR 1834). #1803 can offer postgres and a bucket in place of volumes.
  `internal/authgate/keys.go` reads API keys through oneauth's `APIKeyAdmin`.
- **#1722** (linked sources) has all 24 tickets closed. Its exercise passes all 13 checks at
  `dd30166a`. It closes once its 3 Later lines move.
- **Parked work lives in each mission's `## Later` list.** The 62 `waiting` issues were closed into
  those lists on 2026-11-21. Anything that fits no mission is on #1806, and
  `is:closed label:waiting reason:"not planned"` finds them all. A mission moves its Later lines
  before it closes.
- **Other missions, none active.** #1611 (markup, starts at #1612 and #1613), and four whose first
  ticket writes the exercise: #1708 (#1712), #1709 (#1713), #1710 (#1714) and #1711 (#1715, which
  needs a datasheet PDF agni-samples may carry). #1176 holds only Later lines.

## Open, ranked

1. **#1803**, the Helm chart, reusing the compose bundle's flags behind an ingress. It wants
   panyam/ci-actions#14 (kind-ingress) and #15 (helm-oci-publish). #1804 (the guide) walks an
   admin through the bundle after it. PR 1834 moves to goapplib v0.7.30.
2. **#1808**, bootstrap. Linked packs (PR 1819) give it `LinkPack` to call. Its scope is still
   open: the leaning is a file beside `agni.yaml`, not inside it, that also declares runtime
   mounts, plus what happens to a design the file drops and whether reports are made at start.

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
