# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material goes in gitignored `HANDOFF*.md` notes.

Last pruned 2026-11-28, at `24359663`.

## At a glance

**Mission #1708 is active**, to run agni check on a real board and trust every verdict, at 20 of
33 tickets. Its exercise is `make exercise-checks` (#1712), which holds Glasgow revC3's and Jetson's
verdicts to the hand-audited files in `cmd/agni/testdata/verdict-audit`. It last ran on 2026-11-28
at `24359663`, with every audited verdict matching (Glasgow 259 of 259, Jetson 586 of 586). `TestVerdictAuditServerParity` holds a
server's verdicts on both boards to the CLI's, over `check --remote` and a stored design (#1922). The audit runs Jetson with
`examples/jetson-project/params`. Before blaming a clamp or a connector in an audit row, check
`component.dnp`, since two of the first rows were parts never fitted (`build/check-rule.md`).

#1799 (enterprise) closed on 2026-11-23, and CI keeps running `make exercise-enterprise`,
`exercise-enterprise-bundle` and `kind-up exercise-enterprise-helm`. `guide/deploying/` walks an
admin through it, and nobody has followed it by hand on a fresh VM yet. v0.11.10 is the latest
release.

- **agni is on goapplib v0.7.39, oneauth v0.3.22, servicekit v0.2.6 and ci-actions v0.1.5.**
  `--trusted-proxy` reads through servicekit's `TrustedProxies` (PR 1829). `--oidc-*` signs people
  in through an OpenID Connect provider beside or instead of GitHub, and `agni login --with oidc`
  uses its device flow (PR 1842). The exercise signs in through the App only. Its App lacks the
  Members permission, so alice and mallory come in by name and bob's `org:acme` sign-in is refused.
- **A checklist can be a folder of item files** (`items_dir`), in a descriptor (PR 1835) or a
  standalone manifest, and `--checklist checklist/` takes the folder itself (PR 1839). `agni
  checklists packs/<id> --remote` lists a pack's checklists, and a server design takes its
  checklists from the pack it is reviewed under. The example projects keep their boards under
  `boards/` (PR 1840), so a checkout that copied the Jetson boards before reruns
  `make -C examples/jetson-project board revision rev-b`.
- **`--store` takes `file://`, `sqlite://`, `postgres://` and `datastore://`** (PRs 1822 and 1825),
  and two servers can share one store. `--uploads` takes `s3://` and `gs://` with a local
  `--uploads-cache` (PR 1834). The Helm chart takes either in place of its claim.
  `internal/authgate/keys.go` reads API keys through oneauth's `APIKeyAdmin`, and `agni users ls`
  and `rm` read accounts through its `ChannelAdmin` (oneauth v0.3.22).
- **Parked work lives in each mission's `## Later` list.** The 62 `waiting` issues were closed into
  those lists on 2026-11-21. Anything that fits no mission is on #1806, and
  `is:closed label:waiting reason:"not planned"` finds them all. A mission moves its Later lines
  before it closes.
- **Other missions.** #1611 (markup, starts at #1612 and #1613), and two whose first ticket
  writes the exercise: #1709 (#1713) and #1711 (#1715, which needs a datasheet PDF agni-samples
  may carry). #1710 has its exercise now, at 1 of 11, next #134.
  `make exercise-formats` exits 1 at `b8ba5401`, because royalblue54L-feather does not read (#681).

## Open, ranked

Mission #1708's queue, by `~/.claude/skills/retriage/queue.sh`. The audit has no open bug rows, so
the queue moves to rules that skip or overclaim subjects the audit does not cover yet: #157 (the
reach walk skips bus-like nets), then #159, #162 and #163. #1944 holds what #88 left, verdicts a
lossy read stops stating without saying so.

Later work is #370 then
#373, #380, #456, #374, #716 and #739 (read a Datalog-derived relation from Go). The query language
roadmap lives on panyam/jaala. Modules (jaala#3) and aggregation inside rules (jaala#4) have both shipped, and agni tracks the latest jaala, so a
derived question goes in `stdlib/lib` before it becomes a Go relation.

`OUT_OF_SCOPE.md` has five rows waiting on a decision about the ledger itself, not on code: the
`core/svg` move, the `.edn` sniffing row (its WS6-008 ticket was wrong), the WS9-035 row (agni
issue 541 half-settled it), the `Witness.Statement` row that asks for a ticket, and the
`agni version` row whose trigger is "never".

One item is gated, #681.

## Worth knowing before the next change

- **`reverse-blocking.undecided.kicad_sch` is the only fixture that reaches an inconclusive
  finding.** It is a barrel jack feeding an MCU through a P-FET, and `reverse-blocking-absent` cannot
  decide that circuit from a netlist. input-protection fails it too, since its walk crosses the FET
  (agni 1941). The browser suite mounts it as `conformance=`.
- **`severitySections` in `web/src/findings.ts` has no renderer, on purpose.** It is a parity oracle.
  It pins the client against `GetCheckReport`. It exists to catch drift, and "improving" it would
  hide that drift.
- **Some failures in a fresh worktree are not yours.** `TestCheckWebAssets` and, in the `datasheet`
  module, `TestCheckWorkbenchAssets` need a built web bundle, and `TestSampleBoard*` needs the gitignored samples corpus (`make samples`, or an ABSOLUTE symlink to the main checkout's `tools/samples`, since a relative one resolves from `tools/`). The Glasgow oracle tests also need `tools/samples/remote/kicad-models`, which `make samples-oracle` fetches. `proto-check` needs `protoc-gen-es`, so run `make web-install` first.
- **The two `DesignHash` implementations differ on purpose** (see `OUT_OF_SCOPE.md`). Every caller now
  passes a resolved tier, so unifying them would change nothing a caller can observe.
