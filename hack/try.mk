# What `make try` needs built (agni issue 1600). The root Makefile's ui and wasm targets rebuild every
# time, and the wasm alone takes most of a minute, so a reviewer trying a second journey waited for a
# build of nothing new. Here each output is a stamp under the ignored bin/ that depends on its
# sources, so make rebuilds only what changed. The browser setup builds agni itself, through Go's
# build cache. Run from the repo root as `$(MAKE) -f hack/try.mk`.
GO ?= go

# The id `make wasm` and the browser setup stamp into their builds, from the uncommitted changes. A new
# id rebuilds the wasm, so the page's engine and the server agree on the browser cache's version.
BUILD_ID := $(shell git diff HEAD 2>/dev/null | sha256sum | cut -c1-16)
GO_SRC := $(wildcard $(shell git ls-files '*.go' go.mod go.sum))
UI_SRC := $(wildcard $(shell git ls-files web/src web/build.mjs web/package.json pnpm-lock.yaml))
STAMPS := bin/try

.PHONY: all
all: $(STAMPS)/wasm-$(BUILD_ID) $(STAMPS)/ui

$(STAMPS)/wasm-$(BUILD_ID): $(GO_SRC)
	GOOS=js GOARCH=wasm $(GO) build -ldflags "-X github.com/panyam/agni/internal/wasmengine.buildID=$(BUILD_ID)" -o web/static/agni.wasm ./cmd/agni-wasm
	install -m 644 "$$($(GO) env GOROOT)/lib/wasm/wasm_exec.js" web/static/wasm_exec.js
	mkdir -p $(STAMPS) && rm -f $(STAMPS)/wasm-* && touch $@

$(STAMPS)/ui: $(UI_SRC)
	cd web && pnpm build
	mkdir -p $(STAMPS) && touch $@
