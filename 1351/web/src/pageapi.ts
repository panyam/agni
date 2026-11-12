import type { ViewerLocation } from "./router.js";
import type { ViewerPresenter } from "./viewer.js";

// ViewerPageAPI lists what a host may ask of a LIVE viewer page (agni issue 1150). A host can move
// the page to a location, focus a verdict or a finding, highlight an entity, or open a sheet. A
// link (ViewerLocation in the URL) is the cold form of the same requests and applyLocation is its
// hot form, so a link and a call open the same sheet with the same highlight. navparity.test.ts
// holds the three ways in (a link, a click, a call) to that.
//
// It is the TypeScript face of ViewerPageService (protos/agni/v1/viewer/page.proto). pageservice.ts
// dispatches that service over it for any transport. Today the transport is postMessage from a
// parent page (embed.ts). Later it can be a server-held session or an MCP app host (agni issues
// 1154, 1155).
//
// Every method that moves the page reports its new location to the host, which writes it into the
// address bar (the presenter's syncLocation), so after a call the URL is the link to what is shown.
// A method that cannot act on its input leaves the page as it was.
export interface ViewerPageAPI {
  // location is where the page is now, every field filled as the URL would spell it. It changes
  // nothing.
  location(): ViewerLocation;

  // applyLocation moves the page to a location as following a link to it would. When the location
  // names another design (mount and path) or another `flatten`, the page opens it. When it names
  // the open design or none, it applies to the page as it stands. There an empty string, an empty
  // list or an undefined field leaves its part alone, so a caller sends only what it wants changed.
  // Applying the location the page is at changes nothing.
  //
  // The location can carry a `verdict` with `rule` and `subjects` hints. If the page has not
  // answered that verdict, it first runs the rule about those subjects, as a cold link does. It
  // resolves to whether that verdict was found and drawn, false when the location named none. It
  // rejects when no design is open and the location names none, or when it names a sheet the open
  // design lacks.
  applyLocation(loc: Partial<ViewerLocation>): Promise<{ location: ViewerLocation; verdictResolved: boolean }>;

  // locateVerdict focuses one verdict by its id (ViewerLocation.verdict, as `agni check --verdicts`
  // prints it), opening a sheet it is drawn on when the shown sheet has none of its subjects, and
  // draws its proof. When `rule` is given, with `subjects` optional, the page first asks about a
  // verdict it has not answered, as it does for a link carrying the same hints. It resolves false
  // for an id it could not find, leaving the page as it was. Calling it again with the same id
  // redraws the same proof.
  locateVerdict(id: string, hints?: { rule?: string; subjects?: string[] }): Promise<boolean>;

  // selectFinding focuses a finding of the last run by its subject (a net name, a ref-des), as
  // clicking its row does, opening `sheet` when given and otherwise the first sheet it is drawn on
  // when the shown sheet has none. `netId` picks one of two nets sharing a name. Unlike a second
  // click on a row, selecting the finding already focused does not clear it. It resolves false when
  // the run has no such finding, leaving the page as it was.
  selectFinding(subject: string, opts?: { sheet?: string; netId?: string }): Promise<boolean>;

  // locateEntity highlights a net, part or pin over the current findings, as clicking a query cell
  // does, opening `sheet` first when given. kind is "net", "component" or "pin", with `pin` the pin
  // designator for a pin. An entity the design does not draw highlights nothing.
  locateEntity(kind: string, subject: string, opts?: { pin?: string; sheet?: string }): Promise<void>;

  // showSheet opens one sheet of the open design by its id. It rejects for an id the design lacks.
  showSheet(id: string): Promise<void>;
}

// pageApi narrows a presenter to the page contract. It holds no state of its own, so any number of
// transports can share one.
export function pageApi(p: ViewerPresenter): ViewerPageAPI {
  return {
    location: () => p.location(),
    async applyLocation(loc) {
      const verdictResolved = await p.applyLocation(loc);
      return { location: p.location(), verdictResolved };
    },
    locateVerdict: (id, hints) => (id ? p.resolveVerdict(id, hints?.rule ?? "", hints?.subjects ?? [], !!hints?.rule) : Promise.resolve(false)),
    async selectFinding(subject, opts) {
      const netId = opts?.netId ?? "";
      if (!subject || !p.hasFinding(subject, netId)) return false;
      const focused = p.focusedFinding();
      // selectFinding toggles the focus off on a repeated click with no sheet. A call is a request
      // for a state, so the focused finding is kept, on the sheet it is on.
      if (focused.subject === subject && focused.netId === netId && !opts?.sheet) return true;
      await p.selectFinding(subject, opts?.sheet, netId);
      return true;
    },
    async locateEntity(kind, subject, opts) {
      if (opts?.sheet && !p.hasSheet(opts.sheet)) throw new Error(`the open design has no sheet "${opts.sheet}"`);
      await p.locateEntity(kind, subject, opts?.sheet, undefined, opts?.pin ?? "");
    },
    async showSheet(id) {
      if (!p.hasSheet(id)) throw new Error(`the open design has no sheet "${id}"`);
      await p.showSheet(id);
    },
  };
}
