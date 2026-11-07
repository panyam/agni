## net.group

### What it is

`net.group(group, kind, net)` yields one row per member net of each net group the design declares.
`group` is the group's name as the design tool wrote it. `kind` is `diff_pair` for a differential
pair and `match_group` for a match group. `net` is the member net. A design that declares no groups
yields no rows, and today only an Allegro board declares any.

### For hardware engineers

Say a layout engineer pairs `USB1_DP` with `USB1_DM` in the constraint manager, and puts a DDR byte
lane's eleven nets in one match group. The board records both groups. The router then couples the
pair and tunes the lane to one length. This relation reads those groups back, so a review asks about
the pairs the designer declared rather than the pairs a name happens to suggest.

A pair whose names follow no suffix convention still shows up here. The BeagleBone AI pairs
`DDR1_CLK0` with `DDR1_CLK0N` in a pair named `DIFFPAIR2`, and `SSTX1+` with `SSTX1-`. A net can sit in
a pair and a match group at once, as each DDR strobe pair sits in its byte lane.

### For software engineers

A projection of `ir.Design.net_groups`, one row per (group, net). It joins to everything keyed by net
name. `?net` is not unique, since a net in a pair and in a match group gives two rows. `?group` is not
unique either, since a group gives one row per member. A pair always has exactly two rows. The reader
leaves out a pair table that reaches some other number of nets.

The relation reports what the design states, never a guess. `diff-pair-naming` reads the same
declaration through `Model.DeclaredPair` before it falls back to the name vocabulary.

### Go projector

`netGroupFacts` in `stdlib/relations/facts.go`, over `Model.NetGroups`. Rows come in the reader's
order, which is sorted by group name and then by net. An empty result means the design declares no
groups. It does not mean the design has no differential signals.

### Datalog

The declared pairs, one row per pair with both members:

```
net.group(?pair, "diff_pair", ?a), net.group(?pair, "diff_pair", ?b), ?a < ?b => ?pair, ?a, ?b
```

The match groups and how many nets each holds:

```
net.group(?g, "match_group", ?n) => ?g, count(?n)
```
