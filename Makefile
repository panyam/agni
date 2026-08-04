GO ?= go
# Design used by the stats/check convenience targets. A committed example fixture by
# default; point EDN at your own design to run against real data.
EDN ?= examples/common/designs/i2c-sensor.edn

.PHONY: all proto tidy tidyall build agni install stats check vet ir-model-check test web-test web-install testall examples-test catalog-docs catalog-docs-check serve demo ghserve ghbuild ui natimage natup natdown natlogs

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
testall: vet ir-model-check ui test examples-test web-test catalog-docs-check

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
serve: ui
	$(GO) run ./cmd/agni serve --addr $(ADDR) $(MOUNTS) $(EXTRA_MOUNTS) $(NATIVE_FLAGS) $(PDF2DOC_FLAG) $(SYMBOL_FLAGS) web

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
EXAMPLE_MODS := $(dir $(wildcard examples/*/go.mod))
examples-test:
	@for d in $(EXAMPLE_MODS); do \
		echo "== $$d =="; \
		( cd $$d && $(GO) build ./... && $(GO) test ./... ) || exit 1; \
	done
	@echo "examples: all modules build + test OK"
