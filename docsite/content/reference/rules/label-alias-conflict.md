---
title: "label-alias-conflict"
description: "One net carries two different sheet-scoped labels in the same scope."
---

### What it means

After net solving, one net's collapsed alias list holds two or more
distinct sheet-scoped label names within a single sheet scope.

### Why engineers want it

Labels connect by name, so a wire carrying labels A and B pulls
everything named A and everything named B into ONE net — usually a leftover label after a
rename, or two nets drawn into accidental contact. KiCad's own ERC warns on it. The naming
pass picks one winner and the second name silently vanishes from the netlist.

### Impact

If the merge was unintended, this is a short circuit drawn with text. If it was
intended, every search, review note, and cross-reference under the losing name misses.

![One wire carrying two distinct label names is flagged; the same name on both halves is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/label-alias-conflict.svg)

### Scope note

Sheet-scoped names only (rank != 0), compared WITHIN one scope: a
hierarchy net legitimately carries one qualified name per sheet it crosses (/amp1/CTRL
joining the root is not a conflict), and a design-wide rail name plus a local nickname is
normal (the rival-rail case is power-tap-conflict). Semantics cross-checked against
kicad-cli sch erc.

### Query structure

select nets whose alias list clashes within a scope.

    select N in nets
      where exists scope : count(distinct name : (scope, name) in aliases(N), scoped(name)) >= 2

Reads: net.attributes. Tier R.
