---
title: "Semantic diff"
description: "Comparing two design revisions at the level of meaning rather than text."
---

Two revisions of a schematic rarely differ in a way a text diff can explain. Every export
renumbers internal ids, reflows the file, and rewrites coordinates, so a line-by-line
comparison reports almost the whole file as changed. The diff engine works one level up, over
the neutral intermediate representation that both revisions are read into. It compares two IR
designs and classifies what changed. Because it runs on the neutral IR, the same comparison
works over any format that reads into it, whether EDIF, KiCad, or IPC-2581. The output says
what changed and how much it matters, not merely that the files differ.

## Identity strategy

Matching entities across revisions uses semantic keys, never the format-native id. A native id
is regenerated on every export, so matching on it would report the entire design as changed.

- Components match by reference designator, the `R5` or `U1` label on the part.
- Nets match by name.
- A net whose name changed is recovered by its connection signature, the canonical sorted set
  of `refdes.pin` endpoints the net touches. If the endpoints are the same, it is the same net
  under a new name.

The rename case is the familiar problem of a renamed variable. A textual diff sees a deletion
and an addition. Matching on what the variable connects to recovers the rename instead.

## Change taxonomy

The classification mirrors the categories a hand-run netlist comparison already uses.

| Class | Meaning | Impact |
|---|---|---|
| Equal | net matched by name, identical connectivity and attributes | none, not reported |
| **Soft** | matched, connectivity identical, only attributes differ (for example `net_class`) | cosmetic |
| **Hard** | matched, connectivity (pin membership) changed | electrical |
| **Renamed** | same connectivity, different name | a rename, not a delete plus add |
| **New** / **Deleted** | present on only one side, and not a rename | structural |

Components report Added, Removed, or Changed, where Changed covers the part-reference set and
the `Value` attribute. On the axis of cosmetic versus electrical, Soft is cosmetic and Hard is
electrical. Deciding whether a given change is acceptable is a separate step. That decision is a
[rule](../rules-and-checks/) evaluating the diff, not the diff itself.

## Rename detection

After matching by name, the leftover deleted and added nets are paired by connection signature.

```
delBySig = { signature -> name }  for deleted nets, keeping only signatures that are
                                   non-empty AND unique within the deleted set
addBySig = same, for added nets
for each signature in delBySig that also appears in addBySig:
    -> Renamed{ from: delBySig[sig], to: addBySig[sig] }
leftover deleted -> Deleted ;  leftover added -> New
```

Uniqueness and non-emptiness are the safety rails. They stop two unrelated nets that happen to
share a connection set, or two empty nets, from being mispaired as a rename. When a signature is
ambiguous the engine declines to guess and reports New and Deleted instead.

## Provenance-annotated findings

Every net finding carries the net's source locator in each revision, the `source_file` and
`native_id` on each side, nil on the side where the net does not exist. That is what makes a
finding traceable back to a place in each file: this net changed, here in the old file and here
in the new. The annotation is keyed by the semantic match, not by the native id.

## Hard cases

- Net rename is handled by the signature pairing above.
- Reference-designator renumbering, a component renamed from `R1` to `R5` with the same part and
  connections, is the component analogue of a net rename. It is not yet detected.
- An electrically identical reroute, the same endpoints wired through different copper, is
  invisible to a netlist diff by design. Routing is geometry, not connectivity, so it does not
  appear.
- Nets with no reference designator, such as power and ground, and bus or member pins, are
  stable only if the reader emits stable keys for them. Unstable keys would surface as false Hard
  changes.
- An ambiguous rename signature is reported as New and Deleted, never guessed.

## How it is checked

A hand-built revision pair in the unit tests exercises every class, Hard, Soft, Renamed, New,
Deleted, Equal, and the provenance annotation. Beyond that, `agni diff` is run on a real corpus
pair with genuine rewires to confirm the classification holds on files a tool actually produced.

## Not handled yet

- Provenance on component findings. Nets carry it today, components do not.
- Component rename detection, the reference-designator renumbering case above.
