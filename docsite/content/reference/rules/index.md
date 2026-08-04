---
title: "Rules catalog"
description: "Every built-in check rule, grouped by category."
---

The built-in EE rule catalog. Each rule links to its full reference: what it means, why it matters, the guards it applies, and a fires-versus-fine diagram. This page is generated from the shipped catalog, so it always matches the engine.

## connectivity

| Rule | Severity | What it checks |
|---|---|---|
| [crystal-load-caps](crystal-load-caps/) | warning | A passive crystal has an oscillator terminal with no load capacitor to ground. |
| [dangling-endpoint](dangling-endpoint/) | warning | A wire endpoint terminates on nothing (no pin, junction, label, or other wire). |
| [duplicate-net-name](duplicate-net-name/) | warning | Two electrically distinct nets carry the same name. |
| [duplicate-ref-des](duplicate-ref-des/) | error | A reference designator is claimed by more than one distinct physical part. |
| [floating-input](floating-input/) | warning | An input pin sits on a net with no driver and no pull, so its level is undefined. |
| [i2c-pull-up](i2c-pull-up/) | error | An I2C net (SDA/SCL) has no pull-up resistor. |
| [label-alias-conflict](label-alias-conflict/) | warning | One net carries two different sheet-scoped labels in the same scope. |
| [led-polarity](led-polarity/) | error | An LED's anode sits on ground and its cathode on a power rail — mounted backwards. |
| [nc-pin-connected](nc-pin-connected/) | error | A pin marked no-connect is wired into a net with other members. |
| [output-output-conflict](output-output-conflict/) | error | Two or more driving pins (outputs / power sources) share a net and fight each other. |
| [resonator-redundant-load-caps](resonator-redundant-load-caps/) | warning | A ceramic resonator with integrated load capacitors also has an external load cap to ground on a terminal. |
| [single-pin-net](single-pin-net/) | info | A net connects to fewer than two pins (a floating stub), and is not an intentional no-connect. |
| [unconnected-component](unconnected-component/) | warning | A component appears on no net (none of its pins land on any signal). |
| [unconnected-pin](unconnected-pin/) | warning | A pin lands on no net and is not marked no-connect. |
| [unspecified-pin-with-driver](unspecified-pin-with-driver/) | warning | A pin with no declared electrical type sits on a driven net. |
| [wire-no-junction](wire-no-junction/) | warning | A wire endpoint lands mid-span on another wire with no junction dot. |

## power

| Rule | Severity | What it checks |
|---|---|---|
| [bulk-cap](bulk-cap/) | warning | A named power rail carries no capacitor at all (no bulk reservoir). |
| [decoupling-present](decoupling-present/) | warning | A power rail feeds power-input pins but has no decoupling capacitor on it. |
| [esd-clamp-not-tvs](esd-clamp-not-tvs/) | info | An externally-exposed signal net is clamped by a Zener, not a fast ESD TVS. |
| [esd-protection](esd-protection/) | info | An externally-exposed signal net (on a connector) has no TVS device. |
| [input-protection](input-protection/) | warning | A connector feeds a power-input pin directly with no fuse or TVS in the path. |
| [power-input-not-driven](power-input-not-driven/) | error | A power-input pin sits on a net with no power source (no power-output and no power flag). |
| [power-tap-conflict](power-tap-conflict/) | warning | One net is tapped by two different design-wide names (power symbols or global labels). |
| [test-point-coverage](test-point-coverage/) | info | A power rail or ground net has no test point, on a board that uses them. |

## naming

| Rule | Severity | What it checks |
|---|---|---|
| [diff-pair-naming](diff-pair-naming/) | warning | A differential-pair positive net (_P / _DP / trailing +) has no complementary negative net. |

## board

| Rule | Severity | What it checks |
|---|---|---|
| [annular-width](annular-width/) | error | A via's annular ring is thinner than the loosest common fabrication floor (0.075mm). |
| [copper-clearance](copper-clearance/) | error | Copper of two different nets sits closer than the 0.127mm fabrication floor. |
| [hole-size](hole-size/) | error | A via's drill is smaller than the loosest common mechanical-drill floor (0.2mm). |
| [track-width](track-width/) | error | A routed track is narrower than the loosest common fabrication floor (0.127mm). |

## datasheet

| Rule | Severity | What it checks |
|---|---|---|
| [cap-voltage](cap-voltage/) | error | A capacitor's datasheet rated voltage does not clear the worst rail it touches times the derate factor. |
| [rail-nominal-out-of-recommended](rail-nominal-out-of-recommended/) | warning | A power-input pin sits on a rail whose nominal voltage is outside the part's recommended operating supply range. |
| [supply-exceeds-abs-max](supply-exceeds-abs-max/) | error | A power-input pin sits on a rail whose nominal voltage exceeds the part's absolute-maximum supply rating. |

## integrity

| Rule | Severity | What it checks |
|---|---|---|
| [bus-not-modeled](bus-not-modeled/) | info | A bus's member signals are not resolved into distinct nets. |
| [pin-net-conflict](pin-net-conflict/) | info | A pin appears in more than one net's connections — malformed input. |

