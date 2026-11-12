---
title: "part.audience"
description: "a team/license entitled to see a part's datasheet data (record-only, needs datasheet parameters)"
---

### What it is

`part.audience(mpn, who)` yields one row per team or license identifier entitled to see a part's
datasheet data, keyed by manufacturer part number (`mpn`). A part annotated for two teams emits two
rows; a part with no annotation emits none. The identifiers are free-form deployment tokens, such as
a team name or a license id. They are parsed from the PartSpec's comma-separated `audience`
attribute.

Like `param.max`, this relation is in the datasheet tier, so it is EMPTY without `--params`. Over
`--speclib` it ranges the whole seeded corpus, every part. Over a design it ranges only the parts
joined to that design.

It is RECORD-ONLY today (WS10-010). Nothing enforces it, and a request for an un-entitled part is not
withheld yet. WS10-011 is that gate. It is a ParamProvider that returns nil for an un-entitled
caller, so rules are silent by construction. In the meantime this relation captures the entitlement
on the data and makes it queryable.

### For hardware engineers

Datasheet data is vendor-licensed, so a shared spec library may hold parts that your team is not
licensed to see. This relation records who each part's data is for. So you can ask the spec library
"which parts is my team entitled to?" or "who else can see this part?" before that entitlement is
enforced anywhere. An unset audience means the part was not annotated, NOT that no one may see it.
Until enforcement ships, an unset part is visible to all.

### For software engineers

It is a per-record ACL label with no reference monitor wired up yet. The `audience` field is
attached to the data, this relation projects it, and a future gate reads the same field to actually
allow/deny. The split is deliberate. The policy metadata is captured now. Capturing it is cheap and
needs no proto change, since it travels in the PartSpec `attributes` map. It is enforced once there
is more than one tenant to enforce against.
Keyed by `mpn`, so it joins the other datasheet relations (`param.max`, `component.mpn`) on the same
identity.

### Go projector

`audienceRows(mpn, spec)` in `stdlib/relations/facts.go` emits one row per entry of
`param.Audience(spec)`, which parses the `audience` attribute. `audienceFacts` projects it
design-scoped, over the joined specs, in `Facts`. `SpecLibFacts` projects it spec-library-scoped,
over every spec the corpus holds. So the same rows appear whether you query a design or
`--speclib`. It is record-only, since the projector reads the annotation and nothing consults it to
withhold data.

### Datalog

    part.audience(?mpn, ?who)                        # every (part, entitled team) pair in scope
    part.audience("ACME-LDO", ?who) => ?who          # who may see one part
    part.audience(?mpn, "powertrain") => ?mpn        # every part a team is entitled to
