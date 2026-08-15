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

---

## A datasheet parameter binds to a pin by NAME first, never by pin number

**Question.** A `param.Pin` records both a name (`VCCB`) and a per-package number (`11`). When a rule
resolves a design's pin to a spec's pin, the number looks like the precise key and the name looks
like the loose one. Why is it the other way round?

**Answer. Because a pin NUMBER is a fact about a PACKAGE, and a parameter is a fact about the DIE.**
The same silicon ships in several bodies and each body numbers its terminals differently, so a
number-keyed join is silently wrong for any part seeded from one package and placed in another.
Silently is the operative word: the wrong terminal is still a real terminal with real limits, so the
comparison runs, produces a confident answer about the wrong thing, and nothing downstream looks
broken. The seeded TXB0104 carries the case as data — number 11 is the `B3` data I/O in the TSSOP-14
and the `VCCB` supply in the UQFN-12.

A name is copied off the same pin function table by both the vendor and the symbol library, so it
survives repackaging. Its one weakness is that it is not unique, and the number exists to repair
exactly that: it is a TIE-BREAKER, not a fallback.

**What this leaves open, and what it does not.** The number is still used, in two bounded ways: to
separate several pins sharing a name inside a package the design is known to place, and, with no
package identified, when every declared package agrees on it anyway. What is closed is leading with
it. Where the two channels disagree, `param.ResolvePin` refuses rather than picking, because either
channel alone would have produced a confident wrong answer.

The full precedence, the four refusal sentinels, and the degrade-safe path for a spec with no pin
data are in [the datasheet layer](https://panyam.github.io/agni/architecture/datasheet-layer/#pin-binding),
stated once there and implemented once in `param.ResolvePin`. The physical background is
[pins and packages](https://panyam.github.io/agni/reference/pins-and-packages/).

**Reopen if** a design source appears that identifies a placed package with certainty, for every
component, without going through an orderable-MPN suffix. Even then the answer is not "key by
number", it is that the tie-breaking channel becomes reliable more often. The name still leads,
because it is the one that means the same thing in every body.

---

## The review gate counts answered items, not covered ones

**Question.** `Covered()` is the documented coverage axis, `Total - NotAutomated`, and the review
report has printed it since the outcome vocabulary landed. When `agni review` grew a CI gate
(agni issue 199), should the floor read that number?

**Answer. No, and the ticket's own motivating example is the reason.** Issue 199 opens with "a
checklist can go from 40 covered items to 12 because a `--params` directory moved". Measured on
`examples/tutorial-project/` before writing the flag, removing `params/` moves the covered count by
**zero**. The affected item's rule is still in the catalog and still selected; it merely has no facts
to read, so `check.Available` gates it and the item reads `not-applicable` — which `Covered()` counts
as covered. `NotAutomated` moves only when a rule leaves the CATALOG, which is what a moved
`profiles/` or a renamed `conventions.yaml` does, not a moved corpus.

So a floor over `Covered()` would have shipped the flag and not caught the case it was written for,
in the direction that reads as success. `Tally.Answered()` is the second count: `Pass + Fail +
Provisional + ComputedNA`, the items the run actually decided.

**What this leaves open.** `Covered()` is unchanged and still rendered. The two answer different
questions — "do we have a mechanism for this" and "did we get an answer" — and a checklist where they
diverge is reporting something true. The one assignment worth arguing about is that `ComputedNA`
counts as answered and `NotApplicable` does not: the first is the DESIGN settling the question, which
is a real determination, and the second is the question going unasked.

**Reopen if** a third count is proposed. The bar is the same one this cleared: name the regression the
existing counts cannot see, and measure it on a real fixture before adding a number a team will gate
on. The full rationale is in [the checks contract](https://panyam.github.io/agni/architecture/checks-contract/).

---

## Both review gates are opt-in, and a provisional does not trip the default one

**Question.** `agni review` has always exited 0. Once it can gate, should it gate by default — and
should `provisional` count as a failure?

**Answer. No to both, and the first was measured rather than argued.** Default-on breaks 32 existing
CLI test invocations (the review fixtures fail on purpose, which is what makes them fixtures), three
targets in `examples/tutorial-project/Makefile`, and both `agni review` invocations in tutorial rung
8 — none of which anyone could have opted out of in advance, because the flag did not exist to be set.
It would also have destroyed the red-before-green signal for the gate's own tests, since a suite where
everything is red cannot show that a new test is red for the right reason. The benefit was one less
flag to type.

`provisional` is a genuine failure resting on mock or below-floor datasheet data. Gating on it by
default fails a pipeline on data QUALITY rather than on design quality, and the reliable outcome of a
gate that goes red for reasons a team cannot act on is that somebody switches the gate off. It gates
when named: `--fail-on-outcome fail,provisional`.

**What this leaves open.** The vocabulary `--fail-on-outcome` accepts is deliberately the WHOLE
outcome set, not the two or three a team is likely to want. Each outcome exists because a check that
did not evaluate had been scoring as a pass, and a team deciding `needs-data` should block their
release is making exactly the judgment the vocabulary was built to allow.

**Reopen the default-on half if** the compatibility cost goes away, which means the fixtures and the
tutorial rungs no longer depend on a zero exit. That is a real possibility and not close. Do not
reopen the provisional half on the argument that a real defect can hide behind one: that is true, and
the answer is a ratified corpus or an explicit `--fail-on-outcome fail,provisional`, not a default
that trains people to disable the gate.

---

## Saving a workbench draft does not validate it

**Question.** The extraction workbench writes a `PartSpec` that `param.Validate` would reject, over
and over, all day. Should `SavePartSpec` refuse the ones that are structurally incoherent — two pins
sharing an id, a parameter bound to a pin that does not exist — to keep bad data off disk?

**Answer. No. Saving records what the author has; judging it is separate.** A refused save costs
work, and it obliges every editing action to preserve the invariant or leave a document its author
can neither fix nor escape through the UI. That obligation is not hypothetical: `deletePin` and
`deletePackage` already had to unbind and drop numbers respectively, purely so an ordinary delete did
not strand the document.

**The argument for refusing rested on a false premise, which is the part worth remembering.** It was
justified by "an incoherent spec on disk breaks `param.LoadSet` for the whole corpus". It cannot:
`LoadSet` reads `*.textproto` (`datasheet/param/set.go`) and the workbench writes
`<stem>.partspec.json`. A draft cannot reach a corpus by sitting on disk, so there was nothing on the
other side of the trade. Check the premise before designing around it.

**What replaced it.** `SavePartSpecResponse` carries the problems back, classified as STRUCTURAL
(wrong now) or COMPLETENESS (merely unfinished, which every draft is). The write always succeeds. The
client renders them and computes nothing, which also deleted a TypeScript reimplementation of the
same rules that the refusing version had needed.

**Reopen if** a draft ever becomes something another consumer loads automatically. Today the only
route from draft to corpus is manual, and issue #209 is where that step — and full `param.Validate`
with it — belongs.

---

## A pin-to-pin bound is a value with a modality, not a comparison operator

**Question.** The first pin-to-pin constraint anyone reads is "VCC(A) must be less than or equal to
VCC(B)", and the obvious shape for it is a comparison operator between two pin references. #191 asks
for a tracking relation. Why is it not `{subject, op, reference}`?

**Answer. Because the operator is the least stable part of the sentence.** Five instances read across
four vendors, and three of the five carry a non-zero offset that an operator cannot express at all:

| Document | The bound |
|---|---|
| TXB0104 §9; Nexperia NXB0104 Rev. 4, Table 4 fn [1] (p4) | zero: "must be less than or equal to" |
| Microchip LAN8671, p246 | 0.5 V: "shall never exceed the VDDAU pin by more than 0.5 V" |
| NXP NVT2008/NVT2010 Rev. 1, fn [1] (p6) | 1 V: "should be at least 1 V higher than Vref(A)" |
| TI MSP430FR2355 (SLASEC4D), p113 | stated BY REFERENCE to the Absolute Maximum Ratings section |
| NXP S32K3xx Rev. 14, p92 | 100 mV, and only for transient, not for DC |

So the shape is a bound on the DIFFERENCE, subject minus reference, which reuses `RangeValue` and its
absent-bound semantics unchanged. "Less than or equal to" is a max of 0; a symmetric "within ±0.3 V
of" is min -0.3 and max +0.3. Nothing new is invented, and the three non-zero cases fit.

**Modality is part of the claim, not decoration.** "Shall never exceed" and "should be at least 1 V
higher for best translator operation" are different statements about the same kind of relation, and a
contract that records only the numbers cannot tell #192 whether a violation is an error or a note.
The vendor's own modal verb is the only evidence of which it is.

**What this leaves open.** Two questions this does not answer, both with real instances above:

- The **by-reference** bound. The MSP430FR2355 points at another table instead of printing a number,
  so the relation may need to name a parameter rather than carry a literal.
- The **regime qualifier**. The LAN8671 scopes its bound to power-up, power-down and normal
  operation; the S32K3xx scopes its 100 mV to transient and explicitly not to DC. A bound recorded
  without its regime is wrong on both, and in opposite directions.

**Reopen if** a wider survey than the 62-document one behind this finds non-zero offsets rare enough
that the value slot is dead weight. The measured split is three of five, so that is not the current
evidence.

---

## A datasheet's power-sequencing requirement stays out of the parameter contract

**Question.** Datasheets state sequencing between pins as plainly as they state ordering: the
TXB0104 requires OE held low until both supplies are ramped and stable, the LAN8671 (p252) requires
that "VDDA must not be powered for an extended period of time without VDDP also at operational
levels", and the NVT2010 (p1) requires EN LOW through power-up and power-down. It is common, it is
safety-relevant, and #191 asks which pin-to-pin forms the contract should carry. Why is this not one
of them?

**Answer. No, and not because it is unimportant. Nothing can ever evaluate it, and nothing is lost by
leaving it out.** Two independent reasons, either sufficient:

A netlist carries no ordering-in-time evidence whatsoever. There is no rule #192 could write over the
design IR that would check a sequencing requirement, so admitting the shape would put a constraint in
the contract that every run silently skips. That is the exact failure the skip-not-false-pass
discipline exists to prevent, one layer earlier.

And the text is already retained. Sequencing prose lives in the pin table's description column, which
`Pin.description` preserves verbatim, so the workbench and the parts panel can already show a human
the sentence. A dedicated field would therefore be a field with a producer and no consumer, which is
what PR 202 removed `Package.pin_count` for.

**What this leaves open.** Displaying it better. The sentence is on `Pin.description` today, but
nothing marks it as a sequencing requirement rather than ordinary prose, so a reader has to notice it.
Surfacing it is a workbench question, not a contract one.

**Reopen if** a design source starts declaring intended power-up order, most likely as an `intent`
declaration alongside `StrapGroup`. A datasheet's sequencing requirement is unevaluable against a
netlist, but it is perfectly evaluable against a second artifact stating what the designer intended.
That would make this a comparison between two declarations rather than a check against a design, and
the answer changes.

---

## Machine-wide config carries where-bytes-are, never what-is-checked

**Question.** `agni.yaml` exists for mounts and symbol search paths. Naming conventions, interface
profiles and seeded parameters are configured far more often than mounts are. Why can they not go in
the same file, defaulting for every design and overridden per project where it matters?

**Answer. No, and it is the bug per-design config was built to fix.** Before projects existed, those
tiers could only be `agni serve` startup flags, and a deployment mounting a mixed set applied one
team's config to every design it read: an overlay's profiles superseded the built-ins for every board
on the server, and an overlay's rail lexicon changed net roles on designs that never asked. Both were
correct in isolation and aimed at the wrong design. A machine-wide `conventions:` is that failure
with a different file name.

The rule that separates them is not what a knob does but **how it fails**. A wrong mount produces a
loud immediate error: the file is not there. A wrong naming vocabulary produces a confident wrong
answer that looks exactly like a right one. Config whose absence or wrongness is SILENT belongs where
it is scoped — to a project, reaching only the designs that declared it.

That is also why symbol search paths are analysis config rather than environment config even though
they only locate bytes. A schematic naming a library nothing resolves reads short, the missing parts
are simply absent, and the run reports fewer findings with no error to explain them. `agni.yaml`
carries a machine-wide symbol-path DEFAULT for a system-installed vendor library, which is honest at
that scope; a project's own libraries belong in its descriptor, where they travel with the design and
reach a served surface too.

**What this leaves open.** Sharing across projects, which is the real need behind the question, is
`extends` — declared in a descriptor, scoped to the project that wrote it, reaching no design that did
not ask. Declared beats inherited. `agni.yaml` rejects unknown keys so the boundary is enforced rather
than documented.

**Reopen if** a tier appears whose wrongness is loud rather than silent. That is the property that
decides, not how convenient a global would be.

---

## `check` stays the primitive; it does not become a rendering of a review run

**Question.** `review.Run` already calls `check.Run`, both surfaces compose config through one seam,
and both produce a `CheckResults` document. So why are they two execution paths? Issue 198 proposed
collapsing them: `check` becomes an auto-manifest review with the store as the only difference between
a dry run and a kept one.

**Answer. No, and the reason it looked attractive has been removed.** The pressure behind it was that
one surface could not say something the other could: tick a rule this design cannot support and the
Checks panel showed an empty list, indistinguishable from a clean board, while the review layer had a
whole vocabulary for it. That was fixed in #220 by reporting `check.Available`'s verdict for the rules
already selected — the same gate `review.Run` consults, asked at the finding tier. No manifest, no
per-item execution, and the panel now distinguishes "checked and clean" from "never ran".

What remains after that is two genuinely different questions over one engine. `check` sweeps N rules
once and reports findings on a SEVERITY axis. `review` scores N checklist items, each bound to rules
and each scoped to that item, on a COVERAGE axis. Merging them means either handing a flat sweep a
checklist it does not have, or handing a checklist a design-wide union it never ran — and the second
is a claim the review layer explicitly refuses to make (`CheckResults.findings` is empty for a review
run for exactly this reason).

**The cost argument that blocked it also did not survive, and that is worth recording separately,
because it was cited in #220's favour as well.** Issue 198's step 5 held that an auto-manifest turns
one sweep over N rules into N sweeps of one, on the viewer's default-open panel. Measured on
`examples/tutorial-project` with a generated 44-item manifest, one item per catalog rule, warmed, five
runs each: `check` 46 ms/run, the auto-manifest review 27 ms/run. The per-item shape is faster. That
is what the arithmetic predicts — each item binds about one rule, so `items × entities` is the same
order as `rules × entities`, and both paths gate on `check.Available` before any entity work. So the
right reason to keep them apart is the vocabulary, not the clock.

**What this leaves open.** Two of issue 198's steps survive on their own merits and are tracked
separately: an ad-hoc manifest a caller can run without authoring a file (issue 256), and grouping
items that share a binding before sweeping (`OUT_OF_SCOPE.md`, no driver since the measurement).

**Reopen if** a surface appears that needs the coverage vocabulary over a flat rule sweep — an
auto-manifest is then the natural shape and this decision is the thing standing in its way. Do not
reopen on performance without measuring on a design large enough for the per-sweep fixed cost to
matter; the tutorial fixture is 19 components and cannot show that.

---

## A type crossing a runtime boundary is a proto; only engine-internal projections are Go types

**Question.** `check.Finding` and `review.Report` are Go types with proto twins and hand-written
converters, while `param`, `doc`, `ir` and `geom` use the generated types directly. Both patterns are
live, so which one does a new type follow?

**Answer. Whether it crosses a runtime boundary, and nothing else.** `param.proto`'s own header states
the rule: it is a cross-runtime contract shared by the Go engine, the TypeScript surfaces and future
extractors in whatever language suits them, and hand-written parallel types are exactly the drift a
shared schema prevents. A type that will be rendered in a browser or produced by an external service
is that class, whatever package it happens to live in.

Two types shipped as Go and were converted for this reason. A **candidate** fact crosses twice by
design, to a browser where a person accepts it and to an extractor that may be a separate service, so
leaving it in Go guaranteed a hand-written TypeScript twin later. A **work item** was the subtler
case: its neighbour `review.Report` is a Go type with a twin, so it looked consistent, but its own
member `UnmetDependency` was already proto, which made a Go wrapper a THIRD representation of one
thing rather than a second.

**What this leaves open.** `check.Finding` and `review.Report` are the same inconsistency one level up
and are deliberately untouched: converting them means changing the CLI and service converters, which
is a refactor with its own risk rather than a fix. When one is next reworked, this is the rule.

**A separate proto PACKAGE, not always the nearest one.** A candidate went to `agni.v1.candidate`
rather than into `param`, because a proposal is not a fact: it has no standing until a person accepts
it, and a lifecycle stage that must never reach a corpus by itself does not belong inside the contract
a corpus is made of. Proximity of subject matter is not the test; whether the thing is the same KIND
of claim is.

**Reopen if** a type is genuinely never leaving the process. The cost of proto is real (regeneration
on both runtimes, presence semantics), and paying it for something that will only ever be a Go
intermediate is waste.

## A stale verification is untrustworthy data, not trustworthy data

**Question.** The review layer's ratification axis decides whether a fail ran on data worth acting on.
It judged by extraction method and confidence. Once a value can carry a human verification pinned to a
document revision, what does that axis do when the document has moved past the revision that was
checked? Treating it as untrustworthy demotes every finding resting on a revised document to
Provisional, which is safe and potentially very noisy.

**Answer. Stale and unknown are both untrustworthy.** A verification is a claim about a specific
revision. Once the corpus holds a different one, the claim is not evidence about the document in hand,
and the honest report is "someone checked this, but not this version of it" — which is exactly what
Provisional means here: a re-confirm task, not a defect and not a clean pass. `unknown` (a
verification exists but no revision is recorded to compare against) goes the same way, because a
caller that cannot check must not be told the answer is fine.

The noise concern is real and it is the point rather than a side effect. A vendor revision genuinely
does invalidate the evidence under every value read from that document, and re-confirming a known row
against a known page is a much smaller job than finding it in the first place. Provisional is already
the bucket for exactly this: work a human owes the corpus.

**What made it non-optional.** `param.MarkVerified` raises `confidence` to 1.0 to keep the older
signal in step, and nothing ever lowers it. So a confidence-only test does not merely miss staleness,
it INVERTS it: a verification of a superseded revision scores as the most trustworthy data in the
system precisely because a person once checked it. Leaving the axis alone was not a neutral choice.

**What is deliberately unaffected.** A value with no verification record at all reads `unverified` and
is judged on method and confidence exactly as before. Anything else would demote every hand-seeded
fixture in the corpus the moment this landed, and "nobody has verified this" was never a claim that a
document revision could falsify.

**Reopen if** a deployment finds re-confirmation cost dominates. The upgrade is diff-aware
invalidation: if the cited table's own content hash did not change across the revision, carry the
verification forward. `derive.Patch` already keys on a table content hash, so the input exists. That is
strictly harder and should be driven by a real corpus, not designed against a guess.

## A document revision is recorded for the reader, and never compared

**Question.** Staleness is decided by content hash. A hash is unreadable, so a stale fact could report
only "these two hashes differ", which nobody can act on. Should the schema also carry the revision as
printed ("SCES650K"), and if so where?

**Answer. Yes, on `Verification`, as display only.** Two parts, and the second is the one that is easy
to get backwards.

*Why it is needed at all.* Provisional exists to generate a re-confirm task a human picks up. "Verified
against SCES650K, corpus now holds SCES650L, page 4" is that task. Two hashes is not. The revision is
the only part of the record a person can act on.

*Why it is NOT on `SourceDoc`.* That was the obvious home and it does not work. A re-seed rewrites
`SourceDoc`, both hash and title, and that rewrite is precisely the event that makes a verification
stale. A revision recorded there would be destroyed by the one thing that makes it worth having. It has
to be snapshotted onto the `Verification` at the moment of verification, frozen beside the hash it was
taken with. `SourceDoc.title` continues to name the revision the corpus holds now, so a citation
carries both sides and a report can name each.

**`MarkVerified` takes the document, not a hash.** The key and the snapshot are read from one place so
they cannot disagree. Passing them separately would permit a record that goes stale correctly and then
names the wrong revision to the person asked to re-confirm it, which is wrong in the only way nothing
downstream can detect.

**Never a comparison input, and this is load-bearing rather than cautious.** Vendors reissue silently
without moving the printed revision, and move the printed revision without changing content. Two files
stamped "Rev K" may differ; two differing strings may describe identical bytes. Deciding staleness on
the printed name reintroduces exactly the silent decay the hash prevents, with a better cover story.
The field is also not orderable (K/L/M, 1.0/1.1, A/B, bare dates, "Rev K.1"), so there is no general
"how many revisions behind". The proto comment says all of this at the field, because the pressure to
short-circuit on it will arrive from someone who has not read this file.

**Reopen if** a vendor-neutral structured revision ever becomes extractable and someone wants ordering.
The answer would still not be to compare it; it would be to render it better.

## A text-width estimate stays at 0.6 em per glyph

The width estimate shared by the caption-condense decision and the free-text column fit
(`glyphAdvanceEm`) held exactly while the backend drew one monospace face. It is now an average over
a proportional one, and the question is whether to re-fit it.

**No, and the measurement is the reason.** Across 31 realistic schematic runs (net names, ref-des,
values, packages, pin names and numbers) the weighted average is **0.6147**, spanning 0.514 for
`6.3V` to 0.736 for `DGND`. So 0.6 under-predicts by 2.4%, and both callers only decide *whether* to
condense. A 2.4% shift in that threshold is not worth moving the column fit and every golden.

**The trap this records.** An earlier estimate of 0.64 came from eyeballing two uppercase net names
and overstated the error threefold, which was almost enough to justify the change. A per-run figure
from a couple of samples is not an average; if this is reopened, it needs a measured corpus and a
reason that needs the precision.

## A format's own text-size convention is converted in the reader, not the renderer

EDIF's `textHeight` is a line pitch rather than an em size, so it is divided down to a glyph height
where the file is read. The alternative was a per-format interpretation in the render layer.

**The reader owns it, because it is a fact about the format.** `geom`'s height field means glyph
height and the renderer maps it straight to `font-size`, so translating a format's spelling into the
contract's meaning is exactly what a reader is for. Putting it in the renderer would also have been
actively wrong: KiCad states a glyph height directly, and a renderer-side conversion would have
shrunk every KiCad render by 24% for no reason.

## The residual gap on inherited pin-label sizes is not fitted away

Pin labels that inherit their figureGroup's height still render about 1.37x the size the authoring
tool prints, where ones stating their own height land at 0.96x.

**Left alone deliberately.** That tool prints pin names in a STROKE font at a different ratio from
the Arial it uses elsewhere (1/1.8 against 1/1.3148), and 1.8 / 1.3148 = 1.37 accounts for the gap
exactly. Closing it would mean a per-figureGroup ratio calibrated on one exporter's font choice,
which is fitting rather than reading, and it would be wrong for the next exporter. We render one
face; matching a substituted stroke font's point size is not obviously the goal even when it is
achievable.

**Reopen if** a second export from a different toolchain shows the same per-group ratio, which would
make it a property of the format rather than of one printer.

## A design whose descriptor does not parse is refused, not served with a warning

`ProjectResolver.Overlay` returns an error when the descriptor governing a design fails to parse,
and the surfaces refuse the run. The alternative considered was to serve the run and MARK the
results as computed without the project's configuration.

**Refusing won, and the reason is where the marking would have to live.** A banner on the page is
not attached to the thing that gets quoted: findings are screenshotted, pasted into tickets, and
exported to reports, and every one of those carries the numbers away from the chrome. On one folder
the difference between composing with and without the project's config was 40 findings the project's
own lexicon would not have raised and 95 it would have, so a marked-but-served run is a wrong answer
with a caption that does not travel with it.

It also settles a CLI/server disagreement rather than adding one. `agni check` was already fatal on
this input by a different route, so serving it in the viewer meant the same file was an error in one
surface and a badge in the other.

**Reopen if** a deployment needs to render a half-configured folder — but the marking has to ride on
the RESULTS, per-finding, not on the page. If it cannot, this answer stands.

---

## An empty mount root is hidden from the design tree, not exempted from pruning

When the tree learned to prune folders with no readable design under them, mount roots were exempted:
a mount is something an operator configured by hand, and one silently missing from the sidebar reads
as a broken mount rather than an empty one. That exemption is now gone. Mounts are pruned by the same
rule, and the tree reports how many it hid.

The argument that decided it is that the exemption was optimizing the design page for a tenant that
is leaving. A mount of datasheets is the case that motivated keeping empty roots visible, and
datasheets are becoming their own service, at which point that mount stops being served to this page
at all. Shaping the design tree around a temporary neighbour buys a worse tree now and nothing later.

The discoverability objection was real and is answered in the UI rather than in the listing:
`ListMounts` returns `pruned_mounts`, and the sidebar says how many folders it hid. So the operator
sees "1 folder hidden (no designs)" instead of a shorter list with no explanation. A mount whose root
cannot be read at all is kept and shown, since an unreadable root is a mistake to surface, not
evidence of emptiness.

**Reopen if** a mount is ever expensive to walk enough that pruning at page load costs more than the
empty root did. The answer then is a cache, not an exemption. Also reopen if the design page ever
does serve a second file kind, since the rule is "no file this client can open", not "no design".
