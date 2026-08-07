---
title: "Rules catalog"
description: "Every check rule the catalog ships, grouped by category, with its source."
---

The EE rule catalog. Each rule links to its full reference: what it means, why it matters, the guards it applies, and a fires-versus-fine diagram. The Source column flags where a rule comes from — a built-in, a design-intent check (`intent/`), a datalog-authored rule (`dl/`), or an interface profile (`profile/`). This page is generated from the shipped catalog, so it always matches the engine.

## connectivity

| Rule | Source | Severity | What it checks |
|---|---|---|---|
| [crystal-load-caps](crystal-load-caps/) | built-in | warning | A passive crystal has an oscillator terminal with no load capacitor to ground. |
| [dangling-endpoint](dangling-endpoint/) | built-in | warning | A wire endpoint terminates on nothing (no pin, junction, label, or other wire). |
| [dl/power-pin-mistyped](dl-power-pin-mistyped/) | datalog | warning | A pin named like power/ground but not typed power_in sits alone on its net. |
| [duplicate-net-name](duplicate-net-name/) | built-in | warning | Two electrically distinct nets carry the same name. |
| [duplicate-ref-des](duplicate-ref-des/) | built-in | error | A reference designator is claimed by more than one distinct physical part. |
| [floating-input](floating-input/) | built-in | warning | An input pin sits on a net with no driver and no pull, so its level is undefined. |
| [i2c-pull-up](i2c-pull-up/) | built-in | error | An I2C net (SDA/SCL) has no pull-up resistor. |
| [label-alias-conflict](label-alias-conflict/) | built-in | warning | One net carries two different sheet-scoped labels in the same scope. |
| [led-polarity](led-polarity/) | built-in | error | An LED's anode sits on ground and its cathode on a power rail — mounted backwards. |
| [nc-pin-connected](nc-pin-connected/) | built-in | error | A pin marked no-connect is wired into a net with other members. |
| [output-output-conflict](output-output-conflict/) | built-in | error | Two or more driving pins (outputs / power sources) share a net and fight each other. |
| [profile/esd](profile-esd/) | profile | warning | An interface signal leaves the board through a connector with no ESD clamp. |
| [profile/missing-pullup](profile-missing-pullup/) | profile | warning | An interface signal that needs a pull-up reaches no rail. |
| [profile/signal-dangling](profile-signal-dangling/) | profile | warning | An interface signal net has fewer than two connections (a dangling stub). |
| [profile/signal-missing](profile-signal-missing/) | profile | error | A signal a required interface declares is absent from the design. |
| [profile/termination](profile-termination/) | profile | warning | A bus that requires termination has no terminating device across its pair. |
| [resonator-redundant-load-caps](resonator-redundant-load-caps/) | built-in | warning | A ceramic resonator with integrated load capacitors also has an external load cap to ground on a terminal. |
| [single-pin-net](single-pin-net/) | built-in | info | A net connects to fewer than two pins (a floating stub), and is not an intentional no-connect. |
| [unconnected-component](unconnected-component/) | built-in | warning | A component appears on no net (none of its pins land on any signal). |
| [unconnected-pin](unconnected-pin/) | built-in | warning | A pin lands on no net and is not marked no-connect. |
| [unspecified-pin-with-driver](unspecified-pin-with-driver/) | built-in | warning | A pin with no declared electrical type sits on a driven net. |
| [wire-no-junction](wire-no-junction/) | built-in | warning | A wire endpoint lands mid-span on another wire with no junction dot. |

## power

| Rule | Source | Severity | What it checks |
|---|---|---|---|
| [bulk-cap](bulk-cap/) | built-in | warning | A named power rail carries no capacitor at all (no bulk reservoir). |
| [decoupling-present](decoupling-present/) | built-in | warning | A power rail feeds power-input pins but has no decoupling capacitor on it. |
| [esd-clamp-not-tvs](esd-clamp-not-tvs/) | built-in | info | An externally-exposed signal net is clamped by a Zener, not a fast ESD TVS. |
| [esd-protection](esd-protection/) | built-in | info | An externally-exposed signal net (on a connector) has no TVS device. |
| [input-protection](input-protection/) | built-in | warning | A connector feeds a power-input pin directly with no fuse or TVS in the path. |
| [power-input-not-driven](power-input-not-driven/) | built-in | error | A power-input pin sits on a net with no power source (no power-output and no power flag). |
| [power-tap-conflict](power-tap-conflict/) | built-in | warning | One net is tapped by two different design-wide names (power symbols or global labels). |
| [test-point-coverage](test-point-coverage/) | built-in | info | A power rail or ground net has no test point, on a board that uses them. |

## naming

| Rule | Source | Severity | What it checks |
|---|---|---|---|
| [diff-pair-naming](diff-pair-naming/) | built-in | warning | A differential-pair positive net (_P / _DP / trailing +) has no complementary negative net. |

## board

| Rule | Source | Severity | What it checks |
|---|---|---|---|
| [annular-width](annular-width/) | built-in | error | A via's annular ring is thinner than the loosest common fabrication floor (0.075mm). |
| [copper-clearance](copper-clearance/) | built-in | error | Copper of two different nets sits closer than the 0.127mm fabrication floor. |
| [hole-size](hole-size/) | built-in | error | A via's drill is smaller than the loosest common mechanical-drill floor (0.2mm). |
| [track-width](track-width/) | built-in | error | A routed track is narrower than the loosest common fabrication floor (0.127mm). |

## datasheet

| Rule | Source | Severity | What it checks |
|---|---|---|---|
| [cap-voltage](cap-voltage/) | built-in | error | A capacitor's datasheet rated voltage does not clear the worst rail it touches times the derate factor. |
| [fet-vdss-below-switched-rail](fet-vdss-below-switched-rail/) | built-in | error | A MOSFET sits on a rail at or above its datasheet drain-source breakdown voltage. |
| [rail-nominal-out-of-recommended](rail-nominal-out-of-recommended/) | built-in | warning | A power-input pin sits on a rail whose nominal voltage is outside the part's recommended operating supply range. |
| [regulator-output-exceeds-abs-max](regulator-output-exceeds-abs-max/) | built-in | error | A regulator's datasheet output voltage exceeds the absolute-maximum supply rating of a part it feeds. |
| [supply-exceeds-abs-max](supply-exceeds-abs-max/) | built-in | error | A power-input pin sits on a rail whose nominal voltage exceeds the part's absolute-maximum supply rating. |

## integrity

| Rule | Source | Severity | What it checks |
|---|---|---|---|
| [bus-not-modeled](bus-not-modeled/) | built-in | info | A bus's member signals are not resolved into distinct nets. |
| [intent/module-count](intent-module-count/) | intent | warning | The number of components for a declared module does not match the design intent. |
| [intent/module-missing](intent-module-missing/) | intent | warning | A functional block the design intent declares required is absent from the design. |
| [intent/property-ac-coupled](intent-property-ac-coupled/) | intent | warning | A net the design intent declares AC-coupled is carried by no series capacitor. |
| [intent/property-reset-polarity](intent-property-reset-polarity/) | intent | warning | A net the design intent declares as a reset is biased to its ASSERTED level, holding the part in reset. |
| [intent/protection-discharge](intent-protection-discharge/) | intent | warning | A rail the design intent declares needs a discharge path has no bleeder resistor. |
| [intent/protection-ovp](intent-protection-ovp/) | intent | warning | A rail the design intent declares needs OV protection has no TVS/zener clamp. |
| [intent/subsystem](intent-subsystem/) | intent | warning | An architectural subsystem the design intent declares is missing a required part or net. |
| [intent/voltage-domain-mismatch](intent-voltage-domain-mismatch/) | intent | warning | A declared voltage domain's rail is absent or named for a different nominal voltage. |
| [pin-net-conflict](pin-net-conflict/) | built-in | info | A pin appears in more than one net's connections — malformed input. |

