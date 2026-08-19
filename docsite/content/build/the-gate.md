---
title: Running the gate
---

# Running the gate

`make testall` is the full gate: vet, engine tests, example modules, the web bundle build, and the
web typecheck plus vitest. Green means ship-ready, and CI runs exactly this command. It also lies to
you in three specific ways, and most of this page covers them.

## The three traps

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

## What the gate does NOT run

`make browser-test` is a separate suite of layout assertions driven through a real Chromium against
a real server (agni issue 323). It is deliberately outside `testall`.

jsdom has no layout engine, so every element reports a zero-sized box, `elementFromPoint` answers
nothing useful, and no CSS rule has any effect. The unit suite can therefore prove what a panel
RENDERS and nothing about what a reader can SEE, which is how a badge strip that painted over the two
columns beside it shipped through a fully green run. The browser suite closes that gap and pays for
it in speed and in needing a browser on the machine, so keeping it out of the gate means a machine
without one never turns CI red for a reason unrelated to the change under test.

It needs a browser once per machine (`cd web && pnpm exec playwright-core install chromium`) and
starts its own server on a port the kernel picks, so it does not fight a dev server you already have.

**Add to it sparingly.** Anything assertable in jsdom belongs in `src/*.test.ts`, where it runs on
every gate rather than when somebody remembers. The rule for what goes in the browser suite is that
pixels are the claim. And read the layout traps in `build/evidence.md` before writing one: two
versions of the first test in that file went green with the CSS under test deleted.

## What a run leaves behind

Also expect: testall leaves `examples/render-board/render-board` and `examples/validate/validate`
built. Both are covered by per-example `.gitignore` files, so do NOT `git add` them. A reader test
also drops `readers/kicad/testdata/*.kicad_prl`, which `.gitignore` covers and which is never part
of a change. **Golden SVGs** fail by design on any render-affecting change. Regenerate deliberately
with `go test ./core/render/ -run Golden -update` and inspect the diff before committing.

**A tutorial run's committed output should no longer go stale on you.** It carries an `#agni-run`
stamp that is a hash of its inputs, and that hash used to cover every file in the fixture DIRECTORY.
The tutorial's own `make report` target writes into `examples/tutorial-project/`, gitignored, so
anyone who had followed the tutorial hashed two files nobody else had and every gate run rewrote a
committed output they had not touched. `git checkout --` on it became part of the routine.

It now hashes the fixture's git-TRACKED files, so a committed stamp is valid in every checkout (agni
issue 357). The general rule, if you add another generated artifact of this kind: a value written INTO
a committed file has to be a function of committed content, or no single value can be correct for two
people at once. Regenerating and committing is the fix that looks right and only moves the staleness
to whoever has not run the thing yet.

`kicad-cli` writes a `.kicad_prl` beside ANY board it reads, not only under test, so a
directory-wide `git add` will sweep them up. The ignore rule is deliberately SCOPED to
`readers/kicad/testdata/` rather than a blanket `*.kicad_prl`, because the two under
`cmd/agni/testdata/conformance/` are tracked on purpose: those fixtures carry them so a project read
sees the full sibling set. Do not "clean them up". If you point kicad-cli at a new folder, add an
ignore rule there, and stage by explicit path rather than by directory.

## Generated code

**After ANY proto change run BOTH `make proto` (Go) AND `make proto-web` (TS).** Additive fields
build green, so a skipped TS regen used to go unnoticed until the next regen churned. `make
proto-check` now fails the gate on either half being stale, and it runs inside `testall`, so this is
enforced rather than remembered. It names which half drifted and which command fixes it.

**Never hand-edit a generated file, and that includes reformatting it.** A commit that regrouped the
imports in `gen/go/agni/v1/param/param.pb.go` turned main red, because `proto-check` compares the
generated tree byte for byte, and **every open PR inherited the failure**, since CI checks out the
merge with main. The same commit sanitized a generated rule page differently from the `stdlib/`
source it is generated from, which the next `catalog-docs` run would have silently undone. Edit the
SOURCE and regenerate. A sweep over the tree (a prose pass, a formatter, an import tidy) has to
exclude `gen/` and the generated docsite directories, or it writes changes that cannot survive.

Unlike `catalog-docs-check`, **it carries no commit-first ordering trap**: it generates into a
throwaway tree and diffs, so regenerating and running the gate before committing works fine.

**An editor reporting the generated types as MISSING is usually a stale language server, not stale
generated code.** After a branch switch or a merge, the LSP can insist `Module ... has no exported
member 'Foo'` and `Property 'bar' does not exist` for symbols that are present in the file. It reads
exactly like the real hazard above, so it earns a name of its own: the two are told apart by
checking the FILE and a fresh `cd web && npx tsc --noEmit`, never the editor. A green `make testall`
alongside editor errors is the cache, not a bug; genuine staleness fails the gate.