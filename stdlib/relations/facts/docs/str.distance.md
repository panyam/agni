## str.distance

### What it is

`str.distance(string, other, distance)` binds the edit distance between two strings.

`str.distance` comes from jaala's standard library. The [Edit
distance](https://panyam.github.io/jaala/reference/builtins/#edit-distance) section of jaala's
built-ins reference says which arguments must be bound and what an absent or malformed input does,
and its examples run in jaala's own tests.

### For hardware engineers

Two nets whose names differ by one character are often one net misspelled, such as `SPI_MISO` and
`SPI_MIS0`. A schematic tool reads them as two nets and leaves both half-connected. A distance of 1
between two names on one board is worth a look, although `CANH` and `CANL` show that it is often
correct.

### For software engineers

It is the textbook Levenshtein distance over runes, run on one row of the table. Put a bound on it
with a comparison, as `?d <= 1`, the way a fuzzy search does.

### Datalog

Every pair of nets whose names are one edit apart, each pair once:

```
entity(?a, "net"), entity(?b, "net"), ?a < ?b, str.distance(?a, ?b, ?d), ?d <= 1 => ?a, ?b, ?d
```
