// What this browser opened lately, for the landing page's Recent list.
//
// Per-user state, so it lives in localStorage beside the workbench's own preferences (bank.ts),
// not on the server. Two reasons it could not live there even if a write path existed: the server
// has no user identity to key a list to, so one visitor's recents would be everyone's, and the
// project and design resources are read-only on purpose (CONSTRAINTS C23). A recent is a fact about
// a person, not about a design.
//
// Recording happens where a design or datasheet is actually OPENED (main.ts's location report,
// datasheets.ts's open), never on a browse-page preview. A list that fills up while you look
// around stops answering the question it exists for, which is "where was I".

// RecentKind is which page reopens the entry. It is stored rather than derived, because deciding
// from the extension would put a second copy of the design/datasheet split in this module.
export type RecentKind = "design" | "datasheet";

// Recent is one opened artifact. label is snapshotted at open time so the landing page renders
// with no round trips; mount and path are kept apart (rather than as one URI) because both pages
// address an artifact by that pair.
export interface Recent {
  kind: RecentKind;
  mount: string;
  path: string;
  label: string;
  at: number;
}

const KEY = "agni.recents";

// LIMIT is what a landing page can show without becoming a second file tree. Past a dozen, "recent"
// has stopped being the useful property and what the user wants is the browser or a pin.
export const LIMIT = 12;

// idOf identifies an entry for de-duplication. Kind is part of it: the same path cannot be both,
// but a mount rename can make one look like the other, and a stale entry that reopens on the wrong
// page is worse than a duplicate row.
function idOf(r: { kind: RecentKind; mount: string; path: string }): string {
  return JSON.stringify([r.kind, r.mount, r.path]);
}

// isRecent guards a decoded entry. Stored JSON is user-writable and survives across versions, so a
// row that lost a field is dropped rather than rendered as a link to nowhere.
function isRecent(v: unknown): v is Recent {
  const r = v as Partial<Recent> | null;
  return (
    !!r &&
    (r.kind === "design" || r.kind === "datasheet") &&
    typeof r.mount === "string" &&
    typeof r.path === "string" &&
    typeof r.label === "string" &&
    typeof r.at === "number"
  );
}

// loadRecents returns the entries most-recently-opened first, or an empty list when there are none
// and when what is stored cannot be read.
export function loadRecents(): Recent[] {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return [];
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter(isRecent).sort((a, b) => b.at - a.at).slice(0, LIMIT);
  } catch {
    return [];
  }
}

// noteOpen records an artifact as just-opened and returns the new list. Re-opening something moves
// it to the front rather than adding a row, which is what makes the list shrink to what you
// actually work on instead of growing to everything you have ever touched.
//
// The caller passes `at` rather than the module reading a clock, so a test states the ordering it
// is asserting instead of stubbing time.
export function noteOpen(entry: Omit<Recent, "at">, at: number = Date.now()): Recent[] {
  const fresh: Recent = { ...entry, at };
  const kept = loadRecents().filter((r) => idOf(r) !== idOf(fresh));
  const next = [fresh, ...kept].slice(0, LIMIT);
  try {
    localStorage.setItem(KEY, JSON.stringify(next));
  } catch {
    // A full or disabled store costs the user their history, not their navigation. Recording is a
    // side effect of opening something, so it must never be able to break the open.
  }
  return next;
}

// baseName is the label a recording site uses when it has nothing better: the file's own name. It
// lives here beside the store so the two call sites cannot label the same list differently.
export function baseName(path: string): string {
  const i = path.lastIndexOf("/");
  return i < 0 ? path : path.slice(i + 1);
}

// clearRecents empties the list. Exposed for the landing page's own control: a recents list with no
// way to clear it is a privacy problem on a shared machine.
export function clearRecents(): void {
  localStorage.removeItem(KEY);
}
