package profiles

// The interface-profile rules compile to datalog (query.RuleFromQuery) and read the built-in EDB
// relations (pin.role, component-on-net, rail, ...). Those relations are stdlib/relations content
// since issue 10, registered by import side effect; the production binary blank-imports the package,
// so the profiles test binary must too, or every compiled rule queries an empty fact base and finds
// nothing. package profiles may import relations directly (relations does not import profiles, so
// there is no cycle).
import _ "github.com/panyam/agni/stdlib/relations"
