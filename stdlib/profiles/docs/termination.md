## termination

### What it means

A profile whose bus needs termination (a differential pair like CAN's CANH/CANL, or RS-485) is in
use, but no single component bridges its two designated signal nets — the terminating resistor is
missing.

### Why engineers want it

A differential bus with no termination reflects its own signal off the unterminated end. At low
speed the link often works, so the omission survives bring-up; under load or at rate the reflections
corrupt data and the fault reads as intermittent, hard-to-localize bus errors rather than an obvious
dead link. Termination is easy to forget precisely because its absence is not immediately fatal.

### How it is checked

`terminated(?h)` is derived when a high-suffix net (e.g. `_CANH`) `reaches` a low-suffix net
(`_CANL`) through the series-passive walk. That walk crosses only 2-net R/L/ferrite/fuse elements,
so it finds the terminating resistor across the pair but does NOT cross the multi-pin transceiver
that legitimately drives both lines — and because it is transitive it also accepts a split
termination (two 60Ω resistors with a midpoint). When the interface is in use (the confidence gate
holds) and nothing terminates the pair, the high net is reported as unterminated. The two suffixes
are the requirement's `high`/`low` params, so the same compiler serves any differential bus that
names its pair by convention.

### For software readers

It is a topology assertion: "somewhere in the graph, one node must connect these two specific edges."
The signals can each be present and fully wired (presence and dangling checks pass) while the
bridging element between them is absent — like two services that are both up and reachable but with
no configured link between them.
