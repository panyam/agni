## board.layer

### What it is

`board.layer(net, layer)` yields one row per copper layer a net's routed tracks occupy. `net`
is the net name (the join key to `ir.Net.name`); `layer` is the layer word the board reader
carried through verbatim, normalized into the KiCad copper vocabulary (`F.Cu`, `B.Cu`, ...). A
net routed on the top and bottom copper of a two-layer board yields two rows; a net kept on one
layer yields one.

### For hardware engineers

This tells you where a net's copper actually runs. During a review you query it to confirm a net
stayed where you intended: a signal you expected to keep on an inner layer showing up on `F.Cu`,
or a rail you meant to plane on one layer appearing on several. Only layers carrying at least one
track segment for the net appear, so a net that is only a pad or a via with no routed track
contributes no layer row.

### For software engineers

Think of the routed board as a graph whose edges (track segments) each carry a layer label.
`board.layer` is the projection of that edge set down to its distinct layer labels per net: a
set-valued index from net to the layers it touches. Rows are 1:many with a net (one per distinct
layer), deduplicated and sorted, so the same layer never repeats for a net. An empty result means
either the design has no board tier at all (a netlist-only load) or no net has a routed track with
a layer.

### Go projector

`boardFacts` in `check/facts.go` walks `Model.BoardNets()` and, for each net, calls the helper
`netLayers(bn.Segments)`, which collects the distinct non-empty `Layer` values across the net's
track segments (deduplicated via a seen-set and sorted). It emits one `board.layer(net, layer)`
row per layer.

The board tier is EMPTY on a netlist-only design. `Model.BoardNets()` returns nothing unless the
design was loaded with board geometry (`NewModelWithBoard`, fed a `.kicad_pcb` or IPC-2581 board
sidecar). For a query this is silent-by-construction: `board.layer` yields zero rows on any design
without board geometry, the same posture the datasheet tier takes without `--params`. A query
returning nothing does not mean a net has no layers; it can mean the design carries no board at
all.

### Datalog

Every net and the copper layers it occupies:

```
board.layer(?n, ?layer) => ?n, ?layer
```

Which components sit on nets routed on the back copper (join `board.layer` to the netlist tier):

```
board.layer(?n, "B.Cu"), component-on-net(?r, ?n) => ?r, ?n
```

Both need a board-bearing design (a `.kicad_pcb` or an IPC-2581 file); on a netlist-only load they
return nothing.
