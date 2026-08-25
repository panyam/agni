---
title: "Absolute maximum rating"
label: "absolute maximum rating"
summary: "The stress level beyond which a part may be damaged. It is a damage threshold, not an operating target, and the vendor promises nothing about behaviour up there."
level: EE5
---

A damage threshold. The vendor is saying that beyond this point the part may be destroyed, and that
none of its other promises were evaluated up there. Exceeding it is not "using the full range".

The number that says where a part still *works* is the recommended operating condition, and it is
always the lower of the two. Between them sits a band where the part is probably not damaged and
nothing at all is guaranteed about what it does.

<svg viewBox="0 0 660 292" role="img" aria-labelledby="absmax-title" style="width:100%;height:auto;font-family:inherit"><title id="absmax-title">Three voltage bands: recommended operating below 15 V, an unpromised band from 15 to 20 V, and damage above the 20 V absolute maximum</title><g fill="none" stroke="currentColor" stroke-width="1.5" opacity="0.85"><rect x="70" y="24" width="96" height="62" fill="currentColor" opacity="0.14"/><rect x="70" y="86" width="96" height="72" fill="currentColor" opacity="0.06"/><rect x="70" y="158" width="96" height="94" fill="var(--accent-color)" opacity="0.16"/><rect x="70" y="24" width="96" height="228"/></g><g stroke="var(--accent-color)" stroke-width="2"><line x1="70" y1="86" x2="166" y2="86"/><line x1="70" y1="158" x2="166" y2="158"/></g><g fill="currentColor" font-size="12.5" text-anchor="end" opacity="0.75"><text x="60" y="90">20 V</text><text x="60" y="162">15 V</text><text x="60" y="256">0 V</text></g><g fill="currentColor" font-size="13.5"><text x="186" y="60" font-weight="600">Damage</text><text x="186" y="78" font-size="12.5" opacity="0.72">the part may not survive this</text><text x="186" y="118" font-weight="600">Works, but nothing is promised</text><text x="186" y="136" font-size="12.5" opacity="0.72">no specification was evaluated here</text><text x="186" y="200" font-weight="600">Recommended operating</text><text x="186" y="218" font-size="12.5" opacity="0.72">where the datasheet's other numbers hold</text></g><g fill="var(--accent-color)" font-size="11.5" font-weight="600"><text x="186" y="92">absolute maximum</text><text x="186" y="164">recommended max</text></g><g fill="currentColor" font-size="11.5" text-anchor="middle" opacity="0.6"><text x="118" y="272">a supply pin</text></g></svg>

The two get confused because they are printed in adjacent tables in the same units, and a design that
mistakes one for the other looks fine until a part fails in the field. So the layer keeps them as
separate `LimitKind` values (`ABSOLUTE_MAX` and `RECOMMENDED_OPERATING`) rather than one number, and a
rule declares which it means.

Rules that read it: [`supply-exceeds-abs-max`](../../rules/supply-exceeds-abs-max/) compares a supply
pin's rail against the part's rating, and
[`pin-out-of-recommended`](../../rules/pin-out-of-recommended/) catches the quieter case of a pin
sitting in the middle band.

A rating is only meaningful next to the conditions it was measured under, so a value carried without
them is treated as under-specified rather than compared. That is the same discipline the
[datasheet layer](../../../architecture/datasheet-layer/) applies to every parameter.

**Where the course teaches it:**
[chapter 7](../../../learn/07-reading-a-datasheet/) is the whole chapter, and
[Two numbers that look alike](../../../learn/07-reading-a-datasheet/#two-numbers-that-look-alike-ee5)
is the distinction on its own.
