---
title: "pin-exceeds-abs-max"
description: "A supply pin sits on a rail whose nominal voltage exceeds that pin's own absolute-maximum rating."
---

### Remedy

Move this terminal to a rail inside its own rated maximum. A part's supplies are often rated differently from one another, so work from this pin's number rather than the part's.

### What it means

A component joined to a seeded datasheet spec has a supply pin on a rail whose name states a
nominal voltage above **that pin's own** absolute-maximum rating, read from a limit row the
datasheet binds to that terminal.

### Why engineers want it

A part can have several supply pins rated differently. A voltage translator sits between two
domains and has one supply per side, commonly 1.2 to 3.6 V on one and 1.65 to 5.5 V on the other,
with absolute maxima to match. Asked as a question about the PART, that has no single right answer.

`supply-exceeds-abs-max` answers it by taking the most restrictive row and applying it to every
supply pin. That is conservative, and on a part whose terminals genuinely differ it is wrong in the
direction that costs a reviewer the most: it reports a violation where there is none, because it
checks the 6.5 V terminal against the 4.6 V one. This rule compares each terminal against the limit
its own datasheet row states, so a correct design stops being flagged and a real breach names the
pin it is on.

### Relationship to supply-exceeds-abs-max

They never both fire. This rule acts only on a part whose spec carries pin bindings, and
`supply-exceeds-abs-max` defers on exactly those parts. Everything else, including every spec
seeded before pin binding existed, is answered by the alias path exactly as before.

### Evidence honesty

Every input that cannot be trusted is a skip, never a guess:
- no MPN, unseeded MPN, no seeded set, or a spec with no pin bindings -> this rule is silent and
  the alias path answers instead;
- a design pin that does not resolve to exactly one spec pin -> skipped. Resolution leads with the
  pin NAME, uses the designator only to break a tie inside an identified package, and REFUSES when
  the two disagree or when a shared name cannot be separated. A guessed terminal would produce a
  confident finding about the wrong thing;
- limit rows that are under-specified or carry text-only conditions are not compared
  (param.MachineComparable);
- a rail name with no parseable nominal, or with conflicting nominals, is not compared.

### Query structure

join components to specs by MPN, resolve each supply pin onto a spec pin, and compare the rail's
nominal against the abs-max rows bound to that pin.

    select C in components where spec(C) has pin bindings
      for P in pins(C) where electrical_type(P) == power_in
        S = resolve_pin(spec(C), name(P), designator(P), package(C))   -- may refuse
        for R in abs_max rows bound to S
          nominal(net(P)) > max(R) -> finding

Reads: param.pin, param.pin_range, pin.electrical_type, net.name, on_net. Tier R.
