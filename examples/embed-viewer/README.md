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

## An agent driving the tabs you have open

The same calls reach a viewer tab from an agent over MCP, with no parent page at all (agni issues
1154, 1173). Start a server with viewer rooms and point Claude Code, or any MCP client, at its
`/mcp`:

```bash
go run ./cmd/agni serve --mcp --viewer-rooms --addr 127.0.0.1:8080 --web-dir web --mount tut=examples/tutorial-project
claude mcp add --transport http agni http://127.0.0.1:8080/mcp
```

Then walk it in this order:

1. Open `http://127.0.0.1:8080/designs/tut/designs/gateway/view` in two tabs. Each shows "Let agents
   drive this tab" beside the sheet tabs, off.
2. Tick the switch in the second tab only.
3. Ask the agent to show the gateway's board sheet. It calls `viewer_show_sheet` with
   `design: tut/designs/gateway`, and the second tab opens the Board sheet without reloading. The
   first tab stays where it was.
4. Ask it where the viewer is. `viewer_get_location` answers with the second tab's design and sheet.
5. Ask it to run the checks and show you a net with a copper clearance error. It calls `check`, which
   names `CAN1_CANL` among others, then `viewer_locate_entity` with `kind: net` and that net as
   `subject`, and the tab lights the net.
6. Tell the agent to wait for your question in the viewer. It calls `next_request` with
   `design: tut/designs/gateway`. Click a net in the second tab, type "what else is on this net?" into
   the Ask box beside the switch, and press Ask. The agent hears the question with the tab's sheet and
   the selected net, so it can answer with a `query` on that net without asking which one you mean.
7. Untick the switch and ask the agent to move the tab again. The call comes back refused, saying no
   tab has agreed, and the tab does not move. The Ask box disappears with the switch.

A tab answers the server's room bridge alone, so another page holding a relay token for the same room
cannot call it directly. `web/browser/viewerroom.spec.ts` runs this walk with real tabs on both MCP
wires, and checks that a second page on the server's origin with a genuine token gets no answer.
`make exercise-viewer-page` runs it beside the postMessage parent.

## How it is built

`parent.html` is the whole client: it waits for the viewer's `ready` message, then posts
`{ agni: "agni-viewer/v1", id, method, body }` and matches each reply by its `id`. The protocol is
described in `docsite/content/build/embedding-the-viewer.md`, and `web/browser/embed.spec.ts` drives
this page across two origins in the browser suite.
