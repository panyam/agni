# Decisions — settled questions and why

Questions that were asked, answered, and should not be re-litigated without new evidence. One
section each.

This file exists because these kept being filed as deferred work. They are not: there is nothing to
pick up, so they can never be closed, and a to-do list that contains them slowly fills with entries
nobody can act on. They are also not constraints — `CONSTRAINTS.md` holds *enforceable* rules with a
Verify, and diluting it with unenforceable prose would weaken the thing that makes it useful.

What belongs here: a question someone will otherwise ask again, whose answer cost real investigation,
and where the answer is "no" or "not that way". What does not: anything actionable (a ticket),
anything enforceable (`CONSTRAINTS.md`), and anything waiting on a trigger (`OUT_OF_SCOPE.md`).

Reopening one is fine. Doing it without reading why it was closed is not.

---

## Netclass consistency is a house-style rule, not a catalog rule

**Question.** Should "one net, two netclasses" ship as a default-on catalog rule? It was the second
half of the netclass work and looks like an obvious defect check.

**Answer. No, and it would be wrong.** Multi-membership is *legitimate* in KiCad: it unions every
match, then cascades per-class values by priority. It is merely *discouraged* in Altium, whose
clearance matrix errors on it. So two netclasses on one net is a house-style question, not a
universal defect, and shipping it default-on would fire on correct KiCad projects.

**What this leaves open.** A house or profile that genuinely wants deterministic single-class
membership can have it — authored as a PROFILE-scoped or overlay rule, never a default-on catalog
one. The engine half is already done: `net.netclass` is 1:many, so the rule is expressible as a
self-join today.

**Reopen if** a format appears where multi-membership is genuinely invalid rather than discouraged.
That would be a fact about that format, and belongs in its reader, not in a universal rule.

---

## `power-input-not-driven` staying off for EDIF is the permanent answer

**Question.** Should EDIF get a symmetric power-OUTPUT stamp, so `power-input-not-driven` can run
there the way it does elsewhere? The gate (`design.types_power_out`) looks like a temporary
limitation.

**Answer. No. The gate is correct permanent behaviour.** The rule catches a rail that is drawn but
never sourced. On a real EDIF export — already through layout and native ERC — that is essentially
never genuine: every firing observed on real hardware was false, with the source present but
under-typed. The one place it could catch something, schematic-stage EDIF, has exactly the same
under-typing that makes it unreliable.

The underlying reason is not fixable by more stamping: **EDIF genuinely cannot distinguish
"undriven" from "source-under-typed."** A rule that cannot tell those apart should not be the one
telling you a rail is dead.

**What was gained anyway.** The value of that work landed in the two power-in-only rules
(`decoupling-present`, `input-protection`) and the datasheet supply-pin rules, none of which need the
output side.

**Reopen if** undriven-rail detection on EDIF is genuinely needed. Even then, do not stamp
`POWER_OUT` by name — `VOUT` is also a signal name. The cheaper and more honest path is device-class
SOURCE recognition: a regulator, PMIC, or switch output on the rail implies driven. Then the gate can
drop.

---

## The engine does not carry an `owner` axis

**Question.** A checklist item can be blocked on a person rather than on data. Should the engine
carry that, so a review can report "waiting on you"?

**Answer. No.** `owner` — who unblocks an item — is customer and overlay workflow metadata. Carrying
it in the shareable engine crosses the open-core boundary this project keeps against market and
strategy material. A `review.Item.Owner` was considered specifically and rejected on that basis.

**What the engine can do instead.** It can prove "blocked" only from an engine-visible signal, and
each concrete blocker has one:

| blocker | engine-visible signal | outcome |
|---|---|---|
| design intent | `--intent-path` not loaded | `needs-design-intent` |
| a datasheet value | `param_symbol` present, unseeded in the model | `needs-data` |
| a naming vocabulary | was the `--conventions` source loaded | `needs-config` (not yet derived) |

The pattern generalises to any blocker the engine can see, and to none that it cannot.

**The genuine residue.** A human asserting an item is satisfied, with no design, binding, or config
evidence at all, should stay overlay metadata and read plain `not-automated` from the engine. The
overlay's own checklist rollup carries the `owner` nuance, which is where it belongs.

**Care when extending this.** `needs-data` and `needs-config` count as COVERED in the tally, so a
mis-derived blocker moves the coverage number a team reads. Deriving the next one is real work and is
tracked as its own issue.

---

## Datasheet units are converted at READ time, in one table, and not shared with `core/classify`

**Question.** Three questions that arrive together whenever someone meets `datasheet/param/unit.go`.
Why convert units at all, when the whole parameter layer's posture was that unlike units are
under-specified? Why convert when an extractor READS a row rather than normalizing once when a spec
is SEEDED, which is what C20's left-shift rule would suggest? And why is there a second unit table
when `core/classify` already has one?

**Answer to the first: because refusing was not neutral.** The premise was right, that a silent scale
factor inside a pass/fail rule is where a unit bug hides. The conclusion was wrong. Refusing to
convert did not remove the risk, it relocated it from "wrong number" to "no check at all", and no
check at all is the failure this codebase treats as most serious. An extractor that dropped a
millivolt row left its rule comparing nothing, and a rule that reports nothing scores a **pass**.
Five rule families were silently passing designs with real defects (agni issue 148). What makes
conversion safe is location: one table beside `UnderSpecified` and `MachineComparable`, every
extractor reading through it, no rule containing a number.

**Answer to the second: `RangeValue` has no `input` field.** Normalizing at ingestion is safe for
`ir.Quantity` precisely BECAUSE it keeps the source text in `input`, so the normalization is
non-lossy and a wrong reading can be audited against what the source actually said. A `PartSpec`
parameter has no such field, so normalizing on load would destroy the as-printed property that lets
a seeded row be checked against its datasheet page by eye, and would silently change what the params
panel and the `param` relations display. Converting at read time reproduces Quantity's split without
a schema change: the spec plays `input`, the extractor's return value plays `value`.

**Answer to the third: the two notations disagree on the character that matters most.**
`core/classify` parses a component's value text off a design, under IEC 60062's RKM code, where `M`
is MEGA and case is not significant. A printed unit symbol is the opposite: `m` is milli, `M` is
mega, and `mΩ` and `MΩ` differ by nine orders of magnitude. Sharing the table would import a decision
that is correct on the design side and inverts three orders of magnitude on the datasheet side. It
would also be the first `datasheet/` to `core/` import, which C17's layering does not have.
`TestUnitVocabulariesAgree` in `core/check` (the one package importing both) holds them to the same
canonical base spellings, so the part that could genuinely drift is pinned without an import.

**What this does NOT settle.** The parameter ONTOLOGY is still absent and still wanted:
`canonical_id` stays empty and `symbol` is still matched through per-corpus alias maps in the model
layer. Units are separable from that work and were done first because the SI prefixes are specified
and vendor-independent, where parameter names are neither.

**Reopen if** a unit turns up whose scale is genuinely context-dependent, or if the ontology work
subsumes the table. Do not reopen to merge the two tables without first re-reading why `M` means
different things on the two sides.
