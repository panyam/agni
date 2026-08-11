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

---

## The query engine does not implement three-valued logic

**Question.** `query.Value` now carries `Absent`, so a field the source never stated is distinguishable
from one stated as the empty string. SQL's answer to the same problem is `NULL` plus three-valued
logic: a comparison involving NULL is neither true nor false but UNKNOWN, and UNKNOWN propagates.
Should this engine do that? It is the well-trodden answer, and `absent = absent` being TRUE here is a
visible deviation from it.

**Answer. No, and the deviation is deliberate.** UNKNOWN is not a third result you can add to
comparisons alone. It has to thread through negation (what does `not R(?x)` mean when `R` holds
UNKNOWN for `?x`?), through aggregation and grouping, through the fact index, and through the
projection that renders a row. Every one of those is a semantic decision with its own compatibility
question, and the total is a rewrite of the evaluator's core rather than a feature.

What it buys, for this engine's actual users, is close to nothing. `absent = absent` under SQL rules
is UNKNOWN, so `param.range(?a, ?s, ?k, ?min1, ?_), param.range(?b, ?s, ?k, ?min2, ?_), ?min1 = ?min2`
would not match two parts that both state no minimum. "Both unstated" is precisely the answer an
engineer running that search wants. The SQL reading is correct for a database that must not conflate
"unknown value" with "no value", and this layer only ever has the second.

**What was taken instead.** The two places where treating absence as a value would give a WRONG answer
are closed directly: an absent operand never participates in an ORDERING comparison (there is no order
between "unstated" and 5), and an absent value indexes under its own bucket so it cannot collide with
a stated empty string. Absence is queryable through `absent(?x)`. Equality is identity, and identity
is total.

**Reopen if** a case turns up where `not R(...)` or an aggregate genuinely needs to distinguish "no
row" from "a row whose field is unstated" and cannot express it with `absent`. That would be evidence
the two-valued reading is losing information, which is the only argument that should move this.
`TestAbsentEqualsAbsentDeviatesFromSQL` is named for the deviation so it fails loudly rather than
being quietly "corrected" toward SQL by someone who has not read this.

---

## The served design loader reads the ref it is given; the viewer resolves and says so

**Question.** The CLI resolves a design's declared entry: point it at a board companion and it reads
the netlist the descriptor names, printing a line saying it did. `serve`'s loader does not — it reads
exactly the artifact the request named. Should it resolve too, so the two surfaces behave alike?

**Answer. No, and the asymmetry is the point.** The CLI's redirect is safe BECAUSE it can print a
line. A browser has no equivalent: a user picks a file in a tree, and a loader that quietly swapped in
a different one would leave them looking at something they did not choose with nothing on screen to
say so. The same behaviour that is honest at a terminal is invisible in a viewer.

So the client resolves and SHOWS. `ProjectService.ResolveDesign` gives the viewer the design, its
project, and its declared entry; the project bar states which project is in effect and, when the open
file is a companion, says that analysis reads the entry instead. Acting on it stays the user's move.

**What this is not.** It is not a gap waiting to be closed by making the loader smarter, and it is not
a claim that silent resolution is wrong in general — the CLI does exactly that. It is a claim that
"resolve silently" and "resolve visibly" are different features, and which one is correct depends on
whether the surface can afford to say what it did.

**Reopen if** a served surface appears that has no way to display a notice and genuinely needs the
redirect — a headless API consumer, say. Even then the answer is more likely a field on the response
saying which artifact was read than a loader that swaps files without telling anyone.

---

## A design in no project gets a guessed picker list, not an empty one

**Question.** When a design resolves to a project, the config pickers offer what the project DECLARES:
the vocabulary picker its conventions file, the review picker its checklist. When a design belongs to
no project, nothing has declared anything, so the browser cannot know which kind a given YAML file is.
It falls back to listing every YAML sitting beside the design. On a mount of loose files that can be
dozens of entries, most of which will not resolve as either kind. Should the fallback be narrowed, or
dropped in favour of offering nothing?

**Answer. Keep the guess, and keep it wide.** The two failure modes are not symmetric. Offering a file
that turns out to be the wrong kind costs one clear error, once, naming the file and the field that
did not parse — the user learns something and picks again. Hiding a file that WAS the right kind costs
them their own config with no error and nothing to look at, and the only signal is a picker that does
not list a file they know is there. A silent omission is not discoverable; a loud rejection is.

Narrowing by filename convention (`*conventions*.yaml` and the like) would trade the loud failure for
the silent one and buy a shorter list with it, which is the wrong side of that trade. Reading each
file to classify it would mean duplicating two parsers in the browser, which is why the server owns
validation (`GetNamingConvention` / `GetReviewManifest`) in the first place.

**What this is not.** It is not an argument that guessing is fine in general. Where a declaration
EXISTS the pickers use it and do not guess at all, and they keep the kinds apart, because a checklist
offered as a vocabulary fails exactly the way an intent file did (agni issue 175, PR 184). This is
only about the case where there is nothing to read.

**Reopen if** a deployment turns up where the loose-file case is the common one rather than the
exception, and the list is long enough that the right file is genuinely hard to find. The fix then is
probably to make the server classify a directory in one rpc, not to guess better in the browser.

---

## The project store revalidates on read rather than watching the filesystem

**Question.** `internal/projects.FSStore` re-walked its mounts on every call, ~19 ms on a mount with a
few hundred directories, which a browse UI pays on every listing (agni issue 176). That issue offered
two ways out: a filesystem watch, or a cache that still stats what it depends on before answering.
A watch is strictly faster, since a warm answer costs nothing at all. Why take the slower one?

**Answer. Because the two fail differently, and only one of them fails loudly.** A watch is correct
exactly as long as every event is delivered. When one is dropped — an editor writing through a
temporary file, a network mount, a container bind mount, a platform limit on watched descriptors —
the cache goes stale and there is no way to notice. The answer is confident, wrong, and identical in
every observable way to a correct one. Worse, it is unfalsifiable after the fact: nothing in a
findings list records which version of a descriptor produced it.

Revalidate-on-read can only ever be as wrong as the filesystem is. It re-stats every dependency
before returning, so an operator editing a descriptor while the server runs is seen on the very next
request, which is the property issue 176 required of any cache here. The cost of that choice is
measured rather than assumed: the largest case tested lands at 1.2 ms against ~0 ms for a watch,
versus 19 ms for no cache at all. Buying the last millisecond with a silent-staleness risk is the
wrong trade in a tool whose entire value is that its answers can be trusted.

The stats are cheap for a structural reason worth keeping in mind if this is ever revisited: a stat
is a fraction of a `ReadDir` and comparing a stat is a fraction of parsing YAML, so the revalidation
is cheap exactly where the work is expensive. That is what makes the safe option affordable.

**What this is not.** It is not a claim that watches are wrong in general, and not a rule about
caching elsewhere. `cmd/agni/osprojectconfig.go` holds no cache at all for the same freshness reason
and would copy this shape, not reopen this trade, if a deployment ever felt its cost.

**Reopen if** a deployment appears where 1.2 ms per listing is genuinely the bottleneck AND the
mounts are on a filesystem whose event delivery can be trusted. Even then the answer is more likely
a watch that INVALIDATES the existing revalidating cache (turning a stat into a no-op on the common
path while a dropped event costs only a stat, not a stale answer) than a watch the cache believes.
