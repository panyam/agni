# Synthetic reproducers for panyam/agni #1085–#1098

Minimal netlists for issues filed from the Checksmith → agni migration work. Names are deliberately generic (no customer boards). Each subfolder has an `EXPECTED.md` with **current** vs **desired** behavior and commands to verify.

| Issue | Topic | Fixture |
|------:|-------|---------|
| [#1085](https://github.com/panyam/agni/issues/1085) | Hierarchical EDIF (detailed) | `1085-hierarchical-edif/` |
| [#1086](https://github.com/panyam/agni/issues/1086) | Hierarchical EDIF (minimal) | same as #1085 |
| [#1087](https://github.com/panyam/agni/issues/1087) | Telesis duplicate footprint MPN | `1087-telesis-duplicate-footprint/` (`same_footprint_multiline_refs.tel`) |
| [#1088](https://github.com/panyam/agni/issues/1088) | Telesis MPN (minimal) | same as #1087 |
| [#1089](https://github.com/panyam/agni/issues/1089) | Pull-up hop limit (4 hops) | `1089-pullup-four-hops/` |
| [#1090](https://github.com/panyam/agni/issues/1090) | Pull-up hops (minimal) | same as #1089 |
| [#1091](https://github.com/panyam/agni/issues/1091) | All pull-up paths in evidence | `1091-pullup-parallel-paths/` |
| [#1092](https://github.com/panyam/agni/issues/1092) | Pull-up paths (minimal) | same as #1091 |
| [#1093](https://github.com/panyam/agni/issues/1093) | Tabulate interface coverage | `1093-interface-coverage/` |
| [#1094](https://github.com/panyam/agni/issues/1094) | Tabulate (minimal) | same as #1093 |
| [#1095](https://github.com/panyam/agni/issues/1095) | Query count ratios | `1095-gnd-tp-ratio/` |
| [#1096](https://github.com/panyam/agni/issues/1096) | Query ratios (minimal) | same as #1095 |
| [#1097](https://github.com/panyam/agni/issues/1097) | Docs: legacy export vs semantic diff | `1097-legacy-export-quirks/` |
| [#1098](https://github.com/panyam/agni/issues/1098) | Docs (minimal) | same as #1097 |

Issue map (filed batch):

| # | Topic |
|---|--------|
| 1085–1086 | Hierarchical EDIF |
| 1087–1088 | Telesis MPN |
| 1089–1090 | Pull-up hop limit |
| 1091–1092 | All pull-up paths |
| 1093–1094 | Tabulate interface coverage |
| 1095–1096 | Query count ratios |
| 1097–1098 | Docs: legacy export quirks |

Older references (#718, #835, #820, #821) are the same themes as #1085–#1092.

## Verify with agni CLI

From an agni checkout (v0.3.0+):

```bash
agni stats --as-named path/to/fixture.edn
agni stats --as-named path/to/fixture.tel
```

Optional full pipeline:

```bash
agni trial path/to/fixture.edn
```

For Telesis MPN (#1087), after ingest:

```bash
agni query 'component.mpn(?c, ?m) => ?c, ?m' --snapshot … --limit 20
```

(Exact CLI flags match your installed `agni query --help`.)

## Upstreaming

These are sized for GitHub issue threads. Good homes in agni upstream:

- `readers/edif/testdata/` for `.edn` reader cases (see existing `hier.edn`)
- `cmd/agni/testdata/conformance/` for rule/query fixtures
