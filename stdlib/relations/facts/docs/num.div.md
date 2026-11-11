## num.div

### What it is

`num.div(a, b, c)` relates three numbers by `c = a / b`.

`num.div` comes from jaala's standard library. The
[Arithmetic](https://panyam.github.io/jaala/reference/builtins/#arithmetic) section of jaala's
built-ins reference says which arguments must be bound and what an absent or malformed input does,
and its examples run in jaala's own tests.

### For hardware engineers

A current-limiting resistor is a quotient. A rail driving 10 mA into a load needs the rail voltage
over 10 mA, which is 330 Ω on a 3.3 V rail. `num.div` computes it for every rail, and the result
comes out in ohms.

### For software engineers

`num.div` is `/` as a relation. Bound on the result it solves for an operand, so `num.div(3.3V, ?r,
0.001A)` binds the resistance. The agni table prints the number alone, and `num.unit` reads the unit
back.

### Datalog

The resistance that draws 10 mA from each rail:

```
net.nominal_voltage(?n, ?v), num.div(?v, 0.01A, ?r), num.unit(?r, ?u) => ?n, ?r, ?u
```

Ohm's law over literals, 5 V across 1 kΩ:

```
num.div(5V, 1000ohm, ?i), num.unit(?i, ?u) => ?i, ?u
```
