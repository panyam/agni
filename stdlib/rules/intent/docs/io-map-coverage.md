## io-map-coverage

### What it checks

Which nets in the design the IO map declares, and which it says nothing about. One verdict per net.

**Nothing here is a defect.** A net the map does not name is a question nobody asked, not a fault in
the board. The rule exists so that number is visible rather than absent.

### Why coverage is the number that matters here

A design with sixteen hundred nets and a map declaring two hundred of them has two hundred checked
and fourteen hundred **unexamined**, which is not the same as clean. Report only the two hundred and
the result is green about a fraction of the board, with the size of that fraction nowhere in the
output.

This is the other half of a pair, and the two get confused constantly because they look alike and are
opposite defects:

| | usually means | reported by |
|---|---|---|
| the map declares a net the netlist does not have | a real disconnection | `intent/io-map-net-absent`, as a fail |
| the netlist has a net the map does not declare | an incomplete map | this rule, as `not-considered` |

### Rails and grounds are counted, and said so

A pin map does not usually name `GND` or a supply rail, and on a real board those are a large share
of the nets. They are still counted in the denominator, and their verdict says which they are.

Carving them out would be the tool deciding which absences are acceptable, and that is the judgment
that lets a real gap hide. A map that forgot an entire peripheral bank would read as well-covered if
the arithmetic quietly excused a third of the board. Naming them in the reason gives a reader the
discrimination without the tool making the call.

### Reading the result

The rule heading carries the whole tally, so the coverage number is the heading and not something to
count by hand:

```
intent/io-map-coverage  47 pass, 1553 not-considered
```

The per-net rows are the addressable form of the same thing, so a specific net can be asked about
rather than only the total. In the terminal the row list is capped, with the elided count stated; the
heading is always complete.

### Declaring it

Nothing extra. It reads the same `io_map` section the other three rules read, and it is compiled
whenever a map is declared.

### Fixing a finding

There is nothing to fix in the design. Either declare the nets the map does not cover, or accept the
coverage and read the other IO-map results as being about the covered fraction alone.
