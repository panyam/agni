# panyam/agni #1190 — Single HTML: checks + query-set tables

Fixtures:

- `demo_board.edn` — two nets, one component
- `project.yaml` — references `queries/demo.yaml`
- `queries/demo.yaml` — lists net entity names

## Desired behavior

One documented export (e.g. `agni report … --format html`) embeds:

1. Check results (catalog run)
2. Query table **Nets** with rows `N1`, `N2`

## Current behavior

Check HTML and query `--set` outputs are separate artifacts.

## Verify (after feature lands)

```bash
cd cmd/agni/testdata/github-issues/1190-combined-html
agni serve --mount m=. --addr :8779 &
agni report mount://m/designs/demo-board/demo_board.edn --remote http://127.0.0.1:8779 \
  --format html -o combined.html
```

Today:

```bash
agni stats --as-named demo_board.edn
```
