## part.audience

### What it is

`part.audience(mpn, who)` yields one row per team or license identifier entitled to see a part's
datasheet data, keyed by manufacturer part number (`mpn`). A part annotated for two teams emits two
rows; a part with no annotation emits none. The identifiers are free-form deployment tokens (a team
name, a license id) parsed from the PartSpec's `audience` attribute (comma-separated).

Like `param`, this is the datasheet tier: it is EMPTY without `--params`, and over `--speclib` it ranges
the whole seeded corpus (every part), where over a design it ranges only the parts joined to it.

It is RECORD-ONLY today (WS10-010). Nothing enforces it — a request for an un-entitled part is not
withheld yet. That gate (a ParamProvider that returns nil for an un-entitled caller, so rules stay
silent-by-construction) is WS10-011; this relation exists so the entitlement is captured on the data
and queryable in the meantime.

### For hardware engineers

Datasheet data is vendor-licensed: a shared spec library may hold parts your team is not licensed to see. This
relation records who each part's data is for, so you can ask the spec library "which parts is my team entitled
to?" or "who else can see this part?" before that entitlement is enforced anywhere. An unset audience
means the part was not annotated, NOT that no one may see it — until enforcement lands, unset is
visible to all.

### For software engineers

Think of it as a per-record ACL label with no reference monitor wired up yet: the `audience` field is
attached to the data, this relation projects it, and a future gate reads the same field to actually
allow/deny. It is a deliberate split — capture the policy metadata now (cheap, no proto change: it
rides the PartSpec `attributes` map), enforce it when there is more than one tenant to enforce against.
Keyed by `mpn`, so it joins the other datasheet relations (`param`, `component.mpn`) on the same
identity.

### Go projector

`audienceRows(mpn, spec)` in `check/facts.go` emits one row per entry of `param.Audience(spec)` (which
parses the `audience` attribute). It is projected design-scoped by `audienceFacts` (over the joined
specs, in `Facts`) and spec library-scoped by `SpecLibFacts` (over every spec the corpus holds), so the same rows
appear whether you query a design or `--speclib`. Record-only: the projector reads the annotation, nothing
consults it to withhold data.

### Datalog

    part.audience(?mpn, ?who)                        # every (part, entitled team) pair in scope
    part.audience("ACME-LDO", ?who) => ?who          # who may see one part
    part.audience(?mpn, "powertrain") => ?mpn        # every part a team is entitled to
