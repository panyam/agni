// Package constraints sweeps the repo's SOURCE for CONSTRAINTS.md rules that no single package owns,
// so `go test ./...` (and therefore `make testall`) enforces them.
//
// It is one of three homes, and a new check goes in the home that matches what it reads. A rule
// about the PACKAGE GRAPH or the module goes in the root `deps_test.go`, beside the embedding
// surface (C13) and the rule primitive (C30). A rule whose subject IS one package is tested in that
// package. C13's transport and filesystem clauses are in `service/transport_guard_test.go`, C29 is in
// `core/facts`, and the model contract is in `core/model/deps_test.go`. This package holds the rest.
// Each such rule's violation is a line of source in a directory nobody would think to guard.
//
// These are TESTS rather than commands in the document for the reason C29 records. Both halves of a
// structural violation compile and pass, so a command written into prose goes stale without anything
// surfacing it.
package constraints
