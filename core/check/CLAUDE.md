# core/check

This file covers the rule runtime, trace and walk, the pin-map rules, and the rail and connector helpers that rules share.

## Running rules

- **A rule's `Eval` takes the caller's context, and a run stops when the context is done** (agni 795). `check.Run`, `RunVerdicts` and `review.Run` return `ctx.Err()` and drop the findings so far, because a partial list would read as a clean remainder. `RunBackground` is the context-free form for tests and one-shot tools.
- **Every datalog evaluation reads its work budget off the same context** through `query.EvalOptions(ctx)` (agni 792). A new `Eval` call site that omits those options escapes a served budget silently.
- **`agni serve --query-budget` enforces a budget and is off by default.** `--query-budget-warn` logs each costly query with a suggested budget. A request's `work_budget` narrows the server's budget and never raises it. A stopped query returns `resource_exhausted`, and a stopped rule returns one inconclusive finding.
- **A missing registration blank-import empties the catalog.** Without `_ "github.com/panyam/agni/stdlib/rules/builtin"`, `check.BuiltinRules()` returns nothing, and a verdict sweep reports "0 pass, 0 fail, across 0 rules". Three of the four registration points fail this way. The root CLAUDE.md's package layout covers `agni.New`.
- **A rule proves a PASS with entities as well as prose**, through one of two mechanisms. A Go-walk requirement returns its hops, and `check.PullUpVerdict` emits every resistor and net as ordered context with a role. A query-backed requirement uses `Domain.Evidence` (agni 662).
- **`Domain.Evidence` is a third goal and not a wider domain.** It runs over the passing set, and its rows project through the rule's own `ContextVars`. The domain is the considered set, passing and failing alike. A failing subject has no proof to name, so one query cannot both list the failures and bind the evidence.
- **The ESD requirement's evidence is a second head, `esd_by`.** Widening `esd_ok` does not work, because the finding goal negates `esd_ok` and a negated atom must be unary.

## Trace and walk

- **`agni trace <design> --from U7.3 --to U12.4` follows a signal through the series parts between two pins.** It prints the route, the nets, and the probe points on each. `--render <file.svg>` draws the route on the design's own schematic, or on an auto-layout when there is none, and says which.
- **A trace endpoint's pin is a designator or the part type's functional name.** `check.LookupPin` resolves it, and the IO-map rules use the same lookup (agni 1193). An ambiguous or loose-only name is refused with its candidates.
- **`trace --server` mints a link that re-asks the question in the viewer.** The link carries the question and not a conclusion, so it needs no content hash.
- **Trace keeps three outcomes apart.** They are a route, no route within the radius, and an endpoint that names nothing in the design. Only the last exits non-zero, because a misspelled pin and two unconnected pins are opposite problems.
- **A trace answer says where each net and each endpoint is drawn**, per net and not one sheet for the whole answer (agni 657). A reader of a route across three sheets wants to choose.
- **A trace endpoint resolves by its placement, not by its net**, because it is a pin on a part. Resolving by net can pick a sheet that carries the middle net and none of the parts.
- **The drawn locations are filled on a no-route answer too**, because the two nets that fail to join are drawn somewhere. An empty field means drawn nowhere. The CLI takes the sheet off the response and does not choose one, so C32 holds there.
- **`agni walk <design> --from R1` walks outwards hop by hop** (agni 1339, `check.WalkFrom`, `DesignService.WalkDesign`). The viewer offers it from Properties into the Trace tab (agni 1341). Hop 1 is the start's own nets.
- **A walk kind picks which parts a hop crosses**, and `ListWalkKinds` lists the kinds. `series` crosses what trace crosses and passes through rail-named nets, so walk and trace agree. `small-parts` crosses parts with up to four connected pins and stops at ICs, connectors and rails. `all` caps a hop at 64 nets.
- **A walk crosses an IC by its pin pairs**, from the lexicon's `pin.through_pair` (`A{n}`/`B{n}`, `{n}A`/`{n}Y`, `SDA1`/`SDA2`, agni 1338). A translator then walks one channel and not every pin. An unpaired pin of a paired part crosses nothing, and only the crossed channel counts as crossed.
- **A part that crosses into a net the walk already reached goes in `also_entered`**, so a drawing of the walk can light it.
- **A walk takes a goal question** with `--goal pin|pull-up|rail` and `WalkGoal` on the request (agni 1340), asked from a reached net named by `--at`. Each route runs from the start along the walk to that net. It then continues through trace's series walk or the pull-up rule's own search, so a goal has no engine of its own. The answer keeps trace's three outcomes, and only an unresolved goal exits non-zero.

## The declared pin map

- **A declared pin map is an intent form, and it compiles to up to five rules.** `io_map` in a design's `intent:` section says which net connects to which pin of which device.
- **Three rules ask whether the design kept the map**: `io-map-pin-mismatch`, `io-map-net-absent` and `io-map-far-end`.
- **`io-map-coverage` takes the netlist as its considered set**, so it reports how much of the design the map never mentions. A map that declares 200 of 1600 nets leaves 1400 unexamined, and unexamined is not clean. An undeclared net is `not-considered` and never a fail.
- **Rails stay in the coverage denominator, and their reason says so.** Excusing them would have the tool decide which absences are acceptable.
- **`io-map-function` compiles when a row declares a `function`.** It checks the function against `Pin.alternates`, the functions in the part's pin-mux table (agni 667). The pin-mux table is a third recipe table kind, `MuxTableRule`, beside the parameter and pin-function tables.
- **`io-map-function` needs the datasheet tier.** A run with no corpus skips it, and the pin map's `function_check` column reads empty, not not-considered.
- **`check --verdicts --format csv --table pin_map` writes the map one row per pin.**

## Rails, connectors and intent

- **A connector's exposure to the field is declared in the design's intent and never classified** (agni 831), as `intent: components: {J1: {exposure: internal}}`. One part can be a module socket on one carrier and a cable entry on another, so `internal_connector` is no longer a class.
- **The exposure rules ask `Model.ExposedConnector`**, so intent reaches the model as well as the catalog. It travels as a value, `Overlay.DesignIntent`, through `ov.ReadOptions()` into `BuildModel`. A model built without the design's overlay checks every connector as exposed.
- **A request's intent (`--intent-path`) replaces the design's intent for that run.** There is no server-wide intent.
- **Only a request that sends its own convention replaces the server's `--conventions`.** A request carrying other config keeps the server's convention.
- **A surface that reads a profile without running it asks `Overlay.ProfileIndex`** (agni 833). The coverage panel and a review's interface-presence gate are two such surfaces. A project's profile supersedes the built-in profile of the same name.
- **A 0 Ohm link splits one rail into two nets.** A rule asking what sits on a rail calls `check.FirstOnRail` (agni 1015). It crosses fitted zero-ohm links and `net_tie` parts, and never a ground.
- **A rail-named net is not always a rail.** A regulator's pins take the name of the supply they produce, so `12V_FB`, `12V_SW`, `12V_MODE1` and `12V_VDRV` match the rail vocabulary and none carries 12V. Four roles mark them, and `Model.IsRailNet` subtracts them once for every consumer (agni 680).
- **`guide/naming-conventions.md` lists the six net vocabularies** and says when to narrow one. DECISIONS.md says why the two voltage relations are not exhaustive.
- **A connector on a net that the board drives is an outlet** (agni 1016). `check.BoardSource` finds the driver, and the power-entry rules mark the net not-considered. The rule pages describe each shape.
- **`check.BoardSource` accepts four driver shapes.** The first is a switch node behind an inductor plus a feedback net, found structurally because a vendor buck types every pin passive. The second is a `power_out` pin. The third is a part's switch pin behind the inductor with its feedback pin on the walk, read from pin names, because a fixed-output buck ties FB to its output (agni 1253). The fourth is a regulator fed from a net that none of the parts it sources drives, which `independentDriver` reads as a cascade and not a port.
- **A `power_out` pin does not count when the walk also reaches another power-converting part's input**, so a dual-role USB-C port is still judged. A boost converter's input also reaches its switch node, and the feedback requirement keeps that input judged.
- **A connector or a clamp is never the load** (agni 1257). `decoupling-present` skips a connector's own `power_in` pin. `hasPowerInput` skips a TVS's supply pin, so a USB clamp does not read as a load reached through passives alone.
