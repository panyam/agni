# diff

Compare two revisions of a design and read the semantic change taxonomy. It is the
walkthrough form of `agni diff <old> <new>`.

## What it shows

- `diff.Designs(a, b)` over the neutral IR, so the diff reports what changed electrically, not
  which bytes moved.
- The net change taxonomy: renamed (same connectivity, new name), hard (connectivity changed),
  soft (attribute-only), new, deleted; and component add / remove / change.
- Matching on semantic keys (net name, the `(ref_des, pin)` connection set), never a
  format-native id, so a re-exported file with regenerated ids still diffs cleanly.

## Run it

```bash
make run        # plain text, interactive
make demo       # TUI styled boxes
make runquiet   # non-interactive defaults (CI-safe)
make doc        # render the walkthrough to markdown
```

Defaults to the bundled `rev-a.edn` / `rev-b.edn` pair, which differs by one change of each
class (a rename, a hard rewire, a new net, a deleted net, plus an added component). Enter your
own two paths to diff any pair.

## How it is built

The narration lives in [`walkthrough.md`](walkthrough.md), loaded by demokit's
`FromMarkdown`. `main.go` uses two [`common.AskPath`](../common/input.go) inputs (`old`, `new`)
and binds the `old` / `new` / `run` steps (`diff.Designs(a, b).Render`). See
[`../CONVENTIONS.md`](../CONVENTIONS.md) for the shared layout.
