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

{{ includeFile "figures/absolute-maximum-rating.svg" }}

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
