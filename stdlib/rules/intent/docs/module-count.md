## module-count

### What it means

When the design intent declares an exact count for a module (2 CAN transceivers, 4 radios), this
rule fails if the number of design components matching that module's criterion differs. It is the
complement of `module-missing`, which asks "is at least one present" where this rule asks "are
there exactly N", so too few OR too many both fail.

### Why engineers want it

A dropped or duplicated channel is invisible to a presence check. A two-CAN design that lost one
transceiver in a schematic edit still "has a CAN transceiver", so `module-missing` passes; only a
count check catches the missing second channel. Over-count catches the opposite mistake (a
copy-paste that left a stray instance).

### Impact

The design has the wrong number of a required block, such as a missing redundant channel, a dropped
interface, or a duplicated part that doubles cost and load. It matches the declared architecture in
kind but not in quantity.

![A module declared with count 2 but only one present is flagged; two present is fine](images/module-count.svg)

### Scope note

Only modules that set a count are checked, so a declaration with modules but no counts compiles to
no count rule. Counting by MPN needs no `--params`, since every model joins the MPNs the design carries.
Like every intent rule, it takes the expectation from the declaration and never enumerates it from
the netlist.
