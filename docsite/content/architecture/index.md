---
title: "Understand it"
description: "How the internals fit together, by subsystem."
---

These pages explain how Agni works, grouped by subsystem rather than by the order the pieces
were built. Read them in any order; each stands on its own.

- **[Ingestion and IR](ingestion-and-ir/)** &mdash; readers into one neutral representation, with
  provenance and maturity tiers.
- **[Net solving and hierarchy](net-solving/)** &mdash; how implicit connectivity becomes nets, and how
  the multi-sheet hierarchy walk works.
- **[Geometry and rendering](geometry-and-rendering/)** &mdash; the geometry sidecar and the SVG/WebGL
  renderers.
- **[Semantic diff](semantic-diff/)** &mdash; comparing two revisions at the level of meaning, not text.
- **[Rules and checks](rules-and-checks/)** &mdash; the evaluation model and the rules-assert /
  analysis-computes boundary.
- **[Datasheet layer](datasheet-layer/)** &mdash; parameters and source documents as contracts, and the
  join into checks.
- **[Web app and presenter](web-app/)** &mdash; the browser viewer and the presenter contract.
- **[Stack and platform](stack/)** &mdash; the Go engine, the proto IR, and the boundaries.
