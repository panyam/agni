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

## Author spec-first

Proven vocabulary goes in the Spec AST. Anything multi-clause goes behind a registered SpecFunc
that declares its own reads and primitives, so the derivation stays honest across that boundary.
The example is one FFI (`has_test_points`, the channel gate) plus existing facts:

    Over: nets
    Where: has_test_points(design)
       and not external(N)
       and (global(N) or power_driven(N) or rail_name(N) or ground_name(N))
       and not exists T in N.connections where class(T) == test_point

Bind it with `spec.Rule(Rule{...})`. `Reads` and `Primitives` derive from the body, so they
cannot drift from what the rule actually does. Two init-order traps, both learned from fixtures:
register FFIs inside the rule variable's own initializer (package variables initialize before
`init` funcs run, and binding validates the Call targets), and call `registerBuiltinSpecFuncs()`
first if your spec calls the shared helpers like `rail_name` or `ground_name`.

The twin discipline: proven vocabulary is spec-only, as here. New interpreter vocabulary (a new
entity set, a new fact, a new traversal) ships with a Go `Eval` as the canonical twin until the
vocabulary soaks, with parity asserted between the two.

## One file, one line, one doc

- `check/rule_<name>.go`: the rule.
- `check/index.go`: one registration line.
- `check/docs/<name>.md`: the single source of the rule's `Detail`, embedded at build time. The
  harness fails CI without it. Write it in full as proper `###` sections under the rule's `##`
  title, not bold run-ins: What it means, Why engineers want it, Impact, an ASCII sketch of
  fires-versus-fine, a Scope note recording every guard decision from the step above, the query
  structure, and a "For software readers" section mapping the EE concepts to structural analogies
  (a test point is a metrics endpoint, and the rule reads as "critical paths must emit telemetry")
  with a diagram beside it.

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
   policy-edge feedback nodes, which is what set the severity.
4. A reference implementation, when one exists. Agreement with `kicad-cli sch erc` or
   `kicad-cli pcb drc` is the strongest evidence a rule's semantics are right. It pinned the
   connection-point rules and the naming priorities. Design-for-test has no open reference, so
   say that in the PR.

## Ship it

The PR carries the reviewer's guide, the hardware-context primer, the before/after transcript,
and a record of every deferred edge. The rule's doc is already written, so the catalog, the
viewer, and the next reader all get the explanation the day it merges.
