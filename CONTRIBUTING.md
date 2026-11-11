# Contributing to agni

Thanks for your interest. This is an early open-source project and contributions are
welcome, especially readers for new formats and check rules.

## Getting set up

`make build` requires Go 1.26.9 and pnpm (for the web viewer bundle). `make testall` also needs
`buf` 1.61 on your PATH, `python3` for the Python client tests, a Chromium for the browser tests
(`cd web && pnpm exec playwright-core install chromium`), and network access to fetch the pinned
sample corpus.

```
pnpm install                      # once: the web workspace (web/, web-shared/, datasheet/web/)
make build                        # web bundle + go build ./...
make testall                      # the full gate CI runs
```

`make testall` is the gate and CI runs exactly it, so run it locally before opening a pull request.
Green means vet passed, the generated code and every module's `go.mod` are current, the engine,
example, web, browser and Python tests passed, and the generated docs and tutorial captures match
what the engine produces now. [The gate page](https://panyam.github.io/agni/build/the-gate/) has
the full list and the ways a run misreads.

## Read this first

- [CONSTRAINTS.md](CONSTRAINTS.md) holds the enforceable architectural rules (C1 to C36). They keep
  the engine format-neutral and the layering clean. Read them before proposing a change. If a PR
  violates one, the review asks it to change, or to justify amending the constraint.
- The [docs site](https://panyam.github.io/agni/overview/) is the engineering source of truth, and
  the architecture is the settled part of the project.

## Common contributions

- A **new format reader** is its own package exposing
  `Read(io.Reader, sourceFile) (*ir.Design, error)`, wired in with one entry in
  `readers/formats/registry.go`. See
  [Adding a format reader](https://panyam.github.io/agni/build/format-reader/) and the reader notes
  in [Ingestion and IR](https://panyam.github.io/agni/architecture/ingestion-and-ir/). Reconcile new
  concepts against the cross-format map (C9). Ship a runnable example with it (C10).
- A **new check rule** is one `stdlib/rules/builtin/rule_<name>.go`, one line in
  `stdlib/rules/builtin/register.go`, and one `stdlib/rules/builtin/docs/<name>.md`. That doc is the
  source of the rule's prose, and `stdlib/rules/builtin/docs_test.go` enforces it 1:1.
  [Authoring a check rule](https://panyam.github.io/agni/build/check-rule/) walks through it in
  practice.
- For a **datalog query relation or example**, see
  [Querying](https://panyam.github.io/agni/guide/querying/). A new relation follows the edit list in
  `stdlib/relations/facts/docs/_TEMPLATE.md`.
- An **interface profile** is data rather than code. It is a YAML declaration of an interface's
  signals and the checks it requires, and it compiles into datalog rules. Built-ins live in
  `stdlib/profiles/builtins/*.yaml`, and an out-of-tree one loads through
  `agni check --profile-path <dir>`. A signal declares exactly one net-name matcher, documented in
  `stdlib/profiles/matcher.go`. Reach for `suffix` first (optionally narrowed by a conjunctive
  `prefix`), since it is the readable convention. Reach for `glob` or `regex` only when the bus
  identity is the prefix and the suffix is shared with an unrelated bus. Suffix matching cannot
  express that one case.

## Expectations for a pull request

- Tests and docs. New behavior gets a test, readers get hand-authored fixtures in the package's
  `testdata/`, and a user-facing change updates the relevant page under `docsite/content/`.
- Keep it format-neutral. Analyses read the IR, not source files. Do not add an IR field only one
  format would populate (C9).
- Write prose in docs, commit messages, and PR text as plain declarative sentences, with no
  marketing cadence and no em-dashes. PR bodies additionally follow the prose conventions at the
  end of this file.

## Reporting bugs and requesting formats

Open an issue. For a bug, a small design file that reproduces it helps a lot, hand-authored rather
than taken from a confidential design. For a new format, start with a pointer to the spec and a
sample export.

---

**The rest of this file is operating discipline for maintainers and coding agents running several
checkouts of this repo at once.** An outside contributor sending a single PR does not need it. Every
rule below is here because it cost real work at least once.

## The gate

`make testall` is the full gate. CI runs exactly it, as four parallel jobs (agni 1147). The jobs are
`testall-test`, `testall-scan`, `testall-docs` and `browser-test`, and `testall-core` runs the first
three. `browser-test` runs as four shards (`make browser-test SHARD=i/4`). Read `docsite/content/build/the-gate.md` before trusting a run. It lists three ways to misread
the gate. A wrapper can report its own status rather than make's and turn red into green. A
commit-first ordering rule applies. Each clone needs its own `pnpm install`. The page also lists one
way a fine tree reads red. A stray `agni serve` on :8080 fails three verdict-link tests, so reproduce
against unmodified `main` before reporting a regression. It also covers what a run leaves behind and
the rules for generated code.

- **From a worktree, run `make -k testall`.** A plain `make` stops at the first failure, and then
  `python-test`, `tutorial-runs-check` and `docsite-images-check` never run.
- **Build in a worktree with the default Go, never `GOTOOLCHAIN=go1.26.9`.** Go 1.26 finds a
  repository only by a `.git` directory. A linked worktree's `.git` is a file, so Go 1.26 runs no git
  and stamps `(devel)` (golang/go#58218, fixed in go1.27). A `(devel)` wasm has an empty
  `StoreVersion` and keeps nothing. `browser-test` and rung 12's capture then fail, and a timed walk
  reports `design=built` where it should read `design=store` (#957).
- **`tutorial-runs-check` regenerates captures and does not read the prose quoting them.** A
  tutorial can cite numbers that a change moved, and the gate stays green. A capture step that exits
  non-zero and captures nothing is a build error unless its spec sets `exit: true` (agni 825).
- A checkout from before agni 853 may hold an untracked `web/static/seed/`. `fixture-copies-check`
  flags it, so delete it.
- `make browser-test` is in the gate and needs a Chromium. `docsite/content/build/the-gate.md` says
  what belongs in it.

### Security scanners

The Makefile pins `STATICCHECK`, `GOSEC`, `GITLEAKS` and `GOVULNCHECK`, and each scanner runs through
`go run` at its pinned version. They scan the two shipped modules, the root and `datasheet`.

- The gate runs `lint` (staticcheck), `seccheck` (gosec, medium and up) and `secrets` (gitleaks over
  the whole history).
- The gate also runs `binaries-check` (`hack/check_no_binaries.sh`). It refuses a tracked compiled
  executable by its magic bytes, and any tracked file over 5 MB. A bare `go build` in an example's
  folder writes a binary named after the folder (#1405).
- `vulncheck` stays out of the gate, since the advisory database moves without the code.
  `vulncheck.yml` runs it weekly and per tag, and `make audit` runs all four scanners.
- Fix a gosec finding, or annotate it on its own line as `// #nosec G<n> -- <why>`. Never exclude it
  in the Makefile. The annotation carries the reason to an outside scanner too.
- **gosec runs under the Go version that `go.mod` names (`GOTOOLCHAIN`).** Under a newer Go its SSA
  rules (G115, G703, G710, G112, G122) skip silently, and a local run reads clean.
- A narrowing of a count or coordinate goes through `core/num`, which saturates. A design file can
  come from anyone, and a wrapped `int32` reads as a real value.
- gitleaks installs as `github.com/zricethezav/gitleaks/v8`. That is its module path, whatever its
  repo is called.
- `SECURITY.md` is the reporting policy.

### Tidy modules

**A module that builds may still be untidy, and the gate checks both.** `tidyall-check` runs `go mod
tidy` over the root and every example module, and fails on any difference. It takes about 5 s and
runs near the front. So a stale `go.mod` shows up there rather than as a later `examples-test`
failure. An untidy module keeps building until a later change needs a requirement it never recorded.
`make tidyall` fixes it.

`.github/dependabot.yml` lists every module directory, with the examples by glob, and groups security
updates (PR 782). A bump that touches the root alone leaves every nested module stale and fails
`tidyall-check`. A new module outside `examples/` needs a line there.

## Wiring, per subsystem

Each kind of addition below has a fixed edit list, and a missed edit fails silently. Most rows name a
test that catches the miss. Where the last column says NOTHING, no test guards the gap, and the row
names the gap so a reader sees it.

| Adding | Edits | Read | Enforced by |
|---|---|---|---|
| A docsite page | 4 (5 for a new section) | `docsite/README.md` | `docsite/nav_test.go` |
| A `learn/` chapter | 4, plus the level-index entries | `docsite/README.md` | `docsite/learn_levels_test.go` |
| A web viewer panel | 4, plus 2 more if it docks | `docsite/content/architecture/web-client.md` | `web/src/composition.test.ts`, `dock.test.ts` |
| A canvas note strip (undrawn, stale-link, hierarchy) | 5 | `web/src/undrawn.ts` and `web/src/stalelink.ts` as the two worked examples; the hole goes inside `.canvas-notes` in `ViewerPage.html`, which stacks the strips so two never paint over each other | the compiler for the `ViewSink` channel, `composition.test.ts` for the template hole |
| A web page | 6 | `docsite/content/architecture/web-app.md` | its own boot test (one per page) |
| A field in the viewer shell (`ViewerPage.html`) | 3 (the template, the page struct in `cmd/agni/webpage.go`, `servedFields` in `web/src/composition.test.ts`) | `docsite/content/architecture/web-services.md` | `composition.test.ts` throws on an unfilled field; `agni site` renders the same struct |
| A format reader | see the page | `docsite/content/build/format-reader.md` | see the page |
| A check rule | see the page | `docsite/content/build/check-rule.md` | see the page |
| A query relation | 7, plus `make catalog-docs` | `stdlib/relations/facts/docs/_TEMPLATE.md` | `facts_docs_test.go`, `TestCatalogMatchesSchema`, `catalog-docs-check`, `TestColumnKindsMatchGolden` |
| A library member (derived relation) | 4 (the rule in a `stdlib/lib/*.dl` file with a `#` summary and typed head, its page in `stdlib/lib/docs/`, `columnkinds.golden`, `make catalog-docs`) | `docsite/content/build/library-member.md` | `TestEveryMemberIsDocumentedAndTyped`, `TestEveryMemberHasItsDoc`, `TestDocQueriesValidate`, `TestColumnKindsMatchGolden`, `catalog-docs-check` |
| A glossary term | 2 (the term page, one index line) | `docsite/README.md` | `docsite/terms_test.go` |
| A hand-written `agni …` fence | 1, plus `docCommandCount` | `docsite/README.md` | `cmd/agni/doccommands_test.go` |
| A multi-command `agniRun` block | 1 (`steps:` in the spec, one per command) | `docsite/README.md` | `tutorial-runs-check` |
| A fixture copied from another directory | 1, plus a group in `hack/fixture_copies.txt` | `build/the-gate.md` | `hack/fixture_copies_check.sh` |
| A file added to a capture's fixture directory | 1, plus `make tutorial-runs` AFTER committing it | `build/the-gate.md` | `tutorial-runs-check`, but only once the file is committed |
| A format-neutral ingestion pass | 3 (the pass, the `Loader.ReadDesign` call, `hack/ir_model_baseline.txt` for C19) | `build/format-reader.md` | a cross-format e2e test you write; NOTHING catches a pass that is never called |
| A new EVIDENCE TIER for a derived IR field | 7 (the pass, the source enum value, the `Loader` field, the `Loader.ReadDesign` call, `service.ReadOptions` + `Overlay.ReadOptions`, the `check.Model` call, `hack/ir_model_baseline.txt`) | C9's evidence-tier variant, `architecture/ingestion-and-ir.md` | the C19 ratchet catches the pass; NOTHING catches a tier that reaches one surface and not the other (agni 710) |
| A host that reads designs | 1 (go through `formats.Loader`, never a bare reader, and through `service.LoaderIn` so its reads can be kept, agni 895) | `build/evidence.md` | `TestReadCarriesTheIngestionPasses` in `examples/common`; nothing guards a NEW host |
| An engine-drawn image that the docs embed | 1 (a line in `docsite/images.sh`, then `make -C docsite images`) | `build/the-gate.md` | `docsite-images-check` |
| A hand-authored diagram | 2 (the file in `docsite/figures/`, one `{{ includeFile }}` in the page) | `docsite/README.md` | `docsite/includefile_test.go` |
| An architectural constraint | 3 (the rule in `CONSTRAINTS.md`, a test in one of four homes, a `Verify` naming that test) | `build/the-gate.md`, and `CONSTRAINTS.md`'s own header | the test you wrote; NOTHING checks that a rule has one |

**The relation row's sixth edit is `service/testdata/columnkinds.golden`.** The file snapshots how
every query column types (agni 687). Regenerate it with `UPDATE_GOLDEN=1 go test ./service/ -run
TestColumnKindsMatchGolden`, and read the diff. A new relation only adds lines, so any other change
means an existing column retyped.

**The seventh is `dimensionlessNumericRelations` in `stdlib/relations/facts_test.go`.** It applies to
a relation whose number is a bare count. `TestNoRelationPublishesAnUnlabelledNumber` fails until the
relation is listed there on purpose (agni 727).

## Working across multiple checkouts

Concurrent sessions work against separate clones (or worktrees) of this repo, one per lane of work.

- **Do NOT run two sessions in one checkout.** They share HEAD, so a branch switch or commit in one
  thrashes the other.
- **A checkout being "yours by topic" does not mean it is free.** Before `git checkout -b`, run
  `git branch --show-current` and a FULL `git status`, never piped through `head`. Piping it through
  `head` truncated the evidence once. Another session may be mid-work there.
- **Use `git -C /abs/path <cmd>` for every git command.** Shell cwd persists across tool calls and
  WILL drift between repos. Four wrong-repo branches were created from stale cwds despite a `cd`
  discipline, and `-C` cannot drift.
- **Commit only your slice, by explicit path. Never `git add -A` or `git add -u`.** In a SHARED
  checkout that alone is not enough, because `git commit` commits the whole index, including entries
  another session staged. Use an explicit pathspec (`git commit -m "..." -- <paths>`), or check
  `git status` for foreign staged entries first.
- **A pathspec commit then has two failure modes of its own.** It silently drops your OWN late edits
  and any directory you forgot to list. A commit naming `protos/ gen/ internal/ docsite/` dropped a
  one-line `cmd/` call-site fix, `make testall` passed locally because the WORKING TREE had it, and
  CI failed to compile on the pushed branch. Local green proves nothing about a pathspec commit. It
  also takes WORKING-TREE content over the index, so a staged `git rm --cached` named in the
  pathspec gets recommitted instead of deleted. After every pathspec commit, run `git status` and
  account for each remaining dirty file. When the change spans more than two directories, or
  involves a deletion or an untracking, stage exactly your slice and commit WITHOUT a pathspec.
- **`make proto` regenerates every proto**, and so do `make proto-web` and `make proto-py`,
  dirtying files a parallel session may own. Stage only your proto's generated files. If a regen
  dirties a file whose proto you did not touch, another checkout committed stale generated output.
  `git checkout` it and tell that session rather than adopting it.
- **Pull after a push.** Shared schema files (`protos/`) are coordination points.
- **Never `git pull` a branch that is mid-rebase, and be careful pulling one at all.** With
  `pull.rebase` set, a pull on a feature branch rebases the INCOMING commits onto yours. That order
  is backwards. On a branch already in a conflicted rebase, the pull re-injects conflict markers into
  files you have just resolved. You see markers reappear in a file you know you fixed, and they name
  a commit you were not merging. `git rebase --abort` restores the branch, including a rebase you had
  already completed, because that commit still exists. Then `git fetch` and `git rebase origin/main`
  as two steps.
- **Verify a rebase left the branch mergeable, and prefer local evidence.**
  `git merge-base --is-ancestor origin/main HEAD` says fast-forward or not immediately. GitHub's
  `mergeable` field lags a force-push by a minute, and it will still say `CONFLICTING` after you have
  fixed it.

## PR workflow

- **Check the branch again immediately before you COMMIT, not only before you branch.** A long
  session's branch can move underneath it. Running the `/checkpoint` skill mid-feature switches the
  checkout back to `main`, so the next `git commit` puts the feature on `main` instead. The push then
  sends the feature branch at its OLD position and succeeds, and the failure surfaces much later as
  `gh pr create` reporting "No commits between main and <branch>". Recovery is cheap when the tree is
  clean (`git branch -f <branch> <sha>`, `git branch -f main origin/main`), so the only cost is
  failing to notice. Run `git branch --show-current` before the commit.
- **Verify ANY command by its EXIT CODE, never by grepping or truncating its output.** `git push | tail -1` swallows a
  failure, and `git push 2>&1 | grep <branch>` reports success on a FAILED push, because the branch
  name appears inside the failure message. Run `git push; echo "EXIT=$?"`. The same failure hits every
  piped command, push included. `gh pr create ... | tail -3` reported `EXIT=0` on a run that had
  FAILED with an auth error, because the pipeline's status is `tail`'s. Redirect to a file and echo
  `$?` on its own line, then read the file.
- **`Closes #A and #B` closes only A.** GitHub parses the keyword PER ISSUE, so a PR fixing two
  tickets needs `Closes #A, closes #B`. The second issue is left open and silently reads as
  unfinished work while its fix is already in `main`. Check both after the merge rather than
  trusting the body.
- **Verify `merged: true` via the API before ANY post-merge cleanup**
  (`gh api repos/.../pulls/N --jq .merged`). A PR once sat closed-UNMERGED while everyone believed
  it had merged, and `git branch -d` still allowed the local delete, because it checks the tracking
  ref rather than main.
- **RETARGET A STACKED PR BEFORE MERGING ITS PARENT.** This repo deletes a branch on merge, and
  GitHub auto-CLOSES any PR still pointing at the deleted branch, so merging the parent silently
  closes the child. Recovering it takes four steps, because you cannot reopen a PR whose base branch
  is gone, and you cannot change the base of a closed PR. The parent merged from one exact commit.
  Recreate the base ref at that commit
  (`gh api repos/OWNER/REPO/git/refs -f ref='refs/heads/<base>' -f sha=<FULL 40-char sha>`), reopen,
  retarget to `main`, then delete the ref again. Deleting it
  does not re-close the PR, because the ref is no longer its base. Creating the ref needs the FULL
  sha; an abbreviated one fails with a 422 that does not say so. Nothing is lost either way, since
  the branch itself survives, but check the tip against the PR head before assuming that.
- **Level the two branches before opening a stacked PR, parent first.** Merging `main` into the child
  and the parent independently creates divergent merge commits, and the stacked diff then shows
  `main`'s commits as if they were the child's changes (26 files instead of 23, with
  `mergeable: false`). Merge `main` into the parent, push, then merge the parent forward into the
  child.
- **A merged PR does not mean the BRANCH is merged. Check the tip against what the PR merged.**
  A commit pushed to the branch after the merge is stranded. The PR reads merged, GitHub offers to
  delete the branch, `git branch -d` accepts it, and the work is gone with nothing anywhere saying
  so. Two of roughly twenty audited branches had done this, and they stranded 135 lines of
  documentation. Those lines were recovered only because someone asked whether a branch was stale.
  Compare
  `gh api repos/.../pulls/N --jq .head.sha` against `git rev-parse origin/<branch>`, or sweep the
  repo with

      for b in $(git for-each-ref --format='%(refname:short)' refs/remotes/origin/); do
        n=$(git rev-list --count origin/main..$b); [ "$n" != 0 ] && echo "$n ahead: $b"
      done

  Look at any branch ahead of `main` whose PR already merged. Content can still be in `main` by
  another route, so confirm by grepping the branch's added lines against `main` rather than by
  trusting the count. Before believing a zero, validate that grep on lines that ARE in `main`.
- **NEVER `gofmt -w` a whole directory.** This repo's committed import blocks are not gofmt-sorted,
  so a directory-wide run silently reordered imports in 18 untouched files. **A SINGLE file is not
  safe either**, since adding an import makes `gofmt -w` re-sort that file's whole block, and the
  re-sort reads as an unrelated hunk. Format only what you edited, then re-read the block and
  restore the original order.
- **Deferred work survives the PR in `OUT_OF_SCOPE.md`.** A PR body's "Out of scope" section is
  invisible once merged, so every deferred item gets a one-liner there (source PR plus pickup
  trigger) in the same PR. The ledger is only for work nobody would pick up deliberately. Such work
  becomes cheap the moment an unrelated trigger fires. Ticket-worthy items get a GitHub issue
  instead, and a question that was asked and ANSWERED goes in `DECISIONS.md`, since it can never be
  closed.

### Shell traps

- **`git checkout -b <branch> origin/main` can FAIL and still move your working tree.** This repo has
  two remotes whose fetch refspecs both map to `refs/remotes/origin/main` (`origin` and `panyam`), so
  the branch creation dies at the tracking step with "ambiguous information for ref" AFTER the
  checkout has updated the index. You are left on the old branch with the new tree STAGED against it.
  That looks like a large accidental change. To recover, run
  `git branch --no-track <branch> origin/main` then `git checkout <branch>`. The recovery needs no
  destructive command. Reach for `git reset --hard` only
  after confirming nothing uncommitted is yours, and remember another session's uncommitted edits can
  be sitting in the same checkout.
- **A `cd` persists across calls, and stray files end up in the repo root.** A scratch file
  written to a relative path goes wherever the last `cd` left you. So a heredoc meant for a temp
  directory can drop a `.md` or a `.png` into the tree, and a directory `add` then stages it for a
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
  `gh pr edit` fed that way returns 0 with the body damaged, so read a PR body back after every edit.
- **zsh reads `$var:r` as a history modifier.** `git push origin $c:refs/heads/x` strips the
  colon and what follows from `$c` and pushes a nonsense refspec. Write `"${c}:refs/heads/x"`.
- **zsh does NOT word-split an unquoted `$var`.** `files=$(ls ...)` then `for f in $files` iterates
  ONCE over the whole blob, and a sweep that did this compared one file and reported success. Glob
  directly in the `for`, or use an array. **The nastiest form is a before/after comparison loop**:
  `for c in "check --verdicts" "stats"; do ./bin-a $c > a; ./bin-b $c > b; diff a b; done` passes each
  string as ONE argument, so both binaries print `unknown command` and the diff reports them
  identical. A refactor was reported behaviour-preserving on four commands that had never run. Write
  a function taking `"$@"`, and sanity-check the captured line COUNT before believing a diff.
- **`wait "$PID"` returns IMMEDIATELY when a trap is installed.** A script that starts a server, sets
  `trap cleanup EXIT INT TERM`, and then blocks on `wait` does not block, and falls straight through to
  whatever follows. A demo driver meant to hold a server open ran its whole script instead, twice, and
  the symptom looked like the guard clause not firing rather than like `wait` returning. Poll the child
  instead: `while kill -0 "$PID" 2>/dev/null; do sleep 1; done`.
- **A quoted assignment prefix is not an assignment in zsh, and `env VAR=~/x` does not expand the
  tilde.** `"FOO=$HOME/bin" cmd` runs `FOO=...` as a COMMAND and exits 127; `env FOO=~/bin cmd` passes
  a literal `~`. Both produce a "not found" that reads like a broken script rather than a broken
  invocation. Use `env FOO="$HOME/bin" cmd`.
- **Piping a long-running script into `head` makes it look like it fell through.** `head` exits after
  its lines, the writer takes SIGPIPE, and with no `set -e` bash carries on to the next command, so
  output appears past the point the script should have blocked. Redirect to a file and read the file.
- **To undo a temporary red-check edit, restore a copy of the file or reverse the edit with the tool
  that made it, never `git checkout`.** `git checkout <file>` and `git checkout HEAD -- <file>`
  restore from a COMMIT, not from "before I typed that". On a file carrying uncommitted work they
  destroy all of it, including the change the red-check was proving, and the next run passes over
  nothing. Commit first, even a WIP commit, or copy the file aside before breaking it. Restore it in
  a command of its own. Chained after the test with `&&`, the restore is skipped when the broken file
  fails to build, and the next backup copies the broken file. Removing a package's last use of an
  import breaks the build the same way, so disable a call in place (`&& false`) rather than deleting
  it. The symptom is confusing rather than obvious, since a test that just passed starts failing and
  the cause looks like the change rather than the undo.
- **`go test -cpuprofile` leaves `<pkg>.test`, a 30 MB binary, in the working directory.** The file
  is untracked and not ignored. Delete it before staging, and never `git add -A`.
- **A capture restamp belongs in its OWN commit, after the fixture that moved it.** A capture's stamp
  hashes every TRACKED file in its fixture directory, so adding one file there restamps every capture
  rooted at it. One change put 25 `.output` files in the diff, each a single `#agni-run` line, none of
  them a change in output. Commit the fixture, regenerate, then commit the restamps separately. A
  reviewer then does not read two dozen one-line stamp diffs interleaved with the work.
  The commit-first half is not optional either, since the stamp is computed from what git tracks.
- **A two-tree comparison needs each side in its OWN subshell.** `( cd $before && cmd ); ( cd $after && cmd )`,
  never `cd $before && cmd_a; cmd_b`, because the `cd` persists and BOTH commands run in the worktree.
  That reports the two sides identical, the same false result as the zsh loop above. Check that the
  two outputs differ somewhere you expect them to before believing a diff of zero.
- **`--format json` output is protojson, whose whitespace VARIES BETWEEN BUILDS on purpose**, so two
  binaries produce byte-different json for identical data (one measured diff of 1478 lines was
  entirely `"k":  v` against `"k": v`). Never byte-compare it. Parse both sides and compare the
  decoded values, or compare `check --verdicts` instead, which is stable.
- **Use `git worktree add <tmp> main`, never `git stash`, to reconstruct a whole BEFORE state** such
  as a "before" binary. Stash leaves untracked new files on disk referencing stashed-away code.
  Stashing ONE tracked file is a different move and a good one
  (`git stash push -- path/to/file.go`, run the test, pop). It proves a new test is red for the
  reason you think, rather than red because the package does not compile. The
  [evidence page](docsite/content/build/evidence.md) says what that red-check tells you and the two
  ways it misleads.

## Ways to be told a thing worked when it did not

Each of these exits 0.

- **Before believing a measurement or a green test, read `docsite/content/build/evidence.md`.** A
  negative result needs a positive control. A positive rate needs a precision check. Every new test
  needs a red-check.
- **A stray `agni serve` answers instead of yours.** A newer binary can fail to bind its port, and
  the "address already in use" line scrolls past. Run `pkill -f "agni serve"` before believing
  anything served, and re-check the version that answered. **That `pkill` also matches the shell
  running it.** Typed ahead of `make testall` on one command line, it kills the gate with exit 144
  and no output. Run `pgrep -af` first. Something other than agni may hold :8080, such as a
  code-server, and that process is not yours to kill.
- **`go test` prints FAIL for a build failure too.** A red-check that greps for `^ok` reads a package
  that does not compile as a pass. Confirm that a red-check names the assertion it failed on. A
  red-check that reverts a declaration fails the same way. Suppose a new test refers to a symbol in a
  source file, and the red-check reverts that file. The test then fails to compile and proves
  nothing. Disable the behaviour and keep the symbol.
- **`catalog-docs-check` reads `git status`, not a diff against a temp copy.** Regenerating is not
  enough. The generated file has to be committed, or the gate stays red on a correct tree.
  `tutorial-runs` works the same way.
- **A new test file can replace an existing one of the same name** (agni 987). The gate stays green,
  because the old file's test is gone. `git status` showing `M` for a file you meant to create is the
  sign, so check the name first.
- **A test that loops over rows passes over zero rows.** Give it a count guard (agni 727).
- **Time two tools on the same clock.** A timer started inside Python runs after the interpreter and
  the client library load. That time does not compare with another tool's launch-to-exit time.
  Compare launch to exit on an idle machine, checking the load average first, or say which clock a
  number is.

## A new constraint owes a test

A rule in `CONSTRAINTS.md` owes a test, never a command typed into the document. The file's header
keeps the count of rules the gate enforces and rules that stay review questions. What a test reads
decides which of four homes it goes in.

- A test of the package graph or the module goes in the root `deps_test.go`.
- A test of one package's own rule goes beside that package, as in `service/transport_guard_test.go`
  and `core/facts`.
- A sweep over source goes in `internal/constraints`.
- A parity test of what two surfaces print for one answer goes beside the surface, as C35 does in
  `cmd/agni/tabulate_test.go`.

`docsite/content/build/the-gate.md` has the full account. Two shapes from it are worth copying. A
graph or single-writer check needs a positive control, so a pattern matching nothing fails rather
than reading as clean. An invariant narrower than any sweep becomes a ratchet with an allowlist
rather than being weakened, as C24's "never compared" did.

## Proving a comment-only change

Compare the code with its comments stripped, rather than reading the diff. A sweep over hundreds of
files hides one edited code line easily. For Go, compare the `go/scanner` token streams of each file
and of `git show <base>:<file>`, skipping comments. For TypeScript, print both through the compiler
API's printer with `removeComments`. Give the check a positive control, such as a deliberate `var _ =
0`. A control that only adds whitespace changes no token and proves nothing (PR #754).

## Check a ticket against the tree before planning against it

A closed defect can sit OPEN on the issue list for weeks. In one September 2026 session, three of the
four issues ranked highest were already fixed: 522 (the guard was already in 26cae20), 561 (five
commits, none naming it in the subject) and 578 (a duplicate of two later tickets).

Grepping for the issue number finds none of them, because a fix is committed under its own name.
Running the ticket's own reproduction does find them. So does one line of prose in an adjacent
artifact. For example, `readers/kicad/oracle_corpus.baseline` opens with "Known families still open,
as of the agni issue 561 fix", and `checkNegationAnchored`'s doc comment in jaala's
`datalog/eval.go` cites 522.

This cuts both ways. In the same session an issue filed that morning (684) claimed two relations
disagreed; one query against a real board showed they already agreed, and the fix it proposed would
have been a regression. Ranking a backlog off issue bodies alone produces a confident wrong plan.

**A ticket's own MEASUREMENT can be the wrong half.** 677 said a conventions file naming an unknown
component class "parses, loads, and does nothing"; the loader had refused it since before the ticket
was written, and the real defect ran the other way, with three shipped classes missing from the list
of classes a config may name. 682 printed a table of stamps proving a fetched fixture hashed as if
it had no fixture. The table came from a re-implementation of the hash, and that copy left out a
guard in the real hash. Re-run the measurement through the SHIPPED code path, not a replica of it,
and probe the command a user would type. Both tickets still had a real defect underneath, so this is
a reason to re-measure rather than to close.

## PR prose conventions

The **`start-pr` skill** defines the shape of a PR body, and it is distributed separately from this
repo. Its sections are what changes, prerequisite knowledge, a reviewer's guide, a decision log, and
a before/after artifact. The reviewer's guide opens with a one-paragraph ELI and carries it through
the reading order. Follow the skill rather than a copy kept here, because a copy drifts and this one
already had.

The rest of this section covers only what agni adds on top. This engine sits where electrical
engineering meets software, and most reviewers are strong in one and cold in the other. agni adds
two sections to the skill's skeleton, ahead of the reviewer's guide. The local order is what
changes, the circuit, the hardware primer, prerequisite knowledge, then the reviewer's guide.

- When semantics depend on hardware behavior (a rule's electrical meaning, derating, rail/pin
  conventions, why a limit matters physically), add a section titled "The circuit, for software
  readers" right after "What changes", plus a link to `docsite/content/reference/analogy.md`. A
  pseudocode walkthrough alone leaves the hardware nouns opaque.
- Every PR whose logic touches hardware concepts gets a hardware primer, a `## Hardware context (for
  software readers)` section. The diff relies on some EE terms, and the primer maps each one to a
  STRUCTURAL software analogy (series element = inline middleware that splits a net; rail = global
  singleton the walk must not follow an import into; junction dot = explicit join marker in
  whitespace-significant syntax; TVS = pressure-relief valve beside the path, not inline). Define
  only what the PR actually uses. It does not replace the ELI paragraph that follows, since the
  primer supplies the nouns and the ELI supplies the idea.
- The prerequisite block pulls from `learn/` for domain knowledge,
  `tutorials/` for tool usage, `architecture/` for design rationale, `build/` for extending the
  engine. When the PR is about a rule, prefer the LEARN chapter over the rule's catalog entry,
  because the entry explains the check and the chapter explains the instinct. If no page covers the
  change, say so. That statement is a real entry rather than an empty one, and several chapters
  exist because someone wrote it.
- **Link a prerequisite page to the PUBLISHED site**, not to a repo path, since a relative path does
  not resolve in a PR body. The site is `https://panyam.github.io/agni/`, one URL per page at
  `/<section>/<page-basename-without-.md>/`, and a heading anchor is the slugified heading. When the
  PR CHANGES a page it names, link both. The published page reads better but serves `main`, so
  the reviewer needs the PR-diff link to see the new text. Saying which is which takes one clause,
  and it saves a reviewer from reading the old version, which the PR exists to replace.
- **Run the prose checks over the PR BODY FILE, not over `git diff`.** A body is written to a scratch
  file and never appears in a diff, so a check that greps the diff passes while the body carries
  exactly what it was meant to catch. Five merged PR bodies shipped em-dashes
  in their reading-order lines while every one of their diffs was clean.
- Past PRs here have used ELI analogies such as fire extinguishers for a protection radius, a
  wiring diagram vs a floor plan for connections vs pin declarations, and game mods for registering
  an extension vs authoring one.
- **A mermaid label must contain no quote characters, escaped or otherwise.** `&quot;` inside a
  `["..."]` node label decodes to a bare `"`, closes the string early, and GitHub renders a parse
  error instead of the diagram (`got 'STR'`). Write labels as plain text with `<br/>` for line
  breaks and keep the literal strings in the prose above the diagram.
  To parse-check before you post rather than after, extract each ` ```mermaid ` block to a file,
  DECODE its HTML entities, and run `mmdc -i block.mmd -o block.svg`. Two details of that command
  decide whether the check works. The first version of this note got both wrong.
  **`-o /dev/null` does not work**, because mmdc rejects any output path not ending `.md`,
  `.markdown`, `.svg`, `.png` or `.pdf`, so it exits non-zero on every diagram and reports a
  failure that says nothing about the syntax. And **the check reproduces GitHub only after the
  decode**. mmdc reads `&quot;` as literal text and parses it happily, while GitHub decodes it to a
  bare `"` before mermaid sees it. So a check of the raw block PASSES a diagram that fails on GitHub,
  and this rule exists to catch that exact diagram. Decoded, the same label fails with the
  `got 'STR'` above. Looking at the rendered PR is still the only way to catch a diagram
  that parses and reads badly.
- **Rendering captures come from a fixture, never a customer board.** Capture the before/after pair
  from the showcase boards (`cmd/agni/testdata/conformance/showcase.{passes,fires}.kicad_*`),
  `examples/tutorial-project/`, a public board from the pinned samples corpus that the demo already
  shows (Jetson through `examples/jetson-project`, under its own open licence), or a tiny
  hand-authored fixture, for the reasons in `CLAUDE.md` under "What does not belong in this repo". A
  browser-automation screenshot is written to the CWD. The CWD is the REPO ROOT, and a bare `.png`
  there is not ignored (`bin/` and `.playwright-mcp/` are). So move captures out of the tree before
  staging, and stage by explicit path. That is the stray-file trap above, narrowed to one file type.
  To embed the pair, commit the images to `pr-assets/`, reference them as
  `.../raw/<full-SHA>/pr-assets/x.png` pinned to the commit rather than the branch, then `git rm`
  them in a follow-up commit so they never reach `main`. Do it before the PR merges, so the image
  commit is in `main`'s history. If you add it after the merge, it lives only on the PR's branch
  (GitHub deletes a merged branch, so pushing recreates it). That branch then has to be kept, or the
  image eventually stops resolving. An orphan branch named `pr-assets/<PR>` that holds only the
  images also works, at any time, and it too has to be kept (#1404 and #1421 did this).
  ImageMagick cannot render agni's SVG (it fails on the font list), so lay the frames side by side
  in an HTML page, serve it locally, and screenshot it in a browser. `BoardSVG` has no option to
  show one layer, but each layer draws in its own `<g data-layer="...">`, so a `<style>` hiding the
  others shows one.
- **A rendering before/after carries a third frame, from the format's own tool, whenever one
  exists.** That means `kicad-cli sch export svg` or `pcb export svg` for KiCad, xschem and Lepton
  for their schematics through `make natrender`, and gerbv for Gerbers. Render the same fixture
  three ways (`main`, the branch, the native tool) and lay them side by side. The native frame is
  the oracle, and it shows gaps the change never touched. In agni 1345 the native frame was there
  to confirm a rotation fix, and it showed three more defects, empty transistor bodies, pin names
  across their pins and no DNP crosses, which became agni 1348. Say in the PR what the native frame
  still draws differently, and file what is left. A format with no native tool on the machine says
  so rather than dropping the frame silently.
