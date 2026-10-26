// The viewer's app events: what a reader did, reported once the viewer has done it. A guided tour
// advances on them (reviewtour.ts, over tsappkit's Tour), so an action the viewer reports no event
// for cannot be a tour step, and the fix is an event here rather than a listener inferring intent
// from the DOM (panyam/goapplib#128). They travel on the page's EventBus under these names.

import type { EventBus } from "@panyam/tsappkit";

export interface ViewerEvents {
  // A checklist run finished. checklist is the picker's label for it; ok is false when the run
  // failed or the reader cancelled it.
  "checklist.ran": { checklist: string; ok: boolean };
  // A checklist page was saved.
  "report.saved": { checklist: string };
  // A check run finished, with how many findings the list shows.
  "checks.ran": { findings: number; cancelled: boolean };
  // A finding was clicked and its subject located.
  "finding.selected": { rule: string; subject: string };
  // A query set opened as tabs, one per query.
  "queryset.opened": { title: string; queries: number };
  // A cell of a query answer (or one of its sheet badges) located its entity.
  "cell.located": { kind: string; subject: string };
  // A served question about the selection opened in a tab of its own.
  "query.asked": { title: string };
  // An area of the drawing was selected, with how many parts it holds.
  "area.selected": { components: number };
}

// viewerEventNames lists every event, for a listener that follows all of them.
export const viewerEventNames = [
  "checklist.ran",
  "report.saved",
  "checks.ran",
  "finding.selected",
  "queryset.opened",
  "cell.located",
  "query.asked",
  "area.selected",
] as const satisfies readonly (keyof ViewerEvents)[];
// An event missing from the list fails to compile here.
const listsEvery: Exclude<keyof ViewerEvents, (typeof viewerEventNames)[number]> extends never ? true : never = true;
void listsEvery;

// EmitViewerEvent reports one event, typed by its name.
export type EmitViewerEvent = <K extends keyof ViewerEvents>(type: K, data: ViewerEvents[K]) => void;

// viewerEmitter emits on bus, or nowhere when there is none (a test, an embedding with no bus).
export function viewerEmitter(bus: EventBus | null): EmitViewerEvent {
  return (type, data) => bus?.emit(type, data);
}
