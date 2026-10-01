# extension-template

This is a **minimal, copyable scaffold** for a private extension, meaning a Go module that extends the
public agni engine with your own format reader and rules, without forking it. Unlike
[`examples/extension`](../extension/README.md) (a worked demonstration), this is a bare starting
point full of `TODO:` markers.

The step-by-step walkthrough is the docsite page **Extending and embedding the engine** (`docsite/content/build/extending.md`).
The short version:

1. Copy this whole directory out of the repo.
2. In `go.mod`: rename the module, **delete the `replace`**, and pin a published engine version.
3. Fill in `myfmt/` (your format reader → `formats.Register`) and `myrules/` (your rules →
   `check.RegisterSource`), renaming the packages and the import paths in `main.go`.
4. `go build ./... && go test ./...`.

```
$ go run .
usage: agni-extension <design-file>
registered custom format: true
registered custom rules: 1 in the catalog
```

The catalog holds one rule because the template registers its own and does not import the
built-in catalog (`stdlib/rules/builtin`).

Dependencies point **extension → engine only**, and the engine never imports your extension
(CONSTRAINTS C18).
