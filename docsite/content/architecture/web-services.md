---
title: "Web service contract"
description: "Which Connect service behind agni serve owns which RPC, and the contract details a caller has to know."
---

This page is the reference for the wire contract of `agni serve`. How those calls reach the engine,
and how a design reaches the browser in the first place, is on
[Web app and presenter](../web-app/).

## The services

Everything is Connect, proto-first, in JSON or binary, with a generated TypeScript client. The
split follows resource lifetimes. Workspace navigation, one design's rendering, checks over a
design, the two-design diff, ad-hoc datalog search, stored reviews, declared projects and datasheet
documents are independent concerns with independent cadences.

| Service | RPC | What it does |
|---|---|---|
| Workspace | ListMounts | the mount names a tree roots on; `opens` drops the ones holding nothing that client can open and returns how many, so the sidebar can account for a mount an operator configured and cannot find |
| Workspace | ListDir | one directory level, each file labeled with its reader `format` and the `kind` of client that opens it (design, datasheet, or neither); `opens` declares what the caller can open, which drops folders with none of it anywhere beneath them (a bounded server-side walk, since one level of listing cannot see that far) and is what lets the two trees prune the same mounts to opposite answers |
| Design | GetDesign | load and summarize one design: sheet list, effective layout, available layouts, native availability |
| Design | GetSheet | one rendered sheet, where `format` picks PACKED (columnar bytes for WebGL), SVG (the verification backend), or NATIVE (the format's own tool) |
| Design | HighlightSheet | resolve highlight spec layers against one sheet: PACKED yields primitive-index groups, SVG a transparent same-frame overlay document |
| Design | GetLayoutReport | how an auto-layout drew each component (glyph, box, provided symbol, or unresolved) |
| Design | TraceDesign | walk from one pin to another through the series parts between them and return the route, the same answer `agni trace` prints |
| Check | ListRules | the rule catalog with tags and per-design availability, static per build, fetched once |
| Check | CheckDesign | run a rule subset and return findings, where each subject joins the packed primitive keys for highlighting |
| Check | GetExpectations | the design's expectation sidecar as its own resource, reconciled against findings client-side |
| Check | GetCheckReport | the severity-organized report, the same shape as `agni check --format report` |
| Check | GetInterfaceCoverage | per detected interface profile, each required signal with its matched net and state, from the same datalog the profile rules compile to |
| Check | GetComponentParams | every component whose MPN resolves to a seeded PartSpec, with that spec's parameters; empty when no corpus is loaded |
| Check | GetNamingConvention | resolve a stored convention config into a value an OverlayConfig carries, parsed and validated |
| Diff | DiffDesigns | semantic diff of two designs plus the highlight maps, the wire form shared with `agni diff --format json` |
| Query | RunQuery | evaluate an ad-hoc datalog query over the design's fact base, returning columns and provenance-linked rows, the same engine as `agni query` |
| Query | RunQueries | a named set of queries over one read of the design, each result shaped as RunQuery would return it |
| Query | ListRelations | the relation catalog with arg labels, summary, and kind, driving the panel's click-to-insert picker |
| Review | GetReviewManifest | resolve a stored checklist into a manifest value, parsed and validated |
| Review | CreateReview | run a checklist against one design and store the result, returning the stored run |
| Review | GetReview | one stored run, by resource name |
| Review | ListReviews | stored runs newest first, paginated, filterable by design |
| Review | DeleteReview | remove a stored run |
| Project | GetProject, ListProjects | the declared projects visible across the server's mounts |
| Project | GetDesign, ListDesigns | a declared design's identity and which file around it is which (a different question from Design.GetDesign) |
| Project | ResolveDesign | whether a file belongs to a declared design, and which; resolving to nothing is a normal answer |
| Datasheet | GetDocument, ExtractDocIR | the doc-IR for a datasheet, and running the configured `--pdf2doc` producer to derive it |
| Datasheet | GetPartSpec, SavePartSpec | the datasheet's shared PartSpec, saved with optimistic concurrency |
| Datasheet | GetAnnotations, SaveAnnotations | the per-author region annotations, read as the union of every author's overlay |

A few contract details bite if missed.

- **Effective values echo back.** GetDesign returns the layout actually used, since a request for
  an unavailable layout resolves rather than erroring, and the client adopts the echo. The same
  holds for the sheet id. The client navigates by the ids GetDesign returned, never by index
  guesses, because sheet ids are non-numeric on purpose and a numeric selector means a positional
  index.
- **A highlight must mirror its sheet.** An overlay is only meaningful over the base render it was
  framed for, so the sheet, layout, and symbols in a HighlightSheet call must match the GetSheet
  it overlays.
- **The board is a sheet.** A `.kicad_pcb` renders as a synthetic "board" sheet, so
  navigation, deep links, and both highlight paths needed no board-specific client plumbing.
- **Diff is all-or-nothing.** If either side fails to load, the call fails. There is no partial
  diff.
- **Query evaluates on the server over netlist facts.** RunQuery loads the design, builds the
  model, and runs the same evaluator the CLI runs. The datasheet parameter relation answers only
  when the server has a corpus, from `agni serve --params <dir>` or the design's own project
  `params/`, and returns no rows otherwise. The evaluator is the dependency-free Go of the `jaala`
  datalog module, so a later revision could evaluate it in the browser instead.
- **A design is named, a checklist is sent.** CreateReview carries the review manifest as a value
  while the design stays an artifact URI. A design is megabytes, needs a reader chosen by
  extension, and is re-requested across many calls, so the call names it and the server reads it
  from the mount. A checklist is a small declaration the caller already holds, and a service
  that took a path for it would need a filesystem to do its job. GetReviewManifest is the bridge for
  a client that holds a URI and no filesystem. The server reads and validates the checklist once,
  and the client sends back the value it got. The CLI skips it, because reading the file the user named is its own job.
- **Reviews and projects are resources, and everything else is a verb.** A review RUN outlives
  the call that made it, so it has a name and the four standard methods, with paging and filtering
  following AIP. A project and its designs carry names an operator declared, so they get `Get` and
  `List` and no mutators. Every other rpc here is addressed by an artifact URI, so its arguments are
  its whole identity and a resource name would be ceremony. That split is CONSTRAINTS C23.
- **Stored runs need a volume.** `agni serve --review-store <dir>` names a WRITABLE directory,
  separate from the design mounts, so persisting a run never writes into a mount. In a container it is a mounted volume. Without the flag the four review resource methods
  answer with a failed-precondition naming it, rather than running the checks and dropping the
  result. Runs stored there are visible to every client of the server; `agni serve` has no
  authentication yet.
- **The viewer can ask under its own vocabulary.** A naming convention is picked from the mount,
  resolved server-side into a value, and carried on every rule-running request as an `OverlayConfig`.
  It REPLACES the server's `--conventions` default for that request rather than adding to it, so the
  top bar names which vocabulary produced the answers on screen. Without that indicator a reader
  cannot tell a fixed design from a stopped rule, because replacement can stop a rule running and a
  rule that stops running produces no findings.
- **A stored run embeds the checklist it scored.** The document carries a manifest SNAPSHOT, not
  just the manifest's name. A checklist is an editable file, so a name would resolve to whatever it
  says today, and last quarter's review would re-render against this quarter's questions with its
  outcomes intact underneath.
