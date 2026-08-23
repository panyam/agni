# Contributing to agni

Thanks for your interest. This is an early open-source project and contributions are
welcome, especially readers for new formats and check rules.

## Getting set up

Requires Go 1.26 and pnpm (for the web viewer bundle).

```
cd web && pnpm install && cd ..   # once
make build                        # web bundle + go build ./...
make testall                      # the full gate CI runs: vet, tests, bundle, web unit tests
```

`make testall` is the gate. A green run means vet passed, the engine tests passed, the
example modules built, the web bundle built, and the web unit tests passed. CI runs exactly
this, so run it locally before opening a pull request.

## Read this first

- [CONSTRAINTS.md](CONSTRAINTS.md) — the enforceable architectural rules (C1–C21). They keep
  the engine format-neutral and the layering clean. Read them before proposing a change; a PR
  that violates one will be asked to change or to justify amending the constraint.
- [Docs site overview](https://panyam.github.io/agni/overview/) — the engineering docs. The
  architecture is the settled part of the project.

## Common contributions

- **A new format reader.** Each reader is its own package exposing
  `Read(io.Reader, sourceFile) (*ir.Design, error)`, wired in with one entry in
  `readers/formats/registry.go`. See the reader notes in [Ingestion and IR](https://panyam.github.io/agni/architecture/ingestion-and-ir/)
  and reconcile new concepts against the cross-format map (CONSTRAINTS C9). Ship a runnable
  example with it (CONSTRAINTS C10).
- **A new check rule.** One `stdlib/rules/builtin/rule_<name>.go`, one line in
  `stdlib/rules/builtin/register.go`, and one `stdlib/rules/builtin/docs/<name>.md` (the source
  of the rule's prose, enforced 1:1 by `stdlib/rules/builtin/docs_test.go`). The practical
  walkthrough is [Authoring a check rule](https://panyam.github.io/agni/build/check-rule/).
- **A datalog query relation or example.** See [Querying](https://panyam.github.io/agni/guide/querying/).
- **An interface profile.** A profile is a data value, not code: a YAML declaration of an
  interface's signals and the checks it requires, compiled into datalog rules. Built-ins live in
  `stdlib/profiles/builtins/*.yaml`; an out-of-tree one loads through `agni check --profile-path
  <dir>`. A signal declares exactly one net-name matcher — `suffix` (optionally narrowed by a
  conjunctive `prefix`), `glob`, or `regex` — documented in `stdlib/profiles/matcher.go`. Reach for
  `suffix` first; it is the readable convention. Reach for `glob` or `regex` only when the bus
  identity is the prefix and the suffix is shared with an unrelated bus, which is the one case
  suffix matching cannot express.

## Expectations for a pull request

- Tests. New behavior gets a test; readers get hand-authored fixtures in the package's
  `testdata/`.
- Docs. A new rule needs its `check/docs/<name>.md`; a user-facing change updates the
  relevant page under `docsite/content/`.
- Keep it format-neutral. Analyses read the IR, not source files. Don't add an IR field only
  one format would populate (CONSTRAINTS C9).
- Prose style in docs, commit messages, and PR text: plain declarative sentences, no
  marketing cadence, no em-dashes.

## Reporting bugs and requesting formats

Open an issue. For a bug, a small design file that reproduces it helps a lot (a hand-authored
fixture rather than a confidential design). For a new format, a pointer to the spec and a
sample export is the best start.

## Working across multiple checkouts

Concurrent sessions work against separate clones (or worktrees) of this repo, one per lane of work.

- **Do NOT run two sessions in one checkout.** They share HEAD, so a branch switch or commit in one
  thrashes the other.
- **A checkout being "yours by topic" does not mean it is free.** Before `git checkout -b`, run
  `git branch --show-current` and a FULL `git status`, never piped through `head` (that is how the
  evidence got truncated once). Another session may be mid-work there.
- **Use `git -C /abs/path <cmd>` for every git command.** Shell cwd persists across tool calls and
  WILL drift between repos. Four wrong-repo branches were created from stale cwds despite a `cd`
  discipline. `-C` cannot drift.
- **Commit only your slice, by explicit path. Never `git add -A` / `git add -u`.**
- **`git add` by path does NOT protect a SHARED checkout.** `git commit` commits the whole index,
  including what another session staged. Use an explicit pathspec (`git commit -m "..." -- <paths>`)
  or check `git status` for foreign staged entries first.
- **A pathspec commit silently drops your OWN late edits, and whole directories you forgot to list.**
  A file changed after you wrote the list just stays dirty and the PR merges without it. So does a
  directory you never named: a commit listing `protos/ gen/ internal/ docsite/` dropped a one-line
  `cmd/` call-site fix, `make testall` passed locally because the WORKING TREE had it, and CI on the
  pushed branch failed to compile. Local green proves nothing about a pathspec commit. After every
  one, `git status` and account for each remaining dirty file — and prefer staging exactly your slice
  and committing WITHOUT a pathspec when the change spans more than two directories.
- **A pathspec commit takes WORKING-TREE content, overriding the index**, so a staged
  `git rm --cached` named in the pathspec gets recommitted instead of deleted. For deletions and
  untracking, stage exactly the intended slice and commit WITHOUT a pathspec.
- **`make proto` regenerates every proto**, dirtying files a parallel session may own. Stage only
  your proto's generated file. If a regen dirties a file whose proto you did not touch, another
  checkout committed stale generated output. `git checkout` it and tell that session rather than
  adopting it.
- **Pull after a push.** Shared schema files (`protos/`) are coordination points.

## PR workflow

- **Verify a push by its EXIT CODE, never by grepping its output.** `git push | tail -1` swallows a
  failure, and `git push 2>&1 | grep <branch>` reports success on a FAILED push because the branch
  name appears inside the failure message. Run `git push; echo "EXIT=$?"`.
- **Verify `merged: true` via the API before ANY post-merge cleanup**
  (`gh api repos/.../pulls/N --jq .merged`). A PR once sat closed-UNMERGED while everyone believed
  it had merged, and `git branch -d` still allowed the local delete because it checks the tracking
  ref, not main.
- **NEVER `gofmt -w` a whole directory.** This repo's committed import blocks are not gofmt-sorted,
  so a directory-wide run silently reordered imports in 18 untouched files. Format only files you
  edited, and read `git status` before staging. **Formatting a SINGLE file is not safe either**: if
  your change adds an import, `gofmt -w` on that one file re-sorts its whole import block, which
  reads as an unrelated hunk in the diff. Re-read the block after formatting and restore the
  original order.
- **Out-of-scope items survive the PR in `OUT_OF_SCOPE.md`.** A PR body's "Out of scope" section is
  invisible once merged, so every deferred item gets a one-liner there (source PR + pickup trigger)
  in the same PR. Ticket-worthy items skip the ledger and get a GitHub issue instead: the ledger is
  only for work nobody could pick up deliberately, which becomes cheap the moment an unrelated
  trigger fires. A question that was asked and ANSWERED is neither — it can never be closed, so it
  goes in `DECISIONS.md` instead of quietly filling the ledger with entries nobody can act on.

Four shell traps that have burned real work. **A `cd` persists across calls, and the repo root is
where a stray file lands.** A scratch file written to a relative path goes wherever the last `cd`
left you, so a heredoc that was meant for a temp directory can drop a `.md` or a `.png` into the
tree, and a directory `add` then rides it into a public repo. Write scratch files to an absolute
path, and read `git status` before staging. (The screenshot case under PR prose conventions below is
this same trap, narrowed to one file type.) **Backticks inside a double-quoted `git commit -m` run
as COMMAND SUBSTITUTION**, so a message quoting `` `reserved` `` committed the sentence with the word
missing and no error. Anything with backticks, `$`, or `!` goes through `git commit -F <file>` or a
quoted heredoc (`<<'MSG'`), never `-m "..."`. **zsh does NOT word-split an unquoted `$var`**, so
`files=$(ls ...)` then `for f in $files` iterates ONCE over the whole blob (a sweep silently compared
one file and reported success). Glob directly in the `for`, or use an array. And use
**`git worktree add <tmp> main`, never `git stash`**, to build a "before" binary: stash leaves
untracked new files on disk referencing stashed-away code. That is about reconstructing a whole
BEFORE state. Stashing ONE tracked file (`git stash push -- path/to/file.go`, run the test, pop) is
a different move and a good one: it is how you prove a new test is red for the reason you think,
rather than red because the package does not compile. What that red-check tells you, and the two ways
it misleads, is on the [evidence page](https://github.com/panyam/agni/blob/main/docsite/content/build/evidence.md).

## PR prose conventions

This engine sits where electrical engineering meets software, and most reviewers are strong in one
and cold in the other. These sections exist so a PR is reviewable by both.

- **ELI12, on every PR, no exceptions.** An `## ELI12` section right after "What changes", carrying
  the load-bearing idea through ONE concrete everyday analogy (fire extinguishers for a protection
  radius; a wiring diagram vs a floor plan for connections vs pin declarations; game mods for the
  registration vs authoring seam). It explains the IDEA, not the diff, and it is aimed at a smart
  reader with no context on this codebase or this domain. Test the analogy against the diff's actual
  edge case first, because one that breaks under pressure teaches the reviewer something false.
- **Hardware primer.** Every PR whose logic touches hardware concepts gets a `## Hardware context
  (for software readers)` section BEFORE the reviewer's guide: each EE term the diff leans on mapped
  to a STRUCTURAL software analogy (series element = inline middleware that splits a net; rail =
  global singleton the walk must not follow an import into; junction dot = explicit join marker in
  whitespace-significant syntax; TVS = pressure-relief valve beside the path, not inline). Define
  only what the PR actually uses, where a cold reviewer needs it. This does not replace the ELI12,
  which sits above it and makes the whole PR approachable.
- **Prerequisite reading, named.** A `## Prerequisites` block after the ELI12 and before the
  reviewer's guide, listing two to four docsite pages a reviewer would need to evaluate the change,
  each with a clause saying what it supplies. Pull from `learn/` for domain knowledge, `tutorials/`
  for tool usage, `architecture/` for design rationale, `build/` for extending the engine. Prefer the
  LEARN chapter over the rule's own catalog entry when the PR is about a rule: the catalog entry
  explains the check, the chapter explains the instinct behind it. Omit the block when nothing
  genuinely applies, since a forced entry is worse than none, and **say so explicitly when no page
  covers the change** — that is a signal the course has a gap, and several chapters came from exactly
  that observation.
- **The circuit, for software readers.** When semantics depend on hardware behavior (a rule's
  electrical meaning, derating, rail/pin conventions, why a limit matters physically), add this
  section right after "What changes", plus a link to `docsite/content/reference/analogy.md`. A
  pseudocode walkthrough alone leaves the hardware nouns opaque.
- **A mermaid label must contain no quote characters, escaped or otherwise.** `&quot;` inside a
  `["..."]` node label decodes to a bare `"`, closes the string early, and GitHub renders a parse
  error instead of the diagram (`got 'STR'`). Write labels as plain text with `<br/>` for line
  breaks and keep the literal strings in the prose above the diagram, which reads better anyway.
  Parse-check before you post rather than after: extract each ` ```mermaid ` block to a file and run
  `mmdc -i block.mmd -o /dev/null`, which exits non-zero on the syntax GitHub would reject. That
  catches the whole class without a round trip. Looking at the rendered PR is still the only way to
  catch a diagram that parses and reads badly.
- **Visual before/after.** Any PR changing rendering includes an actual image pair under
  `## Before / after`. **PNG, not SVG**, because GitHub sanitizes SVG in PR bodies. Capture from the
  showcase boards (`cmd/agni/testdata/conformance/showcase.{passes,fires}.kicad_*`),
  `examples/tutorial-project/`, or a tiny hand-authored fixture. **Never from a real customer
  board**, for the reasons below.
  A browser-automation screenshot lands in the CWD, which is the REPO ROOT, and a bare `.png` there
  is not ignored (`bin/` and `.playwright-mcp/` are). Move captures out of the tree before staging
  and stage by explicit path, or a stray screenshot rides into a public repo on a directory `add`.
  To embed the pair in the PR body, use the committed-asset + pinned-raw-URL pattern from the global
  CLAUDE.md: commit to `pr-assets/`, reference `.../raw/<full-SHA>/pr-assets/x.png`, then `git rm` it
  in a follow-up commit so it never reaches `main`.
