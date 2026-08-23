.PHONY: run build components node-deps search-index preview gh-pages clean

# Dev server with live reload on http://localhost:8085/agni/. Builds the
# playground bundles first so a page that opts into one (front-matter
# `playground: viewer`) has its gen include present.
run: components
	AGNI_DOCS_ENV=dev AGNI_DOCS_PORT=:8085 go run .

# Install the node deps the playground build and the search index need (esbuild,
# pagefind). Idempotent. Both are checked, because a clone that predates pagefind
# has node_modules/esbuild already and would otherwise skip the install.
node-deps:
	@[ -d node_modules/esbuild ] && [ -d node_modules/pagefind ] || pnpm install

# Build the interactive docs components (esbuild) into static/js/gen and emit
# the templates/gen.*.html includes BasePage.html pulls in.
components: node-deps
	node build.mjs

# One-shot static build into ./dist (no server). Copies static/ in, because the generator emits
# only pages: without this dist has no CSS, no JS bundle, no images and no rendered designs, so a
# built site silently loses every stylesheet and every interactive component. That was invisible
# for as long as the only consumer was `gh-pages`, which copied static/ itself on the way out.
build: components
	AGNI_DOCS_ENV=production go run . -build
	rm -rf dist/static
	cp -r static dist/static
	$(MAKE) search-index

# Build the client-side search index over the BUILT site, into dist/pagefind.
# Indexing dist rather than content/ is deliberate: _hidden/ pages never reach dist, so they
# are excluded from the index without a second exclusion list to keep in sync. Pagefind drops
# <nav>, <header> and <footer> on its own, so the index holds page prose and not the sidebar
# repeated 170 times.
#
# This runs as the last step of `build` and NOT in `gh-pages`, so a local `make build` and the
# deploy produce the same tree. gh-pages just pushes dist.
search-index: node-deps
	pnpm exec pagefind --site dist

clean:
	rm -rf dist static/js/gen

# Fold ONE built page into a self-contained HTML file for reviewing a branch before it merges.
#   make -C docsite preview PAGE=learn/05-who-drives-this-net
# Use this rather than `gh-pages` below, which cannot preview anything: Pages is configured
# build_type "workflow", so the live site is the artifact docs.yml uploaded on the last push to main
# and the gh-pages branch has not been served since the MkDocs tree was retired.
preview: build
	@./preview.sh $(PAGE) $(or $(OUT),preview.html)

# Deploy ./dist to the gh-pages branch of panyam/agni.
#
# DEAD as configured. GitHub Pages for this repo serves the docs.yml workflow artifact
# (build_type "workflow"), not this branch, so a force-push here changes nothing on the live site.
# Its own closing echo has always named the missing step and nobody ever took it. Kept because
# switching Pages back to branch-serving is a live option (it would enable per-branch previews under
# a path prefix), and deleting the target would hide that. Do not run it expecting a deploy.
gh-pages: build
	@if [ -d dist/.git ]; then echo "Error: dist is a git repo; remove it first." && exit 1; fi
	cd dist && \
		touch .nojekyll && \
		git init -q && \
		git add -A && \
		git commit -qm "Deploy docs to GitHub Pages" && \
		git branch -M gh-pages && \
		git remote add origin git@panyam-github:panyam/agni.git && \
		git push -f origin gh-pages
	@rm -rf dist/.git
	@echo "Deployed. Set GitHub Pages to serve from the gh-pages branch."

# The board images the tutorial pages embed in <agni-viewer> are rendered by the engine from the
# committed tutorial fixture, so a rendering change or a fixture edit can be picked up by re-running
# this rather than by noticing the picture went stale. Run from the repo root, where the engine
# module lives (docsite is its own module and does not depend on it).
TUT = examples/tutorial-project/designs/gateway
designs: ## Re-render the tutorial board images the docs embed
	@cd .. && go run ./cmd/agni render $(TUT)/gateway.kicad_sch --symbol-path $(TUT)/symbols \
	  -o docsite/static/designs/gateway-schematic.svg
	@cd .. && go run ./cmd/agni render $(TUT)/gateway.edn --layout force \
	  -o docsite/static/designs/gateway-netlist.svg
