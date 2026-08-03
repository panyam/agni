## <relation-name>

<!--
WS14-005 per-relation fact doc template. Copy this file to facts/docs/<relation>.md, where
<relation> is the EXACT catalog name (e.g. net.bus_like, pin_net_conflict). The harness
(facts_docs_test.go) requires:
  - the file opens with "## <relation>" (its own name, matching the catalog),
  - <relation> is a registered relation (in query.Catalog / a Rel* const),
  - every images/<file> this doc references exists under facts/docs/images/.
Files whose name starts with "_" (like this one) are skipped by the harness.
The one-line Summary stays in query/catalog.go; this file is the richer Detail.
-->

### What it is

One or two sentences: what tuple the relation yields and what each argument means. Reference
the catalog signature, e.g. `net.bus_like(net)`.

### For hardware engineers

The electrical / schematic meaning, in the terms an EE thinks in. What real circuit condition
does a row describe, and when would you query for it during a review?

### For software engineers

The structural analogy (see ANALOGY.md): the relation as a projection/index over the design
graph. What it joins to, whether rows are 1:1 or 1:many with an entity, and any "absent means X"
semantics.

### Go projector

Name the projector func in check/facts.go and the Model read it materializes (the member method
it calls), so a reader can trace the fact back to its source. Note the row cardinality and what an
empty result means.

### Datalog

One or two runnable queries over this relation, copy-pasteable into `agni query` or the web panel.
Prefer a projection first, then a join that shows why the relation is useful.

```
<relation>(?a, ?b) => ?a
```

### Schematic (optional)

![alt text](images/<relation>.svg)

An SVG schematic card is encouraged where the relation has a visual condition (a topology, a
conflict, a fan-out). Same style as check/docs/images: a light card, fine-vs-flagged side by side,
valid XML with only the five built-in entities (no named entities like `&mdash;`). Omit for
relations whose meaning is purely tabular (e.g. component.mpn).
