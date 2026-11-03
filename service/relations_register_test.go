package service

// The query and review services run the datalog engine and the built-in / datalog / profile rule
// catalog. All of these read the built-in EDB relations. Since issue 10 those relations live in
// stdlib/relations and register by import side effect. The production binary in cmd/agni
// blank-imports the package, so this service test binary must too. Otherwise every query and
// relation-reading rule sees an empty fact base. package service may import relations directly,
// since relations does not import service.
//
// reviewquery is imported for the same reason, one layer up. core/review holds no query language
// (C29), so a manifest's inline query binding needs the datalog bridge registered. Without it the
// binding fails to compile at Load.
//
// lib is the shipped derived-relation library, imported so the column-kind snapshot covers its
// members as the binary serves them.
import (
	_ "github.com/panyam/agni/stdlib/lib"
	_ "github.com/panyam/agni/stdlib/relations"
	_ "github.com/panyam/agni/stdlib/reviewquery"
)
