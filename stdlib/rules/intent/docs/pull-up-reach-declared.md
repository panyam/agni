## pull-up-reach-declared

### What it means

A design's intent can declare a pull-up reach on a net, which tells `i2c-pull-up`, the two pull-up
count rules and the profile pull-up requirements how many series resistors the walk from that net may
cross before it reaches a rail. This rule checks each such declaration against the design. It fails
when the declared net name is not a net of the design.

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

A misspelled or stale declaration leaves the bus it meant to name held to the default reach. A pull-up
the designer placed deliberately behind a deeper series network then reads as missing, and the
declaration that was meant to explain it is silent about why.

### Scope note

It confirms that the net exists and nothing more. A reach declared on a net that is real but that no
pull-up rule considers, such as a net that is neither I2C nor a profile signal, changes nothing and
still passes here. An undeclared net keeps the default of 3, so a design with no `pull_up_reach_hops`
is checked exactly as before.
