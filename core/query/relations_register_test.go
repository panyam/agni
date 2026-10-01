package query_test

// This external test package (query_test, distinct from the internal query test files) blank-imports
// stdlib/relations so the query test BINARY registers the built-in EDB relations before any test runs.
// The query engine owns no relations since issue 10, so without this the internal tests' NewBase
// would project an empty fact base. Go links the internal and external test files into one binary,
// so the internal tests see the registered relations.
import _ "github.com/panyam/agni/stdlib/relations"
