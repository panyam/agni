.PHONY: run build components node-deps gh-pages clean

# Dev server with live reload on http://localhost:8085/agni/. Builds the
# playground bundles first so a page that opts into one (front-matter
# `playground: viewer`) has its gen include present.
run: components
	AGNI_DOCS_ENV=dev AGNI_DOCS_PORT=:8085 go run .

# Install the node deps the playground build needs (esbuild). Idempotent.
node-deps:
	@[ -d node_modules/esbuild ] || pnpm install

# Build the interactive docs components (esbuild) into static/js/gen and emit
# the templates/gen.*.html includes BasePage.html pulls in.
components: node-deps
	node build.mjs

# One-shot static build into ./dist (no server).
build: components
	AGNI_DOCS_ENV=production go run . -build

clean:
	rm -rf dist static/js/gen

# Deploy ./dist to the gh-pages branch of panyam/agni.
gh-pages: build
	rm -rf dist/static
	cp -r static dist/static
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
