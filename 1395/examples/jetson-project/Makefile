# The Jetson tutorial project's commands. Every target is a plain `agni` invocation, so the tutorial
# can show you the command and you can see it here unchanged.
AGNI ?= go run github.com/panyam/agni/cmd/agni
DIR = designs/jetson

# The board is Antmicro's, under Apache-2.0, and lives in the pinned samples corpus rather than in
# this repo. `board` copies it in beside our own design.yaml, which it never overwrites.
SAMPLES = ../../tools/samples/boards/jetson-agx-thor-baseboard

.PHONY: help board rev-b stats check diff

help: ## List targets
	@grep -hE '^[a-z-]+:.*##' $(MAKEFILE_LIST) | sed 's/:.*##/\t/'

board: ## Copy the fetched board into designs/jetson (run `make samples-oracle` at the repo root first)
	@test -d $(SAMPLES) || { echo "no board at $(SAMPLES): run 'make samples-oracle' at the repo root"; exit 1; }
	@for f in $(SAMPLES)/*; do [ "$$(basename $$f)" = design.yaml ] || cp -R $$f $(DIR)/; done
	@echo "copied the Jetson baseboard into $(DIR)"

rev-b: ## Build rev B from the board copy, by the documented edit in tools/make_rev_b.py
	@python3 tools/make_rev_b.py $(DIR) designs/jetson-rev-b
	@echo "built designs/jetson-rev-b"

stats: ## What the tool read (run this before trusting anything downstream)
	@$(AGNI) stats $(DIR)

check: ## Run the rule catalog with every tier this project supplies
	@$(AGNI) check $(DIR)

diff: ## Compare rev A with rev B
	@$(AGNI) diff $(DIR) designs/jetson-rev-b
