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
