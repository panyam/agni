---
title: "Authoring a check rule"
description: "The path from a review-checklist item to a shipped, trustworthy check rule, worked through one real rule."
---

A check rule asserts something about the design as it was read: "every rail carries a test
point". It does not compute the way analysis does, it does not fix anything, and it has to be
checkable from the IR the readers produce today. The architecture behind the pieces used here
lives in [Rules and checks](../../architecture/rules-and-checks/) and
[Net solving and hierarchy](../../architecture/net-solving/). This page is the practical path,
worked through one real rule, `test-point-coverage`, at every step.

## Is it a rule at all?

Ask what the rule reads. If the answer includes data no reader emits yet (net classes, datasheet
limits before the parameters tier exists), the reader work comes first and the rule waits.

The worked example comes from a design-for-test checklist: rails and ground should be probe-able.
It reads component classes (is anything a test point), net membership, and rail-ness. All three
are present in the IR. So it is a rule.

## Write the sentence, then the guards

A rule is one sentence plus its exceptions, and the exceptions are what keep an engineer from
muting it. Write the sentence first:

    A power rail or ground net must carry a test-point component.

Then interrogate it the way a real corpus will:

- What if the board has no test points at all? A demo board is not wrong, it just has no
  test-point convention. Fire only when the design places test points somewhere. This kind of
  channel gate once took a rule from 1836 findings to zero.
- What is "a rail" when the format carries no direction data? Name is the only rail evidence a
  bare EDIF netlist has, so fall back to rail facts or rail/ground name heuristics.
- What about nets the read did not fully cover? Skip nets marked external.
- Who decides the policy edge cases, like a feedback node named like a rail that is deliberately
  left unprobed? The reviewer does. Use `info` severity and say so in the rule's doc.
- **Is there a tier of evidence better than the name?** A rail's voltage comes from its name
  (`check.NominalVoltageFromName`), which is a convention: silent on a rail nobody named for its
  voltage, and wrong on a name that outlived a design change. Sometimes connectivity answers the
  same question outright, and then it should be asked first. The pin-tracking rules bound the
  difference between two pins, and two pins on ONE net are one node, so that difference is exactly
  zero with no name read. That tier settles a "must not exceed 0V" bound as satisfied and a "must be
  at least 1V" bound as violated, on a design whose nets carry no voltage token at all.
  `regulator-output-exceeds-abs-max` is the other instance, reading a rail's volts off the feeding
  regulator's own datasheet. Reach for the convention as the fallback, not the first answer.
- **If you do read a name, is the net actually a rail?** `net.nominal_voltage` token-scans a whole
  name, so a signalling level encoded in a SIGNAL net's name parses as a rail nominal (agni issue
  194: `U3_12_U7_4_3V3` yields 3.3 while classifying as neither rail nor ground). Gate on
  `Model.IsRailNet`, the narrow role question, before reading either name.
- **What does that gate cost you when the project has not configured it?** A role gate is not free:
  it inherits the role's whole configuration surface. `IsRailNet` reads a stamp derived from the
  naming lexicon, and the built-in vocabulary is start-anchored (`VCC`, `VDD`, `+3V3`), so a project
  naming rails function-first matches almost none of it. Measured on a real 1700-net board, declaring
  the project's patterns moved the rail count from 13 to 91: gating without that config silently
  answers a narrower question. Gate anyway, and make the gap visible rather than assuming the config
  is there. `rail-not-classified` is the tripwire that does it.

## If the sentence names two entities, carry both

A `Finding` has one `Subject`, and the subject is what a reader has to CHANGE. That is not always the
entity the sentence is about, and when it is not, the reader gets sent somewhere the message never
mentioned.

`crystal-load-caps` is the case this rule exists for. Its message reads "crystal terminal net XOUT1
has no load capacitor" and its subject is the crystal, which is correct, because the crystal is the
part someone edits. But the sentence is about a NET, so clicking the finding highlighted a part; and a
crystal has two terminals, both inside the highlighted symbol, so the drawing could not even say which
one was at fault. The reader was told a net was wrong and shown a part.

So: **when a message names a design entity other than its subject, carry it in `Finding.Context`**,
each entry with a `Role` naming the part it plays. The panel renders them as clickable chips beside
the message.

In a Go rule:

    out = append(out, check.Finding{
        Kind:    check.KindComponent,
        Subject: ref,                       // what a reader changes
        Message: "crystal terminal net " + n.Name + " has no load capacitor",
        Context: []check.ContextSubject{{
            Kind: check.KindNet, Subject: n.Name, NetID: n.Id, Role: "terminal",
        }},
    })

In a datalog rule, the entity is already bound in the answer row; name the variable and its role:

    SubjectVar:  "y",
    Message:     "crystal terminal net {net} has no load capacitor",
    ContextVars: []query.ContextVar{{Var: "net", Kind: check.KindNet, Role: "terminal"}},

Four things to get right:

- **Never repeat the subject as its own context.** The chip would navigate to where the reader
  already is. On a `KindPin` rule the subject is the ref/pin pair, so `{pin}` is the subject and only
  the net is context.
- **Order is yours and it is significant.** Entries come out in the order you declare them, and the
  panel renders chips in that order, so declare them in the order your message names them.
- **A role is a short lower-case noun**, not a description: `terminal`, `rail`, `source`, `sense`. It
  is an open vocabulary, deliberately, because the useful word is rule-specific. Roles need not be
  unique within a finding, because two entities can play the same part ("A and B both strap to
  address N").
- **Entities only, never values.** Thresholds, voltages and units belong in the message. A chip is
  something a reader can click and land on in the drawing.

Set `NetID` on a net context entity where you have it, so a net that shares its name with another
highlights the right instance rather than all of them.

## Author spec-first

Proven vocabulary goes in the Spec AST. Anything multi-clause goes behind a registered SpecFunc
that declares its own reads and primitives, so the derivation stays honest across that boundary.
The example is one FFI (`has_test_points`, the channel gate) plus existing facts:

    Over: nets
    Where: has_test_points(design)
       and not external(N)
       and (global(N) or power_driven(N) or rail_name(N) or ground_name(N))
       and not exists T in N.connections where class(T) == test_point

Bind it with `spec.Rule(check.Rule{...})`. `Reads` and `Primitives` derive from the body, so they
cannot drift from what the rule actually does. One init-order trap remains, learned from fixtures:
register a rule's own FFI inside the rule variable's own initializer, because package variables
initialize before `init` funcs run and binding validates the Call targets. The shared helpers like
`rail_name` or `ground_name` need no such care from a built-in rule: the catalog lives in package
`stdlib/rules/builtin`, which imports `core/check`, so `check`'s own package init registers those
FFIs before `builtin`'s variable initializers run, and they are available for free.

The twin discipline: proven vocabulary is spec-only, as here. New interpreter vocabulary (a new
entity set, a new fact, a new traversal) ships with a Go `Eval` as the canonical twin until the
vocabulary soaks, with parity asserted between the two.

## One file, one line, one doc

- `stdlib/rules/builtin/rule_<name>.go`: the rule.
- `stdlib/rules/builtin/register.go`: one line adding the rule to the `rules` slice. The built-in
  catalog is its own package now, not part of the core engine. It installs itself through the same
  public `check.RegisterBuiltins` seam an overlay uses, so `core/check` owns no rules of its own.
- `stdlib/rules/builtin/docs/<name>.md`: the single source of the rule's `Detail`, embedded at build time. The
  harness fails CI without it. Write it in full as proper `###` sections under the rule's `##`
  title, not bold run-ins: What it means, Why engineers want it, Impact, an ASCII sketch of
  fires-versus-fine, a Scope note recording every guard decision from the step above, the query
  structure, and a "For software readers" section mapping the EE concepts to structural analogies
  (a test point is a metrics endpoint, and the rule reads as "critical paths must emit telemetry")
  with a diagram beside it.

## Say what to do about it

A rule carries four pieces of prose, and `Remedy` is the one it is easiest to leave off. `Summary`
is the one-liner, `Impact` is what goes wrong when the rule is violated, `Detail` is the long-form
markdown, and `Remedy` is what to DO about it, in the imperative, as one engineer would say it to
another. Without it a reader who accepts that the finding matters still has to already know the fix,
which is most of the distance between a report and an action.

`TestEveryRuleStatesARemedy` (in `tools/catalogdocs`) holds the whole catalog to this, so a new rule
does not ship without one.

Three things keep it honest:

- **It is generic over the RULE, not the subject.** The remedy names the class of change ("add a
  bulk capacitor where the rail enters"), never a designator. A remedy templated on the bound
  subject is a later tier and waits for verdicts to carry the binding.
- **Where the real fix needs a value the engine cannot derive, say what to size it from and stop.**
  The pull-up an I2C bus wants depends on its capacitance and clock rate, neither of which the
  netlist states. `i2c-pull-up` says to size it from those and names no resistance. Inventing a
  plausible number here would be the same silent-authority problem verdicts exist to remove, one
  layer up.
- **Where the finding is about the ANALYSIS rather than the design, say so.** `rail-not-classified`
  and `symbol-unresolved` are remedied by giving the tool more (`--conventions`, `--symbol-path`),
  and their remedies say explicitly that nothing is yet known to be wrong with the board.

A rule generated per-declaration (the `intent` and `profile` families) keys its remedy by KIND rather
than writing it at the builder, because the fix for a missing OV clamp is the same sentence on every
rail that declares one. See `docRemedies` in `stdlib/rules/intent/docs.go` and `requirementCaption`
in `stdlib/profiles/docs.go`: the runtime rule and the docsite exemplar read one source, so there is
no second copy to drift.

## Say what you looked at, not only what failed

A rule may also set `EvalVerdicts`, which reports one `check.Verdict` per subject it was applied to,
passes included. Where it is set it is the rule's single source of truth and `Eval` is its projection
(`check.VerdictsToFindings`), which `TestVerdictParity` holds them to.

It is optional, and nil is not the same as an empty considered set. A rule that has not been
converted is DECLINING to state one, and reporting "this rule considered nothing" on its behalf is
the same silence-reads-as-data mistake one level up from the one verdicts remove.

**Enumerate, then judge.** The shape that works is two functions: one yielding the subjects the rule
applies to, one deciding a single subject. `Eval` is then `map(judge, enumerate)`. Both rules
converted so far factored this way without a fight, and it is what makes a single verdict answerable
on its own.

**Produce the outcome and its evidence in ONE call.** `check.CompareToBound` returns both from one
comparison, and that shape is the point rather than a convenience. A rule that decided the outcome
and then separately assembled a witness would fail nothing when the second step was skipped, and a
pass with no evidence is exactly what this removes.

**An enumerator that drops a subject must say so.** Every step that cannot be taken safely (a pin
that will not resolve, a net with no voltage in its name, a datasheet binding no row of the kind)
used to skip silently, which reports the same nothing as a rule that never looked. Those are
`NOT_CONSIDERED` verdicts carrying the step that stopped them. Distinguish that from a subject that
is simply OUT OF SCOPE: a pin that is not a supply terminal is not a subject of a supply rule, and
yields no event at all.

**One subject, one verdict.** A verdict is keyed by `(rule, kind, ref)`, so a rule that emits two
about one subject produces a duplicate identity. That is invisible while verdicts only project down
to findings and breaks the moment they are addressable. Where several inputs bear on one subject,
reduce them: the per-pin datasheet rules pick the row the design has least margin against, matching
what the part-level rule already did across a part's pins.

### Two traps when converting a rule that has a spec twin

`check.VerdictsToFindings` returns a NON-NIL empty slice, matching `check.Report`. `TestSpecParity`
compares a rule's `Eval` to its declarative twin with `reflect.DeepEqual`, and a nil slice is not
DeepEqual to an empty one, so a converted rule would diverge from its twin on every clean design
while agreeing about every finding.

The twinned rules therefore have three things that must agree: the Go body, the spec twin, and the
verdicts. Converting one is worth doing early to find out whether that is comfortable, because it
affects about a quarter of the catalog.

## A rule has one severity, so a severity axis means two rules

`check.Run` stamps `Finding.Severity` from the rule's own metadata over whatever `Eval` set, and
that is deliberate: a rule states its identity once, and the docsite catalog and `--fail-on` both
read the declared severity as the truth about its findings. So a per-finding severity cannot be set
from inside `Eval`, and a fact that carries its own severity axis becomes TWO rules sharing one
walker.

Two instances, both in the datasheet tier. `pin-exceeds-abs-max` and `pin-out-of-recommended` split
one comparison by `LimitKind`. `pin-tracking-violated` and `pin-tracking-advisory` split one
comparison by the datasheet's `Modality`, because "shall never exceed" and "should be at least 1 V
higher for best operation" are different claims and one severity misstates one of them.

The split is also what a caller wants: separate rule names are what let a team gate CI on the
requirement and merely report the recommendation.

Where a value is legal but leaves the severity unknown, send it to the louder rule and mark the
finding `Inconclusive` rather than dropping it. `param.Validate` requires a pin relation's kind,
bound and provenance but not its modality, so an unstated modality is a legal spec: reporting it as
an error would invent a requirement, and dropping it would pass a breach in silence.

## Comparing is safe, subtracting is not

A rule that SUBTRACTS two name-derived voltages meets binary floating point head-on: `3.3 - 1.8` is
`1.4999999999999998`, which both prints as noise in a finding and misjudges a bound of exactly 1.5.
The older datasheet rules only ever compared a rail against a limit, so the problem first appeared
with pin tracking. Round the result (microvolt resolution is finer than any bound a datasheet
states) and assert the PRINTED number in a test, which is how this one got caught.

## Two independent channels, when one channel is ambiguous by construction

Some questions cannot be answered from one signal no matter how carefully you read it. Is a net named
`..._3V3` a 3.3 V rail, or a signal that swings at 3.3 V? Both are named that way, legitimately, by
the same teams. No naming grammar separates them, and a rule that fires on the name alone is noise
dressed as a finding.

The way out is a **second signal from an unrelated source**, and requiring the two to agree.
`rail-not-classified` wants a voltage in the net's name AND a pin the PART declares as a power input:
one channel comes off the net, the other off the part's own pin declaration, so their agreement is
evidence in a way either alone is not. Measured on real boards that took the candidate set from 101
to 45, and separated a board with a genuine configuration gap (45) from one without (5).

Two things to keep honest about it. **Name the channels in the rule's doc**, because a reviewer
needs to know the finding rests on a coincidence of two weak signals rather than one strong one. And
**check what happens when a channel is structurally absent**: on a source format that cannot type
power pins, the second channel is always missing and the rule silently never fires. That is safe, but
it makes the rule a no-op on that format rather than a partial answer, and the doc should say so.

## Fixtures are the rule's contract

Conformance fixtures are executable expectations. The sidecar drives both the harness and the
viewer's expectations panel. Author three shapes:

- fires: the defect, plus every incidental firing listed, because the harness is exhaustive.
- passes: the same topology done right. `fires: {}` is the strongest assertion the harness holds.
- the guard case: the channel fixture proving the rule stays silent where the convention is
  absent (rails, zero test points, nothing fires).

KiCad authoring details that bite everyone once: pins bind at wire endpoints while labels and
junction dots bind mid-span. The pin connect point is origin + (local_x, −local_y). Power-symbol
fixtures need the `{}` `.kicad_pro` stub or their nets stay external. Expect the showcase boards
to react. They are the load-bearing anti-false-positive gates, so a new rule firing there is a
decision to make deliberately. Cover the passes board (it must stay silent) and list the fires
board's incidental firings in its sidecar.

## Verify at four levels

1. Unit guard matrix: one test that exercises every guard both ways.
2. Conformance: the fixtures above, in CI.
3. Corpus plus the real exports: sweep old-versus-new binaries and attribute every delta
   per-file, per-rule. The corpus alone is not enough. The real exports carry the scale and the
   tool dialects that expose whole classes of false positive (the 1836-finding lesson). Hand-trace
   what fires. The example's five real-board findings decomposed into three genuine gaps and two
   policy-edge feedback nodes, and that decomposition set the severity.
4. A reference implementation, when one exists. Agreement with `kicad-cli sch erc` or
   `kicad-cli pcb drc` is the strongest evidence a rule's semantics are right. It pinned the
   connection-point rules and the naming priorities. Design-for-test has no open reference, so
   say that in the PR.

## Ship it

The PR carries the reviewer's guide, the hardware-context primer, the before/after transcript,
and a record of every deferred edge. The rule's doc is already written, so the catalog, the
viewer, and the next reader all get the explanation the day it merges.
