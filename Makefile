GO ?= go
# Design used by the stats/check convenience targets. A committed example fixture by
# default; point EDN at your own design to run against real data.
EDN ?= examples/common/designs/i2c-sensor.edn

.PHONY: all proto tidy tidyall build agni install stats check vet ir-model-check test web-test web-install testall examples-test docsite-test catalog-docs catalog-docs-check serve demo ghserve ghbuild ui natimage natup natdown natlogs image dockserve dockstop tag tag-push

all: proto build

# Regenerate Go from the proto IR (run from protos/ where buf config lives).
proto:
	cd protos && buf generate

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
	$(GO) build ./...

# Build the agni CLI into bin/.
agni:
	$(GO) build -o bin/agni ./cmd/agni

# Install the agni CLI into GOBIN (falls back to GOPATH/bin).
install:
	$(GO) install ./cmd/agni

# Convenience runs against the local EDIF netlist.
stats: agni
	./bin/agni stats $(EDN)

check: agni
	./bin/agni check $(EDN)

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

# The full deterministic gate: vet, the browser bundle build (which enforces the
# single-Solid-core invariant, see web/build.mjs), engine (Go) tests, example modules, web unit
# tests, and the docsite catalog freshness check. Green = ship-ready. CI runs exactly this
# (.github/workflows/ci.yml). The bundle build comes before the engine tests: TestCheckWebAssets
# (cmd/agni) asserts web/static/app.js exists, and the bundle is a gitignored build artifact.
testall: vet ir-model-check ui test examples-test web-test catalog-docs-check docsite-test

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
# to (invoked as "<PDF2DOC> <pdf> -o <sibling>"). Empty disables extraction; needs docling. E.g.
#   make serve PDF2DOC="python3 tools/pdf2doc/pdf2doc.py"
PDF2DOC ?=
PDF2DOC_FLAG := $(if $(strip $(PDF2DOC)),--pdf2doc '$(PDF2DOC)')
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
# from the read-only design mounts, and empty by default because a server that stores runs should
# say where, rather than inheriting a guess. E.g.
#   make serve REVIEW_STORE=/tmp/agni-reviews
REVIEW_STORE ?=
REVIEW_FLAGS := $(if $(strip $(REVIEW_STORE)),--review-store $(REVIEW_STORE))
serve: ui
	$(GO) run ./cmd/agni serve --addr $(ADDR) $(MOUNTS) $(EXTRA_MOUNTS) $(NATIVE_FLAGS) $(PDF2DOC_FLAG) $(SYMBOL_FLAGS) $(OVERLAY_FLAGS) $(REVIEW_FLAGS) web

# One-command self-contained demo. Builds the web bundle and serves the viewer with only the
# shareable demo/ boards mounted (no private data). Open the printed URL, pick a board in the
# left tree, and explore the render, checks, and query panels. See demo/README.md.
demo: ui
	@echo "Agni demo: open http://localhost$(ADDR) and load showcase.fires.kicad_pro (or .passes)"
	$(GO) run ./cmd/agni serve --addr $(ADDR) --mount demo=demo web

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
# natup runs sshd detached and authorizes PUBKEY; ssh in to run `agni native render` (writes to a
# bind-mounted dir) or `ssh -X` for `agni native open` (GUI to XQuartz). Pass design folders with
# NATIVE_DOCKER_MOUNTS (docker -v). A workspace Makefile can bind-mount your designs and add
# file-driven natrender/natopen wrappers.
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
# image instead of `go run`. Each "--mount NAME=PATH" becomes "-v PATH:/workspace/NAME", and the
# image's own CMD already passes --mount-root /workspace, so nothing needs a --mount flag inside.
#
#   make dockserve                                     # the fixture mounts, from the image
#   make dockserve DESIGNS=~/boards                    # plus a folder of your own
#   make dockserve EXTRA_MOUNTS="--mount corpus=/data" # the same flag serve takes
#
# THREE of serve's parameters are refused here rather than forwarded, because the image does not
# contain what they describe and the failure would otherwise be silent:
#
#   NATIVE_TOOLS  no kicad-cli/xschem/Lepton inside (see the Dockerfile header). Use serve, or
#                 reach the tools through the nattools container.
#   PDF2DOC       no Python/docling inside, and the value is a host path.
#   OVERLAY_FLAGS host paths that do not resolve in the container. Pass OVERLAY_DIR instead: the
#                 folder is mounted at /overlay and the flags are rebuilt against it.
#
# REVIEW_STORE crosses the same way OVERLAY_DIR does, mounted at /var/lib/agni/reviews, except
# WRITABLE: stored runs are the one thing this server produces rather than reads. It is created on
# the host first so the bind mount does not materialize as a root-owned directory.
#
# SYMBOL_PATH is IGNORED rather than refused, because the image ships better libraries than a host
# path would name (AGNI_SYMBOL_PATH, baked in stage 3). Refusing it would block a caller who simply
# has the variable set for serve. It is announced on startup so the substitution is never a guess.
#
# The reason these are hard errors and not warnings: --symbol-path or --profile-path pointing at a
# directory that does not exist inside the container yields a SHORT read, and a short read is the
# quiet kind of wrong. The rules evaluate cleanly over it and report fewer findings, with no error
# to explain them. See the Dockerfile header for the same argument about `go install`.
DESIGNS ?=
# The overlay catalog (profiles/, conventions.yaml) as a single host DIRECTORY, mounted read-only.
# serve takes assembled flags; the container needs the folder, since it has to cross the boundary.
OVERLAY_DIR ?=
# Escape hatch for anything else the run needs: extra -v, --user, -e, a different --network.
DOCKER_FLAGS ?=
DOCKER_NAME ?= agni-dockserve

dockserve:
	@if [ -n "$(strip $(NATIVE_TOOLS))" ]; then \
	  echo "dockserve: NATIVE_TOOLS is not available in the image (no kicad-cli/xschem/Lepton inside)." >&2; \
	  echo "           Use 'make serve' for native golden renders." >&2; exit 1; fi
	@if [ -n "$(strip $(PDF2DOC))" ]; then \
	  echo "dockserve: PDF2DOC is not available in the image (no Python/docling inside), and the" >&2; \
	  echo "           value is a host path. Use 'make serve' for the datasheet Extract action." >&2; exit 1; fi
	@if [ -n "$(strip $(OVERLAY_FLAGS))" ]; then \
	  echo "dockserve: OVERLAY_FLAGS names host paths that do not exist in the container." >&2; \
	  echo "           Pass OVERLAY_DIR=<dir> instead; it is mounted at /overlay." >&2; exit 1; fi
	@if [ -n "$(strip $(SYMBOL_PATH))" ]; then \
	  echo "dockserve: ignoring SYMBOL_PATH; the image ships its own KiCad/xschem/gEDA libraries."; fi
	@docker image inspect $(IMAGE) >/dev/null 2>&1 || $(MAKE) image
	@set -e; \
	abs() { case $$1 in /*) printf %s "$$1" ;; \
	                     \~*) printf %s "$$HOME$${1#\~}" ;; \
	                     *) printf %s "$(CURDIR)/$$1" ;; esac; }; \
	vols=""; \
	for spec in $(filter-out --mount,$(MOUNTS) $(EXTRA_MOUNTS)); do \
	  name=$${spec%%=*}; path=$$(abs "$${spec#*=}"); \
	  if [ ! -e "$$path" ]; then echo "dockserve: mount '$$name' has no such path: $$path" >&2; exit 1; fi; \
	  vols="$$vols -v $$path:/workspace/$$name"; \
	done; \
	if [ -n "$(strip $(DESIGNS))" ]; then \
	  d=$$(abs "$(strip $(DESIGNS))"); \
	  if [ ! -e "$$d" ]; then echo "dockserve: DESIGNS has no such path: $$d" >&2; exit 1; fi; \
	  vols="$$vols -v $$d:/workspace/$$(basename $$d)"; fi; \
	overlay=""; \
	if [ -n "$(strip $(OVERLAY_DIR))" ]; then \
	  o=$$(abs "$(strip $(OVERLAY_DIR))"); \
	  if [ ! -d "$$o" ]; then echo "dockserve: OVERLAY_DIR has no such directory: $$o" >&2; exit 1; fi; \
	  vols="$$vols -v $$o:/overlay:ro"; \
	  if [ -d "$$o/profiles" ]; then overlay="$$overlay --profile-path /overlay/profiles"; fi; \
	  if [ -f "$$o/conventions.yaml" ]; then overlay="$$overlay --conventions /overlay/conventions.yaml"; fi; \
	  if [ -z "$$overlay" ]; then \
	    echo "dockserve: OVERLAY_DIR=$$o holds neither profiles/ nor conventions.yaml" >&2; exit 1; fi; \
	  echo "dockserve: overlay$$overlay"; fi; \
	review=""; \
	if [ -n "$(strip $(REVIEW_STORE))" ]; then \
	  r=$$(abs "$(strip $(REVIEW_STORE))"); \
	  mkdir -p "$$r"; \
	  vols="$$vols -v $$r:/var/lib/agni/reviews"; \
	  review="--review-store /var/lib/agni/reviews"; fi; \
	echo "serving $(IMAGE) at http://localhost:$(patsubst :%,%,$(ADDR))/"; \
	docker run --rm --name $(DOCKER_NAME) -p $(patsubst :%,%,$(ADDR)):8080 \
	  --user $$(id -u):$$(id -g) $$vols $(DOCKER_FLAGS) $(IMAGE) \
	  serve --addr :8080 --mount-root /workspace $$overlay $$review web

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
# Run this PERIODICALLY and locally: it is deliberately NOT in `testall`, because the stamps cover the
# spec and the fixture but NOT the engine build, so a code change does not invalidate a capture on its
# own. That keeps the docs pipeline off the per-push path at the cost of catching a regression here
# rather than immediately. Commit whatever it changes, after reading it.
tutorial-runs:
	@find docsite/content/tutorials/runs -name '*.output' -delete
	@cd docsite && $(GO) run . -build >/dev/null 2>&1
	@git status --short -- docsite/content/tutorials/runs || true
	@echo "tutorial captures regenerated; review the diff above before committing"
