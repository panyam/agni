# Naming conventions

Your team probably has a house style for net and reference-designator names: power nets look
like `+3V3`, signal nets are `UPPER_SNAKE`, no ref des is reused. A conventions file writes
that style down once, as patterns, and `check` enforces it on every design. This is the
simplest way to add your own rules, and it needs no code.

## Write a conventions file

A conventions file is a small YAML document naming one or more rules. Each rule is a set of
allowed patterns and a set of exemptions:

```yaml
# example.yaml
name: example
rules:
  - name: signal-net-naming
    severity: info
    why: "signal nets are UPPER_SNAKE; rails and stubs are exempt"
    allow: ["^[A-Z][A-Z0-9_]*$"]
    exempt: ["^\\+", "^GND"]
```

- **`allow`**: a name is fine if it matches any of these patterns.
- **`exempt`**: names matching any of these are never checked (here, rails starting with
  `+`, and `GND`).
- **`name`** (top level): the namespace. The rule above appears in the catalog as
  `example/signal-net-naming`.

## Run it

Compose the file into a run with `--conventions`:

```
agni check design.edn --conventions example.yaml --rule example/signal-net-naming
```

If every name conforms, the rule runs and stays quiet:

```
no findings (1 rule(s) run)
```

Tighten the `allow` set and non-conforming names surface. For example, a convention that
only allows the exact name `SIG` reports everything else:

```
findings by rule:
  strict/sig-only        2

first 2:
  [info] strict/sig-only: VCC (net name matches no allowed naming pattern)
```

## Tool-generated names are exempt by default

CAD tools invent stub names for connections you never named: KiCad `unconnected-(...)`,
`Net-(...)`, gEDA/EDIF `N$…` autonames. These are never your house style and are **exempt
automatically**, so a convention only ever flags a name a human actually chose. You do not
have to list them in `exempt`.

## Scope of a match

By default a pattern matches the **leaf** of a qualified name. A hierarchical net like
`/amp2/CTRL` is matched on `CTRL`, so one convention works across every sheet without
encoding the hierarchy into the pattern.

## Where to go next

- [Checks and reports](checks-and-reports.md): conventions findings read like any other,
  and `--fail-on` can gate on them.
- [CLI reference](cli-reference.md): the `--conventions` flag.
