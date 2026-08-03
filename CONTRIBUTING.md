# Contributing to agni

Thanks for your interest. This is an early open-source project and contributions are
welcome, especially readers for new formats and check rules.

## Getting set up

Requires Go 1.26 and pnpm (for the web viewer bundle).

```
cd web && pnpm install && cd ..   # once
make build                        # web bundle + go build ./...
make testall                      # the full gate CI runs: vet, tests, bundle, web unit tests
```

`make testall` is the gate. A green run means vet passed, the engine tests passed, the
example modules built, the web bundle built, and the web unit tests passed. CI runs exactly
this, so run it locally before opening a pull request.

## Read this first

- [CONSTRAINTS.md](CONSTRAINTS.md) — the enforceable architectural rules (C1–C19). They keep
  the engine format-neutral and the layering clean. Read them before proposing a change; a PR
  that violates one will be asked to change or to justify amending the constraint.
- [Docs site overview](https://panyam.github.io/agni/overview/) — the engineering docs. The
  architecture is the settled part of the project.

## Common contributions

- **A new format reader.** Each reader is its own package exposing
  `Read(io.Reader, sourceFile) (*ir.Design, error)`, wired in with one entry in
  `formats/registry.go`. See the reader notes in [Ingestion and IR](https://panyam.github.io/agni/architecture/ingestion-and-ir/)
  and reconcile new concepts against the cross-format map (CONSTRAINTS C9). Ship a runnable
  example with it (CONSTRAINTS C10).
- **A new check rule.** One `check/rule_<name>.go`, one line in `check/index.go`, and one
  `check/docs/<name>.md` (the source of the rule's prose, enforced 1:1 by
  `check/docs_test.go`). The practical walkthrough is
  [Authoring a check rule](https://panyam.github.io/agni/build/check-rule/).
- **A datalog query relation or example.** See [Querying](https://panyam.github.io/agni/guide/querying/).

## Expectations for a pull request

- Tests. New behavior gets a test; readers get hand-authored fixtures in the package's
  `testdata/`.
- Docs. A new rule needs its `check/docs/<name>.md`; a user-facing change updates the
  relevant page under `docsite/content/`.
- Keep it format-neutral. Analyses read the IR, not source files. Don't add an IR field only
  one format would populate (CONSTRAINTS C9).
- Prose style in docs, commit messages, and PR text: plain declarative sentences, no
  marketing cadence, no em-dashes.

## Reporting bugs and requesting formats

Open an issue. For a bug, a small design file that reproduces it helps a lot (a hand-authored
fixture rather than a confidential design). For a new format, a pointer to the spec and a
sample export is the best start.
