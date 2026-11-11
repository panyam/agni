## str.contains

### What it is

`str.contains(string, substring)` is true when `substring` appears anywhere in `string`.

`str.contains` comes from jaala's standard library. The [String
tests](https://panyam.github.io/jaala/reference/builtins/#string-tests) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

Net and part names carry a house convention, and a search by fragment finds what the convention
groups. `USB` in a net name finds both halves of the pair and the VBUS feed. `LM` in an MPN finds a
family of regulators. A negated test finds what is missing from a set, such as the parts on a USB
net whose own name does not say USB.

### For software engineers

`str.contains` is `strings.Contains` over two bound strings, exposed as a two-argument filter. It is
the `LIKE '%x%'` of the query language, without wildcards and without case folding. For a
case-insensitive search use `str.match(?s, "(?i)usb")`, or fold first with `str.lower`. For a
pattern use `str.match` or `str.glob`.

### Datalog

The parts on any net whose name holds "CAN":

```
component.net(?r, ?n), str.contains(?n, "CAN") => ?r, ?n
```

The parts whose MPN does not hold "LM":

```
component.mpn(?r, ?m), not str.contains(?m, "LM") => ?r, ?m
```
