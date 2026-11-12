# panyam/agni #1097 / #1098 — docs: legacy export vs semantic diff

These fixtures are **documentation/demo** inputs. agni should **not** replicate legacy export bugs; the docs issue is to explain discrepancies.

## 1. Letter-only pin names

Files: `letter_pin_rev_a.edn`, `letter_pin_rev_b.edn` (identical connectivity)

- Ref `D_K_ONLY`, pin `K` (letter-only), net `NET_CATHODE_SIDE`.
- **agni:** keeps `D_K_ONLY-K` on the net; diff between A/B is empty.
- **Legacy compare:** regex requiring a digit in pin names may drop `K`, producing fake new/deleted net rows.

## 2. Single-pin named net

File: `one_pin_net.tel`

- Net `STRAP_1PIN` connects only `R_STRAP.1` (second pin unconnected in `$NETS`).
- **agni:** lists the net.
- **Legacy export:** may omit one-pin nets “to match .tel”, shrinking net count.

## 3. EDIF escaped MPN

`edif_escaped_mpn.edn` spells the cell `PART&07`, which is not a legal EDIF identifier: `&` escapes
only an identifier that starts with a digit. `edif_renamed_part.edn` is the legal form the guide page
(`docsite/content/guide/older-comparison-tools.md`) runs, a stand-in identifier `PART_07` with the real
name in `rename` and the part number in a property.

File: `edif_escaped_mpn.edn`

- Cell/MPN token `PART&07` (EDIF escape for `PART-07`).
- **agni:** decode to `PART-07` in MPN fields.
- **Legacy:** may leave `PART&07` or `PART0707` style keys in parts tabs.

## Demo script

1. `agni diff` letter_pin A vs B → no semantic change.
2. `agni stats` on `one_pin_net.tel` → net count includes `STRAP_1PIN`.
3. Query MPN on `edif_escaped_mpn.edn` → decoded hyphenated MPN.

Contrast with legacy spreadsheet behavior in slides; link to #1087 for the **real** Telesis MPN bug (duplicate footprint), not truncated package lines.
