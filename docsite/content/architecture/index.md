---
title: "Understand it"
description: "How the internals fit together, by subsystem."
---

These pages explain how Agni works, grouped by subsystem rather than by the order the pieces
were built. Read them in any order. Each stands on its own.

- **[Projects and designs](projects-and-designs/)** explains why the engine names a design and a set
  of designs, so "which file is this design" and "whose config applies" stop being guesses.
- **[Ingestion and IR](ingestion-and-ir/)** follows the readers into one neutral representation,
  with its provenance and maturity tiers.
- **[Net solving and hierarchy](net-solving/)** shows how implicit connectivity becomes nets, and how
  the multi-sheet hierarchy walk works.
- **[Geometry and rendering](geometry-and-rendering/)** describes the geometry sidecar and the
  SVG/WebGL renderers.
- **[Semantic diff](semantic-diff/)** compares two revisions by meaning rather than by text.
- **[Rules and checks](rules-and-checks/)** covers the evaluation model and the rules-assert /
  analysis-computes boundary.
- **[Checks contract](checks-contract/)** defines the check-result document, and says why a run's
  evidence is an artifact rather than terminal output.
- **[Datasheet layer](datasheet-layer/)** treats parameters and source documents as contracts,
  and traces their join into checks.
- **[Web app and presenter](web-app/)** covers the browser viewer, the mount model, and the
  presenter contract.
- **[Web service contract](web-services/)** lists the Connect services behind `agni serve` and the
  contract details a caller has to know.
- **[Picking and querying](web-picking/)** explains how a reader names an entity in the viewer and
  what the viewer can then say about it.
- **[Working in the web client](web-client/)** walks through the edits a new panel takes, and the
  traps that ship green in CI and broken in the browser.
- **[Stack and platform](stack/)** records why the Go engine, the proto IR and the TypeScript view
  sit where they do, and where the boundaries between them fall.
