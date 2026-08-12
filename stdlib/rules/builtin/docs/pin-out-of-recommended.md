## pin-out-of-recommended

### What it means

A component joined to a seeded datasheet spec has a supply pin on a rail whose name states a
nominal voltage outside **that pin's own** recommended operating range, above its maximum or below
its minimum.

### Why engineers want it

Outside the recommended operating range the datasheet's guaranteed specifications no longer hold.
The part may still function, but margin, accuracy and lifetime are no longer assured, and the
failure is the kind that survives the bench and appears in the field.

`rail-nominal-out-of-recommended` cannot answer this for a part with more than one supply. It
declines outright, because the range is two-sided and applying the wrong terminal's window invents
an over- or under-voltage that is not there. Multi-supply parts are therefore unchecked today. This
rule reads the range per terminal, so those parts are checked rather than skipped.

### Relationship to rail-nominal-out-of-recommended

They never both fire. This rule acts only on a part whose spec carries pin bindings, and
`rail-nominal-out-of-recommended` defers on exactly those parts. Its single-recommended-row
restriction still governs everything else.

### Evidence honesty

Every input that cannot be trusted is a skip, never a guess:
- no MPN, unseeded MPN, no seeded set, or a spec with no pin bindings -> this rule is silent and
  the alias path answers instead;
- a design pin that does not resolve to exactly one spec pin -> skipped, for the reasons on
  `pin-exceeds-abs-max`;
- limit rows that are under-specified or carry text-only conditions are not compared;
- a bound the datasheet did not state is not invented: a row with only a maximum is checked only
  above, and a row with only a minimum only below;
- a rail name with no parseable nominal, or with conflicting nominals, is not compared.

### Query structure

join components to specs by MPN, resolve each supply pin onto a spec pin, and compare the rail's
nominal against the recommended-operating rows bound to that pin.

    select C in components where spec(C) has pin bindings
      for P in pins(C) where electrical_type(P) == power_in
        S = resolve_pin(spec(C), name(P), designator(P), package(C))   -- may refuse
        for R in recommended rows bound to S
          nominal(net(P)) > max(R) or nominal(net(P)) < min(R) -> finding

Reads: param.pin, param.pin_range, pin.electrical_type, net.name, on_net. Tier R.
