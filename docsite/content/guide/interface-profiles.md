---
title: "Interface profiles"
description: "Declare an interface's signals and what must be true of them, and check enforces that reading wherever the interface appears."
---

A bus has a shape. CAN is a CANH/CANL pair with a termination resistor across it, driven by a
transceiver with TXD and RXD. I2C is SCL and SDA, both pulled up. An interface profile writes
that shape down as YAML, and the engine compiles it into check rules. Agni ships profiles for
SPI-NOR, eMMC, CAN, LIN, A2B, PCIe and SGMII. Your own interfaces, and your own reading of a
standard one, go in a directory you hand to `--profile-path`. No Go.

## Write a profile

I2C is not one of the built-ins, which makes it a short first example. Two signals, both
needing a pull-up:

```yaml
# profiles/i2c.yaml
name: I2C
signals:
  - {name: SCL, suffix: SCL, pullup: true, anchor: true}
  - {name: SDA, suffix: SDA, pullup: true}
requirements:
  - {type: signal-missing}
  - {type: missing-pullup}
  - {type: signal-dangling}
```

Point `check` at the directory holding it. `--tag profile=I2C` narrows the run to this
profile's rules:

```
agni check cmd/agni/testdata/conformance/showcase.fires.kicad_pro \
  --profile-path ./profiles/ --tag profile=I2C
```

```
findings by rule:
  profile-overlay/i2c-missing-pullup 1

first 1:
  [warning] profile-overlay/i2c-missing-pullup: SCL (I2C signal net SCL needs a pull-up but reaches no rail)

1 finding(s) total
```

Three rules came out of that declaration, one per requirement. On the variant of the same board
with the pull-ups fitted, all three run and stay quiet:

```
no findings (3 rule(s) run)
```

Rules from `--profile-path` are namespaced `profile-overlay/`, so an overlay rule is never
mistaken for a built-in one.

## The shape of the file

| key | what it is |
|---|---|
| `name` | the profile's name. It appears in rule names, in every finding's message, and as the `profile` tag. |
| `signals` | the interface's roles, each with a `name` (the role, e.g. `SCL`) and one matcher. |
| `requirements` | the checks this profile declares, each naming a registered type. |
| `host` | optional. Binds the interface to the component that declares it. |

`host` takes `{attr: <key>, value: <val>}` to match a component attribute the design carries
(`interface=CAN`), or `{class: <device-class>}` to match what a part's datasheet says it is.
Either or both, and a component matching either one is a host. The class form needs a seeded
parameter set, so without [`--params`](../datasheets/) it finds nothing rather than guessing.

## The four matcher forms

A signal is found by net name, and declares exactly one of these:

| form | example | when it earns its place |
|---|---|---|
| `suffix` | `suffix: _CANH` | the readable default. The role is the tail of the net name. |
| `prefix` + `suffix` | `prefix: PCIE_`, `suffix: _TXP` | the tail is shared with a foreign bus, and a bus prefix tells them apart. |
| `glob` | `glob: ETH_SW*_A_H` | the identity is in the head of the name and the tail is not distinctive. |
| `regex` | `regex: ^ETH_SW\d+_P\d+_.*_H$` | multi-instance naming a glob cannot express. RE2, not auto-anchored, so anchor it yourself. |

Prefix and suffix are conjunctive and count as one form together. Declaring two forms on one
signal is an error, because a signal that can be found two ways has no single convention to
enforce.

## Requirements

| type | what it checks | params |
|---|---|---|
| `signal-missing` | every declared signal appears on a bus that is in use | |
| `host-incomplete` | a declared host is wired to all of the interface's signals | |
| `missing-pullup` | a signal marked `pullup: true` reaches a rail | |
| `signal-dangling` | a signal net has at least two connections | |
| `termination` | a terminating device bridges the bus pair | `high`, `low` |
| `esd` | a signal leaving through a connector has an ESD clamp | |

`termination` is the one that takes params, naming the two bridged net-name suffixes. The
shipped CAN profile is the worked example:

```yaml
# stdlib/profiles/builtins/can.yaml
name: CAN
host: {attr: interface, value: CAN}
signals:
  - {name: CANH, suffix: _CANH, anchor: true}
  - {name: CANL, suffix: _CANL}
  - {name: TXD,  suffix: _TXD}
  - {name: RXD,  suffix: _RXD}
requirements:
  - {type: signal-missing}
  - {type: host-incomplete}
  - {type: termination, params: {high: _CANH, low: _CANL}}
  - {type: signal-dangling}
  - {type: esd}
```

Those files are the built-ins themselves, not copies of them, so they are safe to read as
examples.

## The anchor, and when a profile decides it is looking at your board

A profile that finds none of its signals has to stay silent. A board with no CAN on it is not
a board with a broken CAN bus. Two gates decide this, and both have to clear before the
completeness check runs.

The **in-use gate** wants at least two distinct signals of the profile to appear as nets. One
match is not evidence, because a real design has many `_CS` nets belonging to many things.

The **anchor** is the signal you mark `anchor: true`, the one that is always present when the
interface is. Completeness is reported against it, so a finding reads `CAN interface
(anchored at net CAN_CANH) is missing required signal RXD` rather than naming a net that is not
there. At most one signal may be the anchor, and a profile that declares none generates no
completeness rule at all.

## Re-bind a built-in instead of re-writing it

When the engine already reads your interface and only the net naming differs, a naming map
re-binds the roles and leaves the structure alone:

```yaml
# profiles/can.yaml
override: CAN
suffixes:
  CANH: _CAN_P
  CANL: _CAN_N
```

Unmapped roles keep the built-in's suffix. Requirements, host binding and anchor come across
unchanged. A profile carrying a built-in's name replaces that built-in's rules rather than
running beside it, and the CLI says so on stderr:

```
note: profile-overlay supersedes 5 rule(s): profile/can-signal-missing, profile/can-host-incomplete, profile/can-termination-missing, profile/can-signal-dangling, profile/can-esd-missing
```

Two readings of one interface running together would invent failures rather than merely
duplicate them, since the built-in still anchors on its own naming and reports each re-bound
role as missing. [Authoring an overlay](../../build/overlay/) covers the mechanism.

## Where the flag reaches

`--profile-path` takes a directory of these YAML files, and three surfaces accept it:
`agni check`, `agni review`, and `agni serve`, where the composed profiles reach the web check
panel too. The interface-coverage matrix in the viewer is the exception. It projects the
built-in profiles only, so an overlay profile's findings appear while its coverage row does not.

## What the tooling tells you when the declaration is wrong

A profile that cannot do what it says is refused at load, with the error naming what was
available:

```
error: profiles: i2c.yaml: profile "I2C": unknown requirement type "pullup" (known: esd, host-incomplete, missing-pullup, signal-dangling, signal-missing, termination)
```

```
error: profiles: mybus.yaml: profile "MYBUS": requirement "termination": needs the low param, naming the two bridged net-name suffixes (e.g. high: _CANH, low: _CANL); got map[high:_H]
```

```
error: profiles: mybus.yaml: profile "MYBUS": signal "H" declares 2 matcher forms: set exactly one of suffix/prefix, glob, or regex
```

A matcher that is merely too loose cannot be caught that way, because whether `_H` names one
role or half the board depends on the board. `check` judges that against the design in front of
it. A profile whose signals claim an implausible share of the nets:

```
warning: profile "ETH100": signal "AP" (regex "_H") matches 8 of 32 nets (25%) — the matcher looks too broad to name one role; narrow it with a prefix, a glob, or an anchored regex
```

And a profile that cannot tell two of its own roles apart:

```
warning: profile "MYBUS": signals "DATA" and "CLK" both match net "CAN_CANH" — the profile cannot tell these two roles apart, so whichever rule runs first claims the net
```

Both describe the profile you wrote, not the board, so neither is a finding. They go to stderr,
they never change the exit code, and they stay out of `--format json`.

## What a profile cannot do

A suffix-named profile only fires on designs that follow that naming. Rename the nets and the
interface goes invisible, silently. `host` binding exists to stop that: a component that
declares `interface=CAN` anchors the check no matter what the nets are called. When the
naming is merely different rather than absent, a naming map is the cheaper fix.

## Where to go next

- [Checks and reports](../checks-and-reports/): profile findings read like any other, and
  `--fail-on` can gate on them.
- [Authoring an overlay](../../build/overlay/): shipping profiles alongside private readers
  and rules in your own module.
- [CLI reference](../cli-reference/): the `--profile-path` flag.
