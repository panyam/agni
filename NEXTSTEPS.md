# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material goes in gitignored `HANDOFF*.md` notes.

Last pruned 2026-11-23, at `67e095f7`.

## At a glance

**#1799** (run agni inside an enterprise, against designs and packs in its own GitHub Enterprise
Server) is active from 2026-11-21, at 11 of 14. Its exercise is `make exercise-enterprise`, against
a fake GHES (`tools/fakegithub --seed`), and CI runs it in its own job. At `3dbe743a` all 21 steps
pass. It also runs as `make exercise-enterprise-bundle` against `deploy/enterprise/compose.yaml`
(PR 1832), and `make kind-up exercise-enterprise-helm` against `deploy/helm/agni` on kind (PR 1837),
which needs helm and kind. `guide/deploying/` (PR 1843) walks an admin through the same steps, and
nobody has followed it by hand on a fresh VM yet. #1805 (the demo repo) comes next.

- **agni is on goapplib v0.7.33, oneauth v0.3.21, servicekit v0.2.5 and ci-actions v0.1.5.**
  `--trusted-proxy` reads through servicekit's `TrustedProxies` (PR 1829). `--oidc-*` signs people
  in through an OpenID Connect provider beside or instead of GitHub, and `agni login --with oidc`
  uses its device flow (PR 1842). The exercise signs in through the App only. #1841 moves the
  exercise onto goapplib's fake GHES once panyam/goapplib#240 gives it TLS.
- **A checklist can be a folder of item files** (`items_dir`), in a descriptor (PR 1835) or a
  standalone manifest, and `--checklist checklist/` takes the folder itself (PR 1839). `agni
  checklists packs/<id> --remote` lists a pack's checklists, and a server design takes its
  checklists from the pack it is reviewed under. The example projects keep their boards under
  `boards/` (PR 1840), so a checkout that copied the Jetson boards before reruns
  `make -C examples/jetson-project board revision rev-b`.
- **`--store` takes `file://`, `sqlite://`, `postgres://` and `datastore://`** (PRs 1822 and 1825),
  and two servers can share one store. `--uploads` takes `s3://` and `gs://` with a local
  `--uploads-cache` (PR 1834). The Helm chart takes either in place of its claim.
  `internal/authgate/keys.go` reads API keys through oneauth's `APIKeyAdmin`.
- **Parked work lives in each mission's `## Later` list.** The 62 `waiting` issues were closed into
  those lists on 2026-11-21. Anything that fits no mission is on #1806, and
  `is:closed label:waiting reason:"not planned"` finds them all. A mission moves its Later lines
  before it closes.
- **Other missions, none active.** #1611 (markup, starts at #1612 and #1613), and four whose first
  ticket writes the exercise: #1708 (#1712), #1709 (#1713), #1710 (#1714) and #1711 (#1715, which
  needs a datasheet PDF agni-samples may carry). #1176 holds only Later lines.

## Open, ranked

1. **#1804**, the guide that takes an admin from a fresh machine to a running agni against their own
   GitHub Enterprise Server, through the compose bundle or the Helm chart. Its "Done when" runs
   against the exercise's fake GHES.
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
