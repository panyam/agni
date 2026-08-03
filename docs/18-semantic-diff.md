# 18 — Semantic diff

The diff engine (`diff/`) and the design behind it (roadmap WS2-001). It compares two IR
`Design`s and classifies what changed, on the neutral IR, so the *same* diff runs over any
format that reads into the IR (EDIF, KiCad, IPC-2581, ...). This is the review beachhead:
the value is not "files differ" but "*what* changed and how much it matters."

## Identity strategy

Matching is on **semantic keys**, never the format-native id (`Provenance.native_id` is
regenerated per export, matching on it would report everything as changed; see WS1-004):

- **Components** match by `ref_des`.
- **Nets** match by `name`.
- **Renames** (a net whose name changed) are recovered by connection **signature**: the
  canonical sorted set of `refdes.pin` a net touches.

## Change taxonomy

Mirrors the classes the engineers' own comparison tool uses (the
`netlist_comparison_*.xlsx` ground truth):

| Class | Meaning | Impact |
|---|---|---|
| Equal | net matched by name, identical connectivity + attributes | none (not reported) |
| **Soft** | matched, connectivity identical, only attributes differ (e.g. `net_class`) | cosmetic |
| **Hard** | matched, connectivity (pin membership) changed | electrical |
| **Renamed** | same connectivity, different name | rename, not delete+add |
| **New** / **Deleted** | present on only one side (and not a rename) | structural |

Components report Added / Removed / Changed (part-reference set + `Value`). Mapping to the
"cosmetic vs electrical vs constraint-violating" axis: Soft = cosmetic, Hard = electrical;
**constraint-violating is out of scope here**: that is the rules DSL (WS3) evaluating the
diff, not the diff itself.

## Rename detection

After matching by name, the leftover deleted and added nets are paired:

```
delBySig = { signature -> name }  for deleted nets, keeping only signatures that are
                                   non-empty AND unique within the deleted set
addBySig = same, for added nets
for each signature in delBySig that also appears in addBySig:
    -> Renamed{ from: delBySig[sig], to: addBySig[sig] }
leftover deleted -> Deleted ;  leftover added -> New
```

Uniqueness + non-empty are the safety rails: they stop two unrelated nets that happen to
share a connection set (or two empty nets) from being mis-paired as a rename. When a
signature is ambiguous, we decline to guess and report New/Deleted instead.

## Provenance-annotated findings

Every net finding carries `OldProv` / `NewProv`: the net's source locator
(`source_file`, `native_id`) in each revision (nil on the side where it does not exist).
This is what makes a finding *traceable*: "this net changed, here in the old file and here
in the new." The annotation is keyed by the semantic match, not the native id.

## Hard cases

- **Net rename**: handled (signature pairing).
- **Ref-des renumbering** (a component renamed, e.g. `R1`->`R5`, same part + connections): 
  the component analog of a net rename. Not yet detected; enumerated here as the next step.
- **Electrically-identical reroute** (same endpoints, different copper): invisible to a
  netlist diff by design: routing is geometry, not connectivity, so it does not appear.
- **Ref-des-less power/ground nets, bus/member pins**: their stability depends on the
  reader emitting stable keys (WS1-004); unstable keys would surface as false Hard changes.
- **Ambiguous rename signatures**: declined (reported as New/Deleted), never guessed.

## Validation

- Unit tests (`diff/diff_test.go`): a hand-built revision pair exercising Hard, Soft,
  Renamed, New, Deleted, Equal, and provenance.
- Manual: `agni diff` on the `ecc83` v1/v2 corpus pair (real Hard rewires).
- The EDIF `netlist_comparison_*.xlsx` is the eventual ground-truth oracle, pending an
  earlier EDIF revision `.edn` to diff against (WS6-001 open item).

## Follow-ups

- Provenance on component findings (nets only for now).
- Component rename (ref-des renumbering) detection.
- Validate against the `netlist_comparison` spreadsheet once a real EDIF pair exists.
