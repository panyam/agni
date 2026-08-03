## power-pin-mistyped

### What it means

A pin whose *name* says power or ground (`VDD`, `VCC`, `VBAT`, `GND`, `VSS`, ... — the same
rail-name family that gives it a power/ground `pin.role`) but whose *electrical type* is not
`power_in`, sitting alone on its net (a single-connection stub). The symbol author named the pin
like a supply but typed it like a signal, and then left it wired to nothing.

### Why engineers want it

`power-input-not-driven` already flags an unconnected power pin — but only when the symbol types
the pin as `power_in`. A pin named `VDD` yet typed `passive` (a common symbol-authoring slip) is
invisible to it. This rule covers exactly that gap: name says power, type does not, and the pin is
effectively unconnected.

    U1 pin "VDD", typed power_in, on a stub   -> power-input-not-driven (not this rule)
    U1 pin "VDD", typed passive,  on a stub   -> power-pin-mistyped (this rule)
    U1 pin "VDD", typed passive,  wired to a rail -> fine (not alone on its net)

### Why "alone on its net", not "unconnected"

A KiCad schematic reader synthesizes a one-pin net for every bare pin, so a pin is never literally
"on no net" — an unconnected pin reads as a net with one connection. So the rule keys on net
**fan-out** (`net.pin_count < 2`), not net membership. This is the reason the earlier membership-based
formulation was a no-op on KiCad.

### How it is written

It is a datalog rule (`query.RuleFromQuery`, WS3-038), not Go or Spec:

    bad(?ref,?pin,?net) :- pin.role(?ref,?pin,"power"),  not pin.type(?ref,?pin,"power_in"),
                           pin.net(?ref,?pin,?net), net.pin_count(?net,?c), ?c < 2, has_nc_channel(?_);
    bad(?ref,?pin,?net) :- pin.role(?ref,?pin,"ground"), ... ;
    bad(?ref,?pin,?net) => ?ref, ?pin, ?net

Two rules (power, ground) share one head; each answer row is one finding.

### Where it stays silent (conservative on purpose)

- **`has_nc_channel` gate** — silent on a format that cannot express intentional no-connect, so a
  legitimately single-connection supply pin on such a format is not a false positive.
- **No power/ground role** — a bare netlist with numeric pin names (some EDIF exports) derives no
  role, so no pin qualifies. Silent, not guessed.
- **Correctly-typed pins** — a `power_in` pin is left to `power-input-not-driven`; this rule does
  not double-report it.

### For software readers

`power-input-not-driven` is a type-checked assertion: it trusts the pin's declared *type*.
`power-pin-mistyped` is the lint for when the declared type disagrees with the *name* — like a
field named `passwordHash` typed `PlainString`. The name encodes intent the type contradicts, and
the value (here, the wiring) is also wrong. It fires only where the name-vs-type mismatch and the
missing connection coincide, so it complements the type-checked rule instead of duplicating it.
