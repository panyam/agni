# viewer-agent answers questions from a viewer tab; it is an agent, not a demokit walkthrough.
DESIGN ?= kicad/hier_numeric_root.kicad_sch
SERVER ?= http://127.0.0.1:8080

run: ## Answer questions from the tabs of DESIGN on SERVER
	go run . --server $(SERVER) --design $(DESIGN)

build: ## Build ../../bin/viewer-agent
	go build -o ../../bin/viewer-agent .

test: ## One round against ../../bin/agni, with the tab played in Go
	go test ./...
