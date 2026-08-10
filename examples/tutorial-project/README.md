# Tutorial project

The worked example the tutorial track is built on: one synthetic board and the project files a team
wraps around it. Every part, MPN, vendor, and datasheet value here is invented, so the whole folder
is redistributable and nothing on it is anyone's real design.

## Try it

```
make review
```

That runs the checklist in `review.yaml` over the bundled board with every tier this project
supplies, and prints one outcome per item. `make help` lists the rest.

## Layout

This is the shape a real review project takes. Two kinds of file live here, and the split matters.

**Per-project**, shared by every design:

| Path | What it is |
|---|---|
| `review.yaml` | the checklist: the questions this team asks of every board |
| `conventions.yaml` | house naming: which net names are rails, and what a legal name looks like |
| `profiles/` | interface declarations, one per bus this team designs with |
| `params/` | datasheet parameter sets, one per part worth checking against its limits |

**Per-design**, one set per folder under `designs/`:

| Path | What it is |
|---|---|
| `design.yaml` | the design's name and which file is its entry point |
| `gateway.edn` | the netlist, and `gateway-rev-b.edn` a later revision of it |
| `intent.yaml` | what this board is supposed to be: declared domains, modules, subsystems |
| `reports/<design>/` | the outcome of a run, written by `make report` |

Intent is per-design because each board has its own intended architecture. Conventions, profiles,
and parameters are per-project because they describe the team, not the board.

## What the board is built to show

The board is deliberately imperfect. Each flaw exists so some part of the tool has something true
to report rather than a contrived one.

- **Rails are named function-first** (`PMIC_CORE_3V3`), which the built-in rail vocabulary does not
  match. Without `conventions.yaml` the only rail on the board is `GND`. That is what the lexicon
  half of a conventions file is for, and running `agni check` with and without it shows the
  difference.
- **CAN is present, complete, and terminated, but has no ESD part on the pair.** So the built-in
  CAN profile has exactly one real finding. `profiles/can.yaml` then supersedes that built-in and
  adds a standby signal this project always routes, which the board does not, so a second finding
  appears when the overlay profile loads.
- **LIN is absent entirely.** Its checklist item must not score a pass just because a check for a
  bus that is not there found nothing to complain about.
- **U2's datasheet parameters are a mock placeholder**, not a transcription. Its finding reports as
  `provisional` rather than `fail`, which is the engine declining to call a defect on data nobody
  has checked. `params/acme-buck-3v3.textproto` is the seeded-and-trusted contrast.
- **One intent declaration is deliberately wrong.** The core domain is declared at 3.3 V while its
  rail is an actual 1.8 V rail, so the intent check has a real deviation to find.
- **The entry file is a netlist, which carries no copper.** The board-tier item reads `n/a`, which
  means the question does not apply to what was loaded. That is not the same as a pass.

## A note on the netlist

Two things about the EDIF are load-bearing, and both are easy to get wrong when writing one by hand.

Each part cell carries its own `(designator "<prefix>")` **before** the view. The reader takes the
first designator it finds anywhere in the cell, so a cell without one picks up a *port's* designator
instead. Every component of that type then classifies as unknown, and every rule that quantifies
over a device class silently finds nothing.

A `portRef` names a port by its **designator**, not by the port's name. Use the name and the
connection never joins to the pin, so pin-level rules see an unconnected part.
