package service

// The query and review services run the datalog engine and the built-in / datalog / profile rule
// catalog, all of which read the built-in EDB relations. Those relations are stdlib/relations content
// since issue 10, registered by import side effect; the production binary (cmd/agni) blank-imports the
// package, so this service test binary must too, or every query and relation-reading rule sees an
// empty fact base. package service may import relations directly (relations does not import service).
import _ "github.com/panyam/agni/stdlib/relations"
