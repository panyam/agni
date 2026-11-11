---
title: "Driving an embedded viewer"
description: "Frame the agni viewer in your own page and move it from there, over postMessage, with the same contract a link carries."
---

A page that frames the agni viewer can move it without reloading it. It can open a sheet, focus a
verdict, highlight a net, or apply a whole link's location to the design already on screen. A
review portal, a static check report and an agent's shell all want this, since each keeps its own
page beside the viewer and reloading the viewer throws away the reader's place.

The contract is `ViewerPageService` in `protos/agni/v1/viewer/page.proto`. The viewer page answers it
itself. No server does. Its location message is the viewer's URL state field for field, so a link
and a call open the same sheet with the same highlight. `web/src/navparity.test.ts` holds the viewer
to that.

## Allowing your page

The viewer takes requests from its own origin and from origins its server names, and refuses every
other with `permission_denied`. Start the server with one `--embed-origin` per parent origin, such as
`agni serve --mount demo=./designs --embed-origin https://portal.example`. A value has to be exactly
an origin, meaning a scheme, a host and an optional port, with no path. The viewer compares it with
the sender's origin as a string, so `serve` refuses `https://portal.example/` rather than starting a
viewer that ignores its parent.

One request needs no allowing. The viewer takes an `ApplyLocation` from any page, including a report
opened from disk, when the request carries a `url` and nothing else. Loading that link in the
viewer's window does the same thing, and any page holding the window can already do that. The reply
says only whether it worked, with no location, so a page that was not allowed learns nothing about
the design.

## The envelope

The protocol is one message shape each way. A request names an rpc of `ViewerPageService` and carries
its request in protojson.

```json
{ "agni": "agni-viewer/v1", "id": 1, "method": "LocateVerdict",
  "body": { "id": "demo-pull-up:(net:SDA)", "rule": "demo-pull-up", "subjects": ["net:SDA"] } }
```

The viewer answers the window that asked, at that window's origin, with the same `id`.

```json
{ "agni": "agni-viewer/v1", "id": 1, "ok": true,
  "body": { "resolved": true, "location": { "mount": "demo", "path": "demo-board.kicad_sch", "sheet": "s-power" } } }
```

A failure carries a Connect code and a message instead of a body, such as
`{ "ok": false, "error": { "code": "invalid_argument", "message": "the open design has no sheet \"s-9\"" } }`.

The viewer starts listening once the design in its URL is open, and then posts
`{ "agni": "agni-viewer/v1", "ready": true }` to its parent, so a parent waits for that rather than
guessing when the viewer has booted. Requests are answered one at a time, in the order they arrive.

## The calls

Every response carries the page's location after the call, so a caller learns where the viewer
ended up without asking again.

- `GetLocation` reads where the page is and changes nothing. Its answer also lists the sheet views
  open beside the canvas, each with its id and the sheet it shows.
- `ApplyLocation` moves the page to a location as following a link to it would. A location naming
  another design opens it. A location naming the open design applies to the page as it stands, and
  so does a location naming no design. An empty field leaves its part alone, so a caller sends only
  what it wants changed. The call also takes a `url` in place of a location. The `url` can be any
  link the server minted, and the call applies the fields that link sets.
- `LocateVerdict` focuses one verdict and draws its proof, opening a sheet it is drawn on. The page
  may not have answered the verdict yet. Given a `rule` hint, it then asks that one rule about the
  given subjects first, as it does for a link carrying the same hints.
- `SelectFinding` focuses a finding of the last run, as clicking its row does, except that selecting
  the focused finding again keeps it rather than clearing it.
- `LocateEntity` highlights a net, part or pin, as clicking a query result does.
- `ShowSheet` opens a sheet by its id. It opens the sheet on the canvas, or in the sheet view that
  its `view` names.
- `OpenSheetView` opens a sheet view beside the canvas on one sheet, as the Sheets panel's open
  beside button does. It answers the new view's id, such as `sheetview-2`. A page numbers its views
  from 1 and never reuses a number, so the id of a closed view names nothing. Its `renderer` is
  `svg` or `webgl`. With none, a board view draws in WebGL when the canvas does or the GPU is
  hardware, and any other view draws in the canvas's renderer. The answer says which
  renderer drew the view, since a browser with no WebGL2 draws it in SVG.
- `CloseSheetView` closes one view by its id.

A call can name something the page does not have, such as an unknown verdict or a finding the run
lacks. That call answers `resolved: false` or `selected: false` and leaves the page alone, because a
stale link should not clear what the reader was looking at. `CloseSheetView` answers `closed: false`
for a view that is not open. A sheet the design lacks is `invalid_argument`, and so is a `ShowSheet`
naming a view that is not open.

Say a report link wants the board open beside the schematic. The parent applies the link, then
sends `OpenSheetView` with `sheet: "board"`. The canvas keeps the schematic, and the view beside
it follows whatever the reader selects.

## A parent page

`examples/embed-viewer/parent.html` is a working parent with no build step. Serve its folder from
your origin and open `parent.html?viewer=<a viewer design URL>`. Its `agniCall(method, body)` is the
whole client, about twenty lines, and `web/browser/embed.spec.ts` drives it across two origins in the
browser suite.

A parent written in TypeScript can use the generated Connect client instead.
`web/src/embedclient.ts` builds a `fetch` over postMessage for it to use.

```ts
const transport = createConnectTransport({ baseUrl: "https://viewer.invalid", fetch: frameFetch(frame.contentWindow, viewerOrigin, window) });
const page = createClient(ViewerPageService, transport);
const { location } = await page.showSheet({ sheet: "s-power" });
```

## From a saved report

Every link in a check or checklist report (`--format html`) opens in the window named `agni-viewer`.
With scripts off, clicking one loads it in that window, so a reader working through a report keeps
one viewer tab rather than gaining one per click. With scripts on, the report's own script looks for
that window first, and when it already shows a viewer it sends `ApplyLocation` with the link instead
of loading it, so the viewer keeps the reader's place. If the viewer does not answer within two
seconds, or answers that it could not apply the link, the script loads the link as before. The
script makes no request of its own. A report links into a viewer only when it was written with
`--server`, and only such a report carries the script. A report written with no viewer to link to
is still a plain page with no script at all.

`examples/embed-viewer/two-pane.html` puts a report beside the viewer it links into, in a frame named
`agni-viewer`, and `web/browser/embed.spec.ts` checks that a click there moves the viewer without
reloading it.

## Other transports

postMessage is one way to carry the pair of a method name and a protojson body. The page dispatches
that pair in `web/src/pageservice.ts` whatever carried it, so a server-held session for an MCP tool
(agni issue 1154) and an MCP app host (agni issue 1155) are other carriers over the same dispatcher
and need no handler of their own.
