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
