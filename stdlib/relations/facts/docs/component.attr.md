## component.attr

### What it is

`component.attr(ref_des, key, value)` yields one row per key/value attribute on a component. It
is the general-purpose read of the component's declared properties: the part number
(`MPN`), a declared interface (`interface=CAN`), a manufacturer, or any other property a reader
carried through from the source. A component with no attributes produces no rows.

### For hardware engineers

These are the fields on a placed part beyond its connectivity: the properties an engineer or a
tool wrote onto the symbol. The one that matters most for reviews is a declared interface: when
a design annotates a transceiver with `interface=CAN`, an interface-profile check binds to that
part by its declared property instead of guessing from net names. You query it to see what a
part claims about itself, or to find every part carrying a given annotation.

### For software engineers

This is the component's attribute map projected as triples, the way you would iterate a struct's
tag map. Rows are 1:many with a component (one per attribute), and the value is opaque text. It
is the annotation channel host-binding and naming overlays read: binding by a declared
`interface` attribute is an explicit key lookup, which is why it is preferred over inferring
identity from net-name conventions. An absent row means the component did not declare that key,
not that the key is false.

### Go projector

`componentAttrFacts` in `check/facts.go` walks `Model.Components()` and emits one row per entry
in each component's `Attributes` map. It reads the map directly, so the fact base is exactly
what the reader stored, no derivation. One row per attribute; empty when the source carries no
component attributes.

### Datalog

Every part that declares an interface, and which one:

```
component.attr(?r, "interface", ?v) => ?v
```

Find the components bound to a specific interface (the host-binding lookup an interface profile
uses):

```
component.attr(?r, "interface", "CAN") => ?r
```
