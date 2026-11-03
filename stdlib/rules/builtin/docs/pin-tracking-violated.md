## pin-tracking-violated

### What it means

A component is joined to a seeded datasheet spec. The datasheet **requires** a tracking bound
between two of its pins, stated with a modal verb like "shall never exceed". The difference between
those two pins breaks the bound.

### Why engineers want it

A part with several supplies routinely constrains them against **each other**, not just against
their own limits. A voltage translator requires one side's supply to stay at or below the other's; a
PHY requires one analog supply never to exceed another by more than half a volt; a level shifter
recommends an enable sit a volt above its reference. Each terminal can be individually inside its own
rating while the pair is still wrong, so a per-pin limit check cannot see any of these.

The bound is a **value on the difference**, not a comparison operator. "Must be less than or equal
to" is a maximum of zero, "must never exceed by more than 0.5 V" is a maximum of 0.5, and a symmetric
tolerance is a minimum and a maximum. Encoding it as an operator cannot express the non-zero cases,
which are the majority of the real ones.

### The difference is signed

The bound is on `subject - reference`, so the datasheet's order matters and swapping the two says
the opposite thing. The finding prints the subtraction in that order.

### Two tiers of evidence, and why connectivity goes first

When **both pins sit on one net**, their difference is exactly zero, from connectivity alone, with
no net name read. This is the stronger tier, and it is decisive in both directions. Tying the
terminals *satisfies* a `max 0` bound and *violates* a `min 1` bound. It works on a design whose
nets are not named for their voltages at all.

When **the pins sit on different nets**, the rule falls back to comparing the two rails'
name-declared nominals. Those nominals come from a naming convention rather than measured evidence.
Both nets must carry the rail role before either name is read. A voltage token anywhere in a name
parses as a nominal, so a signal net whose name encodes a signalling level would otherwise be
compared as though it were a supply.

### Relationship to pin-tracking-advisory

They never both fire on one relation. This rule takes the bounds the datasheet **requires**; the
advisory rule takes those it merely **recommends**, at warning severity. The split exists because
"shall never exceed" and "should, for best operation" are different claims, and reporting both at one
severity misstates one of them.

When a relation's modality was never recorded, *this* rule takes it and reports it inconclusive
rather than as an error, so an incomplete spec cannot pass in silence.

### What it skips rather than guesses

Every input that cannot be trusted is a skip, never a guess:
- no MPN, unseeded MPN, or a spec with no pins or no relations -> silent;
- a spec pin that does not map onto exactly one design terminal -> skipped, by the same
  refuse-rather-than-guess resolution that the per-pin limit rules use;
- a terminal on no net, or claimed by several nets (malformed input) -> skipped;
- on the name tier, a net that is not a rail, or a rail name with no parseable nominal -> skipped;
- a bound stated in a unit other than volts -> skipped.

Two cases are reported **inconclusive** rather than as violations, and only when the numbers actually
breach the bound. The first is a bound the datasheet scopes to a regime that this check cannot
evaluate ("transient only, not for DC"). The second is a relation with no recorded modality. Where
the numbers are within the bound, both pass.

### Query structure

join components to specs by MPN, map each relation's two spec pins back onto design terminals, and
compare the difference between those terminals against the bound.

    select C in components where spec(C) has pins and relations
      for R in tracking relations of spec(C) where required(R)
        S = terminal(C, subject_pin(R)); T = terminal(C, reference_pin(R))   -- may refuse
        if net(S) == net(T): diff = 0                                        -- connectivity
        else: require net.rail(net(S)) and net.rail(net(T)); diff = nominal(net(S)) - nominal(net(T))
        diff outside bound(R) -> finding

Reads: param.pin, param.pin_relation, net.role, net.nominal_voltage, net.name, on_net. Tier R.
