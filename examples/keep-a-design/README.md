# keep-a-design

Keep designs on a server: store two revisions of a board as designs of one series, group them in a
project with a design from a mounted folder, and see what a mounted folder will not let you change
(agni issues 1694, 1696). The walkthrough form of `agni serve --store-dir` and the `DesignService` and
`ProjectService` calls a client makes against it.

## What it shows

- A design is named `designs/{design}`, holds artifacts such as a netlist or a board, and is tied to no project.
- A client sends a manifest of hashes first, and only the contents the server asks for.
- Revisions of one board are designs of one `series`, and the server marks the latest.
- A project is a list of design names, from any store, and deleting it deletes no design.
- A design in a mounted folder is read-only.

## Run it

```bash
make run        # plain text, interactive
make demo       # TUI styled boxes
make runquiet   # non-interactive defaults (CI-safe)
make doc        # render the walkthrough to markdown
```

## How it is built

The narration lives in [`walkthrough.md`](walkthrough.md). [`tour.go`](tour.go) builds the services
the way `agni serve --store-dir` does (`cmd/agni/storeservices.go`), over a temporary folder and the
bundled [`../common/designs/demo-project`](../common/designs/demo-project) mounted read-only.
[`main.go`](main.go) binds the five steps. [`tour_test.go`](tour_test.go) runs the same calls and
checks their answers.
