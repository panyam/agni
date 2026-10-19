// Whether the viewer opens the design its address names, or hands it to the Load page first (agni
// issue 1072). The viewer opens only a design the Load page read on this engine and build, from the
// files it holds now, so no panel waits on a first read. The record is a convenience, and the engine
// still checks every stored read by content, so a wrong answer here costs time and never a result.

import { LOADED_PARAM } from "./loaded.js";
import { dropPageUrl, hasFile, parseUrl, swapPage } from "./router.js";

// An Admission is what the viewer does with its address: open it (with the address to show, which
// drops the Load page's arrival marker), or go to another page.
export type Admission = { open: true; replaceWith?: string } | { open: false; goTo: string };

export interface AdmitDeps {
  pathname: string;
  search: string;
  // dropMountPrefix is the browser mount's address prefix, `<base>designs/local/`.
  dropMountPrefix: string;
  // listing is the stamp of the design's files now, "" when they cannot be listed.
  stamp(mount: string, path: string): Promise<string>;
  // loaded reports whether a record says this engine and build read the design with that stamp.
  loaded(mount: string, path: string, stamp: string): boolean;
}

export async function admit(d: AdmitDeps): Promise<Admission> {
  const here = parseUrl(d.pathname, d.search);
  if (!hasFile(here)) {
    // The browser mount's own address was where files were dropped, which is the Load page now.
    return d.pathname.startsWith(d.dropMountPrefix) ? { open: false, goTo: dropPageUrl() } : { open: true };
  }
  const params = new URLSearchParams(d.search);
  if (params.has(LOADED_PARAM)) {
    // Arrived from the Load page, which has just read it. The marker leaves the address bar, so a
    // reload asks the record.
    params.delete(LOADED_PARAM);
    const q = params.toString();
    return { open: true, replaceWith: d.pathname + (q ? `?${q}` : "") };
  }
  let stamp = "";
  try {
    stamp = await d.stamp(here.mount, here.path);
  } catch {
    // The Load page reports a design whose files cannot be listed.
  }
  if (stamp && d.loaded(here.mount, here.path, stamp)) return { open: true };
  return { open: false, goTo: swapPage(d.pathname, d.search, "load") };
}
