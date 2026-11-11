---
title: "str.contains"
description: "reports whether a string contains a substring"
---

### What it is

`str.contains(string, substring)` is true when `substring` appears anywhere in `string`. The first
argument is the text under test, and the second is the piece to look for. The order matters.
`str.contains(?net, "USB")` asks whether a net name holds "USB". `str.contains("USB", ?net)` asks
the reverse.

The match is exact and case-sensitive, so `"usb"` does not match `USB_DP`. An empty substring
matches every string. `str.contains` is a computed predicate (kind `predicate` in the catalog), not
a stored fact. It is a filter, so it tests a value and never produces one. Both arguments must be
bound before it runs, so put a relation that binds the variable earlier in the body.

### For hardware engineers

Net and part names carry a house convention, and a search by fragment finds what the convention
groups. `USB` in a net name finds both halves of the pair and the VBUS feed. `LM` in an MPN finds a
family of regulators. A negated test finds what is missing from a set, such as the parts on a USB net
whose own name does not say USB.

### For software engineers

`str.contains` is `strings.Contains` over two bound strings, exposed as a two-argument filter. It
is the `LIKE '%x%'` of the query language, without wildcards and without case folding. For a
case-insensitive search use `str.match(?s, "(?i)usb")`, or fold first with `str.lower`. For a
pattern use `str.match` or `str.glob`.

### Go projector

`str.contains` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The
Datalog engine's standard library registers it, together with `str.prefix` and `str.suffix`, in
`Strings` in jaala's `stdlib/strings.go`. `core/facts/registry.go` installs that library into the
vocabulary every query uses. Calling it with an unbound argument is an evaluation error rather than
an empty answer.

### Datalog

The parts on any net whose name holds "USB":

```
component.net(?r, ?n), str.contains(?n, "USB") => ?r, ?n
```

The parts whose MPN does not hold "LM":

```
component.mpn(?r, ?m), not str.contains(?m, "LM") => ?r, ?m
```
