---
title: "Decoupling capacitor"
label: "decoupling capacitor"
summary: "A small capacitor at a chip's supply pin, holding charge locally so a burst of switching current does not have to travel down an inductive trace from the regulator."
level: EE3
---

A small capacitor from a supply pin to ground, fitted as close to that pin as the layout allows. It is
a local charge reservoir. The regulator handles the average current a chip draws, and the capacitor
handles the gulp the chip takes when its outputs all switch within a nanosecond of each other.

The load-bearing word is *local*. Copper has inductance, roughly 1nH per millimetre, and inductance
resists a *change* in current. The relation is `V = L · di/dt`. Fifty millimetres of trace is about 50nH, so a demand
that rises by 100mA over 10ns drops half a volt along the way. On a 3.3V part that is a 15% sag,
arriving exactly when the chip is busiest and gone again in nanoseconds.

<svg viewBox="0 0 680 290" role="img" aria-labelledby="decap-title" style="width:100%;height:auto;font-family:inherit"><title id="decap-title">A regulator feeding a chip through a long inductive trace, with a 100nF capacitor from the rail to ground right at the chip's supply pin</title><g fill="currentColor" font-size="11.5" text-anchor="middle" opacity="0.72"><text x="85" y="44">supplies the average</text><text x="290" y="62">about 50 nH of trace inductance</text></g><g fill="none" stroke="currentColor" stroke-width="1.6" opacity="0.85"><rect x="30" y="60" width="110" height="48"/><rect x="520" y="56" width="130" height="76"/></g><g fill="currentColor" text-anchor="middle"><text x="85" y="90" font-size="13.5">regulator</text><text x="585" y="86" font-size="14">chip</text></g><g fill="currentColor" font-size="11" text-anchor="middle" opacity="0.7"><text x="585" y="106">outputs switching</text></g><g fill="none" stroke="currentColor" stroke-width="2"><path d="M140 84 H250"/><path d="M250 84 L258 74 L274 94 L290 74 L306 94 L322 74 L330 84"/><path d="M330 84 H520"/><path d="M478 84 V150 M478 160 V250"/><path d="M458 150 H498 M458 160 H498"/><path d="M585 132 V250 M85 108 V250"/><path d="M40 250 H620"/></g><g fill="currentColor"><circle cx="478" cy="84" r="3.2"/><circle cx="478" cy="250" r="3.2"/><circle cx="585" cy="250" r="3.2"/><circle cx="85" cy="250" r="3.2"/></g><g fill="currentColor" font-size="10.5" text-anchor="end" opacity="0.7"><text x="514" y="76">supply pin</text></g><g text-anchor="end"><text x="450" y="164" font-size="11.5" font-weight="600" fill="var(--accent-color)">100 nF at the pin</text><text x="450" y="206" font-size="11.5" fill="currentColor" opacity="0.72">supplies the fast gulp</text></g><g fill="currentColor" font-size="11" text-anchor="middle" opacity="0.6"><text x="330" y="270">ground</text></g></svg>

The failure this prevents is a quiet one, which is why
[`decoupling-present`](../../rules/decoupling-present/) is a warning rather than an error. A rail with
no decoupling powers up, runs, and passes bring-up. It fails later and intermittently, as a
spontaneous reset, a corrupted register read, an ADC noisier than it should be, or a link that drops
once an hour, on three units out of ten and not the other seven. None of those symptoms points at a
capacitor, and engineers lose weeks to them at the end of a project rather than the beginning.

A **bulk capacitor** is the same idea one level up. It is larger, one per rail rather than one per
pin, and it covers slower swings such as a whole subsystem waking up.
[`bulk-cap`](../../rules/bulk-cap/) looks for it separately, because a rail can carry decoupling at
every pin and still sag when something big turns on.

Two numbers matter once you have decided which capacitor you meant. The vendor states the capacitance
it wants, typically 100nF at each supply pin, and that lives in the parameter layer because no netlist
implies it. The voltage rating is the other one, and exceeding it has a mechanism behind it rather
than a guideline. Ceramic capacitors fail short, so an overstressed decoupling cap becomes a dead
short from the rail to ground. [`cap-voltage`](../../rules/cap-voltage/) checks it with a derating
factor and cites the datasheet row it compared against.

A netlist check can prove the capacitor is present. Only geometry can say whether it is close, and a
capacitor on the correct net placed 20mm away does not decouple anything, because the loop it forms
with the chip has enough inductance to defeat it at the frequencies it was fitted for.

**Where the course teaches it:**
[chapter 3](../../../learn/03-why-every-chip-needs-capacitors/) is the whole chapter, from
[the role](../../../learn/03-why-every-chip-needs-capacitors/#the-role-ee3) through to
[the copper](../../../learn/03-why-every-chip-needs-capacitors/#the-copper-ee7).
