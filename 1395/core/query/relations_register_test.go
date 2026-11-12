package query_test

// This file is in the external test package query_test, apart from the internal query test files.
// It blank-imports stdlib/relations so the query test BINARY registers the built-in EDB relations
// before any test runs. The query engine owns no relations since issue 10. Without this import, the
// internal tests' NewBase would project an empty fact base. Go links the internal and external test
// files into one binary, so the internal tests see the registered relations.
import _ "github.com/panyam/agni/stdlib/relations"
