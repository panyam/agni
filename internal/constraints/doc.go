// Package constraints holds the repo-wide structural checks for CONSTRAINTS.md rules that belong to
// no single tier, so `go test ./...` (and therefore `make testall`) enforces them.
//
// A constraint whose subject IS one package is tested beside that package instead: C13 and C22 in
// `service/transport_guard_test.go`, C29 in `core/facts`, the model contract in
// `core/model/deps_test.go`. What lands here is the rule that spans directories (every reader
// declares a fidelity) or that is about the module rather than any package in it (the engine
// requires no overlay).
//
// These are TESTS rather than commands in the document for the reason C29 records: both halves of a
// structural violation compile and pass, so a command written into prose goes stale without anything
// surfacing it.
package constraints
