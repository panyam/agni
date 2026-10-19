// Dropped files, kept in the browser's IndexedDB so a dropped design survives a reload and the page
// navigation from the Load page to the viewer (agni issue 1072). Before this they lived only in the
// tab's engine worker, so a reload emptied them.
//
// A drop is the files one drop or pick brought in, under a folder of the browser mount named by its
// id (`drop-<time>`), plus the design.yaml a visitor wrote for it under `.agni-overlay/<id>/`. Every
// path here is a path in the browser mount, so what is saved is exactly what is added to the engine.

// Files maps a path in the browser mount to its bytes, the shape WasmEngine.add takes.
export type DropFiles = Record<string, Uint8Array>;

// SavedDrop describes one saved drop for a listing.
export interface SavedDrop {
  id: string;
  // name is what the visitor dropped, a file's name or a folder's.
  name: string;
  // savedAt is when it was dropped, and updatedAt when a file of it last changed, in ms since the
  // epoch. updatedAt is what a record of a dropped design's load is stamped with.
  savedAt: number;
  updatedAt: number;
  bytes: number;
  files: number;
}

// DropStore keeps drops. Every method may reject, for a browser that keeps no IndexedDB (some private
// windows), and a caller says so rather than pretending the drop was kept.
export interface DropStore {
  // save keeps a new drop's files, replacing any drop of the same id.
  save(id: string, name: string, files: DropFiles): Promise<void>;
  // put adds or replaces files of an existing drop, such as its design.yaml.
  put(id: string, files: DropFiles): Promise<void>;
  // get is one drop's description, or undefined when it is not kept.
  get(id: string): Promise<SavedDrop | undefined>;
  // files is every file of one drop, empty when it is not kept.
  files(id: string): Promise<DropFiles>;
  // list is every kept drop, newest first.
  list(): Promise<SavedDrop[]>;
  remove(id: string): Promise<void>;
}

// OVERLAY_DIR is where the page writes a design.yaml beside a dropped design, fshost.OverlayDir on
// the Go side.
export const OVERLAY_DIR = ".agni-overlay";

// dropOf names the drop a path in the browser mount belongs to: its first segment, or the one after
// OVERLAY_DIR. "" for a path no drop holds.
export function dropOf(path: string): string {
  const segs = path.split("/").filter((s) => s !== "");
  const id = segs[0] === OVERLAY_DIR ? (segs[1] ?? "") : (segs[0] ?? "");
  return id.startsWith("drop-") ? id : "";
}

function sizeOf(files: DropFiles): number {
  return Object.values(files).reduce((n, b) => n + b.byteLength, 0);
}

// memoryDropStore keeps drops in memory, for tests and for a browser with no IndexedDB.
export function memoryDropStore(now: () => number = Date.now): DropStore {
  const drops = new Map<string, { meta: SavedDrop; files: DropFiles }>();
  const describe = (meta: SavedDrop, files: DropFiles): SavedDrop => ({ ...meta, bytes: sizeOf(files), files: Object.keys(files).length });
  return {
    async save(id, name, files) {
      const t = now();
      const copy = { ...files };
      drops.set(id, { meta: describe({ id, name, savedAt: t, updatedAt: t, bytes: 0, files: 0 }, copy), files: copy });
    },
    async put(id, files) {
      const d = drops.get(id);
      if (!d) throw new Error(`${id} is not a saved drop`);
      Object.assign(d.files, files);
      d.meta = describe({ ...d.meta, updatedAt: Math.max(now(), d.meta.updatedAt + 1) }, d.files);
    },
    async get(id) {
      return drops.get(id)?.meta;
    },
    async files(id) {
      return { ...(drops.get(id)?.files ?? {}) };
    },
    async list() {
      return [...drops.values()].map((d) => d.meta).sort((a, b) => b.savedAt - a.savedAt);
    },
    async remove(id) {
      drops.delete(id);
    },
  };
}

const DB_NAME = "agni-drops";
const DB_VERSION = 1;
const META = "drops"; // SavedDrop, keyed by id
const FILES = "files"; // { drop, path, bytes }, keyed by path, indexed by drop

function request<T>(r: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    r.onsuccess = () => resolve(r.result);
    r.onerror = () => reject(r.error ?? new Error("IndexedDB request failed"));
  });
}

function done(tx: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error ?? new Error("IndexedDB transaction failed"));
    tx.onabort = () => reject(tx.error ?? new Error("IndexedDB transaction aborted"));
  });
}

// browserDropStore keeps drops in this origin's IndexedDB. The database opens on first use, so a
// page that never touches a drop never creates it.
export function browserDropStore(idb: IDBFactory = globalThis.indexedDB): DropStore {
  let opened: Promise<IDBDatabase> | undefined;
  const db = (): Promise<IDBDatabase> => {
    if (!idb) return Promise.reject(new Error("this browser keeps no IndexedDB here"));
    opened ??= new Promise<IDBDatabase>((resolve, reject) => {
      const r = idb.open(DB_NAME, DB_VERSION);
      r.onupgradeneeded = () => {
        const d = r.result;
        d.createObjectStore(META, { keyPath: "id" });
        d.createObjectStore(FILES, { keyPath: "path" }).createIndex("drop", "drop");
      };
      r.onsuccess = () => resolve(r.result);
      r.onerror = () => reject(r.error ?? new Error("IndexedDB would not open"));
    }).catch((e: unknown) => {
      opened = undefined;
      throw e;
    });
    return opened;
  };
  const filesOf = async (d: IDBDatabase, id: string): Promise<{ path: string; bytes: Uint8Array }[]> =>
    request(d.transaction(FILES).objectStore(FILES).index("drop").getAll(id)) as Promise<{ path: string; bytes: Uint8Array }[]>;
  const store: DropStore = {
    async save(id, name, files) {
      await store.remove(id);
      const d = await db();
      const tx = d.transaction([META, FILES], "readwrite");
      const t = Date.now();
      tx.objectStore(META).put({ id, name, savedAt: t, updatedAt: t, bytes: sizeOf(files), files: Object.keys(files).length } satisfies SavedDrop);
      for (const [path, bytes] of Object.entries(files)) tx.objectStore(FILES).put({ drop: id, path, bytes });
      await done(tx);
    },
    async put(id, files) {
      const d = await db();
      const meta = (await request(d.transaction(META).objectStore(META).get(id))) as SavedDrop | undefined;
      if (!meta) throw new Error(`${id} is not a saved drop`);
      const have = await filesOf(d, id);
      const all: DropFiles = Object.fromEntries(have.map((f) => [f.path, f.bytes]));
      Object.assign(all, files);
      const tx = d.transaction([META, FILES], "readwrite");
      for (const [path, bytes] of Object.entries(files)) tx.objectStore(FILES).put({ drop: id, path, bytes });
      tx.objectStore(META).put({ ...meta, updatedAt: Math.max(Date.now(), meta.updatedAt + 1), bytes: sizeOf(all), files: Object.keys(all).length } satisfies SavedDrop);
      await done(tx);
    },
    async get(id) {
      const d = await db();
      return (await request(d.transaction(META).objectStore(META).get(id))) as SavedDrop | undefined;
    },
    async files(id) {
      const d = await db();
      return Object.fromEntries((await filesOf(d, id)).map((f) => [f.path, f.bytes]));
    },
    async list() {
      const d = await db();
      const all = (await request(d.transaction(META).objectStore(META).getAll())) as SavedDrop[];
      return all.sort((a, b) => b.savedAt - a.savedAt);
    },
    async remove(id) {
      const d = await db();
      const paths = (await filesOf(d, id)).map((f) => f.path);
      const tx = d.transaction([META, FILES], "readwrite");
      tx.objectStore(META).delete(id);
      for (const p of paths) tx.objectStore(FILES).delete(p);
      await done(tx);
    },
  };
  return store;
}
