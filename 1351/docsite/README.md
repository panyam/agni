# Working on the docsite

How this site is wired and where a new page has to be registered. This file documents the build
tooling for `docsite/` and is not itself published, since `content/` is what the site builder reads.

## Registering a page

**FOUR edits for a new page in an existing section, FIVE for a new SECTION.** A page needs the file,
the section's `index.md`, `templates/nav/<Section>Nav.html`, and `content/HeaderNavLinks.json`. A new
section additionally needs `templates/Sidebar.html`, and that one is TWO edits in the same file: the
`{{# include #}}` at the top AND a branch in the `Contains $currentPath` dispatch chain. Miss the
branch and the section silently renders the generic fallback nav.

`docsite/nav_test.go` runs in `make testall` via the `docsite-test` target, and it found two live
drifts when it was added. If you are adding a SECTION, let the test tell you what you forgot rather
than working from this list.

**For a PAGE all four edits are now checked, with one gap named below.**
`TestEveryPageIsReachable` catches a missing `templates/nav/<Section>Nav.html` entry,
`TestSectionIndexLinksEveryPage` catches a missing line in the section's `index.md`, and
`TestHeaderDropdownListsEveryPage` catches a missing `HeaderNavLinks.json` entry (agni issue 574).
That last one checks the content-to-json direction, and `TestHeaderNavLinksResolve` checks
json-to-content. With only the second half, a page shipped reachable from the sidebar and absent
from the header.

**The header check exempts a section that has no top-level dropdown of its own**. Today those
sections are `learn/` and `build/`. The exemption is derived from the file rather than from a list in
the test, so a section is required to be complete only when its own top-level entry carries
`children`. `learn/` lists two of its fifteen pages on purpose, since thirteen chapters would swamp
the menu. Give it a top-level dropdown and every chapter becomes required with no edit to the test.
The exemption leaves one gap. A section can be itemised only as a group heading inside another
dropdown, and `build/` is the live case. Its seven pages are all listed today, but nothing would
notice an eighth going missing. The index check has no exemption, so such a page is still caught
there.

## Links between pages

Write a link to another page as its URL relative to this one, the way the rest of the site does:
`../evidence/` within a section and `../../build/extending/` across one, since `content/x/y.md` is
served at `x/y/`. A bare `](evidence.md)` is left as written and 404s. `TestBuiltSiteLinksResolve`
in `links_test.go` builds the site. It fails on any link into the site that names no page or file,
and it names the page the link is on (agni issue 1169). It checks the BUILT pages rather than the
markdown, because the builder rewrites some links and the nav templates make others. It skips
`demo/`, which `docs.yml` deploys beside the site, and `pagefind/`, which is written after the
build. Fragments are not checked.

## Raw HTML and anchors

**A blank line inside raw HTML SPLITS it, and the render breaks silently.** Content pages may embed
raw HTML (inline SVG figures, the home page's cards) because the renderer passes it through. But
imagine a `<figure>` broken up with blank lines for readability. CommonMark ends an HTML block at
the first blank line, so the figure becomes several blocks, and the fragments after the first are
parsed as markdown. Keep an embedded figure contiguous, with no blank lines between the opening and
closing tag. Nothing in the gate catches
this, since `nav_test.go` checks wiring and not rendering.

**A `learn/` chapter also needs an entry in the level index.** `content/learn/levels.md` maps every
`## Title (EEn)` section to the level it operates at, and `learn_levels_test.go` enforces it in both
directions plus the per-chapter pointer line. The index is hand-maintained. Without the test it
would go short in silence the moment a chapter gained a section. It caught a real omission on its
first run.

Two things about anchors, both learned the expensive way. **Do not put a link inside a heading**,
because the link text becomes part of the generated slug, so `## The role ([EE3](../levels/#ee3))`
produces `the-role-ee3levelsee3` and every inbound link breaks. The level pointer goes on its own
line under Prerequisites. And **the `{#custom-anchor}` syntax is not supported** by this renderer,
which leaks it into the visible heading text and doubles the slug. Use the natural slug the heading
produces.

## Generated command output

**A page's command output is GENERATED, not pasted.** A page holds
`{{ agniRun "content/<section>/runs/<name>.yaml" }}`; the yaml says what to run; a committed
`<name>.yaml.output` holds the capture. The directive emits the command AND the output, so neither is
hand-written and they cannot disagree. Regenerate with `make tutorial-runs` and read the diff before
committing.

**A lesson that is several commands uses `steps:` rather than a multi-line `script:`.** Each step is
one command, one capture and one rendered block, so the output sits under the command that produced
it. A `script:` produces a single block. That is right when the extra lines are setup the reader
must also type. Rungs 4, 5 and 6 open with a `sed -i.orig` that deletes a tier's line from
`project.yaml`, and each wants that line in the same block as the command it changes. A single block
is wrong when both commands print something the reader is meant to compare. Four specs used to paper
over that with `echo "no dot:"` labels between the runs.

```yaml
steps:
  - show:   agni stats readers/kicad/testdata/tjunc.kicad_sch
    script: agni stats readers/kicad/testdata/tjunc.kicad_sch | grep -E 'components|nets'
  - script: agni stats readers/kicad/testdata/tjunc_dotted.kicad_sch
```

Steps share one scratch directory but not one shell, so a step reads what an earlier one WROTE (rung
11 stores a results document and re-renders it) and does not inherit its shell variables. `capture:`
and `match:` apply to every step, `exit:` applies to the last, and a step that captures nothing
renders as the command alone. `script:` and `steps:` are mutually exclusive and a spec setting both
is an error. In the capture file a `#agni-step` line separates one step's output from the next. A
one-step spec writes none, so adding the field regenerated no existing capture.

**Copy on a rendered block takes the COMMAND, not the transcript.** `codeblocks.js` treats a leading
`$ ` as the prompt, strips it, and follows a trailing backslash onto the next line, so a block copies
something you can paste. A fence with no prompt lines copies whole, which is every hand-written one.
`renderRun` prompts only the line that STARTS a command, because a `$` on a continuation line made
the block read as two commands and copied a broken one.

**Any section can have a `runs/` directory**, and `guide/` now does. Both the generator and
`hack/tutorial_runs_check.sh` walk `docsite/content/**/runs/`, so a new one joins the harness by
existing, with no Makefile edit.

`guide/` got one because a typed fence goes wrong in silence and several had. `getting-started.md`
reported 10 findings where its fixture produces 11 and named 29 rules where it runs 78, with the
prose beneath quoting the wrong number back. `querying.md` showed the `rail` relation finding one
rail under the built-in vocabulary when it finds four. The section's argument depended on that
contrast, and the contrast had collapsed. `naming-conventions.md` was worse than stale. It reported
findings from a convention called `strict/sig-only`, which the page never defines, under a header
claiming two over a list of one.
Every guide fence that shows output is generated now, apart from the `agni version` block, which is
build- and host-dependent and therefore uncapturable by construction. The last three to convert were
the `--format markdown`, `json` and `csv` blocks on `checks-and-reports.md`. They needed an engine
fix first. Provenance recorded an absolute host path, so a capture would have committed the machine
that ran it, and the runner refused the output (agni issue 501).

**A fence with no output block is usually better left alone.** A command over a placeholder path
cannot disagree with the tool. Capturing it adds nothing, and the page loses its "your board"
voice. Convert a fence when it shows a TRANSCRIPT.

## The command parse check

**A hand-written `agni …` command is parse-checked, and the check depends on the fence's SHAPE.**
`TestDocumentedCommandsParse` in `cmd/agni` walks every markdown file under `content/`, pulls the
commands out of fenced blocks and runs each through the real command tree, so a renamed subcommand or
a dropped flag fails the gate on the page that uses it rather than only on the page that documents
it. Three rules decide what it reads, and a fence that breaks one is skipped rather than reported:

- A block containing any `$ `-prefixed line is a TRANSCRIPT, so only the prompted lines are treated
  as commands. An `agni open` capture prints a matching `agni check …` line, and this rule keeps
  that output from being parsed as a command.
- A line indented under a line that did not end in a backslash is OUTPUT, and it disqualifies the
  whole block. This rule keeps out the fence showing `agni v0.1.1` and its indented build detail.
  That fence opens with the word agni and is not a command at all.
- A backslash continuation is joined, a trailing `#` comment and a `>` redirect are dropped, and an
  unquoted `<` is kept, because every one in the tree opens a placeholder (`agni check <file>`).

`docCommandCount` pins how many commands that finds, so a fence written in an unrecognised shape
shows up as a coverage DROP rather than as silence. Update it deliberately, and note which way it
moved: down is normal when a fence converts to an `agniRun` spec, because a spec runs its command for
real and needs no parse check.

**The parse check does not run the command**, so it covers three kinds of fence a capture cannot.
`agni serve` never terminates, some commands name the reader's own board, and one is on the page to
show a mistake. In exchange, a well-formed argument with a wrong value gets through. `--format jsn`
parses, and so does a `--web-dir` naming a directory that does not exist. That missing directory was
one of the four regressions in agni issue 469. A fence whose OUTPUT matters wants a spec instead.

**`show:` prefixes every line it displays with `$ `**, so a backslash continuation inside a `show`
block renders as two prompted commands. Keep a shown command on one line even where the page used to
wrap it.

**A section picks one of the two working-directory modes by how its commands read.**
`getting-started` names full paths (`agni check cmd/agni/testdata/conformance/...`), so its specs
set `from_root: true` and run at the scratch root. `checks-and-reports` names bare filenames, so its
specs want the default instead. In the default mode the script runs INSIDE the copied fixture
directory, and a bare `showcase.fires.kicad_pro` resolves exactly as displayed. Pick the one that
makes the shown command the command that ran, and never use `show` to paper over the difference.

`tutorial-runs-check` IS in `testall` (CONSTRAINTS C27). It deletes every capture, regenerates, and
fails on any difference. No other check works here. A capture's freshness stamp covers the spec and
the fixture but NOT the engine build, so an engine change that alters the output leaves every stamp
valid. As a measurement, one rule's coverage line was reworded. A plain docsite build then rewrote 0
captures, and a forced regeneration rewrote 12. Before that target existed, a capture
edited by hand passed the entire gate.

Four things about writing one. **Never make the directive rewrite the page**, because the site
builder reads `content/`, and a build that wrote back into it would loop. **Use the fields
(`capture: stdout|stderr|both|none`, `exit: true`, `match: '<re2>'`) rather than shell plumbing**,
because a positional filter (`sed -n '5p'`) silently shows the wrong line the moment that output
gains one; `match` selects by shape and matching NOTHING is an error. **Add `show:` only when the
script carries plumbing a reader should not see**, since it defaults to the script. And **every run
gets a scratch copy of the fixture**, so a rung that edits the fixture (`sed -i.orig '/^params:/d'
project.yaml`) cannot change the checked-in one. An older rung renamed the checked-in params folder
once, by hand.

**A step that exits non-zero and captures nothing is a build error**, unless the spec sets `exit:
true`. That field shows the status as the block's last line, and it is the declared way to make the
exit status the lesson. A failing step that prints its report is still legal (a gate tripping), and
so is an empty capture from a step that succeeded. Before this rule, rung 7 shipped two captures of a
command that failed outright. Each held only its stamp, and the page showed two empty blocks. The
gate was still green, because regenerating reproduced the emptiness exactly (agni issue 825).
`capture: none` is covered the same way, so a step still has to succeed or say why not, even when the
page hides its output.

**A before/after over a project tier has to delete the tier's line in the script.** Naming the
design is enough for its project to compose the `conventions:` section, `profiles/` and `params/`. So
dropping the flag does not reach the "before" state, and the two captures come out identical. Rung 6
shipped that way. Both of its captures ran with the corpus in place. The page taught a contrast, and
its own output disproved it. Rungs 5 and 6 now open with a line like
`sed -i.orig '/^params:/d' project.yaml`, and rung 4 deletes the section from its scratch copy with
`sed -i.orig '/^conventions:/,/^$/d' project.yaml`. Before adding a tier flag to a capture, check
whether the fixture's `project.yaml` already names the tier.

**`from_root: true` runs the script at the scratch root with the fixture at its full relative path**,
so a command reads as a reader would type it standing in a clone. The `learn/` runs use it; the
tutorials deliberately do not, because they establish one working directory at the top of the course
and every rung is relative to it. It is a mode, and a spec does not fake it with `show`, so the
command displayed is the command that ran. Using `show` to swap in a different path would put an
untested command in front of the reader. Output is unaffected either way, since provenance and
resolution notes are reported relative to the design's project rather than to the invocation.

## Figures and branch previews

**A generated capture COUNTS as an illustration, and that rule decides where a figure goes.** A page
carrying five `agniRun` blocks is already broken up by five pieces of real output, so adding a
diagram to it is decoration. Four `tutorials/` rungs gained a figure, and they are exactly the four
with no capture at all. The rule picked those four, and nobody chose them by hand. Rung 11 is the
longest page on the ladder and got no figure, because it carries eight captures. Unbroken prose is
the trigger, and length is not.

**Figures are generated too.** `make -C docsite images` runs `images.sh`, which re-renders every
engine-drawn image in the docs, the tutorial's board drawings and the schematics in `learn/`
included. `docsite-images-check` in the gate fails when one disagrees with what the engine draws
today (agni issue 453). A new one is a line in `images.sh`, never a hand export.

**To review a branch before it merges, use `make -C docsite preview PAGE=<page>`**, which folds one
built page into a self-contained HTML file that opens anywhere. Do NOT reach for `make gh-pages`,
which force-pushes `dist` to a branch that GitHub Pages does not serve. Pages here is configured
`build_type: workflow`, so the live site is whatever `docs.yml` uploaded on the last push to `main`,
and the gh-pages branch has not been served since the MkDocs tree was retired. The target is kept,
with that note written above it. Switching Pages back to branch-serving would enable real per-branch
preview URLs, and deleting the target would hide that option.

## Hand-written blocks and tutorial drift

Some blocks cannot be generated, so they are hand-written and at most parse-checked. Examples are an
`agni serve` that never returns, an excerpt of a longer output, and a step needing a tool the build
cannot assume (rung 12 shells out to `kicad-cli`). Generate what can be generated rather than
softening the check to cover the rest. Do not tag a generated fence `console`, because Chroma's
console lexer renders the whole body as error tokens.

**Verifying a tutorial's claims keeps finding bugs in the ENGINE, not the docs.** It has happened
three times so far. A rung argued that narrowing a gate makes a board pass, when it reveals the next
failure instead. `agni query` printed an absolute host path in provenance. And two rungs' numbers had
drifted. Treat a mismatch as a question, never as "regenerate and move on", because regenerating
blesses whatever the code currently does. That is right when the doc drifted and wrong when the code
regressed.

**Changing the tutorial FIXTURE can invalidate a rung's lesson, and the fix is a judgement about what
the rung teaches.** Pin functions were seeded into the tutorial's two synthetic specs. The datasheet
then classified those rails regardless, so rung 4's "without this project's naming vocabulary, only
GND is a rail" became false. Both statements were true; they just could not share a run. The rung
now deletes both the `params:` line and the `conventions:` section, so it isolates NAMING as it
intends, and the page says why. Regenerating instead would have shipped a page contradicting its own
output. When a fixture edit changes a capture's CONTENT rather than its stamp, find which page reads
it and decide what that page is for.

## Glossary terms

**When the docsite explains a term more than once, the term belongs in the glossary.**
`{{ explainable "termination" }}` inlines a hoverable link whose popover carries the whole term page,
diagram included, so a page can USE a term without re-teaching it. The gloss lives in ONE file,
`content/reference/terms/<id>.md`, and every use site writes the tag.

Four things about it. **Adding a term is ONE file plus one line in the glossary index**, with no nav
wiring, because `nav_test.go`'s reachability check reads files directly under a section and skips
subdirectories, the same reason the generated catalogs need none. **The frontmatter carries `label`
separately from `title`**, because `title` is the page's heading and `label` is the mid-sentence form
the tag inlines; a term that is always capitalised (`EDIF`) sets `label` that way and needs nothing
else, and `{{ explainableCap "id" }}` covers a term opening a sentence. **A second argument overrides
the label** for a plural or an inflection: `{{ explainable "differential-pair" "pair" }}`. And **the
popover fetches the term PAGE rather than a generated blob**, so there is no second copy of a
definition to rot. `agnirun.go` makes the same argument for captured output.

Two prose conventions go with it, and they answer different questions.

**A `learn/` chapter still teaches a term in full the first time it
introduces it**, and every later mention anywhere on the site is a tag. Chapter 10's opening paragraph
was four inline glosses of terms chapter 1 had already taught, and it is now four tags.

**Within one page, tag a term ONCE, at its first mention.** The tag is a reading
affordance and a page that tags every occurrence spends it. `pull-up` appears twenty times in prose
outside `learn/`, and twenty dotted underlines in one tutorial reads as damage rather than as help.
One tag is enough, because the popover is reachable from it and the glossary is one click further.
`TestATermIsTaggedAtMostOncePerPage` enforces the count. It deliberately does NOT check that the
tagged mention is the first one. To decide whether an earlier plain-text occurrence counts, a check
would have to match an inflected label through code spans, headings and link text. A check that
fires wrongly on an author costs more than the convention is worth. That half is on you.

`terms_test.go` enforces the rest and fails the gate on a tag naming a term that does not exist, a
term with no `label` or `summary`, a term missing from the glossary index, and a term nothing
references. That last one is deliberate. A glossary entry with no caller means someone wrote a
definition and the prose never adopted it.

**The tag works without JavaScript**, rendering an ordinary anchor with the summary in `title` and a
click that opens the full page, so `terms.js` is an upgrade rather than a dependency.

## Mermaid diagrams

**A mermaid diagram has to be RENDERED before it is committed, and parsing is not the check.**
Diagrams go in ` ```mermaid ` fences and `BasePage.html` loads mermaid lazily on pages that contain
one. When the three `build/` pages gained ten blocks, every block parsed on the first attempt, and
four still had to be rebuilt. A nine-step `flowchart LR` came out at a 19.6:1 aspect ratio, and the
site's `max-width: 100%` then scales it down to unreadable. A seven-leaf `TB` taxonomy fanned out
just as wide. A subgraph whose title was longer than its child node rendered with the TITLE CLIPPED.
None of those is a syntax error and none is visible in the source.

Render each block with `mmdc -i block.mmd -o block.png` and look at the aspect ratio before you
commit. **WIDE is the failure, and tall is not.** `max-width: 100%` scales a block wider than the
content column down until its labels are unreadable, so a 19.6:1 fan-out is unusable. A tall one is
never scaled, because `mmdc` caps its output at 784px against an 800px column, so it renders at
natural size and simply occupies more vertical space. Diagrams at 0.74:1 and 0.76:1 read fine on the
site today. Treat roughly 4:1 as the ceiling, and treat a low ratio as a question about whether the
diagram is worth its height rather than a defect.

Past the ceiling, the fixes are to switch `LR` to `TB` (or the reverse, since a wide fan-out becomes a
tall stack), to wrap a long pipeline into rows with subgraphs, or to move the detail from side notes
into the node labels. Prefer plain nodes over a subgraph whose title is long. `CONTRIBUTING.md` covers
the separate quote-decoding trap that applies to a diagram in a PR body rather than a page.

**The browser sweep below does NOT cover mermaid**, because a mermaid block renders client-side from
a `<pre>` and a detached-div fetch never runs it, so a figure sweep that comes back clean says nothing
about the diagrams. Sweep them separately by extracting every fenced block, rendering each with
`mmdc`, and reading the PNG's dimensions. Doing that over all 29 blocks found five past the ceiling,
where two had been spotted by eye, so sweep rather than look. All five are fixed and every block on
the site is now under 4:1.

**Flipping `LR` to `TB` is half the fix on a diagram with two long labels.** In `LR` the width comes
from the chain, so the flip removes it. In `TB` two long labels can sit on the same RANK and set the
width again. `port-protection` and `differential-pair` did that until a label also gained a
`<br/>`. Re-measure after the flip rather than assuming it worked.

## Hand-authored SVG figures

**A hand-authored diagram lives in `figures/` and a page pulls it in with
`{{ includeFile "figures/<name>.svg" }}`. Do not paste an `<svg>` into a markdown page.** The
directive reads the file at BUILD time and returns raw HTML, so the SVG is still inline in the
rendered page. Inlining keeps the theming rule below working. The include also keeps several hundred
characters of path data out of the prose, where it buries the sentence either side of it.

Three things about the arrangement. **A bad path fails SILENTLY**, because `IncludeFile` returns an
empty string when the file does not resolve, so a rename drops the figure from the page and the build
still succeeds. `includefile_test.go` turns that into a gate failure. It also fails on a figure
nothing includes, for the same reason that `terms_test.go` rejects a glossary entry with no caller.

**A blank line inside a figure file is NOT harmless**, and `TestFiguresCarryNoBlankLines` fails the
gate on one. `IncludeFile` splices the file in before the markdown renderer runs, so the SVG is
ordinary raw HTML by the time CommonMark sees it, and CommonMark ends an HTML block at the first
blank line. The chunk after that line passes through raw or not depending on its first line. A line
holding nothing but an open tag (`<g fill="...">`) opens a new block and survives. A line with
content after the tag (`<text x="8" y="44">your machine</text>`) is parsed as a paragraph. The
renderer wraps that chunk in `<p>`, which closes the `<svg>` early. Every element after it then sits
outside the figure and never draws. Four merged figures sat in that state, and each was missing its
closing caption on the live site. Screenshotting one and counting its text runs found them. And
**the file is not served**, because `figures/` sits outside `static/`, so the only way to reach a
diagram is the include, and there is no second copy to drift.

**A count in a lead-in drifts, and a grep finds it.** "Four more need their own room" sat above five
paragraphs on `build/evidence.md`, and "Two of the questions carry a lesson wider than this rule"
sat above three on `build/check-rule.md`. There the third had also drifted from a paragraph into a
bullet, and that is probably how it went unnoticed. Sweep for it by grepping the content tree for a
number word followed by `things|ways|rules|reasons|more|questions|traps`, then counting the bolded
leads and list items in the lines that follow until the next heading. Most hits are ordinary prose
("two different questions") so read the output rather than trusting it, and it is too noisy to be a
gate test. The durable fix is to number the items and drop the total from the lead-in. The count is
then written in one place and cannot disagree with itself.

**Copy the nearest existing figure rather than starting from an empty file.** Forty-nine figures
in, one shape has held. Each has a `viewBox` 640 wide against an 800px column, so a figure never
scales up. Each has a `<title>` whose `id` ends `-title`, carrying the alt description a screen
reader gets. Structure uses `currentColor` between 0.35 and 0.85 opacity, `var(--accent-color)` is
reserved for the ONE thing the figure is about, and a caption line or two sits at the bottom at
`font-size="11" opacity="0.65"`.
`figures/covered-vs-answered.svg` is a plain one to copy and `figures/output-contention.svg` is a
circuit. A figure that should not fill the column sets `max-width` on the root instead and centres
itself. The two package drawings on `reference/pins-and-packages.md` do that, and it is a sanctioned
variant rather than a mistake to copy away.

**Give it a `<title>`, not an `aria-label`, and keep `role="img"`.** Both give a valid accessible
name, so this is a consistency rule rather than an accessibility one, and it exists because the
verification sweep has to SELECT the figures. Select on `svg[role="img"]`, which all 49 carry, and
never on the title's id. A sweep keyed on the title silently skipped the two figures that used
`aria-label` for their whole life, and a skipped figure looks exactly like a clean one in the
results.

**Style that SVG through `--accent-color` and `currentColor`, never a literal.**
`static/css/main.css` defines the palette for both themes, and the docsite has a dark mode. A
hardcoded hex reads fine in whichever theme it was authored in and badly in the other.
`TestFiguresCarryNoColourLiterals` enforces it for everything under `figures/`. The same rule applies
to an `<svg>` written into a TEMPLATE, as `Header.html` and `Sidebar.html` do for UI icons.

**A diagram referenced as an IMAGE cannot follow that rule, so it needs a theme-neutral palette
instead.** An `<img src="....svg">` renders in its own document and inherits nothing from the host
page, so `currentColor` resolves to the SVG's own default and a CSS variable resolves to nothing.
Both diagram sets under `static/images/` are image-referenced
(`![alt]({{.Site.PathPrefix}}/static/images/...)`) and therefore use literals on purpose. `analogy/`
established the palette to match, and `datasheet/` follows it. It uses mid-grey `#808080` for
structure and captions, `#4a90d9` and `#5aa06a` for two accented roles, and `#d98a4a` for a warning
or an override, all chosen to read on either theme. A new hand-authored diagram should go through
`figures/` and the include instead, and so get the real palette back. The image-referenced sets
predate the include, and this paragraph exists because of them.

## Checking rendering in a browser

**Check a hand-authored SVG's geometry rather than reading it.** The failure is text or a box that
sits outside the canvas, which is invisible in the markup and obvious on the page. Parse the file and
walk every `text` and `rect` against the declared `width`/`height`, estimating a text run at about
0.55em per character and honouring `text-anchor`. That check caught a caption running 11px past the
right edge of `datasheet/pin-precedence.svg`. The estimate is a fallback, though. **Playwright against
a local `go run .` is the real check.** It calls `getBBox()` on every `text`, `rect`, `circle`,
`line` and `path` and compares each with `svg.viewBox.baseVal`, which reports the true rendered
geometry rather than a guess. It can sweep every page in one `page.evaluate` by fetching each into a
detached div, and eleven figures were cleared at once that way. Playwright still refuses `file:`
URLs, so serve the site.

**The sweep is THREE checks, and it lives in `web/browser/figures.spec.ts` under `make
browser-test`**, not in the Go gate, because it needs a served site and a real Chromium. **Bounds**
is a `getBBox()` against the viewBox. **Text on text** compares every pair of `text` boxes, because
two runs sitting on each other is a different failure from one leaving the canvas. Of 32 figures
that came back clean on bounds, one had two labels printed over each other. **Text on a wire**
compares each `text` box against every `line` and `path` whose own box is thin on one axis. The
third check is the easiest to get wrong. A label a few pixels off can sit exactly on the rail it
names and be fully inside the viewBox, so the first two checks both pass it.

**`getBBox()` excludes the STROKE, so that third check could not fire for its whole first life.**
A horizontal line's geometric box is zero tall and a vertical one's is zero wide, so a text box can
never overlap it on the thin axis and the check reported nothing on any figure. It read as a clean
result. A red-check of the assertion exposed it, where trusting it had not. Moving a label onto a
ground rail on purpose left the suite green. The fix is to inflate the thin axis by half the stroke
width with a floor, so the box covers the ink rather than the path. Two figures had been reported as
catches of this check and were not; both were found by looking at a screenshot.

**Four things about verifying rendering in a browser, all of which produced wrong numbers first.**
The stylesheet is cached HARD, so a CSS change measured straight after an edit reports the OLD
layout and looks like the change did nothing. Bust it by rewriting each `link[rel=stylesheet]` href
with a cache-busting query before measuring. **A running `go run .` holds an included FIGURE the
same way**. Edit a file under `figures/` and re-fetch it, and the server still serves the version it
started with. No query string helps, because the staleness is server-side, so restart the server.
And a section index is served at `/agni/<section>/`, so `/agni/<section>/index/` correctly 404s and
is not evidence of a broken page. Worst of the four, **`dist` has to be served UNDER the `/agni/`
prefix**, because a page links its stylesheet as `/agni/static/css/main.css`. Served at a server's
root, `dist` 404s every asset, and the page renders with NO CSS while still returning 200. The nav
then lays out as the mobile stacked column at any viewport width. A measurement of it reads as a
broken desktop rule rather than as a missing stylesheet, and that misreading cost two rounds of
diagnosis. Symlink the tree in and serve the parent (`ln -s <abs>/docsite/dist <tmp>/agni`), and
confirm `getComputedStyle(document.querySelector('.main-nav')).display` reads `flex` before trusting
a number.

## Tables and narrow viewports

**Table cells WRAP.** They used to carry `white-space: nowrap`, which laid a prose table out on one
unwrapped line: 47 of the site's 62 tables overflowed their column and the worst ran 5.34x the content
width. Prose in a table is fine now. An unbreakable token still forces a scrollbar, since inline
`<code>` keeps `nowrap` so an identifier is not split mid-name. Avoid a table whose cells hold long
identifiers in BOTH columns. The EDIF construct-to-IR mapping was one, and it needed 1379px in an
800px column. It became a stacked list, where each identifier gets the full width.

**Below 768px that behaviour holds, and the page itself never scrolls sideways.** Measurements at
a 420px viewport covered the course, the architecture pages, the CLI reference and the EDIF primer.
On every one, `document.documentElement.scrollWidth` equals the viewport. A table too wide to wrap
scrolls inside its own box instead (worst seen, the five-column levels table at 2.01x), a `pre`
scrolls the same way, and an included figure scales down with the column rather than overflowing it.
So `.main-nav`'s `overflow-x` is not implicated here, because nothing at this width is an absolutely
positioned child of a clipping ancestor.

**Measure layout on the REAL page, never in a detached div.** The detached-div sweep is right for a
figure, because `getBBox()` reports SVG user-space coordinates that do not depend on where the
element sits. It is wrong for anything the page grid decides. In a detached 800px div, the same
levels table measured 777px against a 452px container, and on the real page it fits its column
exactly. The div has no grid to inherit, so the article width it computes is fiction.

## The header nav

**`content/HeaderNavLinks.json` is hand-formatted with one compact object per line.** Read it as text
before editing. Piping it through a pretty-printer to find the insertion point produces a shape that
does not exist in the file, so the edit fails to match.

**A header entry has two optional fields beyond `name`, `url` and `children`.** `"group": true` on a
CHILD turns it into a subheading inside the dropdown, and it is still a real link. Someone clicking
a heading wants to reach the section index the heading points at. `"columns": 2` lays that
dropdown out in columns, with `break-before: column` starting each group in its own, so the break
means one column per group rather than falling at the halfway point. `Guides` and `Reference` use
both. Nesting is ONE level, since the template renders `children` and nothing deeper.

**Grouping is presentation, so it lives in the presentation layer.** Folding two sections under one
heading moved no content, so `content/guide/` and `content/build/` did not move, and every
published URL and cross-link still resolves. Renaming a content directory to match a menu label would
break both, and the change's whole purpose is that the header reads better.

**`static/js/navdropdown.js` positions an open dropdown, and CSS alone cannot.** It measures and
clamps into the viewport, which no stylesheet can do because it cannot know whether a menu will fit.
Three problems led to it. An 18-entry menu measured 900px against an 863px viewport with nothing
scrolling. The rightmost menu ran past the right edge. And below 768px `.main-nav` is
`overflow-x: auto`, which CLIPS an absolutely positioned child. Positioning `fixed` fixes all three,
since a fixed box cannot be clipped by an ancestor. The script stands down below 768px, where the
menu is a stacked list with its submenus inline and CSS owns it.

**Measure a dropdown only while it is SHOWN.** The first version read `offsetWidth` on `pointerenter`,
while CSS still had it `display: none`. A hidden element measures zero, so the clamp positioned the
menu as though it took no space, which is indistinguishable from having no clamp at all.

**Below 768px the header and the sidebar both collapse behind a button**, wired by the same script.
The bar used to remain on screen and scroll sideways, hiding most sections behind a gesture nobody
discovers, and the sidebar used to render above the article in the one-column grid and push the page
down before a reader saw a word.

**A hover bridge cannot live on the element that SCROLLS.** The header dropdown sits 0.5rem below its
nav link, and crossing that gap counts as a mouseout unless something invisible spans it. The bridge
was a `::before` on `.nav-dropdown` at `top: -0.5rem`. It stopped working the day the panel gained
`overflow-y: auto` for the menus that ran off the bottom of the screen. An overflow value makes the
element a scroll container, and a scroll container clips everything outside its padding box, so the
bridge was neither painted nor hit-tested. It now hangs off `.nav-item`, which does not scroll. It
has a `min-width` matching the panel, so a diagonal move toward a wide menu remains over it.
Probe it with `document.elementFromPoint` in the middle of the gap while hovering. A hit inside the
`.nav-item` is correct, and reading `site-header` there means the bug is back.

**Two CSS blocks are order-dependent and say so inline.** The column overrides and the toggle base
rules each tie on specificity with the rules they override, so SOURCE ORDER decides. Both were
written in the wrong place first. The second left every toggle invisible while the menus were
hidden, so the header had no way to open them. Nothing in the gate catches this, so read the notes
above those blocks in `static/css/main.css` before moving them.

## Hidden pages and search

**`docsite/_hidden/` hides pages from the SITE BUILD, not from the repo.** Files under it are still
tracked and world-readable. A parked section sat there for months in exactly that state. Moving
something to `_hidden/` is a publishing decision and never a confidentiality one. Anything genuinely
sensitive has to leave the repo and its history.

**Search is a post-build index over `dist/`, not over `content/`.** `make build` ends by running
`pagefind --site dist`, which writes `dist/pagefind/`. Indexing the BUILT site is deliberate, because
`_hidden/` pages never reach `dist`, so they are left out of the index without a second exclusion list
to keep in sync. The step lives in `build` rather than in `gh-pages` so a local build and the deploy
produce the same tree.

Search breaks in a visible way if any of three things goes. **`Content.html` carries
`data-pagefind-body`** on the article, because Pagefind drops `<nav>` by itself but NOT `<header>`,
so without it every excerpt opened with the logo and the GitHub link. **The search overlay in
`BasePage.html` carries `data-pagefind-ignore`**, because Pagefind indexes hidden text, so the
overlay's own copy would otherwise be indexed on every page and "index" or "build" would match the
whole site. And **the Pagefind bundle is fetched on first open, not on load**, so a visit that never
searches downloads none of the ~135KB of JS and CSS.

`make run` does not build an index, since it serves from `content/` with no `dist`. The overlay
detects the missing bundle and says so rather than hanging. Use `make build` to exercise search
locally, and note it must be served under the `/agni/` prefix for the bundle path to resolve.

## Content is templated BEFORE it is markdown, so a stray `{{` blanks the page

Every content file is run through `text/template` first. The templater makes `{{ agniRun "..." }}`,
`{{ includeCard "..." }}` and `{{.Site.PathPrefix}}` work. It runs over the whole file, code fences
included, because there is no markdown yet when it runs.

The failure is silent and total. An unparseable action fails the whole page load rather than the
offending line, and the page still builds and still exists at its URL with nothing in it but the
title and the footer. `build/check-rule.md` shipped that way for months. Two Go samples contained
`[]check.ContextSubject{{`, an elided composite-literal brace, and the templater read it as an
action calling a function named `Kind`. Twelve sections of the repo's rule-authoring guide were
absent from the site and nothing reported it.

So **write a Go composite literal with the element brace on its own line**, and treat any `{{` in a
sample (Rust macros, Vue, Handlebars, Go templates as subject matter) as needing the same care.

**A TRANSCLUDED file does not get that pass, so the trap runs the other way.** `includeCard` is called
DURING a host page's template execution and its return value is spliced in afterwards, so nothing
re-scans it. A generated card wrote `{{.Site.PathPrefix}}` in an image path. It rendered correctly
on its own page, but when transcluded the directive reached the browser verbatim. The browser
requested a literal `%7B%7B.Site.PathPrefix%7D%7D`, which 404'd on four images across two guide
pages. `IncludeCard`
substitutes the directive now, and `includecard_test.go` asserts on what it RETURNS plus follows the
substituted path to disk, because a wrong prefix would pass a braces-only check. Anything else
transcluded needs the same treatment.

`TestEveryContentPageTemplateParses` in `docsite/template_test.go` parses every content page with the
site's own `CommonFuncMap`, so a legitimate `{{ agniRun ... }}` passes and only an unknown function or
a malformed action fails. It runs in `make docsite-test`, which is in the gate. If you add a template
function, add it to `Site.CommonFuncMap` and the guard picks it up for free.
