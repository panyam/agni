## num.sub

### What it is

`num.sub(a, b, c)` relates three numbers by `c = a - b`.

`num.sub` comes from jaala's standard library. The
[Arithmetic](https://panyam.github.io/jaala/reference/builtins/#arithmetic) section of jaala's
built-ins reference says which arguments must be bound and what an absent or malformed input does,
and its examples run in jaala's own tests.

### For hardware engineers

Headroom is a difference. A regulator fed from a 12 V input has 12 V minus its output rail across
it, and a linear regulator burns that drop as heat. `num.sub` computes the drop for every rail in
one query.

### For software engineers

`num.sub` is `-` as a relation. Bound on the result it solves for either operand, so `num.sub(5V,
?b, 2V)` binds 3 V. The agni table prints the number alone, and `num.unit` reads the unit back.

### Datalog

The drop from the 12 V input to each rail:

```
net.nominal_voltage(?n, ?v), num.sub(12V, ?v, ?h) => ?n, ?h
```

Solving for the second operand:

```
num.sub(5V, ?b, 2V), num.unit(?b, ?u) => ?b, ?u
```
