# Working on the docsite

How this site is wired and where a new page has to be registered. This is the build tooling for
`docsite/`, not published content: `content/` is what the site builder reads.

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

**A `learn/` chapter has a fifth edit: the level index.** `content/learn/levels.md` maps every
`## Title (EEn)` section to the level it operates at, and `learn_levels_test.go` enforces it in both
directions plus the per-chapter pointer line. It is hand-maintained and would otherwise go short in
silence the moment a chapter gained a section, which is the same failure the generated captures exist
to prevent one layer down. It caught a real omission on its first run.

Two things about anchors, both learned the expensive way. **Do not put a link inside a heading**: the
link text becomes part of the generated slug, so `## The role ([EE3](../levels/#ee3))` produces
`the-role-ee3levelsee3` and every inbound link breaks. The level pointer goes on its own line under
Prerequisites. And **the `{#custom-anchor}` syntax is not supported** by this renderer: it leaks into
the visible heading text and doubles the slug. Use the natural slug the heading produces.

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

**`from_root: true` runs the script at the scratch root with the fixture at its full relative path**,
so a command reads as a reader would type it standing in a clone. The `learn/` runs use it; the
tutorials deliberately do not, because they establish one working directory at the top of the course
and every rung is relative to it. It is a mode rather than something a spec fakes with `show`, so that
the command displayed is the command that ran: using `show` to swap in a different path would put an
untested command in front of the reader. Output is unaffected either way, since provenance and
resolution notes are reported relative to the design's project rather than to the invocation.

**Figures are generated too.** `make -C docsite figures` re-renders the schematics `learn/` embeds,
via `figures.sh`. Outside the gate for the same reason `tutorial-runs` is: a render depends on the
engine build, so a code change would invalidate every figure on every branch.

**To review a branch before it merges, use `make -C docsite preview PAGE=<page>`**, which folds one
built page into a self-contained HTML file that opens anywhere. Do NOT reach for `make gh-pages`: it
force-pushes `dist` to a branch that GitHub Pages does not serve. Pages here is configured
`build_type: workflow`, so the live site is whatever `docs.yml` uploaded on the last push to `main`,
and the gh-pages branch has not been served since the MkDocs tree was retired. The target is kept, with
that written above it, because switching Pages back to branch-serving would enable real per-branch
preview URLs and deleting it would hide that option.

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

**A term the docsite explains more than once belongs in the glossary.** `{{ explainable "termination" }}`
inlines a hoverable link whose popover carries the whole term page, diagram included, so a page can USE
a term without re-teaching it. The gloss lives in ONE file, `content/reference/terms/<id>.md`, and the
tag is what every use site writes.

Four things about it. **Adding a term is ONE file plus one line in the glossary index**, with no nav
wiring, because `nav_test.go`'s reachability check reads files directly under a section and skips
subdirectories, the same reason the generated catalogs need none. **The frontmatter carries `label`
separately from `title`**, because `title` is the page's heading and `label` is the mid-sentence form
the tag inlines; a term that is always capitalised (`EDIF`) sets `label` that way and needs nothing
else, and `{{ explainableCap "id" }}` covers a term opening a sentence. **A second argument overrides
the label** for a plural or an inflection: `{{ explainable "differential-pair" "pair" }}`. And **the
popover fetches the term PAGE rather than a generated blob**, so there is no second copy of a
definition to rot, which is the same argument `agnirun.go` makes for captured output.

Two prose conventions go with it, and they answer different questions.

**Which page keeps the gloss:** a `learn/` chapter still teaches a term in full the first time it
introduces it, and every later mention anywhere on the site is a tag. Chapter 10's opening paragraph
was four inline glosses of terms chapter 1 had already taught, and it is now four tags.

**How often to tag within one page: ONCE per term, at the first mention.** The tag is a reading
affordance and a page that tags every occurrence spends it. `pull-up` appears twenty times in prose
outside `learn/`, and twenty dotted underlines in one tutorial reads as damage rather than as help.
One tag is enough, because the popover is reachable from it and the glossary is one click further.
`TestATermIsTaggedAtMostOncePerPage` enforces the count. It deliberately does NOT check that the
tagged mention is the first one: deciding whether an earlier plain-text occurrence counts means
matching an inflected label through code spans, headings and link text, and a check that fires
wrongly on an author costs more than the convention is worth. That half is on you.

`terms_test.go` enforces the rest and fails the gate on a tag naming a term that does not exist, a term
with no `label` or `summary`, a term missing from the glossary index, and a term nothing references.
That last one is deliberate: a glossary entry with no caller is a definition someone wrote and the prose
never adopted.

**The tag works without JavaScript**, rendering an ordinary anchor with the summary in `title` and a
click that lands on the full page, so `terms.js` is an upgrade rather than a dependency.

**A mermaid diagram has to be RENDERED before it is committed, and parsing is not the check.**
Diagrams go in ` ```mermaid ` fences and `BasePage.html` loads mermaid lazily on pages that contain
one. Every block parsed on the first attempt when the three `build/` pages gained ten of them, and
four still had to be rebuilt: a nine-step `flowchart LR` came out at a 19.6:1 aspect ratio, which the
site's `max-width: 100%` then scales down to unreadable; a seven-leaf `TB` taxonomy fanned out just as
wide; and a subgraph whose title was longer than its child node rendered with the TITLE CLIPPED. None
of those is a syntax error and none is visible in the source.

Render each block with `mmdc -i block.mmd -o block.png` and look at the aspect ratio before you commit.
**WIDE is the failure, and tall is not.** A block wider than the content column is scaled down by
`max-width: 100%` until the labels are unreadable, which is what makes a 19.6:1 fan-out unusable. A
tall one is never scaled, because `mmdc` caps its output at 784px against an 800px column, so it
renders at natural size and simply occupies more vertical space. Diagrams at 0.74:1 and 0.76:1 read
fine on the site today. Treat roughly 4:1 as the ceiling to stay under, and treat a low ratio as a
question about whether the diagram is worth its height rather than a defect.

Past the ceiling, the fixes are to switch `LR` to `TB` (or the reverse, since a wide fan-out becomes a
tall stack), to wrap a long pipeline into rows with subgraphs, or to move the detail from side notes
into the node labels. Prefer plain nodes over a subgraph whose title is long. `CONTRIBUTING.md` covers
the separate quote-decoding trap that applies to a diagram in a PR body rather than a page.

**A hand-authored diagram lives in `figures/` and a page pulls it in with
`{{ includeFile "figures/<name>.svg" }}`. Do not paste an `<svg>` into a markdown page.** The
directive reads the file at BUILD time and returns raw HTML, so the SVG still lands inline in the
rendered page. That is what keeps the theming rule below working, and it keeps several hundred
characters of path data out of the prose, where it buries the sentence either side of it.

Three things about the arrangement. **A bad path fails SILENTLY**, because `IncludeFile` returns an
empty string when the file does not resolve, so a rename drops the figure from the page and the build
still succeeds. `includefile_test.go` is what turns that into a gate failure, and it also fails on a
figure nothing includes, for the reason `terms_test.go` rejects a glossary entry with no caller.
**Blank lines inside the SVG file are harmless**, unlike a figure written straight into a page, so
format it readably. And **the file is not served**: `figures/` sits outside `static/`, so the only
way to reach a diagram is the include, and there is no second copy to drift.

**Style that SVG through `--accent-color` and `currentColor`, never a literal.**
`static/css/main.css` defines the palette for both themes, and the docsite has a dark mode. A
hardcoded hex reads fine in whichever theme it was authored in and badly in the other.
`TestFiguresCarryNoColourLiterals` enforces it for everything under `figures/`. The same rule applies
to an `<svg>` written into a TEMPLATE, which is what `Header.html` and `Sidebar.html` do for UI icons.

**A diagram referenced as an IMAGE cannot follow that rule, so it needs a theme-neutral palette
instead.** An `<img src="....svg">` renders in its own document and inherits nothing from the host
page, so `currentColor` resolves to the SVG's own default and a CSS variable resolves to nothing.
Both diagram sets under `static/images/` are image-referenced (`![alt]({{.Site.PathPrefix}}/static/images/...)`)
and therefore use literals on purpose. The palette to match is the one `analogy/` established and
`datasheet/` follows: mid-grey `#808080` for structure and captions, `#4a90d9` and `#5aa06a` for two
accented roles, `#d98a4a` for a warning or an override, all chosen to read on either theme. A new
hand-authored diagram should go through `figures/` and the include instead, which gets the real
palette back; the image-referenced sets predate that and are the reason this paragraph stays.

**Check a hand-authored SVG's geometry rather than reading it.** The failure is text or a box that
sits outside the canvas, which is invisible in the markup and obvious on the page. Parse the file and
walk every `text` and `rect` against the declared `width`/`height`, estimating a text run at about
0.55em per character and honouring `text-anchor`. That check caught a caption running 11px past the
right edge of `datasheet/pin-precedence.svg`. The estimate is a fallback, though. **Playwright against
a local `go run .` is the real check and it works**: `getBBox()` on every `text`, `rect`, `circle`,
`line` and `path` against `svg.viewBox.baseVal` reports the true rendered geometry rather than a
guess, and it can sweep every page in one `page.evaluate` by fetching each into a detached div. That
is how eleven figures were cleared at once. Playwright still refuses `file:` URLs, so serve the site.

**Two things about verifying rendering in a browser, both of which produced wrong numbers first.**
The stylesheet is cached HARD, so a CSS change measured straight after an edit reports the OLD layout
and looks like the change did nothing. Bust it by rewriting each `link[rel=stylesheet]` href with a
cache-busting query before measuring. And a section index is served at `/agni/<section>/`, so
`/agni/<section>/index/` correctly 404s and is not evidence of a broken page.

**Table cells WRAP.** They used to carry `white-space: nowrap`, which laid a prose table out on one
unwrapped line: 47 of the site's 62 tables overflowed their column and the worst ran 5.34x the content
width. Prose in a table is fine now. What still forces a scrollbar is an unbreakable token, since
inline `<code>` keeps `nowrap` so an identifier is not split mid-name. A table whose cells hold long
identifiers in BOTH columns is the case to avoid; the EDIF construct-to-IR mapping needed 1379px in an
800px column that way and became a stacked list, where each identifier gets the full width.

**`content/HeaderNavLinks.json` is hand-formatted with one compact object per line.** Read it as text
before editing. Piping it through a pretty-printer to find the insertion point produces a shape that
does not exist in the file, so the edit fails to match.

**A header entry has two optional fields beyond `name`, `url` and `children`.** `"group": true` on a
CHILD turns it into a subheading inside the dropdown, and it stays a real link because the section
index it points at is where someone clicking a heading wants to land. `"columns": 2` lays that
dropdown out in columns, with `break-before: column` starting each group in its own, so the break
means one column per group rather than falling at the halfway point. `Guides` and `Reference` use
both. Nesting is ONE level: the template renders `children` and nothing deeper.

**Grouping is presentation, so it lives in the presentation layer.** Folding two sections under one
heading moved no content: `content/guide/` and `content/build/` stay where they are, so every
published URL and cross-link still resolves. Renaming a content directory to match a menu label would
break both for a change whose whole purpose is that the header reads better.

**`static/js/navdropdown.js` positions an open dropdown, and CSS alone cannot.** It measures and
clamps into the viewport, which no stylesheet can do because it cannot know whether a menu will fit.
Three things forced it: an 18-entry menu measured 900px against an 863px viewport with nothing
scrolling, the rightmost menu ran past the right edge, and below 768px `.main-nav` is
`overflow-x: auto`, which CLIPS an absolutely positioned child. Positioning `fixed` fixes all three,
since a fixed box cannot be clipped by an ancestor. The script stands down below 768px, where the
menu is a stacked list with its submenus inline and CSS owns it.

**Measure a dropdown only while it is SHOWN.** The first version read `offsetWidth` on `pointerenter`,
while CSS still had it `display: none`. A hidden element measures zero, so the clamp positioned the
menu as though it took no space, which is indistinguishable from having no clamp at all.

**Below 768px the header and the sidebar both collapse behind a button**, wired by the same script.
The bar used to stay on screen and scroll sideways, hiding most sections behind a gesture nobody
discovers, and the sidebar used to render above the article in the one-column grid and push the page
down before a reader saw a word.

**Two CSS blocks are order-dependent and say so inline.** The column overrides and the toggle base
rules each tie on specificity with the rules they override, so SOURCE ORDER decides. Both were
written in the wrong place first, and the second left every toggle invisible while the menus were
hidden, which is a header with no way to open it. Nothing in the gate catches this, so read the notes
above those blocks in `static/css/main.css` before moving them.

**`docsite/_hidden/` hides pages from the SITE BUILD, not from the repo.** Files under it stay
tracked and world-readable. A parked section sat there for months in exactly that state. Moving
something to `_hidden/` is a publishing decision and never a confidentiality one. Anything genuinely
sensitive has to leave the repo and its history.

**Search is a post-build index over `dist/`, not over `content/`.** `make build` ends by running
`pagefind --site dist`, which writes `dist/pagefind/`. Indexing the BUILT site is deliberate:
`_hidden/` pages never reach `dist`, so they stay out of the index without a second exclusion list
to keep in sync. The step lives in `build` rather than in `gh-pages` so a local build and the deploy
produce the same tree.

Three things about it are load-bearing. **`Content.html` carries `data-pagefind-body`** on the
article: Pagefind drops `<nav>` by itself but NOT `<header>`, so without it every excerpt opened
with the logo and the GitHub link. **The search overlay in `BasePage.html` carries
`data-pagefind-ignore`**, because Pagefind indexes hidden text, so the overlay's own copy would
otherwise be indexed on all 170 pages and "index" or "build" would match the whole site. And **the
Pagefind bundle is fetched on first open, not on load**, so a visit that never searches pays nothing
for the ~135KB of JS and CSS.

`make run` does not build an index, since it serves from `content/` with no `dist`. The overlay
detects the missing bundle and says so rather than hanging. Use `make build` to exercise search
locally, and note it must be served under the `/agni/` prefix for the bundle path to resolve.

## Content is templated BEFORE it is markdown, so a stray `{{` blanks the page

Every content file is run through `text/template` first. That is what makes `{{ agniRun "..." }}`,
`{{ includeCard "..." }}` and `{{.Site.PathPrefix}}` work, and it applies to the whole file: a code
fence does not protect you, because there is no markdown yet when the templater runs.

The failure is silent and total. An unparseable action does not break the offending line, it fails
the whole page load, and the page still builds and still exists at its URL with nothing in it but the
title and the footer. `build/check-rule.md` shipped that way for months: two Go samples contained
`[]check.ContextSubject{{`, an elided composite-literal brace, which the templater read as an action
calling a function named `Kind`. Twelve sections of the repo's rule-authoring guide were absent from
the site and nothing reported it.

So: **write a Go composite literal with the element brace on its own line**, and treat any `{{` in a
sample (Rust macros, Vue, Handlebars, Go templates as subject matter) as needing the same care.

**A TRANSCLUDED file does not get that pass, which is the mirror trap.** `includeCard` is called
DURING a host page's template execution and its return value is spliced in afterwards, so nothing
re-scans it. A generated card writing `{{.Site.PathPrefix}}` in an image path renders correctly on
its own page and reached the browser verbatim when transcluded, which requested a literal
`%7B%7B.Site.PathPrefix%7D%7D` and 404'd on four images across two guide pages. `IncludeCard`
substitutes the directive now, and `includecard_test.go` asserts on what it RETURNS plus follows the
substituted path to disk, because a wrong prefix would pass a braces-only check. Anything else
transcluded needs the same treatment.

`TestEveryContentPageTemplateParses` in `docsite/template_test.go` parses every content page with the
site's own `CommonFuncMap`, so a legitimate `{{ agniRun ... }}` passes and only an unknown function or
a malformed action fails. It runs in `make docsite-test`, which is in the gate. If you add a template
function, add it to `Site.CommonFuncMap` and the guard picks it up for free.
