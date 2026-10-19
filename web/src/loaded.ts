// What the Load page read into the engine's store, so the viewer can tell a design it may open from
// one it sends to the Load page first (agni issue 1072).
//
// A record is a convenience, never evidence. The engine checks every stored read against the files'
// content before using it, so a record that outlived its store (the browser evicted it, a server
// restarted) costs a slower open in the viewer and never a wrong answer. That is why it lives in
// localStorage, where a private window or a blocked site may keep nothing, and why every read and
// write here survives that.

// LoadedKey names one load: the engine that read the design, the build it was, and the design.
export interface LoadedKey {
  engine: string;
  build: string;
  uri: string;
}

// A ListedFile is a design file as ListDesignFiles lists it, by path and content hash.
export interface ListedFile {
  path: string;
  sha256: string;
}

const STORAGE_KEY = "agni.loaded";

// MAX_RECORDS bounds the records kept, oldest dropped first, so a visitor who opens many designs
// does not grow the page's storage without end.
export const MAX_RECORDS = 50;

// filesStamp names a file listing by content, in path order, so a record matches the listing it was
// made from and no other.
export function filesStamp(files: ListedFile[]): string {
  return [...files]
    .sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : 0))
    .map((f) => `${f.path} ${f.sha256}`)
    .join("\n");
}

function keyOf(k: LoadedKey): string {
  return `${k.engine} ${k.build} ${k.uri}`;
}

type Records = { key: string; stamp: string }[];

function read(storage: Storage | undefined): Records {
  try {
    const raw = storage?.getItem(STORAGE_KEY);
    const v: unknown = raw ? JSON.parse(raw) : [];
    return Array.isArray(v) ? v.filter((r): r is Records[number] => typeof r?.key === "string" && typeof r?.stamp === "string") : [];
  } catch {
    return [];
  }
}

// recordLoaded notes that k was read with files stamped stamp. A storage that refuses the write keeps
// nothing, and the viewer then sends the design to the Load page again, which the viewer's own
// arrival marker stops from looping (see LOADED_PARAM).
export function recordLoaded(storage: Storage | undefined, k: LoadedKey, stamp: string): void {
  const key = keyOf(k);
  const rest = read(storage).filter((r) => r.key !== key);
  const next = [...rest, { key, stamp }].slice(-MAX_RECORDS);
  try {
    storage?.setItem(STORAGE_KEY, JSON.stringify(next));
  } catch {
    // Nothing kept; see above.
  }
}

// wasLoaded reports whether k was read with files stamped stamp, which is false for an engine build
// that never read it and for a file that changed since.
export function wasLoaded(storage: Storage | undefined, k: LoadedKey, stamp: string): boolean {
  const key = keyOf(k);
  return read(storage).some((r) => r.key === key && r.stamp === stamp);
}

// LOADED_PARAM marks the viewer's address when the Load page sends it there, so the viewer opens the
// design without consulting a record the Load page may have been unable to write. The viewer removes
// it from the address bar, so a later reload asks the record as usual.
export const LOADED_PARAM = "loaded";
