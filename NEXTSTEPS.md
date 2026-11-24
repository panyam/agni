# Next steps

The ranked list of what to pick up next, checked in so every checkout sees the same one. Each item
is a pointer, and the reasoning, measurements and plan live on the issue. Anything here is a claim to check
against the tree and the issue before acting on it.

**Keep it short and keep it public.** When an item closes, delete it. When you learn something
durable, put it in `CLAUDE.md`, `DECISIONS.md`, `OUT_OF_SCOPE.md` or the issue rather than here. This repo
is public, so nothing naming a customer, their boards or tools, or a private path belongs in this
file. That material goes in gitignored `HANDOFF*.md` notes.

Last pruned 2026-11-24, at `fe04e00e`.

## At a glance

**No mission is active.** #1799 (run agni inside an enterprise against its own GitHub Enterprise
Server) closed on 2026-11-23 at 17 of 17, with `make exercise-enterprise` passing 24 of 24 at
`a394e384`. CI keeps running it, plus `make exercise-enterprise-bundle` (compose) and
`make kind-up exercise-enterprise-helm` (Helm on kind). Its Later lines moved to #1708 (#1227) and
#1806. `guide/deploying/` walks an admin through it, and nobody has followed it by hand on a fresh
VM yet. `agni serve --config serve.yaml` (PR 1856) holds serve's flags and the `--bootstrap`
resources (PR 1852), and the chart passes only `--config`. A stored design's check now runs its
`design.yaml` intent, pushed, added or linked (PRs 1859 and 1866). Push, add and link also keep the companions a
`design.yaml` declares and read the entry it names, so an ODB++ board and an EDIF schematic stay one
design that checks as its folder does (PR 1863). v0.11.2 shipped all of that on 2026-11-23. v0.11.3
made `agni serve` wait for its poll, webhook, bootstrap, prune and sweep work before it exits (PR
1870). v0.11.4 made a linked pack list only what its `project.yaml` names, so boards beside it count
against no size limit, and took servicekit v0.2.6 for GO-2026-6278 (PR 1872). Since then, linking a
declared entry, or its folder, links the companions its `design.yaml` declares (PR 1876).
`make exercise-formats` prints one row per format and corpus file (PR 1874), and `agni serve
--local-accounts` signs people in with a username and password that `agni users` keeps in `--store`
(PR 1878). `agni login --with local` gets a CLI token, and the compose bundle (`AGNI_LOCAL_ACCOUNTS`) and
Helm chart (`localAccounts`) turn it on (PR 1886). Email reset waits on #1884. A GitHub repo past the mount cap now mounts lazily, listing its tree and fetching what a read names, and `--mount-max-bytes` and `--mount-max-files` set the cap (PR 1881). Its follow-ups are on #1880. Rule messages, impact text and help lost their last em-dashes, and the KiCad item separator changed (PR 1882). v0.11.6 shipped PRs 1874 to 1886 on 2026-11-24. Since then, a companion that a link followed records its entry in `companion_of`, so bootstrap counts it as named and `prune: true` keeps it (PR 1892). Its gaps are on #1893. v0.11.7 shipped PRs 1891 (a stored design keeps `flatten_hierarchy`), 1892 and 1894 (checklist item files keep a team's own fields). Since then, `agni serve --grant source=pattern,...` and serve.yaml `access:` open a private source to signed-in people by subject pattern, through goapplib v0.7.38's `connections.WithSourceGrants` (PR 1897). `agnids` takes `--grant` and `--grants-file`, and the enterprise bundle mounts one `access.yaml` into both services (PR 1898). `oidc:group:<id>` and `github:org:<org>` patterns follow claims a sign-in writes on the token, through goapplib v0.7.39's `WithViewerClaims` (PR 1899). v0.11.9 shipped PRs 1898 and 1899 on 2026-11-24.

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
- **Missions to pick from.** #1611 (markup, starts at #1612 and #1613), and four whose first
  ticket writes the exercise: #1708 (#1712), #1709 (#1713) and #1711 (#1715, which needs a
  datasheet PDF agni-samples may carry). #1710 has its exercise now, at 1 of 11, next #134.
  `make exercise-formats` exits 1 at `b8ba5401`, because royalblue54L-feather does not read (#681). #1176 holds only Later lines.

## Open, ranked

Nothing is ranked until a mission is picked. `~/.claude/skills/retriage/queue.sh` lists them.

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
