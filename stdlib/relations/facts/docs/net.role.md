## net.role

### What it is

`net.role(net, role)` yields one row per ROLE a net carries: `rail`, `ground`, `feedback`,
`switching`, `control` or `gate_drive`. A net can hold several at once, so a net that is both a rail
by name and a switch node by suffix produces two rows.

It is the net-side twin of `component.class`. Roles are derived by the engine from the naming lexicon
at ingestion, not read from the design file; the attributes a file declares are `net.attr`.

### For hardware engineers

A role is what the engine thinks a net IS, worked out from what it is called. `12V_OUT` reads as a
rail. `12V_SW` reads as a rail by its prefix and a switch node by its suffix, and the second reading
is the one that matters, because a switch node is not a 12V supply.

Four of the six roles exist to say "this is named after a rail and is not one":

| role | typical names | why it is not a rail |
|---|---|---|
| `feedback` | `12V_FB` | the divider tap, at the regulator's internal reference |
| `switching` | `12V_SW`, `12V_BOOT` | the power-stage node, swinging to the input rail |
| `control` | `20V_EN`, `12V_MODE1` | a logic input, at the sequencer's level |
| `gate_drive` | `12V_VDRV` | the driver's own supply, not the converter's output |

The first two must never be probed. The last two are ordinary signals a test point is welcome on, and
are excluded from rail rules only because they are not rails.

The patterns behind each role are lexicon config a project extends in `conventions.yaml`.

### For software engineers

A tag set on a node, computed once at load. `net.role` is the whole set flattened to rows, so
`net.role(?n, ?r)` enumerates and `net.role(?n, "switching")` filters. An empty result means no net
name matched that vocabulary.

### Go projector

`netRoleFacts` in `stdlib/relations/facts.go` walks `Model.Nets()` and, for each token in
`classify.AllNetRoles()`, emits a row where `Model.HasAnyRole` holds. Going through `HasAnyRole` rather
than reading `ir.Net.roles` directly means a net that skipped the ingestion stamp still answers, via
the same name fallback every other role relation uses.

### Datalog

Every role on every net:

```
net.role(?n, ?r) => ?n, ?r
```

The nets that are named after a rail without being one:

```
net.role(?n, ?r), ?r != "rail", ?r != "ground" => ?n, ?r
```

### Not the same as `rail`

`rail(?n)` is `Model.IsPowerRail`, which holds for a net that is asserted-driven OR global OR a ground
OR carries the rail role. So `rail(?n)` is a CONCLUSION and `net.role(?n, "rail")` is what the lexicon
STAMPED, and the two are different sets on any real board. `feedback(?n)` and `switching(?n)` are
exact shorthands for their `net.role` rows and may be used interchangeably with them.
