---
title: "Derating"
label: "derating"
summary: "Deliberately running a part below its stated limit by a factor the design chooses, so that tolerance, temperature and transients still leave the part inside spec."
level: EE5
---

Running a part below the limit its datasheet states, by a factor the design picks rather than one the
vendor prints. A 10 V rail with a 1.25 derate factor asks for a capacitor rated 12.5 V, not 10 V.

<svg viewBox="0 0 660 156" role="img" aria-labelledby="derate-title" style="width:100%;height:auto;font-family:inherit"><title id="derate-title">A voltage axis showing a 10 V rail, a 12.5 V requirement after a 1.25 derate factor, a 10 V part that falls short and a 16 V part that clears it</title><rect x="340" y="38" width="70" height="30" fill="var(--accent-color)" opacity="0.18"/><g stroke="currentColor" stroke-width="1.2" opacity="0.6"><line x1="60" y1="68" x2="620" y2="68"/><line x1="60" y1="68" x2="60" y2="74"/><line x1="200" y1="68" x2="200" y2="74"/><line x1="340" y1="68" x2="340" y2="74"/><line x1="480" y1="68" x2="480" y2="74"/><line x1="620" y1="68" x2="620" y2="74"/></g><g fill="currentColor" font-size="11.5" text-anchor="middle" opacity="0.6"><text x="60" y="86">0 V</text><text x="200" y="86">5 V</text><text x="340" y="86">10 V</text><text x="480" y="86">15 V</text><text x="620" y="86">20 V</text></g><line x1="340" y1="38" x2="340" y2="68" stroke="currentColor" stroke-width="2"/><line x1="410" y1="38" x2="410" y2="68" stroke="var(--accent-color)" stroke-width="2"/><line x1="410" y1="68" x2="410" y2="140" stroke="var(--accent-color)" stroke-width="1.2" stroke-dasharray="3 3" opacity="0.7"/><text x="340" y="30" fill="currentColor" font-size="12.5" font-weight="600" text-anchor="middle">the rail: 10 V</text><text x="418" y="30" fill="var(--accent-color)" font-size="12.5" font-weight="600">required: 12.5 V</text><text x="418" y="48" fill="currentColor" font-size="11.5" opacity="0.72">rail x 1.25 derate factor</text><rect x="60" y="98" width="280" height="18" fill="currentColor" opacity="0.14" stroke="currentColor" stroke-opacity="0.5"/><rect x="60" y="122" width="448" height="18" fill="var(--accent-color)" opacity="0.18" stroke="var(--accent-color)" stroke-opacity="0.7"/><g fill="currentColor" font-size="11.5"><text x="418" y="111">rated 10 V: misses the derate</text><text x="516" y="135">rated 16 V: passes</text></g></svg>

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
