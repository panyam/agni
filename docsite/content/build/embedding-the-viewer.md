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
and a call land the same way, on the same sheet with the same highlight, and
`web/src/navparity.test.ts` holds the viewer to that.

## Allowing your page

The viewer takes requests from its own origin and from origins its server names, and refuses every
other with `permission_denied`. Start the server with one `--embed-origin` per parent origin, such as
`agni serve --mount demo=./designs --embed-origin https://portal.example`. A value has to be an origin
exactly, scheme and host and an optional port with no path, because the viewer compares it with the
sender's origin as a string, so `serve` refuses `https://portal.example/` rather than starting a
viewer that ignores its parent.

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

Every response carries the page's location after the call, which tells a caller where the viewer
landed without asking again.

- `GetLocation` reads where the page is and changes nothing.
- `ApplyLocation` moves the page to a location as following a link to it would. A location naming
  another design opens it. One naming the open design, or none, applies to the page as it stands,
  and an empty field leaves its part alone, so a caller sends only what it wants changed.
- `LocateVerdict` focuses one verdict and draws its proof, opening a sheet it is drawn on. With a
  `rule` hint a verdict the page has not answered yet is asked first, one rule about the given
  subjects, as a link carrying the same hints is.
- `SelectFinding` focuses a finding of the last run, as clicking its row does, except that selecting
  the focused finding again keeps it rather than clearing it.
- `LocateEntity` highlights a net, part or pin, as clicking a query result does.
- `ShowSheet` opens a sheet by its id.

A call that names something the page does not have (an unknown verdict, a finding the run lacks)
answers `resolved: false` or `selected: false` and leaves the page alone, because a stale link should
not clear what the reader was looking at. A sheet the design lacks is `invalid_argument`.

## A parent page

`examples/embed-viewer/parent.html` is a working parent with no build step. Serve its folder from
your origin and open `parent.html?viewer=<a viewer design URL>`. Its `agniCall(method, body)` is the
whole client, about twenty lines, and `web/browser/embed.spec.ts` drives it across two origins in the
browser suite.

A parent written in TypeScript can use the generated Connect client instead, through the `fetch`
that `web/src/embedclient.ts` builds over postMessage.

```ts
const transport = createConnectTransport({ baseUrl: "https://viewer.invalid", fetch: frameFetch(frame.contentWindow, viewerOrigin, window) });
const page = createClient(ViewerPageService, transport);
const { location } = await page.showSheet({ sheet: "s-power" });
```

## Other transports

postMessage is one way to carry the pair of a method name and a protojson body. The page dispatches
that pair in `web/src/pageservice.ts` whatever carried it, so a server-held session for an MCP tool
(agni issue 1154) and an MCP app host (agni issue 1155) are other carriers over the same dispatcher
and need no handler of their own.
