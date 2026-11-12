# panyam/agni #1087 / #1088 — Telesis `$PACKAGES` footprint collision

## Primary repro (shows bug on v0.3.0)

Fixture: `same_footprint_multiline_refs.tel`

Footprint `FOOT_TP_SHARED` appears on two `$PACKAGES` lines with different MPNs. The first line assigns **both** `TP_A` and `TP_B`; the second line assigns `TP_C`.

### Expected MPN assignment

| Ref | Expected MPN |
|-----|----------------|
| `TP_A` | `MPN_SMALL_PAD` |
| `TP_B` | `MPN_LARGE_PAD` (second line’s MPN once pairing is defined) |
| `TP_C` | `MPN_LARGE_PAD` |

### Current behavior (v0.3.0)

```bash
agni query same_footprint_multiline_refs.tel 'component.mpn(?r,?m) => ?r, ?m'
```

| Ref | Actual MPN |
|-----|------------|
| `TP_A` | `MPN_SMALL_PAD` |
| `TP_B` | `MPN_SMALL_PAD` ← **bug** |
| `TP_C` | `MPN_LARGE_PAD` |

The reader maps footprint → first MPN for all refs that share the footprint key, instead of per line/ref pairing.

## Control (one ref per line)

Fixture: `duplicate_footprint_two_mpns.tel` — one ref per `$PACKAGES` line. Both MPNs assign correctly today; keep as regression guard after the fix.

## Non-goal

Do **not** match exporters that drop the **last** line of a multi-line `$PACKAGES` block entirely (legacy export bug; see #1097).
