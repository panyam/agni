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

- [CONSTRAINTS.md](CONSTRAINTS.md) holds the enforceable architectural rules (C1 to C29). They keep
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

- **Check the branch again immediately before you COMMIT, not only before you branch.** A long
  session's branch can move underneath it: running the `/checkpoint` skill mid-feature switches the
  checkout back to `main`, so the next `git commit` lands the feature there instead. The push then
  sends the feature branch at its OLD position and succeeds, and the failure surfaces much later as
  `gh pr create` reporting "No commits between main and <branch>". Recovery is cheap when the tree is
  clean (`git branch -f <branch> <sha>`, `git branch -f main origin/main`), so the cost is entirely in
  not noticing. `git branch --show-current` before the commit is the whole fix.
- **Verify a push by its EXIT CODE, never by grepping its output.** `git push | tail -1` swallows a
  failure, and `git push 2>&1 | grep <branch>` reports success on a FAILED push, because the branch
  name appears inside the failure message. Run `git push; echo "EXIT=$?"`.
- **`Closes #A and #B` closes only A.** GitHub parses the keyword PER ISSUE, so a PR fixing two
  tickets needs `Closes #A, closes #B`. The second issue stays open and silently reads as unfinished
  work while its fix is already in `main`. Check both after the merge rather than trusting the body.
- **Verify `merged: true` via the API before ANY post-merge cleanup**
  (`gh api repos/.../pulls/N --jq .merged`). A PR once sat closed-UNMERGED while everyone believed
  it had merged, and `git branch -d` still allowed the local delete, because it checks the tracking
  ref rather than main.
- **RETARGET A STACKED PR BEFORE MERGING ITS PARENT.** This repo deletes a branch on merge, and
  GitHub auto-CLOSES any PR still pointing at the deleted branch, so merging the parent silently
  closes the child. Recovering it is worse than it sounds, because the two obvious moves refuse each
  other: you cannot reopen a PR whose base branch is gone, and you cannot change the base of a closed
  PR. The way out is to recreate the base ref at the exact commit the parent merged from
  (`gh api repos/OWNER/REPO/git/refs -f ref='refs/heads/<base>' -f sha=<FULL 40-char sha>`), reopen,
  retarget to `main`, then delete the ref again, which does not re-close it because it is no longer
  the base. `gh pr reopen` needs the FULL sha; an abbreviated one fails with a 422 that does not say
  so. Nothing is lost either way, since the branch itself survives, but check the tip against the PR
  head before assuming that.
- **Level the two branches before opening a stacked PR, parent first.** Merging `main` into the child
  and the parent independently creates divergent merge commits, and the stacked diff then shows
  `main`'s commits as if they were the child's changes (26 files instead of 23, with
  `mergeable: false`). Merge `main` into the parent, push, then merge the parent forward into the
  child.
- **A merged PR does not mean the BRANCH is merged. Check the tip against what the PR merged.**
  A commit pushed to the branch after the merge is stranded: the PR reads merged, GitHub offers to
  delete the branch, `git branch -d` accepts it, and the work is gone with nothing anywhere saying
  so. Two of roughly twenty branches audited had done this, costing 135 lines of documentation that
  were recovered only because someone asked whether a branch was stale. Compare
  `gh api repos/.../pulls/N --jq .head.sha` against `git rev-parse origin/<branch>`, or sweep the
  repo with

      for b in $(git for-each-ref --format='%(refname:short)' refs/remotes/origin/); do
        n=$(git rev-list --count origin/main..$b); [ "$n" != 0 ] && echo "$n ahead: $b"
      done

  Anything ahead of `main` whose PR already merged is the shape to look at. Content can still be in
  `main` by another route, so confirm by grepping the branch's added lines against `main` rather
  than by trusting the count, and validate that grep on lines you know ARE in `main` before
  believing a zero.
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
- **The same expansion happens in an UNQUOTED heredoc, including one feeding a script.** `<<EOF`
  expands; `<<'EOF'` does not. A PR body written through `python3 - <<PY` lost every inline-code span
  and both fenced blocks, because the shell ate the backticks before python ever saw the text, and it
  additionally RAN the `> review.html` redirect inside an example command and left the empty file in
  the tree. The edit reported success, so the damage was only visible by reading the result back.
  Quote the delimiter whenever the body is prose, and read the output back before pushing it.
- **zsh does NOT word-split an unquoted `$var`.** `files=$(ls ...)` then `for f in $files` iterates
  ONCE over the whole blob, and a sweep that did this compared one file and reported success. Glob
  directly in the `for`, or use an array.
- **To undo a temporary red-check edit, reverse it with the tool that made it, never `git checkout`.**
  `git checkout <file>` and `git checkout HEAD -- <file>` restore from a COMMIT, not from "before I
  typed that", so on a file carrying uncommitted work they destroy all of it including the change the
  red-check was proving. This cost real work three times in one session. Commit first, even a WIP
  commit, before any red-check pass; then break and un-break with the same replace run applied
  backwards. The symptom is confusing rather than obvious, since a test that just passed starts
  failing and the cause looks like the change rather than the undo.
- **Use `git worktree add <tmp> main`, never `git stash`, to reconstruct a whole BEFORE state** such
  as a "before" binary. Stash leaves untracked new files on disk referencing stashed-away code.
  Stashing ONE tracked file (`git stash push -- path/to/file.go`, run the test, pop) is a different
  move and a good one, since it is how you prove a new test is red for the reason you think rather
  than red because the package does not compile. What that red-check tells you, and the two ways it
  misleads, is on the [evidence page](docsite/content/build/evidence.md).

## PR prose conventions

The shape of a PR body is defined by the **`start_pr` skill**, distributed separately from this
repo: what changes, prerequisite knowledge, a reviewer's guide that opens with a one-paragraph ELI
and carries it through the reading order, a decision log, and a before/after artifact. Follow the
skill rather than a copy kept here, because a copy drifts and this one already had.

What follows is only what agni adds on top, because this engine sits where electrical engineering
meets software and most reviewers are strong in one and cold in the other. Two extra sections slot
into the skill's skeleton ahead of the reviewer's guide, so the local ramp reads: what changes, the
circuit, the hardware primer, prerequisite knowledge, then the reviewer's guide.

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
- **Which pages the prerequisite block names.** Pull from `learn/` for domain knowledge,
  `tutorials/` for tool usage, `architecture/` for design rationale, `build/` for extending the
  engine. When the PR is about a rule, prefer the LEARN chapter over the rule's catalog entry: the
  entry explains the check, the chapter explains the instinct. Saying that no page covers the change
  is a real entry rather than an empty one, and several chapters exist because someone wrote it.
- **Link a prerequisite page to the PUBLISHED site**, not to a repo path, since a relative path does
  not resolve in a PR body. The site is `https://panyam.github.io/agni/`, one URL per page at
  `/<section>/<page-basename-without-.md>/`, and a heading anchor is the slugified heading. When the
  PR CHANGES a page it names, link both: the published page reads better, and it serves `main`, so
  the reviewer needs the PR-diff link to see the new text. Saying which is which takes a clause and
  saves a reviewer reading the version the PR exists to replace.
- **ELI analogies that have carried a PR here.** Fire extinguishers for a protection radius, a
  wiring diagram vs a floor plan for connections vs pin declarations, game mods for the registration
  vs authoring seam.
- **A mermaid label must contain no quote characters, escaped or otherwise.** `&quot;` inside a
  `["..."]` node label decodes to a bare `"`, closes the string early, and GitHub renders a parse
  error instead of the diagram (`got 'STR'`). Write labels as plain text with `<br/>` for line
  breaks and keep the literal strings in the prose above the diagram, which reads better anyway.
  Parse-check before you post rather than after: extract each ` ```mermaid ` block to a file,
  DECODE its HTML entities, and run `mmdc -i block.mmd -o block.svg`. Two things about that command
  are load-bearing, and the version of this note that shipped first got both wrong. **`-o /dev/null`
  does not work**, because mmdc rejects any output path not ending `.md`, `.markdown`, `.svg`, `.png`
  or `.pdf`, so it exits non-zero on every diagram and reports a failure that says nothing about the
  syntax. And **the decode is what makes the check reproduce GitHub**: mmdc reads `&quot;` as literal
  text and parses it happily, so checking the raw block PASSES the exact diagram this rule exists to
  catch, while GitHub decodes it to a bare `"` before mermaid sees it. Decoded, the same label fails
  with the `got 'STR'` above. Looking at the rendered PR is still the only way to catch a diagram
  that parses and reads badly.
- **Rendering captures come from a fixture, never a customer board.** Capture the before/after pair
  from the showcase boards (`cmd/agni/testdata/conformance/showcase.{passes,fires}.kicad_*`),
  `examples/tutorial-project/`, or a tiny hand-authored fixture, for the reasons in `CLAUDE.md`
  under "What does not belong in this repo". A browser-automation screenshot lands in the CWD, which
  is the REPO ROOT, and a bare `.png` there is not ignored (`bin/` and `.playwright-mcp/` are), so
  move captures out of the tree before staging and stage by explicit path. That is the stray-file
  trap above, narrowed to one file type. To embed the pair, commit the images to `pr-assets/`,
  reference them as `.../raw/<full-SHA>/pr-assets/x.png` pinned to the commit rather than the
  branch, then `git rm` them in a follow-up commit so they never reach `main`.
