# 23 — Authoring a check rule

The practical path from "our review checklist says X" to a shipped, trustworthy rule, 
written for a hardware engineer and a software engineer working together, using a real
rule (test-point-coverage, PR for WS3-026) as the worked example at every step. The
architecture behind each step: docs/19 (the rules layer), docs/22 (how nets are solved).
The condensed checklist lives in the workspace router; this is the narrative version.

## 0. Is it a rule?

A rule ASSERTS something about the design as read: "every rail carries a test point".
It does not compute (analysis does), does not fix, and must be checkable from the IR the
readers produce today. Ask what the rule READS, if the answer includes data no reader
emits yet (netclasses, datasheet limits before the params tier), the reader work comes
first and the rule waits.

The example: DFT checklists ask that rails and ground be probe-able. Reads: component
classes (is anything a test point), net membership, rail-ness. All present. It is a rule.

## 1. Write the sentence, then the guards

Every good rule is one sentence plus its exceptions, and the exceptions ARE the rule, 
they are what keeps engineers from muting it. Write the sentence first:

    A power rail or ground net must carry a test-point component.

Then interrogate it the way the corpus will:

- What if the board has NO test points at all? A demo board is not wrong, it just has no
  DFT convention. -> channel gate: fire only when the design places TPs somewhere (the
  design.nc_channel pattern; that gate once took a rule from 1836 findings to 0).
- What is "a rail" when the format carries no direction data? Name is the only rail
  evidence a bare EDIF netlist has. -> rail facts OR rail/ground name heuristics.
- What about nets the read did not fully cover? -> skip external (docs/22; the WS1-017
  external/global lifecycle).
- Who decides the policy edge cases (a feedback node named like a rail, deliberately
  unprobed)? The reviewer. -> severity info, and say so in the doc.

## 2. Author spec-first

Proven vocabulary goes in the Spec AST; anything multi-clause goes behind a registered
SpecFunc that DECLARES its reads and primitives (derivation stays honest through the FFI
boundary). The example is one FFI (has_test_points, the channel) plus existing facts:

    Over: nets
    Where: has_test_points(design)
       and not external(N)
       and (global(N) or power_driven(N) or rail_name(N) or ground_name(N))
       and not exists T in N.connections where class(T) == test_point

Bind with spec.Rule(Rule{...}): Reads and Primitives derive from the body, so they
cannot drift from what the rule does. Two init-order traps, both fixture-taught: register
FFIs inside the rule VAR's initializer (package vars init before init funcs, and binding
validates Call targets), and call registerBuiltinSpecFuncs() first if your spec calls the
shared helpers (rail_name, ground_name).

Twin discipline (docs/19): proven vocabulary -> spec-only (this rule); NEW interpreter
vocabulary (an entity set, a fact, a traversal) -> ship a Go Eval as the canonical twin
until the vocabulary soaks, with parity asserted.

## 3. One file, one line, one doc

- check/rule_<name>.go: the rule.
- check/index.go: one registration line.
- check/docs/<name>.md: the SINGLE source of Detail (embedded; the harness fails CI
  without it). Write it in full, as PROPER ### sections under the rule's ## title (not
  bold run-ins): What it means / Why engineers want it / Impact, an ASCII sketch of
  fires-vs-fine, a Scope note recording every guard decision from step 1, Query
  structure, and a "For software readers" section mapping the EE concepts to structural
  analogies (a test point is a metrics endpoint; the rule is "critical paths must emit
  telemetry") with a diagram beside it in check/docs.

## 4. Fixtures: the rule's contract in files

Conformance fixtures are executable expectations (sidecar drives the harness AND the
viewer's expectations panel). Author three shapes:

- fires: the defect, plus every incidental firing listed (the harness is exhaustive).
- passes: the same topology done right; `fires: {}` is the strongest assertion held.
- the guard case: the channel/gate fixture proving the rule stays silent where the
  convention is absent (tpchan.passes: rails, zero TPs, nothing fires).

KiCad authoring gotchas that bite everyone once: pins bind at wire ENDPOINTS (labels and
junction dots bind mid-span, docs/22); pin connect point = origin + (local_x, −local_y);
power-symbol fixtures need the {} .kicad_pro stub or WS1-017 keeps their nets external.
Expect the showcase boards to react: they are the load-bearing anti-false-positive gates,
and a new rule firing there is a decision to make deliberately; cover the passes board
(it must stay silent) and list the fires board's incidentals in its sidecar.

## 5. Verify at four levels

1. Unit guard matrix: one test, every guard exercised both ways.
2. Conformance: the fixtures above, in CI.
3. Corpus + the real exports: sweep old-vs-new binaries and ATTRIBUTE every delta
   per-file, per-rule. The corpus alone is not enough; the real exports carry the scale
   and the tool dialects that expose false-positive classes (the 1836-finding lesson).
   Hand-trace what fires: the example's five real-board findings decomposed into three
   genuine gaps and two policy-edge feedback nodes, which set the severity.
4. Reference implementation, when one exists: kicad-cli sch erc / pcb drc agreement is
   the strongest evidence a rule's semantics are right (it pinned the connection-point
   rules and the naming priorities). DFT has no open reference; say so in the PR.

## 6. Ship it

The PR carries the reviewer's guide, the hardware-context primer, the before/after
transcript, and files every deferred edge in OUT_OF_SCOPE.md or a ticket. The rule's doc
is already written (step 3), so the catalog, the viewer, and the next reader all get the
explanation the day it merges.
