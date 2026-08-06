---
description: Plan a unit of work (PR / phase / task) before implementing. Produces a plan; optionally creates a branch and opens a PR. Does NOT start coding until the plan is approved.
argument-hint: [scope description] — append "plan" or "pr" to pick the mode
---

The `$ARGUMENTS` string describes the scope and the desired output. Two modes:

- **plan** (default if unspecified — ASK the user before proceeding) — produce the plan, no code, no branch, no PR
- **pr** — after plan approval: create the branch, implement, push, and open the PR ready-for-review

## Cold-start context (skip if already loaded this session)

Read `CONSTRAINTS.md` if it exists, plus any project guides not already covered by the global ARCHITECTURE/SUMMARY/ROADMAP/NEXTSTEPS rule. Before designing scope, look for existing plans, gap docs, README exercises, or failing acceptance tests — don't reinvent what's already defined.

## Plan structure

Present the plan as a short checklist: files to touch, tests to write, verification steps. When multiple approaches exist, show tradeoffs (pros/cons + recommendation) — don't bury the alternatives. *Why: the user knows constraints better; give them enough to override.*

## Testing discipline

- **Red-before-green.** Each new test must fail before the change and pass after. *Why: confirms the test exercises the change, not just decorates it.*
- **Never relax existing assertions.** Before committing, count added vs removed/weakened assertions and report both numbers. *Why: passing by loosening checks silently erodes coverage.*
- **CI-integrated by default**, not opt-in. *Why: tests that aren't run create false confidence.*
- **Doc-comment a test only when its name + assertions don't already convey intent.** Aligns with the global "default to no comments" rule — test intent IS the non-obvious why, but don't comment when the name already says it.
- **List integration / e2e candidates explicitly**, even if deferred to a follow-up.
- **Reference-impl comparison.** If a canonical implementation exists in another language or repo, plan a behavioral or wire-format check. *Why: spec compliance is verified by interop, not unit tests.*

## Documentation discipline

The global "default to no comments" rule governs *internal* code — readers can study the implementation. Public API is the inverse: callers shouldn't have to. Before committing, audit every public symbol the PR adds or substantially changes.

- **Every exported method, function, type, and package-level var/const that callers from another package can reach gets a doc comment.** Especially: interface implementations, constructors, error sentinels, and anything whose contract is non-obvious from the signature (error-mapping rules, nil-handling, idempotency, ordering guarantees, concurrency promises).
- **Doc the WHY of the contract, not the WHAT of the code.** Restating the signature is noise. Capture: which error sentinels map to which conditions, what "absent" / "missing" / "empty" means, why a method does a Get-then-Delete (or similar non-obvious dance), what invariants the caller can rely on. *Why: implementation details change; contracts shouldn't, and the doc anchors the contract.*
- **Match the project's most-polished precedent, not the closest one.** If `stores/fs/` and `stores/gorm/` document every interface method but `stores/gae/` doesn't, the new file in `stores/gae/` follows the documented precedent — sparse local style isn't a license to ship undocumented public API. *Why: the documented backends are the contract surface other devs read first.*
- **Spotted undocumented neighbors during the audit?** That's a separate follow-up — file via `/ghissue`, don't expand the PR scope. *Why: same as the test-relaxation rule — drive-by docs sweeps bury the actual change.*

## Pushdown / splitdown (only if the project depends on shared sibling libs)

Build a table per work item: `Item | Project-specific? | Pushdown candidate? | Target lib | Rationale`. Consider splitdowns (contract in shared lib, impl here) — pushing a contract is cheap and enables reuse. Plan cross-repo merge order: target merges and releases first, consumer bumps the dep second. *Why: unreleased deps require workarounds that break CI and fresh clones.*

For multi-module repos: bump shared deps in lock-step across all modules. Only bump to tagged releases, never to merged commits.

## PR description (review-ability)

When writing or editing the PR description, structure it so a cold reviewer can navigate it. Use this skeleton — fill the sections that apply, omit the ones that don't.

```
## What changes
<one paragraph — the user-visible or behavioral delta, not a file list>

## Reviewer's guide
Read in this order:
1. [path/to/file_a](PR-DIFF-URL) — start here. <one line: what to look for>
2. [path/to/file_b_test](PR-DIFF-URL) — acceptance for the above.
3. [path/to/file_c](PR-DIFF-URL) — wiring / mechanical.

Skim or skip: <generated, snapshots, fixtures, lockfiles>.

## Diff groups (only if the diff mixes intents — omit for single-purpose PRs)
- Production: A, B
- Tests: C, D
- Mechanical refactor / generated: E

## How it works (pseudocode walkthrough) — include when the core logic isn't obvious from the diff
<Language-agnostic pseudocode of the main flow (parse -> transform -> emit, the key loop,
the non-obvious algorithm), so a cold reviewer follows the logic without reading every line.
Keep this in the PR description, NOT as comments in the code — it respects the repo's
"default to no comments" rule while still explaining the "how". Fold the one or two things a
reviewer would otherwise pause on into the pseudocode. Omit for trivial or purely mechanical
changes that have no algorithm worth narrating.>

## Decision log
- Chose X over Y because <reason>. (Preempt the obvious "why didn't you just…" question.)

## Risk / blast radius
- Affects: <consumers, paths, services>
- Tests covering this: <which>
- Be paranoid about: <areas a reviewer should pressure-test>

## Before / after (default for any observable change — show it, don't just describe it)
<Include an actual before/after artifact so a reviewer SEES what the PR changed. Choose the
artifact TYPE that best fits this change — there is no single required format:
  - behavioral/CLI change -> paste the real command/tool output, before vs after;
  - data/schema change -> the actual values or a small table, before vs after;
  - perf change -> the numbers;
  - structural change -> a mermaid diagram;
  - visual/rendering change -> a rendered image pair (and only here: use PNG, since many
    hosts incl. GitHub sanitize SVG in PR bodies).
Capture it by running on the base branch, then the PR branch. Prefer real captured output
over prose. Only omit when the change is genuinely non-observable (a pure internal refactor
with identical behavior). Project-specific artifact conventions may live in the repo's
CLAUDE.md — follow them.>

## Out of scope (deliberately deferred)
- <thing> → <issue link or "filed as #N">
```

### Mermaid diagrams (before / after)

When the change reshapes structure or flow that prose can't cheaply convey, add a `mermaid` diagram under `## Before / after` so the reviewer sees the shape of the change, not just the file list.

- **When to add one.** The change alters control flow, data flow, call sequence, module/dependency wiring, a state machine, or a schema relationship. Skip cosmetic edits, single-file leaf changes, and purely additive changes with no structural effect. Don't force a diagram onto a PR that doesn't have a shape worth drawing.
- **Before / after.** Prefer a before/after pair (two fenced blocks, or two labelled subgraphs in one block) so the delta is visible at a glance. Diff the topology: draw the nodes the PR touches plus enough neighbours to orient, not the whole system. When you can't cheaply reconstruct the "before" (common at create-time, before the diff is finalized), a single "after" diagram is fine.
- **Pick the type that fits.** `flowchart`/`graph` for wiring and control flow, `sequenceDiagram` for call ordering across components, `stateDiagram-v2` for lifecycle/state changes, `erDiagram` for schema shape.
- **Verify it renders.** GitHub renders fenced ` ```mermaid ` blocks. Confirm the syntax parses before pushing: mismatched brackets, unquoted labels with special characters, and a bare lowercase `end` node all break the render. The anchor pass ignores mermaid blocks, so diagrams survive `~/personal/anchor_pr_files` untouched.

**File links must point to the PR-scoped diff**, not to the file on the branch (the branch view shows the whole file including unchanged content — useless for review). The correct form is:

```
https://github.com/OWNER/REPO/pull/N/files#diff-<sha256(filepath)>
```

The PR number `N` doesn't exist until after `gh pr create`, so PR creation in `pr` mode is a **two-pass flow**:

**Pass 1 — create.** Open the PR with the description in its initial form: file paths in plain backticks (e.g. `` `src/foo.go` ``), no links yet. Capture the PR number from `gh pr create` output, or read it back with `gh pr view --json number --jq .number`.

**Pass 2 — anchor.** Run `~/personal/anchor_pr_files` (no args needed; defaults to the current branch's PR). It substitutes the placeholder paths in the `## Reviewer's guide` section with proper diff-anchored markdown links and pushes the updated body via `gh pr edit`. Idempotent — safe to rerun. For PRs you can't edit, use `--print` to emit a `path → URL` table to stdout instead.

If anything in the diff would surprise a cold reader (non-obvious choice, subtle edge case, intentional weirdness), leave a self-comment on that line in the PR before requesting review. Anchors the explanation to the actual code instead of burying it in the description.

## Follow-ups (delegate, don't reinvent)

- Deferred work or latent bugs spotted in passing → file via `/ghissue` or `/gap-track` immediately. *Why: intentional scope reduction is fine; forgetting is a liability.*
- Doc / roadmap / changelog updates after merge → `/checkpoint`.
- Constraint validation against `CONSTRAINTS.md` → `/stack-audit`.

---

$ARGUMENTS
