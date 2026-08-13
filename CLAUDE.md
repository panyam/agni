# Agni: engine repo orientation

Agni is an EDA tooling engine: a Go engine, a protobuf IR, format readers, diff/checks, a geometry
sidecar, a web viewer, and a CLI. It reads electronic design files in several formats into one
intermediate representation, then runs checks, diffs, queries, and renders over that IR.

**This repo is public, under Apache-2.0.** Anything committed here is world-readable the moment it
is pushed. Read "What does not belong in this repo" below before writing docs, tests, or fixtures.

## Read the docsite first

`docsite/content/` is the engineering source of truth and usually explains a subsystem better than
any summary. **Read the relevant page before working in that area**. Each holds gotchas that are
expensive to rediscover.

| Working on | Read first |
|---|---|
| Ingestion, the IR, a new format reader | `architecture/ingestion-and-ir.md`, `build/format-reader.md` |
| Geometry, rendering, the web viewer | `architecture/geometry-and-rendering.md`, `architecture/web-app.md` |
| Net solving, hierarchy, net identity | `architecture/net-solving.md` |
| A check rule, datalog, interface profiles | `architecture/rules-and-checks.md`, `build/check-rule.md` |
| The checks contract (CLI/service seam) | `architecture/checks-contract.md` |
| Config: what a run is checked against, and where it comes from | `architecture/projects-and-designs.md` |
| Semantic diff | `architecture/semantic-diff.md` |
| The datasheet param/doc/derive layer | `architecture/datasheet-layer.md` |
| Extending the engine from outside | `build/overlay.md` |
| xschem / Lepton native tools | `build/native-verification.md` |
| Learning the domain from a software background | `reference/analogy.md`, `reference/edif-primer.md` |

`guide/` is the user-facing manual (getting-started, concepts, checks-and-reports,
comparing-revisions, querying, naming-conventions, interface-profiles, datasheets, cli-reference).
`tutorials/` walks one board from first read to a CI gate. `reference/` also holds the GENERATED
rule and relation catalogs.

`site/` is stale build output, not a source tree. Some older notes reference a retired `docs/NN-*.md`
mkdocs tree that was folded into `docsite/content/` with audience-first names.

## Package layout

Engine analysis under **`core/`** (`core/check`, `core/review`, `core/render`, `core/diff`,
`core/query`, `core/model`). Format readers under **`readers/`** (`readers/edif`, `readers/kicad`,
`readers/ipc2581`, `readers/xschem`, `readers/geda`, plus `readers/formats`, the registry/Loader).
The shipped rule catalog, fact relations, profiles, and intent under **`stdlib/`**
(`stdlib/rules/builtin/rule_*.go`, `stdlib/rules/datalog`, `stdlib/rules/intent`,
`stdlib/relations`, `stdlib/profiles`). The datasheet stack under **`datasheet/`** (`param`, `doc`,
`derive`). Plus `cmd/agni/`, `internal/`, `intake/`, `census/`, `protos/` + `gen/`, `docsite/`,
`web/`, `hack/`, `tools/`.

Notes written before this layout landed name the old directories. **Grep the SYMBOL or filename,
not the directory.**

`intake/` deserves a callout: it produces a sanitized design summary, and its confidentiality
guarantee is **structural**. The `Skeleton` type has no field that can hold a net name or a
connection, so an intake summary *cannot* express the confidential parts of a design. That turns a
policy ("do not paste a net name") into a property of the type ("there is no field for one"). Keep
it that way. Adding a free-text field to `Skeleton` would quietly dissolve the guarantee.

## Build, test, and the CLI

- `make build` / `make test` / `make agni` / `make install`.
- `make stats` / `make check` run the CLI against a committed fixture (`EDN=...` to override).
- CLI: `agni stats|check|diff|render|query|review|serve <file>`. The reader is chosen by extension
  (case-insensitively), with `.xml`/`.sch` sniffed by root/header. `--symbol-path <dir>` resolves
  external symbol files and searches each dir's SUBTREE, so a dir can be a library root.
- Toolchain: Go 1.26.4 and `buf` 1.61. **Both protoc plugins are pinned as `tool` directives in
  `go.mod` and invoked via `go tool`**, so their versions are data rather than something to match by
  hand. Only `buf` itself has to be on your PATH.
- **A command that reads a design goes through `readDesign` (or a service), never a bare
  `newLoader().ReadDesign`.** That function is where a design's PROJECT config enters the read for the
  six commands no service mediates (stats, diff, emit, render, intake, profilediag), and net roles are
  resolved once at ingestion — so a read that skips it silently uses the built-in naming vocabulary and
  none of the project's declared symbol libraries. All six bypassed it until agni issue 228, which is
  why it is one function rather than six.

**After ANY proto change run BOTH `make proto` (Go) AND `make proto-web` (TS).** Additive fields
build green, so a skipped TS regen used to go unnoticed until the next regen churned. `make
proto-check` now fails the gate on either half being stale, and it runs inside `testall`, so this is
enforced rather than remembered. It names which half drifted and which command fixes it.

Unlike `catalog-docs-check`, **it carries no commit-first ordering trap**: it generates into a
throwaway tree and diffs, so regenerating and running the gate before committing works fine.

**An editor reporting the generated types as MISSING is usually a stale language server, not stale
generated code.** After a branch switch or a merge, the LSP can insist `Module ... has no exported
member 'Foo'` and `Property 'bar' does not exist` for symbols that are present in the file. It reads
exactly like the real hazard above, which is why it is worth naming: the two are told apart by
checking the FILE and a fresh `cd web && npx tsc --noEmit`, never the editor. A green `make testall`
alongside editor errors is the cache, not a bug; genuine staleness fails the gate.

`examples/tutorial-project/` is the shareable review-project fixture the docsite tutorial runs on: a
synthetic gateway ECU in three views (`.edn` plus a rev-b, a `.kicad_sch` with an external symbol
library, a `.kicad_pcb`) with `review.yaml`, `conventions.yaml`, `profiles/`, `params/`, and a
per-design `intent.yaml`. Deliberately imperfect, one flaw per thing a tutorial rung has to show.
The KiCad views are GENERATED from the netlist by `tools/`. From inside `examples/tutorial-project/`,
`make check-views` fails if the three stop describing the same design, and `make regen-views`
rebuilds them after any netlist edit (both targets live in that folder's own Makefile, not the root
one). It is not a Go module, so `make testall`'s `examples/*/go.mod` glob skips it.

**When you build a feature, ship an example** (CONSTRAINTS C10; how-to in `examples/CONVENTIONS.md`).
Each example is its own Go module so the demo kit stays out of the engine `go.mod`, and narration
lives in a sidecar `walkthrough.md` rather than in Go strings.

### `make testall`, the full gate and its three traps

vet + engine tests + example modules + web bundle build + web typecheck/vitest. Green = ship-ready,
and CI runs exactly this.

- **Never judge it through a pipe.** `make testall | tail` reports *tail's* exit code, so a red gate
  reads green and an `&&` chain sails on (this has bitten twice). Run it redirected
  (`> /tmp/t.log 2>&1; echo $?`) and read the tail from the file.
- **`catalog-docs-check` is git-status-based**, so a regenerated docsite file that is not yet
  COMMITTED reads as stale and fails. Anything changing the shipped rule or relation/predicate
  catalog regenerates `docsite/content/reference/`. Order: `make catalog-docs` → **commit** →
  `make testall`. Running testall first on a freshly generated tree is a guaranteed red that says
  nothing about your code.
- **`pnpm install` is per clone.** After merging main, `cd web && pnpm install` if web deps changed.
  Every checkout needs its own install (this has bitten four times). A plain re-install can be
  INSUFFICIENT: a partially-populated `node_modules` survives it and the bundle dies deep inside a
  transitive dep, which reads as a code bug rather than a toolchain one. **When the second error
  differs from the first, stop re-installing:** `rm -rf web/node_modules && pnpm install`. Match on
  the SHAPE (a failure inside a dep you did not touch, right after a checkout switch or fresh clone),
  not on the message.

Also expect: testall leaves `examples/render-board/render-board` and `examples/validate/validate`
built. Both are covered by per-example `.gitignore` files, so do NOT `git add` them. A reader test
also drops `readers/kicad/testdata/*.kicad_prl`, which `.gitignore` covers and which is never part
of a change. **Golden SVGs** fail by design on any render-affecting change. Regenerate deliberately
with `go test ./core/render/ -run Golden -update` and inspect the diff before committing.

`kicad-cli` writes a `.kicad_prl` beside ANY board it reads, not only under test, so a
directory-wide `git add` will sweep them up. The ignore rule is deliberately SCOPED to
`readers/kicad/testdata/` rather than a blanket `*.kicad_prl`, because the two under
`cmd/agni/testdata/conformance/` are tracked on purpose: those fixtures carry them so a project read
sees the full sibling set. Do not "clean them up". If you point kicad-cli at a new folder, add an
ignore rule there, and stage by explicit path rather than by directory.

### Measuring, and trusting a measurement

**A NEGATIVE RESULT NEEDS A POSITIVE CONTROL.** "Zero hits across 62 documents" is a claim about the
instrument until you show the instrument can find a known instance. Three separate absence claims in
this repo turned out to be artifacts of the detector rather than facts about the data: a table shape
the heuristic could not see, a regex that could not match `V CC` so an arity gate silently dropped the
one sentence being looked for, and a corpus sweep that ran a different code path from the one shipping.
Before reporting an absence, plant a known instance and confirm it is found. The fixtures usually
already contain one.

Two habits that make this cheap. **Instrument the gates**: count and sample what each filter REJECTED,
not only what it matched, which is the silence-never-reads-as-coverage discipline applied to your own
tooling. And **exercise the shipped configuration**: a sweep run with different flags from the ones
the feature ships with has validated a different program.

**Anything matching SYMBOL TEXT out of a doc-IR must tolerate an injected space.** Producers flatten
subscripts, so `VCCA` arrives as `V CCA` (~850 such occurrences in one corpus). This has bitten three
times in unrelated places: a prose sweep, the derive pin path where it would have produced pin ids no
symbol library could match, and in-document search. Assume the space is there.

**A feature premised on a house CONVENTION needs its base rate measured on real designs first, and
the fixtures cannot tell you.** Every fixture in this repo names things the way the built-in
vocabulary expects, because that is what made them work, so a convention-shaped feature always looks
well-founded against them. Measured on two real boards, the endpoint-encoding net-name convention an
issue was written around covered 1.1% and 0.06% of nets, while the shipped tutorial project does not
use it at all. The same measurement then found a live silent bug in the opposite direction. Count the
shape on a real design before designing to it; it is one query and it has twice changed what was
worth building.

**A feature no fixture EXERCISES cannot fail a test, and that reads exactly like working.** The
datasheet role tier shipped against a corpus where not one seeded spec declared pins, so the pass had
no evidence to read: every test passed, the real boards were unchanged, and nothing could have gone
red if it were wrong. The fix was to give the shipped fixture the data the feature consumes, which is
what turned it from unfalsifiable into demonstrable (0 rails to 3 on the tutorial netlist). Before
believing a green run, check that some committed fixture actually carries the input.

**A long-lived ticket's PREMISE erodes silently, so re-verify it against the code before planning.**
Three substantial issues this month had aged out before anyone picked them up: one was mostly shipped
already, one rested on a convention the only real boards contradicted, and one had landed in pieces
under other work. Nothing was wrong with any of them when filed; adjacent work moved underneath and
the ticket text kept asserting the old world. Read the comment thread, not just the body, and check
the claims against the tree. It costs minutes and has now saved three wasted PRs.

## Docsite wiring

**FOUR edits for a new page in an existing section, FIVE for a new SECTION.** A page needs the file,
the section's `index.md`, `templates/nav/<Section>Nav.html`, and `content/HeaderNavLinks.json`. A new
section additionally needs `templates/Sidebar.html`, and that one is TWO edits in the same file: the
`{{# include #}}` at the top AND a branch in the `Contains $currentPath` dispatch chain. Miss the
branch and the section silently renders the generic fallback nav.

`docsite/nav_test.go` enforces all of it and runs in `make testall` via the `docsite-test` target,
so a missed edit fails the gate instead of shipping. It found two live drifts when it landed. If you
are adding a section, let the test tell you what you forgot rather than working from this list.

**A blank line inside raw HTML SPLITS it, and the render breaks silently.** Content pages may embed
raw HTML (inline SVG figures, the home page's cards) because the renderer passes it through. But
CommonMark ends an HTML block at the first blank line, so a `<figure>` broken up for readability
becomes several blocks and the fragments after the first get parsed as markdown. Keep an embedded
figure contiguous, no blank lines between the opening and closing tag. Nothing in the gate catches
this: `nav_test.go` checks wiring, not rendering.

**A tutorial's command output is GENERATED, not pasted.** A page holds
`{{ agniRun "content/tutorials/runs/<name>.yaml" }}`; the yaml says what to run; a committed
`<name>.yaml.output` holds the capture. The directive emits the command AND the output, so neither is
hand-written and they cannot disagree. Regenerate periodically with `make tutorial-runs` and read the
diff before committing — it is deliberately NOT in `testall`, because the freshness stamp covers the
spec and the fixture but not the engine build, so a code change does not invalidate a capture on its
own.

Four things about writing one. **Never make the directive rewrite the page**: `content/` is what the
site builder reads, and a build that wrote back into it would loop. **Use the fields, not shell
plumbing** — `capture: stdout|stderr|both|none`, `exit: true`, `match: '<re2>'` — because a positional
filter (`sed -n '5p'`) silently shows the wrong line the moment that output gains one; `match` selects
by shape and matching NOTHING is an error. **Add `show:` only when the script carries plumbing a reader
should not see**, since it defaults to the script. And **every run gets a scratch copy of the fixture**,
so a rung that teaches `mv params params-old` cannot rename the checked-in one — which it did, once,
by hand.

Blocks that cannot be generated stay hand-written and unverified: an `agni serve` that never returns,
an excerpt of a longer output, a step needing a tool the build cannot assume (rung 12 shells out to
`kicad-cli`). Generate what can be generated rather than softening the check to cover the rest. Do not
tag a generated fence `console`: Chroma's console lexer renders the whole body as error tokens.

**Verifying a tutorial's claims keeps finding bugs in the ENGINE, not the docs.** Three times so far: a
rung arguing that narrowing a gate makes a board pass (it reveals the next failure instead), `agni
query` printing an absolute host path in provenance, and two rungs whose numbers had drifted. Treat a
mismatch as a question, never as "regenerate and move on" — regenerating blesses whatever the code
currently does, which is right when the doc drifted and wrong when the code regressed.

**Changing the tutorial FIXTURE can invalidate a rung's lesson, and the fix is a judgement about what
the rung teaches.** Seeding pin functions into the tutorial's two synthetic specs made rung 4's
"without this project's naming vocabulary, only GND is a rail" false, because the datasheet then
classified those rails regardless. Both statements were true; they just could not share a run. The
rung now moves the params corpus aside along with `conventions.yaml` so it isolates NAMING as it
intends, and the page says why. Regenerating instead would have shipped a page contradicting its own
output. When a fixture edit changes a capture's CONTENT rather than its stamp, find which page reads
it and decide what that page is for.

**Style raw SVG through `--accent-color` and `currentColor`, never a literal.** `static/css/main.css`
defines the palette for both themes, and the docsite has a dark mode. A hardcoded hex reads fine in
whichever theme it was authored in and badly in the other.

**`content/HeaderNavLinks.json` is hand-formatted with one compact object per line.** Read it as text
before editing. Piping it through a pretty-printer to find the insertion point produces a shape that
does not exist in the file, so the edit fails to match.

**`docsite/_hidden/` hides pages from the SITE BUILD, not from the repo.** Files under it stay
tracked and world-readable. A parked section sat there for months in exactly that state. Moving
something to `_hidden/` is a publishing decision and never a confidentiality one. Anything genuinely
sensitive has to leave the repo and its history.

## Web viewer wiring

**FOUR edits for a new viewer panel**, and the last one is the one everybody forgets. The island
(`web/src/<panel>.tsx`), its hole in `web/templates/ViewerPage.html` (`data-component="..."`), its
field on `ViewSink` in `web/src/viewer.ts`, and its construction plus wiring in `web/src/main.ts`.

`main.ts` is the composition root and nothing else constructs it, so a missed fourth edit is invisible
to every other test: the presenter's view ports are OPTIONAL by design (an embedding host may leave a
panel out, see `build/overlay.md`), which means an unwired port is a silent no-op rather than a type
error. That has shipped a green-CI, broken-in-the-browser feature twice, once with a client never
passed and once with a view never wired.

`web/src/composition.test.ts` now boots the real `main.ts` under jsdom against the real page and fails
on any of those omissions, so let the test tell you what you forgot. Read its header comment before
changing the wiring; `docsite/content/architecture/web-app.md` has the rationale.

**A test fixture that is a PLAIN OBJECT LITERAL standing in for a proto message is invisible to
`pnpm run typecheck`.** Nesting `Project`'s config fields under `config` left
`projectpresenter.test.ts` structurally wrong and the typecheck green; only the runtime assertion
caught it. So after a proto reshape, `pnpm run typecheck` passing is NOT evidence the web side is
done — run `make testall`. Prefer `create(SomeSchema, {...})` in a new fixture, which does get
checked.

**A JSX expression that reads only PLAIN OBJECT properties never re-runs.** Solid wraps an attribute
or child expression in an effect over the signals it reads, so `class={p.pinRefs.includes(x) ? "on" :
""}` — where `p` came from a list and is not itself reactive — subscribes to NOTHING and renders once.
The data changes, the DOM does not. Read through the accessor instead (`props.spec().parameters.some(
...)`), which tracks. This shipped an inert set of buttons with every unit test green, because the
helpers were correct and only the subscription was missing; a component test is the only thing that
sees it, and `transcribe.tsx` still has none (OUT_OF_SCOPE).

**Never `window.confirm` / `alert` / `prompt` in a panel.** A native dialog blocks the page, which
blocks browser automation outright: the screenshot and drive-the-app flows stop responding with no
error. Use an inline two-step (the `deletePackage` confirm in `transcribe.tsx`), which is also better
UX, since it can name what is about to be lost rather than asking a generic "are you sure".

## Parallel development across multiple checkouts

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

Three shell traps that have burned real work. **Backticks inside a double-quoted `git commit -m` run
as COMMAND SUBSTITUTION**, so a message quoting `` `reserved` `` committed the sentence with the word
missing and no error. Anything with backticks, `$`, or `!` goes through `git commit -F <file>` or a
quoted heredoc (`<<'MSG'`), never `-m "..."`. **zsh does NOT word-split an unquoted `$var`**, so
`files=$(ls ...)` then `for f in $files` iterates ONCE over the whole blob (a sweep silently compared
one file and reported success). Glob directly in the `for`, or use an array. And use
**`git worktree add <tmp> main`, never `git stash`**, to build a "before" binary: stash leaves
untracked new files on disk referencing stashed-away code. That is about reconstructing a whole
BEFORE state. Stashing ONE tracked file (`git stash push -- path/to/file.go`, run the test, pop) is
a different move and a good one: it is how you prove a new test is red for the reason you think,
rather than red because the package does not compile.

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
- **The circuit, for software readers.** When semantics depend on hardware behavior (a rule's
  electrical meaning, derating, rail/pin conventions, why a limit matters physically), add this
  section right after "What changes", plus a link to `docsite/content/reference/analogy.md`. A
  pseudocode walkthrough alone leaves the hardware nouns opaque.
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

## Architectural constraints

`CONSTRAINTS.md` holds the enforceable rules (C1–C22). Read it before proposing changes, and **push
back when a request would violate one**: quote the constraint by name, explain the conflict, and ask
whether to proceed and whether the constraint should change. The point of constraints is that they
survive everyone forgetting why the rule exists. Push back on architectural smell even without a
constraint, and if the direction was wrong, suggest capturing it as one.

## What does not belong in this repo

This repo is public. A file committed here is world-readable from the moment it is pushed, and
`_hidden/` does not change that. Deleting it later does not remove it from history.

Never commit:

- **Customer or proprietary design data.** No real schematics or board files, no net names, no
  reference designators off a private design, no part numbers off a private BOM, no title blocks.
  This includes screenshots and PR images, which is why before/after captures come from the
  synthetic fixtures rather than a real board.
- **Market, competitor, strategy, or opportunity analysis.** This repo is the engine, not the
  business.
- **Paths into private folders**, private corpus locations, or customer names, including in comments,
  test names, commit messages, and PR bodies.
- **Vendor-licensed material** such as datasheet PDFs and extracted parameter documents. Facts
  transcribed from a datasheet into a fixture are fine, because facts are not copyrightable, but
  cite the document revision and page.

  **A FIXTURE IS NOT A CORPUS, and the difference is size.** A fixture carries the few rows its
  tests need, cited, and lives in `datasheet/param/testdata/` so `make testall` passes on a clean
  clone. A seeded corpus is the part's actual parameter set, belongs with the source PDFs, and lives
  OUTSIDE this repo — which is what `SourceDoc.locator`'s corpus-local posture and `--params <dir>`
  already assume. Transcribing a whole vendor table because it makes a better demonstration is how a
  fixture drifts into being an extracted parameter document; `txb0104.textproto` reached 389 lines
  that way and was cut back. If a new fixture is much larger than its neighbours, that is the signal.

  Anything user-facing (`examples/`, the tutorial project) uses SYNTHETIC parts, not transcriptions.

**Sanitize at the point of writing rather than cleaning up later.** The engineering content nearly
always survives sanitizing and only the provenance goes. "Customer item 112 mock-failed against a
fake RSTRAP threshold" becomes "a strap resistor's value is a design choice, not a datasheet
parameter", and the second version is the better issue anyway because it states the general rule.

**If a learning cannot be stated without the private context, it does not belong here.** Write it in
the private workspace instead. When a rule's motivation is general EE practice, it belongs here; when
sanitizing would gut it, it does not.

## Writing style for docs and commits

Plain declarative prose. No em-dashes, no marketing cadence, no hype adjectives, and no rhetorical
"The result: X" constructions. Write separate sentences instead. This applies to the docsite, commit
messages, and PR bodies.
