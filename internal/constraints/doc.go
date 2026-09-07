// Package constraints sweeps the repo's SOURCE for CONSTRAINTS.md rules that no single package owns,
// so `go test ./...` (and therefore `make testall`) enforces them.
//
// It is one of three homes, and which one a new check belongs in follows from what it reads. A rule
// about the PACKAGE GRAPH or the module goes in the root `deps_test.go`, beside the embedding
// surface (C13) and the rule primitive (C30). A rule whose subject IS one package is tested there:
// C13's transport and filesystem clauses in `service/transport_guard_test.go`, C29 in `core/facts`,
// the model contract in `core/model/deps_test.go`. What lands here is the rest, the rule whose
// violation is a line of source in a directory nobody would think to guard.
//
// These are TESTS rather than commands in the document for the reason C29 records: both halves of a
// structural violation compile and pass, so a command written into prose goes stale without anything
// surfacing it.
package constraints
