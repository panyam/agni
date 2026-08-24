GO ?= go

.PHONY: all proto proto-web proto-check tidy tidyall build agni install vet ir-model-check test web-test browser-test web-install testall examples-test docsite-test catalog-docs catalog-docs-check tutorial-runs tutorial-runs-check serve demo ghserve ghbuild ui natimage natup natdown natlogs natrender natopen image dockserve dockstop tag tag-push tutorial-runs setup pdf2doc pdf2doc-all datasheets-status

all: proto build

# Regenerate Go from the proto IR (run from protos/ where buf config lives).
proto:
	cd protos && buf generate

# Regenerate the TypeScript half. Separate command, separate config, and the half people forget:
# additive proto changes leave stale TS building green, so the drift only surfaces on the next
# regen. proto-check below is what makes forgetting it fail instead.
proto-web:
	cd web && pnpm run gen

# Freshness gate: fail when the committed generated code does not match the protos it came from.
#
# WHY THIS IS NOT GIT-STATUS-BASED like catalog-docs-check. That target regenerates IN PLACE and
# inspects `git status`, which forces a regenerate -> COMMIT -> gate ordering: freshly regenerated
# but uncommitted output reads as stale. Proto edits are far more frequent than catalog edits and
# the natural workflow is to regenerate and run the gate before committing, so inheriting that
# ordering would make the gate a tax. Generating into a throwaway tree and diffing has no such
# constraint, and it cannot leave the working tree dirty on failure.
#
# THE TEMP TREE MIRRORS THE REPO LAYOUT ON PURPOSE. Both buf templates write to RELATIVE paths that
# climb out of their config directory (`out: ../gen/go` from protos/, `out: src/gen` from web/), and
# `-o` resolves those relative to whatever it is given. Pointing `-o` straight at a bare temp dir
# makes `../gen/go` escape it and land beside the temp dir instead. So `-o $$tmp/protos` and
# `-o $$tmp/web` reproduce the two config directories' positions and the outputs land inside.
# Both templates also set `clean: true`, which is another reason never to aim this at the real tree.
proto-check:
	@tmp=$$(mktemp -d); \
	trap 'rm -rf "$$tmp"' EXIT; \
	mkdir -p "$$tmp/protos" "$$tmp/web"; \
	(cd protos && buf generate -o "$$tmp/protos") || exit 1; \
	(cd web && buf generate ../protos --template buf.gen.web.yaml -o "$$tmp/web") || exit 1; \
	fail=0; \
	if ! diff -r gen/go "$$tmp/gen/go" >/dev/null 2>&1; then \
		echo "generated Go is stale — run 'make proto' and commit the result:"; \
		diff -rq gen/go "$$tmp/gen/go" 2>&1 | sed 's/^/  /'; \
		fail=1; \
	fi; \
	if ! diff -r web/src/gen "$$tmp/web/src/gen" >/dev/null 2>&1; then \
		echo "generated TypeScript is stale — run 'make proto-web' and commit the result:"; \
		diff -rq web/src/gen "$$tmp/web/src/gen" 2>&1 | sed 's/^/  /'; \
		fail=1; \
	fi; \
	exit $$fail

# Fetch and prune module dependencies (engine module only).
tidy:
	$(GO) mod tidy

# Tidy every module: the engine plus each example module (they have their own go.mod). Run
# after changing imports anywhere the examples consume. EXAMPLE_MODS is defined below.
tidyall:
	$(GO) mod tidy
	@for d in $(EXAMPLE_MODS); do \
		echo "== tidy $$d =="; \
		( cd $$d && $(GO) mod tidy ) || exit 1; \
	done
	@echo "tidy: all modules tidied"

# Build all packages.
build: ui
	$(GO) build -o bin ./...

# Build the agni CLI into bin/.
agni:
	$(GO) build -o bin/agni ./cmd/agni

# Install the agni CLI into GOBIN (falls back to GOPATH/bin).
install:
	$(GO) install ./cmd/agni

# Static analysis over the engine module (the examples-test loop builds the example modules).
vet:
	$(GO) vet ./...

# CONSTRAINT C19 ratchet (WS1-042): fail on a NEW `func ... *ir.Design` outside the allowed
# producer/entry paths — engine processing reads through check.Model. Run --dump to rebaseline.
ir-model-check:
	./hack/ir_model_check.sh

# Engine (Go) tests. The example modules have their own go.mod; see examples-test.
test:
	$(GO) test ./...

# Web unit tests: TypeScript typecheck + the vitest suite. No browser, no server.
web-test:
	cd web && pnpm run typecheck && pnpm test

# Browser tests (agni issue 323): the handful of assertions that need real layout, run against a
# real Chromium driving a real server. NOT part of testall, deliberately.
#
# jsdom has no layout engine, so the unit suite can prove what a panel renders and nothing about
# what a reader can see; a CSS bug once shipped through a fully green run. This closes that, and
# pays for it in speed and in needing a browser on the machine. Keeping it out of the gate means a
# machine without one never turns CI red for a reason unrelated to the change under test.
#
# Needs a browser once per machine:  cd web && pnpm exec playwright-core install chromium
# The suite starts and stops its own agni server on a port the kernel picks, so it does not collide
# with a dev server you already have running.
browser-test: ui
	cd web && pnpm run test:browser

# Regenerate the docsite rule + relation catalog (issue 14) from the shipped engine catalog and
# the embedded per-rule/per-relation Detail markdown. The stdlib docs stay the source of truth;
# this projects them into docsite/content/reference/{rules,relations}/ and the SVG cards under
# docsite/static/images/catalog/. Commit the result.
catalog-docs:
	$(GO) run ./tools/catalogdocs

# Freshness gate: fail when the committed catalog drifts from a fresh run, so a rule or relation
# whose doc changed cannot silently desync the site (the make-roadmap / roadmap-check pattern).
catalog-docs-check: catalog-docs
	@if [ -n "$$(git status --porcelain -- docsite/content/reference/rules docsite/content/reference/relations docsite/static/images/catalog)" ]; then \
		echo "catalog docs are stale — run 'make catalog-docs' and commit the result:"; \
		git status --short -- docsite/content/reference/rules docsite/content/reference/relations docsite/static/images/catalog; \
		exit 1; \
	fi

# The full deterministic gate: vet, generated-code freshness, the browser bundle build (which
# enforces the single-Solid-core invariant, see web/build.mjs), engine (Go) tests, example modules,
# web unit tests, and the docsite catalog freshness check. Green = ship-ready. CI runs exactly this
# (.github/workflows/ci.yml). The bundle build comes before the engine tests: TestCheckWebAssets
# (cmd/agni) asserts web/static/app.js exists, and the bundle is a gitignored build artifact.
# proto-check sits near the front because stale generated code makes every later failure a red
# herring: it compiles and tests green while describing a different schema.
testall: vet ir-model-check proto-check ui test examples-test web-test catalog-docs-check docsite-test tutorial-runs-check

# Web viewer dev server. Builds the browser bundle, then serves it plus the Connect API with
# the in-repo fixture folders mounted (browse them in the left sidebar). Append your own
# corpus with EXTRA_MOUNTS, e.g.
#   make serve EXTRA_MOUNTS="--mount corpus=/path/to/designs --mount boards=$$HOME/boards"
# Override ADDR to change the port, or MOUNTS to replace the fixture set entirely.
ADDR ?= :8080
MOUNTS ?= --mount edif=readers/edif/testdata --mount kicad=readers/kicad/testdata --mount ipc=readers/ipc2581/testdata
EXTRA_MOUNTS ?=
# NATIVE_TOOLS enables native golden renderers by tool name (space-separated), e.g.
#   make serve NATIVE_TOOLS=kicad-cli
NATIVE_TOOLS ?=
NATIVE_FLAGS := $(foreach t,$(NATIVE_TOOLS),--enable-native $(t))
# PDF2DOC configures the doc-IR producer the /datasheets "Extract (first pass)" action shells out
# to (invoked as "<PDF2DOC> <pdf> -o <sibling>"). Empty disables extraction, which is what you get
# until `make setup` has built the docling venv: the default below turns the button on exactly when
# there is an interpreter that can serve it, rather than wiring up a command that fails on click.
# Absolute, because the value outlives this make and is run by the server process. Override with a
# command of your own, or with PDF2DOC= to leave the action off.
#   make serve PDF2DOC="python3 tools/pdf2doc/pdf2doc.py"
# PDF2DOC_PY and the rest of the datasheet tooling are defined in their own section further down.
PDF2DOC ?= $(if $(wildcard $(PDF2DOC_PY)),$(abspath $(PDF2DOC_PY)) $(abspath tools/pdf2doc/pdf2doc.py))
# Recursive, not `:=`, because PDF2DOC above reads a variable defined later in this file.
PDF2DOC_FLAG = $(if $(strip $(PDF2DOC)),--pdf2doc '$(PDF2DOC)')
# SYMBOL_PATH points --symbol-path at an xschem/gEDA symbol library dir (repeatable flag,
# space-separated dirs here) for pin-level nets and faithful symbol artwork on .sch files;
# empty means components + net names + placeholder boxes (see docs/GETTING_STARTED.md).
SYMBOL_PATH ?=
SYMBOL_FLAGS := $(foreach p,$(SYMBOL_PATH),--symbol-path $(p))
# OVERLAY_FLAGS carries the catalog overlay a deployment serves with: --profile-path,
# --intent-path, --conventions. Separate from EXTRA_MOUNTS because these are not mounts but CATALOG
# inputs, and since WS3-109 they compose into every rule-running surface the server exposes. E.g.
#   make serve OVERLAY_FLAGS="--profile-path /path/to/profiles --conventions /path/to/conventions.yaml"
# An overlay is per-DEPLOYMENT config: a profile named after a built-in supersedes it for every
# design this server reads, so point it at an overlay that suits the whole mounted set.
OVERLAY_FLAGS ?=
# REVIEW_STORE is the WRITABLE directory stored review runs live in (--review-store), created if
# absent. It is what the viewer's Review panel reads: without it the review resource methods answer
# "no review store configured" and the panel can show nothing, on any design. Deliberately separate
# from the read-only design mounts. Set it empty to serve without a store:
#   make serve REVIEW_STORE=
#
# The default is per-USER rather than per-checkout, and outside the repo on purpose. A stored run is
# about a DESIGN, not about which clone was serving when you saved it, so several checkouts sharing
# one store is the behaviour you want, and runs saved from a lane do not become untracked noise in
# it. The CLI flag itself stays explicit, with no default of its own: a deployed server should say
# where it writes rather than inherit a guess. This is the dev-convenience layer, where the cost of
# no default is a Review panel that is empty on every design until someone finds out why.
REVIEW_STORE ?= $(HOME)/.agni/reviews
REVIEW_FLAGS := $(if $(strip $(REVIEW_STORE)),--review-store $(REVIEW_STORE))
serve: ui
	$(GO) run ./cmd/agni serve --addr $(ADDR) $(MOUNTS) $(EXTRA_MOUNTS) $(NATIVE_FLAGS) $(PDF2DOC_FLAG) $(SYMBOL_FLAGS) $(OVERLAY_FLAGS) $(REVIEW_FLAGS)

# One-command self-contained demo. Builds the web bundle and serves the viewer with only the
# shareable demo/ boards mounted (no private data). Open the printed URL, pick a board in the
# left tree, and explore the render, checks, and query panels. See demo/README.md.
demo: ui
	@echo "Agni demo: open http://localhost$(ADDR) and load showcase.fires.kicad_pro (or .passes)"
	$(GO) run ./cmd/agni serve --addr $(ADDR) --mount demo=demo

# Documentation site. The live site is the s3gen app in docsite/, which owns its own targets
# (make -C docsite run|build|gh-pages) and deploys via the docs.yml GitHub Actions workflow on
# any push to main touching docsite/**. ghserve/ghbuild are thin aliases to the docsite targets
# for muscle memory; there is no local publish target here on purpose (the workflow is the
# canonical deploy). Regenerate the rule/relation catalog with catalog-docs first if the engine
# catalog changed.
ghserve:
	$(MAKE) -C docsite run

ghbuild:
	$(MAKE) -C docsite build

# Install the web viewer's node dependencies. Run once before the first build (or after
# dependency changes); ui and web-test assume it has run.
web-install:
	cd web && pnpm install

# Build the browser bundle (esbuild + Solid via web/build.mjs) into web/static/. Run
# web-install once first (or after dependency changes).
ui:
	cd web && pnpm build

# Native-tools container (Dockerfile.nattools): a Linux/X11 tool host with kicad-cli, xschem,
# Lepton, and agni, reached over SSH. The agni SERVER runs on the host; this is only the tools.
# natup runs sshd detached and authorizes PUBKEY; natrender/natopen below drive the tools over that
# connection. Pass your design folders in with NATIVE_DOCKER_MOUNTS (docker -v), e.g.
#   make natup NATIVE_DOCKER_MOUNTS="-v $$HOME/boards:/boards"
NATIVE_IMAGE ?= agni-native
NATIVE_NAME ?= agni-nattools
SSH_PORT ?= 2222
NATIVE_DOCKER_MOUNTS ?=
PUBKEY ?= $(HOME)/.ssh/id_ed25519.pub

natimage:
	docker build -f Dockerfile.nattools -t $(NATIVE_IMAGE) .

natup: natimage
	-docker rm -f $(NATIVE_NAME) 2>/dev/null
	docker run -d --name $(NATIVE_NAME) -p $(SSH_PORT):22 \
		-v "$(PUBKEY):/home/agni/.ssh/authorized_keys:ro" $(NATIVE_DOCKER_MOUNTS) $(NATIVE_IMAGE)
	@echo "nattools up: ssh -p $(SSH_PORT) agni@localhost   (X11: ssh -X ...)"

natdown:
	-docker rm -f $(NATIVE_NAME)

natlogs:
	docker logs -f $(NATIVE_NAME)

# The ssh invocation the two file-driven targets below share. LC_ALL is forced to a locale the slim
# image actually has: Lepton's guile aborts on a forwarded LANG it cannot find.
SSH := ssh -p $(SSH_PORT) -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o SetEnv=LC_ALL=C.UTF-8

# Native render inside the running container, written back to the bind mount. FILE and OUT are
# paths INSIDE the container, so both must fall under a directory natup mounted.
#   make natrender FILE=/boards/amp/amp.sch OUT=/boards/amp.svg
natrender:
	@[ -n "$(FILE)" ] && [ -n "$(OUT)" ] || { echo "usage: make natrender FILE=<path in container> OUT=<path in container>"; exit 2; }
	$(SSH) agni@localhost agni native render $(FILE) -o $(OUT)

# Open a design's native GUI in the container; the window appears via XQuartz, so install and start
# that first. FILE is a path inside the container. Blocks until you close the window.
#   make natopen FILE=/boards/amp/amp.sch
natopen:
	@[ -n "$(FILE)" ] || { echo "usage: make natopen FILE=<path in container>"; exit 2; }
	$(SSH) -X agni@localhost agni native open $(FILE)

# The examples under examples/ are each their own Go module (their own go.mod, to keep demokit
# and its terminal-UI deps out of the engine go.mod), so `test` above does not reach them.
# Build and test each example module explicitly. Run this after changing anything the examples
# consume (the public reader/diff/check/render APIs or examples/common).
# The docsite is its own Go module, so the engine's `test` target never reaches it. Its tests are
# the nav-wiring invariants: adding a section takes five coordinated edits across four files and
# nothing else checks them.
docsite-test:
	@cd docsite && go test ./...

EXAMPLE_MODS := $(dir $(wildcard examples/*/go.mod))
examples-test:
	@for d in $(EXAMPLE_MODS); do \
		echo "== $$d =="; \
		( cd $$d && $(GO) build ./... && $(GO) test ./... ) || exit 1; \
	done
	@echo "examples: all modules build + test OK"

# =============================================================================
# Container image
# =============================================================================

# The packaged server (Dockerfile): engine + web viewer + the KiCad/xschem/gEDA symbol
# libraries. Designs come in as bind mounts under /workspace, one -v per folder, no flags.
#   make image                      # build ghcr.io/panyam/agni:dev
#   make image IMAGE_TAG=v0.1.0     # build the release tag
#   make dockserve DESIGNS=~/boards # serve those designs on :8080 from the image
IMAGE_NAME ?= ghcr.io/panyam/agni
IMAGE_TAG ?= dev
IMAGE := $(IMAGE_NAME):$(IMAGE_TAG)

# VERSION reaches the build as --build-arg because the image has no .git to derive it from; see
# the ARG in the Dockerfile. It defaults to IMAGE_TAG so `make image IMAGE_TAG=v0.1.0` produces a
# binary that reports v0.1.0, keeping the image tag and the build's own claim about itself in step.
image:
	docker build --build-arg VERSION=$(IMAGE_TAG) -t $(IMAGE) .

# dockserve is serve's container twin: same MOUNTS / EXTRA_MOUNTS / DESIGNS / ADDR, run from the
# image instead of `go run`.
#
#   make dockserve                                     # the fixture mounts, from the image
#   make dockserve DESIGNS=~/boards                    # plus a folder of your own
#   make dockserve EXTRA_MOUNTS="--mount corpus=/data" # the same flag serve takes
#
# The work is in tools/dockserve.sh, whose header documents the whole contract: which of serve's
# parameters cannot cross into a container, and why refusing one the caller typed but dropping one
# the environment supplied is the same rule rather than two.
DESIGNS ?=
# The overlay catalog (profiles/, conventions.yaml) as a single host DIRECTORY, mounted read-only.
# serve takes assembled flags; the container needs the folder, since it has to cross the boundary.
OVERLAY_DIR ?=
# Escape hatch for anything else the run needs: extra -v, --user, -e, a different --network.
DOCKER_FLAGS ?=
DOCKER_NAME ?= agni-dockserve

# The names the CALLER typed on this command line, out of the three the script may have to refuse.
# $(origin) is the only thing that can tell a typed argument from ambient config, and it exists only
# here, so the answer is computed in make and passed down.
DOCKSERVE_CLI_SET = $(foreach v,NATIVE_TOOLS PDF2DOC OVERLAY_FLAGS,$(if $(filter command line,$(origin $(v))),$(v)))

dockserve:
	@MAKE='$(MAKE)' IMAGE='$(IMAGE)' ADDR='$(ADDR)' \
	  MOUNTS='$(MOUNTS)' EXTRA_MOUNTS='$(EXTRA_MOUNTS)' DESIGNS='$(strip $(DESIGNS))' \
	  OVERLAY_DIR='$(strip $(OVERLAY_DIR))' OVERLAY_FLAGS='$(strip $(OVERLAY_FLAGS))' \
	  REVIEW_STORE='$(strip $(REVIEW_STORE))' NATIVE_TOOLS='$(strip $(NATIVE_TOOLS))' \
	  PDF2DOC='$(strip $(PDF2DOC))' SYMBOL_PATH='$(strip $(SYMBOL_PATH))' \
	  DOCKER_FLAGS='$(DOCKER_FLAGS)' DOCKER_NAME='$(DOCKER_NAME)' \
	  CLI_SET='$(DOCKSERVE_CLI_SET)' \
	  tools/dockserve.sh

# Stop a detached dockserve (one started with DOCKER_FLAGS=-d). A foreground one ends on Ctrl-C.
dockstop:
	-docker rm -f $(DOCKER_NAME)

# =============================================================================
# Release
# =============================================================================
#
# For the CLI, a release is a git tag and nothing else. Go modules resolve versions from tags, so
# `go install github.com/panyam/agni/cmd/agni@v0.1.0` works the moment the tag is pushed, with no
# build artifacts to upload and no separate release pipeline to keep green.
#
# The container image follows automatically. Pushing the tag triggers .github/workflows/release.yml,
# which builds the image FROM that tag, stamps the same version into the binary, pushes it to GHCR,
# and then pulls it back to confirm it reports the tag it is labelled with. So the whole release is:
#
#   make testall                    # the gate; CI runs exactly this
#   make tag-push V=v0.1.0          # the Go release AND, via the workflow, the image
#
# The two versions are no longer kept in step by hand. They were, briefly, and the hazard was that
# `make image IMAGE_TAG=` taking a different value than `make tag-push V=` would ship an image
# labelled one version whose binary reported another, which is the confusion the version stamp
# exists to remove. The tag is now the only input.
#
# `make image` below still exists for building locally without publishing.

# Sub-modules that get tagged alongside the root module. Every IMPORTABLE sub-module (one with
# its own go.mod that a downstream user would `go get`) needs its own tag here, because a
# `replace` directive in it is ignored once someone imports it rather than building it.
#
# Empty on purpose today. The other go.mod files in this tree are examples/*/ (the demokit
# walkthroughs, kept out of the engine go.mod so its dependency set stays lean, per
# examples/CONVENTIONS.md) and docsite/ (the s3gen site). Neither is meant to be imported. Add a
# path here the moment one is, or `go get` against it resolves to a pseudo-version instead of the
# release.
SUB_MODS_TO_TAG :=

# Every ref a release creates: the root tag plus one per importable sub-module. Computed rather
# than repeated so tag and tag-push cannot disagree about what a release consists of, and so an
# empty SUB_MODS_TO_TAG yields exactly one ref rather than a stray "/$(V)".
TAG_REFS = $(V) $(foreach m,$(SUB_MODS_TO_TAG),$(m)/$(V))

# Shared preconditions. V must be present and must be v-prefixed semver, which is not style
# preference: the Go module proxy will not serve a tag in any other shape, and a mistagged
# release is only fixable by deleting a published tag. The already-exists check catches a re-run
# before it half-creates a set of refs.
define check_version
	if [ -z "$(V)" ]; then echo "Usage: make $@ V=v0.1.0"; exit 1; fi; \
	case "$(V)" in \
		v[0-9]*.[0-9]*.[0-9]*) ;; \
		*) echo "V must be v-prefixed semver, e.g. v0.1.0 (the Go module proxy will not serve other shapes)"; exit 1;; \
	esac; \
	if git rev-parse -q --verify "refs/tags/$(V)" >/dev/null; then \
		echo "tag $(V) already exists locally; pick the next version or delete it first"; exit 1; \
	fi
endef

# Create the release tags locally without pushing, so you can inspect them first.
#   make tag V=v0.1.0
tag:
	@$(check_version)
	@echo "Tagging $(V) at $$(git rev-parse --short HEAD) on $$(git branch --show-current)..."
	@for ref in $(TAG_REFS); do \
		echo "  $$ref"; \
		git tag -a $$ref -m "$$ref" || exit 1; \
	done
	@echo ""
	@echo "Tags created locally. Push with:"
	@echo "  git push origin $(TAG_REFS)"
	@echo "or re-run as: make tag-push V=$(V)"

# Tag and push in one step. This is the one that publishes: a pushed tag is immediately
# resolvable by the Go module proxy and is not safely retractable afterwards.
#   make tag-push V=v0.1.0
tag-push:
	@$(MAKE) tag V=$(V)
	git push origin $(TAG_REFS)

# Force-regenerate every tutorial command capture, ignoring the input stamps, and report what moved.
# Commit whatever it changes, after reading it.
#
# The FORCING is the whole point, and it is why tutorial-runs-check below has to run this rather than
# an ordinary docsite build. A capture's stamp hashes its spec and its fixture, NOT the engine, so an
# engine change that alters output leaves every stamp valid and the cached build regenerates nothing.
# Measured: after changing the coverage line's wording, a plain `go run . -build` rewrote 0 captures
# and this target rewrote 12.
#
# Every runs/ directory under content/, not just the tutorials' one. A second section with generated
# captures (learn/) went stale silently the moment it existed, because this target named one path.
tutorial-runs:
	@find docsite/content -path '*/runs/*.output' -delete
	@cd docsite && $(GO) run . -build >/dev/null 2>&1
	@git status --short -- docsite/content || true
	@echo "tutorial captures regenerated; review the diff above before committing"

# Fail when a committed capture disagrees with what the engine produces today.
#
# This was left out of `testall` on purpose, to keep the docs pipeline off the per-push path and
# accept that a regression surfaced later rather than immediately. The measurement that reopened that
# trade is 15 seconds for a full regeneration, against a gate already minutes long.
#
# What went uncaught meanwhile is wider than engine drift. A capture edited BY HAND keeps its stamp
# and passes every existing check, because nothing compared a committed body against a fresh run.
#
# IT SNAPSHOTS AND RESTORES rather than inspecting `git status` like catalog-docs-check, for the
# reason proto-check spells out above: a git-status check forces a regenerate -> COMMIT -> gate
# ordering, so freshly regenerated but uncommitted output reads as stale. Captures move on any
# fixture or output change, which is often, and the natural loop is to regenerate and run the gate
# before committing. This leaves the tree exactly as it found it, pass or fail, and says which target
# to run. The snapshot goes through tar rather than `cp --parents`, which is GNU-only and is not
# on macOS.
tutorial-runs-check:
	./hack/tutorial_runs_check.sh

# =============================================================================
# Datasheet tooling
# =============================================================================
#
# tools/pdf2doc derives doc-IR from a datasheet PDF. It is prototype Python, never in CI, and needs
# docling, so it runs out of a venv `make setup` builds rather than the engine toolchain.
#
# The corpus it sweeps is a folder of parts laid out as <vendor>/<PART>/, each part dir holding its
# source PDF(s) and every generated or HITL artifact beside them (doc-IR, PartSpec). Vendor PDFs are
# licensed material and must not be committed here, so DATASHEET_DIR names a local, gitignored
# folder by default and takes an absolute path to one kept anywhere else.

# One-time (idempotent): build the venv, install docling, and PREFETCH its models so the first
# `make pdf2doc` (or the viewer's Extract button) does not stall on a model download, which is
# decoupled on purpose. Re-run to refresh docling or repair the env. torch has no wheels for python
# 3.14, so the venv is built with 3.12; override VENV_PY for another base.
#
# WHERE THE VENV LIVES is a lookup rather than a fixed path, because the docling dependency set runs
# to gigabytes and several worktrees of this repo should not each carry one. The search order is
# repo-local .venv first, then the PARENT directory's, which is the shared slot when clones sit side
# by side under one root (~/work/agni/{main,feature-x}/ all find ~/work/agni/.venv). Nothing found
# means `make setup` builds the repo-local one. Override VENV_DIR for an env somewhere else, or
# extend VENV_SEARCH to add a slot to the order.
VENV_PY ?= /opt/homebrew/bin/python3.12
VENV_SEARCH ?= .venv ../.venv
VENV_DIR ?= $(patsubst %/bin/python,%,$(firstword $(wildcard $(addsuffix /bin/python,$(VENV_SEARCH))) .venv/bin/python))
$(VENV_DIR)/bin/python:
	$(VENV_PY) -m venv $(VENV_DIR)
	$(VENV_DIR)/bin/python -m pip install --upgrade pip
setup: $(VENV_DIR)/bin/python
	$(VENV_DIR)/bin/python -m pip install --upgrade docling
	$(VENV_DIR)/bin/docling-tools models download
	@echo "setup: $(VENV_DIR) ready (docling + prefetched models)"

# The python pdf2doc runs under: the venv once setup has built one, else whatever python3 is on the
# PATH (which works only if docling is installed there).
PDF2DOC_PY ?= $(if $(wildcard $(VENV_DIR)/bin/python),$(VENV_DIR)/bin/python,python3)
DATASHEET_DIR ?= datasheets

# Derive doc-IR from one PDF and validate it against the contract. OUT is conventionally the PDF's
# sibling <stem>.doc.textproto, which is where the viewer's /datasheets workbench looks for it.
#   make pdf2doc PDF=datasheets/ti/LM1117/LM1117.pdf OUT=datasheets/ti/LM1117/LM1117.doc.textproto
pdf2doc:
	@[ -n "$(PDF)" ] && [ -n "$(OUT)" ] || { echo "usage: make pdf2doc PDF=<file.pdf> OUT=<file.doc.textproto>"; exit 2; }
	$(PDF2DOC_PY) tools/pdf2doc/pdf2doc.py $(PDF) -o $(OUT)
	$(GO) run ./tools/pdf2doc/validate $(OUT)

# Report-only extraction status: per part, whether each PDF has a fresh, stale, or absent doc-IR and
# whether a part-level PartSpec exists. Reads doc-IR content_hash + producer, and writes nothing.
#
# The --toolchain value is the producer string the INSTALLED docling would stamp now, which is what
# lets the walker flag a doc-IR that predates a toolchain bump. It is computed in the recipe rather
# than in a variable so that an unrelated `make test` never pays for a python startup, and it is
# best-effort: docling missing means the flag is omitted and only hash freshness gets reported.
datasheets-status:
	@v=$$($(PDF2DOC_PY) -c "import importlib.metadata as m; print(m.version('docling'))" 2>/dev/null); \
	$(GO) run ./tools/datasheetstatus $${v:+--toolchain docling/$$v} $(DATASHEET_DIR)

# Run pdf2doc on exactly the PDFs the walker flags as not-extracted or stale-source (fresh ones are
# skipped; a stale-toolchain refresh stays a deliberate `make pdf2doc PDF=... OUT=...`). Each PDF's
# doc-IR is written to its sibling <stem>.doc.textproto in the same part dir.
pdf2doc-all:
	@$(GO) run ./tools/datasheetstatus --list $(DATASHEET_DIR) | while read -r pdf; do \
		out="$${pdf%.pdf}.doc.textproto"; \
		echo "pdf2doc: $$pdf -> $$out"; \
		$(PDF2DOC_PY) tools/pdf2doc/pdf2doc.py "$$pdf" -o "$$out"; \
	done
