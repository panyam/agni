.PHONY: run build components gh-pages clean

# Dev server with live reload on http://localhost:8085/agni/
run:
	AGNI_DOCS_ENV=dev AGNI_DOCS_PORT=:8085 go run .

# Build the interactive docs components (esbuild). Optional: the site renders
# without it, and it is only needed once a page opts into a playground.
components:
	node build.mjs

# One-shot static build into ./dist (no server).
build:
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
