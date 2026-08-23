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
ambiguous this pass declines to guess and leaves the nets for the pass below.

### Near matches, off by default

Exact pairing has a cliff. The revision that renames a net is very often the revision that changes
it slightly, because a rename lands alongside a decoupling cap added, a test point dropped, or a
series resistor inserted. One endpoint moves, the signature no longer matches, and the rename you
most needed to survive is the one reported as an unrelated deletion beside an unrelated addition.

`--rename-approx` adds a third pass over what the exact pass could not place. It is a separate
ranked assignment rather than a loosened version of the exact pass, and the ordering is what keeps
a rename with no connectivity change reporting as an exact `renamed` rather than as a guess.

```
for each leftover deleted net with enough SIGNIFICANT endpoints:
    candidates = new nets sharing at least ceil(min_old_coverage_significant * n) of them
    score each candidate, rejecting any that fails a threshold
rank every surviving pair, assign best-first, one-to-one, no reuse
    -> RenamedApprox{ from, to, added, removed, evidence }
```

Endpoints are not equal. A test point coming or going is routine churn and should not cost a net
its identity, while a device pin moving should, so the thresholds come in significant and
all-endpoint pairs. Significance is read from `Component.device_classes`, the normalized set
stamped at ingestion, rather than from how a ref des is spelled.

The pass is **off by default** because it ASSIGNS a best match among candidates rather than
recovering a fact. A wrong pairing claims a net kept its identity across a revision when it did
not, and every downstream reading inherits that claim, so a `renamed-approx` is a distinct kind
that a consumer can filter on and it carries the arithmetic that produced it. A reader has to be
able to disagree.

| knob | default | what moving it trades |
|---|---|---|
| `MinOldCoverage` | 0.70 | fraction of the old net's endpoints that must survive. Lower catches heavier rewires and starts pairing nets that merely overlap. |
| `MinOldCoverageSignificant` | 0.80 | the same over significant endpoints. The one doing most of the work, because probe churn cannot move it. |
| `MinNewCoverage` | 0.35 | fraction of the new net made of old endpoints. Guards a large net swallowing a small one. |
| `MinNewCoverageSignificant` | 0.60 | that guard over significant endpoints. |
| `MaxAddedSignificantFloor` | 2 | how much a net may grow and still read as itself, when half its old significant count is smaller. The floor lets a two-endpoint net become four. |
| `MinSignificantEndpoints` | 2 | below this a net has no shape to match on. |
| `InsignificantClasses` | `[test_point]` | which device classes are excluded from the overlap arithmetic. |

These numbers are a calibrated starting point rather than a proven one. They are the settled values
of a netlist comparison tool that has run against real revision pairs for years, and both failure
directions were observed while arriving at them: looser values mis-paired unrelated power rails,
and tighter values missed obvious renames where one decoupling capacitor had been added or removed.
Produce a precision number on a revision pair that matters before trusting the pass on it, per
[evidence](../../build/evidence/).

One seam is worth knowing. `MinNewCoverage` counts ALL endpoints, so a small net that gains enough
test points can fall under it even though every significant threshold is satisfied. On a two-endpoint
net, three added probes pair and four do not. The significant thresholds insulate the pass from probe
churn and this one does not.

## Provenance-annotated findings

Every net finding carries the net's source locator in each revision, the `source_file` and
`native_id` on each side, nil on the side where the net does not exist. That is what makes a
finding traceable back to a place in each file: this net changed, here in the old file and here
in the new. The annotation is keyed by the semantic match, not by the native id.

## Hard cases

- Net rename is handled by the signature pairing above, and a rename that also changed by the
  opt-in near-match pass beside it.
- Reference-designator renumbering, a component renamed from `R1` to `R5` with the same part and
  connections, is the component analogue of a net rename. It is not yet detected.
- An electrically identical reroute, the same endpoints wired through different copper, is
  invisible to a netlist diff by design. Routing is geometry, not connectivity, so it does not
  appear.
- Nets with no reference designator, such as power and ground, and bus or member pins, are
  stable only if the reader emits stable keys for them. Unstable keys would surface as false Hard
  changes.
- An ambiguous rename signature is reported as New and Deleted by the exact pass, never guessed.
  With `--rename-approx` it may instead be assigned, as `renamed-approx` and with its evidence
  attached, which is a different claim and a separately filterable one.

## How it is checked

A hand-built revision pair in the unit tests exercises every class, Hard, Soft, Renamed, New,
Deleted, Equal, and the provenance annotation. Beyond that, `agni diff` is run on a real corpus
pair with genuine rewires to confirm the classification holds on files a tool actually produced.

## Not handled yet

- Provenance on component findings. Nets carry it today, components do not.
- Component rename detection, the reference-designator renumbering case above. It is the same
  scoring problem with a different key, so the near-match pass is worth generalising over "entity
  with a signature" when a second caller appears rather than reimplementing.
- Near-match thresholds are a value passed to the engine and reachable from the CLI, but no project
  config tier carries them yet, so a house cannot declare its own and have every run pick them up.
