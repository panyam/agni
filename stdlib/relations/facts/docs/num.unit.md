## num.unit

### What it is

`num.unit(number, unit)` binds `unit` to the unit of `number`, such as `"V"` or `"A"`.

`num.unit` comes from jaala's standard library. The [Numbers and
units](https://panyam.github.io/jaala/reference/builtins/#numbers-and-units) section of jaala's
built-ins reference says which arguments must be bound and what an absent or malformed input does,
and its examples run in jaala's own tests.

### For hardware engineers

A datasheet limit table mixes voltages and currents in one column. `num.unit` says which is which,
so a voltage check can keep the volt rows and leave the current limits alone. It also shows the unit
of a computed value, since the agni table prints only the number.

### For software engineers

`num.unit` is a getter on a unit-carrying number. Group by `num.unit` to get one answer per unit.

### Datalog

Each datasheet upper limit with its unit:

```
param.range(?mpn, ?s, ?k, ?min, ?max), num.unit(?max, ?u) => ?mpn, ?s, ?max, ?u
```

Only the limits stated in volts:

```
param.range(?mpn, ?s, ?k, ?min, ?max), num.unit(?max, "V") => ?mpn, ?s, ?k, ?max
```
