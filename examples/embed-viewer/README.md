# embed-viewer

A parent page that frames the agni viewer and moves it over postMessage, through the
`ViewerPageService` contract the viewer page answers itself (agni issues 1150, 1152).

## Run it

From the repo root, after `make ui`, start a viewer that takes requests from the origin you will
serve this folder from, then serve the folder and open the page with the viewer's design URL:

```bash
go run ./cmd/agni serve --addr 127.0.0.1:8080 --web-dir web --mount kicad=readers/kicad/testdata --embed-origin http://127.0.0.1:9000
python3 -m http.server 9000 --bind 127.0.0.1 --directory examples/embed-viewer
open "http://127.0.0.1:9000/parent.html?viewer=http://127.0.0.1:8080/designs/kicad/hier_numeric_root.kicad_sch/view"
```

Press Get location, paste a sheet id into the sheet box, and press Show sheet. The viewer moves
without reloading, and the reply shows where it landed.

## A report beside the viewer

`two-pane.html` frames a viewer and a saved check report side by side. Write the report against the
viewer's server and open the page with both:

```bash
go run ./cmd/agni check readers/kicad/testdata/hier_numeric_root.kicad_sch --mount kicad=readers/kicad/testdata --server http://127.0.0.1:8080 --verdicts --format html -o examples/embed-viewer/report.html
open "http://127.0.0.1:9000/two-pane.html?viewer=http://127.0.0.1:8080/designs/kicad/hier_numeric_root.kicad_sch/view&report=report.html"
```

Clicking a subject in the report moves the viewer beside it without reloading it, since the viewer's
frame is named `agni-viewer`, the window every report link targets. Delete `report.html` afterwards,
since a report written from your own design is that design's data.

## How it is built

`parent.html` is the whole client: it waits for the viewer's `ready` message, then posts
`{ agni: "agni-viewer/v1", id, method, body }` and matches each reply by its `id`. The protocol is
described in `docsite/content/build/embedding-the-viewer.md`, and `web/browser/embed.spec.ts` drives
this page across two origins in the browser suite.
