# Out of scope — the deferred-work ledger

One line per item a PR deliberately deferred without a roadmap ticket. The contract:

- A PR that lists something under "Out of scope" adds it here in the same PR.
- An entry names the source PR (plain text, no backlink) and the natural pickup trigger.
- Remove the entry when the item lands or gets promoted to a roadmap ticket. Items that
  need evidence tracking, affect correctness, or have no adjacent future work get a
  ticket instead of a line here.
- **An entry earns its place only if nobody could pick it up deliberately.** That is the test.
  If someone could schedule it, it is a ticket. If it can never be picked up at all because
  the question is settled, it is a decision and belongs in `DECISIONS.md`. What is left is the
  narrow middle: work that becomes obvious and cheap the moment an unrelated trigger fires, and
  is not worth anyone's attention before then — "rename this when you are next in this file",
  "add the fixture when a design that needs it turns up".
- **Sweep it periodically.** This file is only ever appended to in the course of normal work,
  so it drifts in two directions on its own: entries stay after the thing they asked for is
  built (a stale line reads as current, which is worse than a long list), and one piece of
  work gets written down many times as several PRs each defer their own slice of it. Both
  need a read-through to see; neither is visible one entry at a time. The 2026-08 sweep took
  108 entries to 15, promoting eighteen clusters and singletons to tickets, moving three
  settled questions to `DECISIONS.md`, and deleting what had already landed.
- **Nothing private.** This file is world-readable. A pickup trigger that cannot be stated
  without naming a customer, a private path, or a corpus location belongs in the private
  workspace, not here. Sanitize when writing the line, not later; deleting it afterwards does
  not remove it from history.

| Item | From | Pick up when |
|------|------|--------------|
| Details-panel summary names the sheet the design OPENED on and never updates as you navigate sheets, so it reads stale ("Hier root · faithful" while amp1 is shown). Pre-existing; the WS9-049 tab strip makes it visible by putting the active sheet name right beside it | WS9-049 PR1 | someone re-does the Details panel, or a user reports the mismatch; the fix is to push the summary per sheet rather than per design load |
| `Validate` RPC serving `webapi.ValidateReport` to the viewer | PR 83 | WS9 wants a validation panel; the message shape is ready |
| `.edn` content sniffing, per-format validate knobs, unresolved-vs-empty cause split | PR 83 | ticketed as WS6-008 (listed here only until it lands) |
| Move `svg/` under `internal/` | PR 69 | next PR touching `svg/` |
| Rename `edif.ReadSchematic` to `ReadSchematicGeometry` (match the kicad naming) | PR 69 | next PR touching the edif reader surface |
| Tests for the remaining render-only islands (controlbar, expectationspanel) | PR 67 | next web PR; scaffolding exists; findingspanel landed in PR 84 |
| Corpus-scale parse benchmarks | PR 67 | a perf question actually arises; fixture benchmarks are the tripwire |
| `stats --format json` (canonical wire shape question; `diff --format json` landed with WS9-004) | PR 68 | someone needs machine-readable stats; decide the proto first |
| WebGL rendering for the visual diff (resolve sideSpecs locally over packed keys) | WS9-005 PR | WS9-007 overlay work, or when WebGL fidelity matches SVG |
| Conformance fixtures for ipc2581 / xschem / geda source formats | PR 65 | those readers gain geometry-bearing fixtures worth asserting rules over |
| WS3-072: the remaining per-rule `isPowerRailName`/`isGroundName`/`isFeedbackName` call sites (reach.go, pinrole.go, the rule Evals) still call the name lexicon; only the net.ground/feedback/IsPowerRail projectors read the stamped `net.role`. Mechanical, behavior-preserving (stamped = name-derived in PR1) | WS3-072 PR1 | migrate a call site to `netHasRole` when it is next touched; the FFIs (`rail_name`/`ground_name`/`feedback_name`) operate on a bare name literal with no net binding, so they keep the name lexicon by design |
| WS9-035 params panel reads via a bespoke `GetComponentParams` RPC (returns the structured `PartSpec`) rather than the datalog query surface (`component.mpn` ⋈ the WS10-010 `param(...)` relations). Considered the query route; kept the RPC because the panel's tree needs the full nested spec — conditions, provenance/citation, limit-kind, min/typ/max — and the flat `param(...)` relations do not carry all of that yet | WS9-035 PR 313 | WS3-082 enriches `param(...)` with min/limit_kind (and ideally conditions/provenance); then the panel can migrate to a datalog query over the existing QueryService and drop the RPC + adapter. Separately: point serve's provider at a real service-backed `param.ParamProvider` (the WS10-010 seam) instead of only `--params <dir>` |
| WS3-107: the services still hold a BUILT `*check.Catalog` rather than the sources that composed it, so `Overlay.Catalog` extends a catalog it cannot re-derive. `Catalog.With` makes the drop impossible at the one site that had it, but the shape still permits a future caller to rebuild and lose the base | WS3-107 PR | the ticket's preferred fix: thread a "catalog inputs" value (`[]check.RuleSource`) to every surface so a catalog is always composed from its inputs and this class of drop is structurally impossible. It is a signature change across ~20 call sites, mostly tests, which is why it was kept out of a P1 bug fix |
| WS3-092: `intent.DocRules` stamps every intent doc page `severity: warning`, so the docsite catalog understates the three intent rules whose runtime severity is `error` (`rail-current-capacity` from WS3-095, `sequence-<slug>` here, `load-switch-trip-below-budget` from WS3-085). The runtime findings carry the right severity; only the generated reference page is wrong | WS3-092 PR | the doc-rule projection is touched for any reason; carry the per-kind severity in a map beside `docSummaries` rather than hardcoding one value |
