---
name: verify-lexicon
description: Check that agni reads a board's names the way the board's designer means them. Walks the effective naming lexicon, asks why chosen nets have their roles, compares do-not-populate, part-number and class coverage against the BOM, reads the rail-not-classified findings, and dry-runs a conventions fix with --try before anyone commits it. Use when someone says "verify the lexicon", "does agni read our names right", "check the naming conventions on this board", "why is this net a rail", or is trying agni on a new board.
---

# verify-lexicon

agni decides what a net or a part is from its name, through a lexicon of patterns and keys. The
built-in lexicon is a guess about common naming, so on a new board the question is whether that
guess matches how this board's designer named things. The person running this skill knows the
board (its BOM, which nets are rails, which parts are fitted), and the skill's job is to put agni's
reading next to that knowledge, one claim at a time, so a disagreement shows up as something
specific.

Work through the steps in order. Each one is cheap and the later ones assume the earlier ones.

## Before starting

- Ask for the design path (a folder holding `design.yaml`, or the entry file) and call it `$B`
  below. Use `agni` from PATH, and run `agni --version` once so the report names the build.
- Ask for the BOM, or at least its line count, its do-not-populate list and how part numbers are
  recorded. Steps 3 and 5 compare against it.
- Never paste net names, ref-des, part numbers or the lexicon output into an issue, a chat
  outside this workspace, or a public repo. A board's names are its owner's design data. Report
  back counts and the general shape of a mismatch, such as "rails named with a `P` prefix are not
  read as rails".

## 1. Read the lexicon against the board

```
agni lexicon $B -o /tmp/lexicon.txt
```

The file lists every vocabulary (net roles, pin names, classes, value units, attribute keys) with
each pattern and where it came from (`built-in`, the project, the design, or a conventions file).
Go through it with the user and look for:

- rail patterns that miss how this board names its supplies (`P3V3`, `VDD_1V8_SYS`, `+5V_USB`)
- differential-pair suffixes that differ from the ones in the schematic
- class ref-des prefixes that do not match the board's scheme
- `attribute.mpn`, `attribute.value` and `attribute.dnp` keys that do not match the field names on
  the schematic's symbols

A `built-in` entry that does not fit is a gap in the convention, not a defect in the board.

## 2. Ask about the nets the user knows best

Pick five or six nets with the user and explain each one:

```
agni lexicon $B --net <NET>
```

Choose them to cover the hard cases:

- a main rail, which should be a rail with a reason naming its supply word and a supply pin
- a regulator's feedback or switch-node net (`*_FB`, `*_SW`), which should NOT be a rail even
  though its name carries a voltage
- one half of a differential pair
- a no-connect net
- a net whose name is ambiguous on this board

Each answer gives the roles, the reason for each, and every pattern the name matched. Judge the
reason as well as the role. A right role reached for the wrong reason breaks on the next board.

## 3. Compare part coverage against the BOM

```
agni query $B 'component.net_count(?r, _) => count(distinct ?r)'
agni query $B 'component.dnp(?r) => ?r'
agni query $B 'component.mpn(?r, _) => count(distinct ?r)'
agni query $B 'component.net_count(?r, _), not component.mpn(?r, _) => ?r'
agni query $B 'component.class(?r, ?c) => ?c, count(distinct ?r)'
agni query $B 'component.net_count(?r, _), not component.class(?r, _) => ?r'
```

- The first is the part count. It should match the BOM's placed-part count.
- `component.dnp` should list exactly the parts marked do-not-populate in the BOM. A missing one
  usually means the schematic records it under a field name missing from `attribute.dnp`, or with
  a value missing from `attribute.dnp_values`.
- Parts with no `component.mpn` should be the BOM lines that genuinely have no part number. Extra
  ones point at a part-number field missing from `attribute.mpn`.
- The class tally should look like the board. Unclassified parts are either a class pattern or
  prefix worth adding, or parts that really are nothing in particular (mounting holes, logos, test
  pads). Sort them into those two groups with the user.

## 4. Read the lexicon's own complaints

```
agni check $B --rule rail-not-classified
```

Each finding is a net with a voltage in its name. The net feeds supply pins but got no rail role,
so every rail rule skips it. For each one decide with the user whether it is a rail the
vocabulary misses (a pattern to add in step 5) or a name that only looks like a rail (leave it, and
note why).

## 5. Dry-run a fix before committing it

Write the missing patterns or keys into a conventions YAML holding only the additions. A list
without `replace: true` is added to the built-in one, so this adds one rail pattern and keeps every
built-in:

```yaml
name: board-fixes
lexicon:
  net:
    rail:
      patterns: ["^P[0-9]+V[0-9]+$"]
  attribute:
    dnp:
      keys: ["Fitted"]
```

`agni lexicon $B --format yaml` prints the whole effective lexicon in the same shape. Look up a
list's path there. Do not trim that output into a fix file, because it writes every
list with `replace: true`, and a trimmed list with that flag throws the built-ins away. Then
compare:

```
agni lexicon $B --try fix.yaml
```

It lists every net whose roles change and every finding that appears or disappears. Check that only
the intended nets and findings moved. A finding appearing somewhere unexpected means a pattern is
broader than intended, so narrow it and run `--try` again. Once the dry run shows exactly the
intended change, the YAML goes under `conventions:` in the project's `project.yaml` (or the
design's `design.yaml` for a board-only convention).

## 6. Optionally, look at it in the viewer

```
agni open $B
```

The Lexicon panel shows the same entries beside the drawing, can edit them, and runs the same
`--try` comparison. Clicking a net shows its roles where it is drawn. A name can read fine as text
and be plainly wrong on the page, and the drawing shows that.

## Report

Finish with a short summary for the user to act on. It holds no design names:

- the agni version
- part count, DNP, part-number and class coverage against the BOM (counts, and how many differ)
- how many of the nets explained in step 2 had the right role for the right reason
- how many `rail-not-classified` findings were real misses
- the conventions entries added and confirmed with `--try`, described by shape
- anything the lexicon could not express; each one is worth an agni issue described in general
  terms
