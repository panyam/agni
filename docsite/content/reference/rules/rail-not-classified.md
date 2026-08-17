---
title: "rail-not-classified"
description: "A net named for a voltage feeds a supply pin but is not classified as a rail, so the rail rules cannot see it."
---

### What it means

A net whose **name declares a voltage** also **feeds at least one supply pin**, yet does not carry
the rail role. Two independent channels say "rail" and the engine's classification says otherwise,
so every rail-quantified rule skips it.

This is a read-health tripwire, not a design defect. Nothing here says the board is wrong.

### Why engineers want it

The rail rules, and the `net.nominal_voltage` fact, all quantify over nets carrying the rail role.
That role is stamped at ingestion from a naming lexicon, and the built-in vocabulary is
start-anchored: `VCC`, `VDD`, `+3V3`. A great many house conventions are not. A board that names
rails function-first, as `PMIC_CORE_3V3` or `SENSOR_5V0`, matches none of the built-in patterns.

The failure that follows is the bad kind: quiet. Fewer nets are rails, so fewer rail rules have
anything to quantify over, so the report comes back clean because the rules could not see the rails
rather than because the board is right. Silence reads as coverage.

Measured on a real 1700-net board, supplying the project's rail patterns moved the rail count from
13 to 91. Roughly seven eighths of that board's rails were invisible to every rail rule, with no
error and no warning.

The fix is to declare the project's rail patterns in a `--conventions` lexicon. The shipped tutorial
project does exactly this, and its `conventions.yaml` explains why in the file.

### Why it needs more than the name

A net named `..._3V3` is genuinely ambiguous. It may be a 3.3 V rail, or a signal that *swings* at
3.3 V, and no naming grammar separates the two, because both are legitimately named that way. That
ambiguity is the same one `net.signal_level` exists to represent, seen from the other side.

So firing on every voltage-named net that is not a rail would be noise. This rule requires a
**second, independent channel**: the net must also feed at least one pin the design types as a power
input. Those two channels agreeing is real evidence in a way either alone is not.

On the two real boards available, that discriminates cleanly: 45 nets on the board with an
undeclared lexicon, 5 on the board whose rails the built-in vocabulary already matches.

### Evidence honesty

- A net carrying the rail role is not reported. Once the lexicon is declared, this rule goes silent
  on the nets it was reporting, and that is the intended end state.
- **Ground is excluded.** A ground net carries a role of its own and is never what this is about.
- A net whose name carries no parseable voltage token is not reported, however rail-like it looks.
  The rule reports a classification gap it can evidence, not every rail it suspects.
- A net with no power-input pin is not reported, even if its name declares a voltage. On a format
  that cannot type power pins at all, this rule simply stays quiet rather than guessing.

### Query structure

    select N in nets where not rail_role(N) and not ground(N)
      volts = nominal_from_name(name(N))            -- skip if the name declares none
      supplies = count(P in connections(N) where type(P) == power_in)
      supplies > 0 -> finding

Reads: net.name, net.role, pin.type, on_net. Tier P.
