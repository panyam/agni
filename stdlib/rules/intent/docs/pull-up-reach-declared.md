## pull-up-reach-declared

### What it means

A design's intent can declare a pull-up reach on a net. The reach tells `i2c-pull-up`, the two
pull-up count rules and the profile pull-up requirements how many series resistors the walk from
that net may cross before it reaches a rail. This rule checks each such declaration against the
design. It fails when the declared net name is not a net of the design.

```yaml
intent:
  nets:
    SDA_MAIN: {pull_up_reach_hops: 4}   # isolation resistor, then a level-shifting stage
```

### Why engineers want it

The reach is looked up by the net's exact name, so a declaration spelled `SDA-MAIN` on a board whose
net is `SDA_MAIN` matches nothing. Without this rule the walk would quietly keep the default of 3, the
bus would still fail `i2c-pull-up`, and the declaration would read as one that took effect but did
not help.

### Impact

A misspelled or stale declaration misses the bus it meant to name, and that bus is held to the
default reach. The designer placed a pull-up deliberately behind a deeper series network, and that
pull-up then reads as missing. The declaration was meant to explain it and is silent about why.

### Scope note

It confirms that the net exists and nothing more. Suppose a reach is declared on a real net that no
pull-up rule considers, such as a net that is neither I2C nor a profile signal. That declaration
changes nothing and still passes here. An undeclared net keeps the default of 3, so a design with no
`pull_up_reach_hops` is checked exactly as before.
