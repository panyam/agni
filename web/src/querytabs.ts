// The query panel's tabs (agni issue 902). A question asked about a selection, from Properties' Ask
// rows or the panel's own selection bar, opens a tab of its own, so it never overwrites a query the
// reader was writing, and several answers stay open side by side. The tabs are the panel's own strip
// rather than dockview tabs, so a query tab never lands in a saved dock layout.
//
// This module is the strip's model, pure so the panel and its tests share one set of rules: what a
// tab is called, which tab an action leaves active, and that one tab always remains.

import { type Bindings, LocateReason } from "./query.js";
import type { QuerySet } from "./gen/agni/v1/webapi/query_pb.js";
import { labelFor, type Selection } from "./selection.js";

// Located is the cell a tab last located on the drawing. Switching back to the tab locates it again,
// so the highlight always belongs to the answer in front.
export interface Located {
  kind: string;
  subject: string;
  sheet?: string;
  reason: LocateReason;
  pin?: string;
}

// QueryTab is one tab's own question. The answer to it lives with the host, keyed by id, because the
// host is what the presenter pushes answers to.
export interface QueryTab {
  id: string;
  title: string;
  text: string;
  bindings: Bindings;
  // lastRun is the question the answer on screen is for, so "show all" asks the same one again
  // without its limit even after the reader has started editing the box.
  lastRun: { text: string; bindings: Bindings };
  located?: Located;
  // origin is the query set member a tab was opened from, with the text and bindings it opened with.
  // While the tab still asks exactly that, running it runs the member through RunQueries, so the
  // engine joins the preamble and keeps the set's numeric bindings as numbers (agni issue 1045). Once
  // the reader edits it, it is their own query.
  origin?: SetOrigin;
}

// SetOrigin is where a tab opened from a query set came from.
export interface SetOrigin {
  set: QuerySet;
  index: number;
  text: string;
  bindings: Bindings;
  numbers: string[];
}

// fromOrigin reports whether a question is still the set member a tab was opened from.
export function fromOrigin(o: SetOrigin | undefined, text: string, bindings: Bindings): o is SetOrigin {
  return !!o && tabKey(text, bindings) === tabKey(o.text, o.bindings);
}

// TabStrip is the whole strip: its tabs in order and the one in front.
export interface TabStrip {
  tabs: QueryTab[];
  active: string;
}

// tabKey identifies a tab's question, the query text and its bindings, so a later change can tell an
// Ask row that is already open from one that is not. Binding order does not change the question.
export function tabKey(text: string, bindings: Bindings): string {
  const b = Object.keys(bindings)
    .sort()
    .map((k) => {
      // A list is told apart from a value that happens to read like one.
      const v = bindings[k];
      return Array.isArray(v) ? `${k}\u0002${JSON.stringify(v)}` : `${k}=${v}`;
    })
    .join("\u0000");
  return `${text.trim()}\u0001${b}`;
}

// askTitle names a tab opened by asking a question about a selection, "J1: Which test points reach
// its nets?", so a strip of answers about several parts reads as which question about which part.
export function askTitle(sel: Selection, label: string): string {
  return `${labelFor(sel)}: ${label}`;
}

// blankTab is a tab with nothing in it yet.
export function blankTab(id: string, title: string): QueryTab {
  return { id, title, text: "", bindings: {}, lastRun: { text: "", bindings: {} } };
}

// openTab adds a tab after the active one and brings it to the front.
export function openTab(strip: TabStrip, tab: QueryTab): TabStrip {
  const at = strip.tabs.findIndex((t) => t.id === strip.active);
  const tabs = [...strip.tabs];
  tabs.splice(at < 0 ? tabs.length : at + 1, 0, tab);
  return { tabs, active: tab.id };
}

// closeTab removes a tab. The last tab never closes, since the panel always shows one. Closing the
// tab in front brings forward its right-hand neighbour, or its left one at the end of the strip,
// which is what a browser does.
export function closeTab(strip: TabStrip, id: string): TabStrip {
  if (strip.tabs.length <= 1) return strip;
  const at = strip.tabs.findIndex((t) => t.id === id);
  if (at < 0) return strip;
  const tabs = strip.tabs.filter((t) => t.id !== id);
  if (strip.active !== id) return { tabs, active: strip.active };
  return { tabs, active: tabs[Math.min(at, tabs.length - 1)].id };
}

// updateTab replaces fields of one tab.
export function updateTab(strip: TabStrip, id: string, change: Partial<QueryTab>): TabStrip {
  return { ...strip, tabs: strip.tabs.map((t) => (t.id === id ? { ...t, ...change } : t)) };
}

// activeTab is the tab in front.
export function activeTab(strip: TabStrip): QueryTab {
  return strip.tabs.find((t) => t.id === strip.active) ?? strip.tabs[0];
}
