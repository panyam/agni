# Contributing to agni

Thanks for your interest. This is an early open-source project and contributions are
welcome, especially readers for new formats and check rules.

## Getting set up

Requires Go 1.26 and pnpm (for the web viewer bundle).

```
cd web && pnpm install && cd ..   # once
make build                        # web bundle + go build ./...
make testall                      # the full gate CI runs
```

`make testall` is the gate and CI runs exactly it, so run it locally before opening a pull request.
Green means vet passed, the engine tests passed, the example modules built, the web bundle built,
and the web unit tests passed.

## Read this first

- [CONSTRAINTS.md](CONSTRAINTS.md) holds the enforceable architectural rules (C1 to C26). They keep
  the engine format-neutral and the layering clean. Read them before proposing a change. A PR that
  violates one will be asked to change, or to justify amending the constraint.
- The [docs site](https://panyam.github.io/agni/overview/) is the engineering source of truth, and
  the architecture is the settled part of the project.

## Common contributions

- **A new format reader.** Each reader is its own package exposing
  `Read(io.Reader, sourceFile) (*ir.Design, error)`, wired in with one entry in
  `readers/formats/registry.go`. See the reader notes in
  [Ingestion and IR](https://panyam.github.io/agni/architecture/ingestion-and-ir/) and reconcile new
  concepts against the cross-format map (C9). Ship a runnable example with it (C10).
- **A new check rule.** One `stdlib/rules/builtin/rule_<name>.go`, one line in
  `stdlib/rules/builtin/register.go`, and one `stdlib/rules/builtin/docs/<name>.md`, which is the
  source of the rule's prose and is enforced 1:1 by `stdlib/rules/builtin/docs_test.go`. The
  practical walkthrough is
  [Authoring a check rule](https://panyam.github.io/agni/build/check-rule/).
- **A datalog query relation or example.** See
  [Querying](https://panyam.github.io/agni/guide/querying/).
- **An interface profile.** A profile is a data value rather than code: a YAML declaration of an
  interface's signals and the checks it requires, compiled into datalog rules. Built-ins live in
  `stdlib/profiles/builtins/*.yaml`, and an out-of-tree one loads through
  `agni check --profile-path <dir>`. A signal declares exactly one net-name matcher, documented in
  `stdlib/profiles/matcher.go`. Reach for `suffix` first (optionally narrowed by a conjunctive
  `prefix`), since it is the readable convention. Reach for `glob` or `regex` only when the bus
  identity is the prefix and the suffix is shared with an unrelated bus, which is the one case
  suffix matching cannot express.

## Expectations for a pull request

- Tests and docs. New behavior gets a test, readers get hand-authored fixtures in the package's
  `testdata/`, and a user-facing change updates the relevant page under `docsite/content/`.
- Keep it format-neutral. Analyses read the IR, not source files. Do not add an IR field only one
  format would populate (C9).
- Prose in docs, commit messages, and PR text: plain declarative sentences, no marketing cadence, no
  em-dashes. PR bodies additionally follow the prose conventions at the end of this file.

## Reporting bugs and requesting formats

Open an issue. For a bug, a small design file that reproduces it helps a lot, hand-authored rather
than taken from a confidential design. For a new format, a pointer to the spec and a sample export
is the best start.

---

**The rest of this file is operating discipline for maintainers and coding agents running several
checkouts of this repo at once.** An outside contributor sending a single PR does not need it. Every
rule below is here because it cost real work at least once.

## Working across multiple checkouts

Concurrent sessions work against separate clones (or worktrees) of this repo, one per lane of work.

- **Do NOT run two sessions in one checkout.** They share HEAD, so a branch switch or commit in one
  thrashes the other.
- **A checkout being "yours by topic" does not mean it is free.** Before `git checkout -b`, run
  `git branch --show-current` and a FULL `git status`, never piped through `head`. That is how the
  evidence got truncated once. Another session may be mid-work there.
- **Use `git -C /abs/path <cmd>` for every git command.** Shell cwd persists across tool calls and
  WILL drift between repos. Four wrong-repo branches were created from stale cwds despite a `cd`
  discipline, and `-C` cannot drift.
- **Commit only your slice, by explicit path. Never `git add -A` or `git add -u`.** In a SHARED
  checkout that alone is not enough, because `git commit` commits the whole index including what
  another session staged. Use an explicit pathspec (`git commit -m "..." -- <paths>`), or check
  `git status` for foreign staged entries first.
- **A pathspec commit then has two failure modes of its own.** It silently drops your OWN late edits
  and any directory you forgot to list: a commit naming `protos/ gen/ internal/ docsite/` dropped a
  one-line `cmd/` call-site fix, `make testall` passed locally because the WORKING TREE had it, and
  CI failed to compile on the pushed branch. Local green proves nothing about a pathspec commit. It
  also takes WORKING-TREE content over the index, so a staged `git rm --cached` named in the
  pathspec gets recommitted instead of deleted. After every pathspec commit, run `git status` and
  account for each remaining dirty file. When the change spans more than two directories, or
  involves a deletion or an untracking, stage exactly your slice and commit WITHOUT a pathspec.
- **`make proto` regenerates every proto**, dirtying files a parallel session may own. Stage only
  your proto's generated file. If a regen dirties a file whose proto you did not touch, another
  checkout committed stale generated output. `git checkout` it and tell that session rather than
  adopting it.
- **Pull after a push.** Shared schema files (`protos/`) are coordination points.

## PR workflow

- **Verify a push by its EXIT CODE, never by grepping its output.** `git push | tail -1` swallows a
  failure, and `git push 2>&1 | grep <branch>` reports success on a FAILED push, because the branch
  name appears inside the failure message. Run `git push; echo "EXIT=$?"`.
- **Verify `merged: true` via the API before ANY post-merge cleanup**
  (`gh api repos/.../pulls/N --jq .merged`). A PR once sat closed-UNMERGED while everyone believed
  it had merged, and `git branch -d` still allowed the local delete, because it checks the tracking
  ref rather than main.
- **NEVER `gofmt -w` a whole directory.** This repo's committed import blocks are not gofmt-sorted,
  so a directory-wide run silently reordered imports in 18 untouched files. **A SINGLE file is not
  safe either**, since adding an import makes `gofmt -w` re-sort that file's whole block, which
  reads as an unrelated hunk. Format only what you edited, then re-read the block and restore the
  original order.
- **Deferred work survives the PR in `OUT_OF_SCOPE.md`.** A PR body's "Out of scope" section is
  invisible once merged, so every deferred item gets a one-liner there (source PR plus pickup
  trigger) in the same PR. The ledger is only for work nobody would pick up deliberately, which
  becomes cheap the moment an unrelated trigger fires. Ticket-worthy items get a GitHub issue
  instead, and a question that was asked and ANSWERED goes in `DECISIONS.md`, since it can never be
  closed.

### Shell traps

- **A `cd` persists across calls, and the repo root is where a stray file lands.** A scratch file
  written to a relative path goes wherever the last `cd` left you, so a heredoc meant for a temp
  directory can drop a `.md` or a `.png` into the tree, and a directory `add` then rides it into a
  public repo. Write scratch files to an absolute path, and read `git status` before staging.
- **Backticks inside a double-quoted `git commit -m` run as COMMAND SUBSTITUTION.** A message
  quoting `` `reserved` `` committed the sentence with the word missing and no error. Anything with
  backticks, `$`, or `!` goes through `git commit -F <file>` or a quoted heredoc (`<<'MSG'`), never
  `-m "..."`.
- **zsh does NOT word-split an unquoted `$var`.** `files=$(ls ...)` then `for f in $files` iterates
  ONCE over the whole blob, and a sweep that did this compared one file and reported success. Glob
  directly in the `for`, or use an array.
- **Use `git worktree add <tmp> main`, never `git stash`, to reconstruct a whole BEFORE state** such
  as a "before" binary. Stash leaves untracked new files on disk referencing stashed-away code.
  Stashing ONE tracked file (`git stash push -- path/to/file.go`, run the test, pop) is a different
  move and a good one, since it is how you prove a new test is red for the reason you think rather
  than red because the package does not compile. What that red-check tells you, and the two ways it
  misleads, is on the [evidence page](docsite/content/build/evidence.md).

## PR prose conventions

This engine sits where electrical engineering meets software, and most reviewers are strong in one
and cold in the other. These sections exist so a PR is reviewable by both, and they work as a ramp
rather than as a checklist. The order carries the reviewer from the domain nouns, through the idea,
into the code:

1. `## What changes`
2. `## The circuit, for software readers`, when the semantics depend on hardware behavior
3. `## Hardware context (for software readers)`, when the diff leans on EE terms
4. `## Prerequisite knowledge`
5. `## Reviewer's guide`, opening with the ELI paragraph

- **The circuit, for software readers.** When semantics depend on hardware behavior (a rule's
  electrical meaning, derating, rail/pin conventions, why a limit matters physically), add this
  section right after "What changes", plus a link to `docsite/content/reference/analogy.md`. A
  pseudocode walkthrough alone leaves the hardware nouns opaque.
- **Hardware primer.** Every PR whose logic touches hardware concepts gets a `## Hardware context
  (for software readers)` section, mapping each EE term the diff leans on to a STRUCTURAL software
  analogy (series element = inline middleware that splits a net; rail = global singleton the walk
  must not follow an import into; junction dot = explicit join marker in whitespace-significant
  syntax; TVS = pressure-relief valve beside the path, not inline). Define only what the PR actually
  uses. It does not replace the ELI paragraph that follows, since the primer supplies the nouns and
  the ELI supplies the idea.
- **Prerequisite reading, named.** A `## Prerequisite knowledge` block before the reviewer's guide,
  listing two to four docsite pages a reviewer would need to evaluate the change, each with a clause
  saying what it supplies. Pull from `learn/` for domain knowledge, `tutorials/` for tool usage,
  `architecture/` for design rationale, `build/` for extending the engine. When the PR is about a
  rule, prefer the LEARN chapter over the rule's catalog entry: the entry explains the check, the
  chapter explains the instinct. Omit the block when nothing applies, and **say so explicitly when
  no page covers the change**, which is a signal the course has a gap. Several chapters came from
  exactly that observation.
- **ELI12, on every PR, no exceptions.** One paragraph opening the `## Reviewer's guide`, ahead of
  the numbered reading order, carrying the load-bearing idea through ONE concrete everyday analogy
  (fire extinguishers for a protection radius; a wiring diagram vs a floor plan for connections vs
  pin declarations; game mods for the registration vs authoring seam). It explains the IDEA rather
  than the diff, aimed at a smart reader with no context on this codebase or this domain. Test the
  analogy against the diff's actual edge case first, because one that breaks under pressure teaches
  the reviewer something false. Then carry it into the reading order, phrasing a file's one-line
  note in the analogy's terms wherever that reads clearer, so the idea is still working by step
  three. Drop it on lines where it does not fit rather than forcing it. If the idea needs more than
  a paragraph, either it is not yet reduced to its load-bearing part or the PR wants splitting.
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
  `examples/tutorial-project/`, or a tiny hand-authored fixture, **never from a real customer
  board** (see "What does not belong in this repo" in `CLAUDE.md`). A browser-automation screenshot
  lands in the CWD, which is the REPO ROOT, and a bare `.png` there is not ignored (`bin/` and
  `.playwright-mcp/` are), so move captures out of the tree before staging and stage by explicit
  path. That is the stray-file trap above, narrowed to one file type. To embed the pair, use the
  committed-asset and pinned-raw-URL pattern from the global CLAUDE.md: commit to `pr-assets/`,
  reference `.../raw/<full-SHA>/pr-assets/x.png`, then `git rm` it in a follow-up commit so it never
  reaches `main`.
