---
title: "label-alias-conflict"
description: "One net carries two different sheet-scoped labels in the same scope."
---

### Remedy

Settle on one name for the wire and delete the other label. If the two names were meant to be two signals, the wire between them is what to cut.

### What it means

After net solving, one net's collapsed alias list holds two or more
distinct sheet-scoped label names within a single sheet scope.

### Why engineers want it

Labels connect by name, so a wire carrying labels A and B pulls
everything named A and everything named B into ONE net. The cause is usually a leftover label
after a rename, or two nets drawn into accidental contact. KiCad's own ERC warns on it. The naming
pass picks one winner and the second name silently vanishes from the netlist.

### Impact

If the merge was unintended, this is a short circuit drawn with text. If it was
intended, every search, review note, and cross-reference under the losing name misses.

![One wire carrying two distinct label names is flagged; the same name on both halves is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/label-alias-conflict.svg)

### Scope note

The rule compares sheet-scoped names only (rank != 0), and only WITHIN one scope. A hierarchy net
legitimately carries one qualified name per sheet it crosses, so /amp1/CTRL joining the root is not
a conflict. A design-wide rail name plus a local nickname is normal too, and power-tap-conflict
covers the rival-rail case. Semantics cross-checked against kicad-cli sch erc.

Two names also have to meet on their OWN sheet's wiring. A reused sheet often brings out two
hierarchical labels on separate wires, an address strap or a mode pin, and the parent sheet
configures one instance by wiring both pins to GND. That makes the two names one net, though no
sheet draws them together. A reused sheet is meant to be set up this way, so the rule does not
report it (agni issue 1249). On a hierarchical KiCad read the solver records where each group of
names met (`sheet_groups` on the net), and the rule compares only the names that met on the sheet
they belong to. A read of one sheet, or a format that solves no hierarchy, records nothing, and all
of a net's names count as having met.

### Query structure

select nets whose alias list clashes within a scope.

    select N in nets
      where exists group G in sheet_groups(N), scope = sheet(G) :
        count(distinct name in G : scoped(name), scope(name) = scope) >= 2

With no sheet_groups, the net's aliases are one group, and every scope is compared.

Reads: net.attributes. Tier R.
