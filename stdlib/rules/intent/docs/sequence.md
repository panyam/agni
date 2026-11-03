## sequence

### What it checks

The design intent declares a power-up ORDER, meaning these rails in this order, held in order by a
power-good/enable chain. This is the family doc for every `intent/sequence-<name>` rule. Each declared
sequence compiles to its own rule, so a SoC's power tree and a modem's rails bind and report
independently, and each judges the adjacent links its declaration names.

For each adjacent pair of stages, the rule asserts on the earlier stage's `good` net and the later
stage's `enable` net. It fires three ways:

- a declared gating net is not on the design, so the chain is not there to enforce anything;
- the two nets are not connected, so nothing holds the later stage off;
- the chain runs the other way round, gating the earlier rail on the later one's power-good.

There is no fourth branch. A silent rule means every declared link was found in the design.

### For hardware engineers

Rails cannot come up in any order. A part given its I/O rail before its core rail pushes current
backwards through its input clamp diodes into an unpowered core; a peripheral released from reset
before its supply is good never enumerates; some parts latch up and draw until something gives. The
required order is in the SoC's and the peripherals' datasheets, and it is a real design constraint that
the schematic does not state anywhere.

A board enforces it one of three ways, and only one of them is visible in a netlist:

- a sequencer part (a PMIC stepping its rails from its own configuration or NVM);
- a hardwired chain (rail A's power-good gates rail B's enable);
- firmware driving enables in turn after reading power-good.

### Read this before binding a review item to it

**A pass does not prove the power-up order.** A netlist carries connectivity, not time. The only
trace an order leaves in connectivity is the gating chain, so a silent rule means the declared links
exist in the design. It does not prove the board powers up in the declared order. That also depends
on the delay between one rail being good and the next being enabled, and on any minimum hold time.
Those are timing questions, and no netlist answers them. They are still a human or a simulation ask,
and binding them here would over-claim.

**A board sequenced by a PMIC or by firmware cannot declare a sequence, deliberately.** There are no
gating nets to name, so a sequence declared without a gating pair fails the whole declaration at
load, with a message saying to record the order as a review note instead. With no sequence declared,
no rule is compiled and a review item bound to a sequence rule reads `needs-design-intent` rather
than passing. That reading is accurate, since the mechanism exists and this design carries no
evidence for it. Inventing net names to satisfy the schema would convert a real unknown into a green
tick.

**A link through a controller does not count.** The rule credits a gating link when the two nets are
one net, or when a single SMALL part sits on both. A series resistor, a buffer, a comparator, a
load switch and a discrete FET each count, so the divider that drops an open-drain power-good to an
enable threshold still counts. A part touching more than sixteen nets is not credited. Imagine a
power-good wired to an MCU that also drives the enable. The order then lives in firmware, and
firmware is not in the netlist. Crediting that path would let a board read as correctly sequenced
whenever its supervisory signals converge on one processor.

**Where the evidence is ambiguous the rule credits the link.** A single small part on both nets counts
even though nothing proves it gates the enable. The limit runs the other way for a chain through two
parts in series, which the one-part test reads as absent and reports as a missing link.

### Declaring it

```yaml
sequences:
  - name: SoC power tree
    relation: enable-gated
    order:
      - {rail: VDD_CORE, good: VDD_CORE_PG}
      - {rail: VDD_IO, enable: VDD_IO_EN, good: VDD_IO_PG}
      - {rail: VDD_PERIPH, enable: VDD_PERIPH_EN}
```

`order` is earliest first and needs at least two stages. `rail` names the stage; `good` and `enable`
are the handles the check reads, so a stage that gates nothing after it needs no `good` and the first
stage needs no `enable`. Declare both on a middle stage, and the rule can report a chain wired
backwards rather than merely absent.

`relation` is required, and `enable-gated` is the only value, because a netlist gives evidence for
that ordering and no other. A sequencer part owning both rails, or an explicit delay element, is a
different structure and a different query. It would arrive as a second relation, not as a looser
reading of this one.

The sequence name slugifies into the rule name, so `SoC power tree` is `intent/sequence-soc-power-tree`
and names must slugify uniquely within a declaration.

### Fixing a finding

For a missing link, either the chain was never wired and the enable needs to come from the previous
stage's power-good, or the board really does sequence some other way and the sequence should not be
declared. For a reversed chain, the two ends are swapped in the schematic. This rule matters most
for that defect. A reversed chain looks correct on the page, and it is wrong in the way that damages
parts.
