## sequence

### What it checks

The design intent declares a power-up ORDER: these rails, in this order, held in order by a
power-good/enable chain. This is the family doc for every `intent/sequence-<name>` rule. Each declared
sequence compiles to its own rule, so a SoC's power tree and a modem's rails bind and report
independently, and each judges the adjacent links its declaration names.

For each adjacent pair of stages, the earlier stage's `good` net and the later stage's `enable` net are
the assertion. The rule fires three ways:

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

**What a pass means, exactly.** A netlist carries connectivity, not time. The only trace an order
leaves in connectivity is the gating chain, so a silent rule means the declared links exist in the
design. It does not mean the board is proven to power up in the declared order: the delay between one
rail being good and the next being enabled, and any minimum hold time, are timing questions no netlist
answers. Those stay a human or a simulation ask, and binding them here would over-claim.

**A board sequenced by a PMIC or by firmware cannot declare a sequence, deliberately.** There are no
gating nets to name, so the declaration is rejected at load with a message that says so, no rule is
compiled, and a review item bound to a sequence rule reads `needs-design-intent` rather than passing.
That is the honest reading: the mechanism exists and this design carries no evidence for it. Inventing
net names to satisfy the schema would convert a real unknown into a green tick.

**A link through a controller does not count.** The rule credits a gating link when the two nets are
one net, when a bounded series walk connects them (one pass element, so the divider that drops an
open-drain power-good to an enable threshold still counts), or when a single SMALL part sits on both
(a buffer, a comparator, a load switch, a discrete FET). A part touching more than sixteen nets is not
credited, because a power-good landing on an MCU that also drives the enable means the order lives in
firmware, and firmware is not in the netlist. Crediting that path would let any board whose supervisory
signals converge on one processor read as correctly sequenced.

**Where the evidence is ambiguous the rule does not fire.** A chain running through two active parts in
series reads as absent to the walk above, so the rule prefers a missed finding to a fail that is not a
genuine defect.

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
stage needs no `enable`. Declaring both on a middle stage is what lets the rule report a chain wired
backwards rather than merely absent.

`relation` is required, and `enable-gated` is the only value: it is the one ordering a netlist
evidences. A different structure (a sequencer part owning both rails, an explicit delay element) is a
different query and would arrive as a second relation, not as a looser reading of this one.

The sequence name slugifies into the rule name, so `SoC power tree` is `intent/sequence-soc-power-tree`
and names must slugify uniquely within a declaration.

### Fixing a finding

For a missing link, either the chain was never wired and the enable needs to come from the previous
stage's power-good, or the board really does sequence some other way and the sequence should not be
declared. For a reversed chain, the two ends are swapped in the schematic, which is the defect this
rule is most worth having for: it looks correct on the page and it is wrong in exactly the way that
damages parts.
