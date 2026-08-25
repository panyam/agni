---
title: "Derating"
label: "derating"
summary: "Deliberately running a part below its stated limit by a factor the design chooses, so that tolerance, temperature and transients still leave the part inside spec."
level: EE5
---

Running a part below the limit its datasheet states, by a factor the design picks rather than one the
vendor prints. A 10 V rail with a 1.25 derate factor asks for a capacitor rated 12.5 V, not 10 V.

{{ includeFile "figures/derating.svg" }}

Neither number in that comparison is exact. A rail sits a few percent off its nominal value, a supply
overshoots on a load step, a part's rating was characterised at 25°C and falls as the board warms, and
both sides drift with age. Sitting exactly on the rating means any one of those ordinary movements
takes the design out of spec.

It is constantly confused with the [absolute maximum rating](../absolute-maximum-rating/), and the two
sit at opposite ends of the same axis. An absolute maximum is the vendor's ceiling, printed in the
document, and a fact about the part. A derate factor is the designer's policy about how far below a
limit to sit. Nothing in a datasheet states it, and two teams reading the same page can derate
differently and both be right.

The factor also depends on what is being derated. Around 20% is the common convention for a ceramic
capacitor's voltage. Sizing that same capacitor for stable capacitance under DC bias usually wants 2x
or more, which is a different concern with the same shape, and one no rule in this catalog checks yet.

Rules that read it. [`cap-voltage`](../../rules/cap-voltage/) compares a capacitor's rated voltage
against its worst rail times a fixed 1.25, and states the arithmetic in the finding so the factor is
visible rather than assumed. [`intent-rail-current-margin`](../../rules/intent-rail-current-margin/)
does the same for current with a factor you declare, and leaves the harder failure to
[`intent-rail-current-capacity`](../../rules/intent-rail-current-capacity/), because a supply that
cannot meet the peak at all is one defect rather than two.
[`fet-vdss-below-switched-rail`](../../rules/fet-vdss-below-switched-rail/) covers the transient case,
where inductive kick and hot-plug push a drain well above the nominal rail.

Derating only makes sense against a limit the vendor actually stated, so a rule derates from a
transcribed parameter with its citation attached, never from a number somebody remembered.

**Where the course teaches it:**
[The numbers](../../../learn/03-why-every-chip-needs-capacitors/#the-numbers-ee5) in chapter 3
introduces the derate factor on a capacitor's voltage rating, and
[chapter 7](../../../learn/07-reading-a-datasheet/) is the datasheet contract a factor is applied to.
