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

- [CONSTRAINTS.md](CONSTRAINTS.md) — the enforceable architectural rules (C1–C21). They keep
  the engine format-neutral and the layering clean. Read them before proposing a change; a PR
  that violates one will be asked to change or to justify amending the constraint.
- [Docs site overview](https://panyam.github.io/agni/overview/) — the engineering docs. The
  architecture is the settled part of the project.

## Common contributions

- **A new format reader.** Each reader is its own package exposing
  `Read(io.Reader, sourceFile) (*ir.Design, error)`, wired in with one entry in
  `readers/formats/registry.go`. See the reader notes in [Ingestion and IR](https://panyam.github.io/agni/architecture/ingestion-and-ir/)
  and reconcile new concepts against the cross-format map (CONSTRAINTS C9). Ship a runnable
  example with it (CONSTRAINTS C10).
- **A new check rule.** One `stdlib/rules/builtin/rule_<name>.go`, one line in
  `stdlib/rules/builtin/register.go`, and one `stdlib/rules/builtin/docs/<name>.md` (the source
  of the rule's prose, enforced 1:1 by `stdlib/rules/builtin/docs_test.go`). The practical
  walkthrough is [Authoring a check rule](https://panyam.github.io/agni/build/check-rule/).
- **A datalog query relation or example.** See [Querying](https://panyam.github.io/agni/guide/querying/).
- **An interface profile.** A profile is a data value, not code: a YAML declaration of an
  interface's signals and the checks it requires, compiled into datalog rules. Built-ins live in
  `stdlib/profiles/builtins/*.yaml`; an out-of-tree one loads through `agni check --profile-path
  <dir>`. A signal declares exactly one net-name matcher — `suffix` (optionally narrowed by a
  conjunctive `prefix`), `glob`, or `regex` — documented in `stdlib/profiles/matcher.go`. Reach for
  `suffix` first; it is the readable convention. Reach for `glob` or `regex` only when the bus
  identity is the prefix and the suffix is shared with an unrelated bus, which is the one case
  suffix matching cannot express.

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
