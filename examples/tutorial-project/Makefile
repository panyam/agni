# The tutorial project's commands. Every target is a plain `agni` invocation, so the tutorial can
# show you the command and you can see it here unchanged. There is no code-generation step: the
# checklist in review.yaml is hand-authored, which is the right scale for a checklist this size.
#
# DESIGN selects which folder under designs/ to run against. Each design declares its own entry file
# in design.yaml, so adding a second board needs no edit here.
AGNI   ?= go run github.com/panyam/agni/cmd/agni
DESIGN ?= gateway

DIR   = designs/$(DESIGN)
ENTRY = $(DIR)/$(shell awk -F': ' '/^entry:/{print $$2}' $(DIR)/design.yaml)

# The overlay tiers. Each is passed only when this project actually has one, so a project that has
# not written conventions yet still runs. The intent file is PER-DESIGN, unlike the others.
PROFILE_FLAG     = $(if $(wildcard profiles/*.yaml),--profile-path profiles,)
PARAMS_FLAG      = $(if $(wildcard params/*.textproto),--params params,)
CONVENTIONS_FLAG = $(if $(wildcard conventions.yaml),--conventions conventions.yaml,)
INTENT_FLAG      = $(if $(wildcard $(DIR)/intent.yaml),--intent-path $(DIR)/intent.yaml,)
TIERS = $(PROFILE_FLAG) $(PARAMS_FLAG) $(CONVENTIONS_FLAG) $(INTENT_FLAG)

.PHONY: help stats check review coverage diff report views check-views regen-views

# The KiCad schematic and the board are GENERATED from the EDIF netlist, so all three views describe
# the same design by construction. Regenerate after editing the netlist, then re-run check-views.
PY ?= python3

help: ## List targets
	@grep -hE '^[a-z-]+:.*##' $(MAKEFILE_LIST) | sed 's/:.*##/\t/'

stats: ## What the tool read (run this before trusting anything downstream)
	@$(AGNI) stats $(ENTRY)

check: ## Run the rule catalog with every tier this project supplies
	@$(AGNI) check $(ENTRY) $(TIERS)

review: ## Run the review checklist and print per-item outcomes
	@$(AGNI) review $(ENTRY) --checklist review.yaml $(TIERS)

coverage: ## Per-area rollup: how many items each area actually decided
	@$(AGNI) review $(ENTRY) --checklist review.yaml $(TIERS) --coverage

diff: ## Structural diff of the two bundled revisions
	@$(AGNI) diff $(DIR)/gateway.edn $(DIR)/gateway-rev-b.edn

report: ## Write the review as a self-contained results document you can re-render later
	@mkdir -p reports/$(DESIGN)
	@$(AGNI) review $(ENTRY) --checklist review.yaml $(TIERS) \
	  --results-out reports/$(DESIGN)/review.results.json >/dev/null
	@echo "wrote reports/$(DESIGN)/review.results.json"

views: ## Faithful render of the KiCad view (the netlist has no geometry of its own)
	@$(AGNI) render $(DIR)/gateway.kicad_sch --symbol-path $(DIR)/symbols -o gateway-schematic.svg

check-views: ## The netlist and the KiCad view must still describe the same design (no net delta)
	@out=$$($(AGNI) diff $(DIR)/gateway.edn $(DIR)/gateway.kicad_sch --symbol-path $(DIR)/symbols); \
	  echo "$$out" | grep -q 'new 0  deleted 0  renamed 0  hard 0  soft 0' \
	    || { echo "VIEWS DIVERGED: the netlist and the KiCad schematic no longer agree:"; \
	         echo "$$out"; exit 1; }; \
	  echo "netlist and KiCad view agree (no net delta)"

regen-views: ## Rebuild the KiCad schematic, symbol library, and board FROM the netlist
	@$(PY) tools/gen_kicad_views.py $(DIR)/gateway.edn
	@$(PY) tools/gen_kicad_board.py $(DIR)/gateway.edn
	@$(MAKE) --no-print-directory check-views
