# docsite

This file covers building, testing and previewing the docsite, its figures, and its generated images.

## Building and testing

- **The docsite is its own Go module.** `go test ./docsite/` from the root fails with `[setup failed]`, so run `make docsite-test`.
- `AGNI_DOCS_ENV=production go run . -build` inside `docsite/` builds the pages alone.
- If `make -C docsite build` fails in `components` with `@esbuild/darwin-arm64 could not be found`, the cause is that clone's `docsite/node_modules`, not the change.
- `site/` at the repo root is stale build output, not a source tree. Older notes name a retired `docs/NN-*.md` mkdocs tree. Its pages now live in `docsite/content/` under audience-first names.

## Figures

- **Never paste an `<svg>` into a markdown page.** A hand-authored diagram lives in `docsite/figures/`, and the page carries `{{ includeFile "figures/<name>.svg" }}`.
- `includeFile` inlines the figure at BUILD time. The figure then resolves `currentColor` and `--accent-color` against the page's theme. An `<img src>` renders in its own document and inherits nothing.
- `includefile_test.go` fails the gate on a path that does not resolve, since `IncludeFile` returns an empty string and the build still succeeds. It also fails on a figure that no page includes, on a colour literal, and on a BLANK LINE inside the file.
- A blank line in a figure ends the raw-HTML block, and everything after it drops out of the `<svg>`.
- `docsite/README.md` has the house style, the traps, and the browser sweep that checks a rendered figure's geometry.

## Previewing and deploying

- **Serve `dist` under the `/agni/` prefix** when you check anything there in a browser. At a server's root every asset 404s. The page still returns 200 with no CSS, and the nav measures as its mobile layout at any viewport width.
- `make -C docsite preview PAGE=learn/03-why-every-chip-needs-capacitors` folds one built page into a self-contained HTML file for reviewing a branch.
- **`make -C docsite gh-pages` is DEAD.** Pages serves the `docs.yml` workflow artifact (`build_type: workflow`), so force-pushing that branch changes nothing.
- Pages deploys run one at a time. A run stuck at `waiting` on the `github-pages` environment has no reviewer to approve it, and every later deploy waits at `pending`. Cancel the stuck run with `gh run cancel <id>`, since a later deploy carries its commit.
- **A shared link carries a card** (agni 1205). Every page sets `og:*` and `twitter:card` from `SiteMetadata.json`'s `siteURL` and `socialImage`.
- A card image needs an absolute URL, and the site does not know its host. So the demo landing page takes `agni site --social-image <url>`. `make docs-site` passes the docsite's `static/images/social-preview.png`, drawn from the synthetic showcase fixture.

## Generated images and captures

- `make -C docsite images` re-renders every engine-drawn image the docs embed. `docsite-images-check` in `testall` fails when one is stale (agni 453).
- Never use `--layout force` in `images.sh`. It alone differs between arm64 and amd64 (agni 472).
- `tutorial-runs-check` regenerates every capture and fails on any difference.
- A capture's stamp hashes the spec and the fixture but NOT the engine. Regenerating is the only way to see engine drift.
- **Editing any tracked file inside a fixture restamps every capture over it.** `examples/tutorial-project` has 61 such captures. Commit the fixture edit, then run `make tutorial-runs`, then check that the diff holds stamp lines alone.
- A fixture under `tools/samples/` has nothing tracked, so its captures hash `hack/samples.pin` instead (agni 682).
