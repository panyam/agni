# examples/common/walkthrough.mk
#
# Shared Makefile fragment for Agni examples that ship a demokit walkthrough. A
# per-example Makefile is a one-liner:
#
#   include ../common/walkthrough.mk
#
# Override the binary name if it should differ from the directory basename:
#
#   BIN := my-example
#   include ../common/walkthrough.mk
#
# Agni examples are single-mode: there is no server to run (unlike mcpkit's dual-mode
# --serve). Each target is a way to run or capture the same walkthrough.

BIN   ?= $(notdir $(CURDIR))
TRACE ?= /tmp/agni-$(BIN)-trace.json

run: ## Interactive walkthrough, plain text
	go run .

demo: ## Interactive walkthrough, TUI styled boxes
	go run . --mode=tui

note: ## Interactive walkthrough, Bubble Tea notebook cells
	go run . --mode=notebook

runquiet: ## Non-interactive (default inputs), plain text -- CI-safe
	go run . --non-interactive

record: ## Record a non-interactive run to $(TRACE)
	go run . --non-interactive --record $(TRACE)
	@echo "Trace written to $(TRACE)"

replay: record ## Replay the recorded trace deterministically
	go run . --replay $(TRACE)

doc: ## Render the walkthrough to markdown on stdout (definition, no trace)
	go run . --doc md

build: ## Build the example binary
	go build -o $(BIN) .

.PHONY: run demo note runquiet record replay doc build
.DEFAULT_GOAL := run
