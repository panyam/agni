// Dropping files on the viewer (agni issue 854). The files go into the browser mount, never to the
// server, the engine proposes the designs they make, and the visitor reads and edits each proposal's
// design.yaml before one is opened. Opening happens in the same page, because a browser mount lives
// in this tab's worker and a reload would empty it.
//
// The dialog is plain DOM. Every string in it came from the visitor's files, so it is set as text and
// never parsed as markup.
import { filesFromDrop, filesFromFileList } from "@panyam/tsappkit/wasmhost";
import type { ProposeDesignsResponse } from "../gen/agni/v1/webapi/workspace_pb.js";
import { mountOf, type Files, type WasmEngine } from "./client.js";

// BROWSER_MOUNT is the mount dropped files go into, wasmengine.BrowserMount on the Go side.
export const BROWSER_MOUNT = "local";

// OVERLAY_DIR is where the page writes a design.yaml, fshost.OverlayDir on the Go side. A file there
// reads over the same path in the browser mount, which is the only way to put a descriptor beside a
// design that came out of a zip, since the mount refuses a file under a path that is a file.
const OVERLAY_DIR = ".agni-overlay";

export interface DropDeps {
  // engine holds the dropped files. Without one the page is on the server engine, and a drop is
  // refused with `refuse`'s reason rather than uploaded.
  engine?: WasmEngine;
  propose(uri: string): Promise<ProposeDesignsResponse>;
  // open shows a design, by its path in the browser mount, without reloading the page.
  open(path: string): void;
  refuse(reason: string): void;
  target: HTMLElement;
  dialog: HTMLDialogElement;
  button: HTMLElement;
  input: HTMLInputElement;
}

// installDrop wires the drop target, the "Open files" button and the proposal dialog.
export function installDrop(d: DropDeps): void {
  d.button.hidden = false;
  d.button.addEventListener("click", () => d.input.click());
  d.input.addEventListener("change", () => {
    if (d.input.files?.length) void bring(d, filesFromFileList(d.input.files)).catch((err: unknown) => d.refuse(`Could not read those files: ${String(err)}`));
    d.input.value = "";
  });
  d.target.addEventListener("dragover", (ev) => {
    if (ev.dataTransfer?.types.includes("Files")) ev.preventDefault();
  });
  d.target.addEventListener("drop", (ev) => {
    if (!ev.dataTransfer?.types.includes("Files")) return;
    ev.preventDefault();
    void bring(d, filesFromDrop(ev.dataTransfer)).catch((err: unknown) => d.refuse(`Could not read those files: ${String(err)}`));
  });
}

// bring puts one drop into the browser mount under a folder of its own, asks for the designs it
// makes, and shows them.
async function bring(d: DropDeps, read: Promise<Files>): Promise<void> {
  if (!d.engine) {
    d.refuse("Dropped files are analysed in your browser and never sent to the server, so this page needs the browser engine: add ?engine=wasm to its address");
    return;
  }
  const files = await read;
  const id = `drop-${Date.now().toString(36)}`;
  const placed: Files = {};
  for (const [p, b] of Object.entries(files)) placed[`${id}/${p}`] = b;
  await d.engine.add(BROWSER_MOUNT, placed);
  const proposal = await d.propose(`mount://${BROWSER_MOUNT}/${id}`);
  render(d, proposal);
}

function el<K extends keyof HTMLElementTagNameMap>(tag: K, cls: string, text?: string): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text !== undefined) e.textContent = text;
  return e;
}

function render(d: DropDeps, r: ProposeDesignsResponse): void {
  const dlg = d.dialog;
  dlg.replaceChildren();
  dlg.append(el("h2", "drop-title", r.designs.length ? "What these files make" : "These files make no design"));
  const editable = new Set<string>();
  for (const pd of r.designs) {
    const sec = el("section", "drop-design");
    // A declared design is named by its folder, which names the same design as its entry (C32).
    const entry = pd.design?.entryUri || pd.design?.uri || "";
    const path = entry.replace(/^mount:\/\/[^/]+\//, "");
    sec.append(el("h3", "drop-design-name", `${pd.design?.name ?? path}  (${pd.files.length} file${pd.files.length === 1 ? "" : "s"})`));
    if (pd.note) sec.append(el("p", "drop-design-note", pd.note));
    // One design.yaml per folder: the first proposal in a folder can be declared by it, and any other
    // is opened by its file.
    const declarable = !pd.declared && !editable.has(pd.folder);
    if (declarable) editable.add(pd.folder);
    const yaml = el("textarea", "drop-design-yaml");
    yaml.value = pd.designYaml;
    yaml.readOnly = !declarable;
    yaml.rows = Math.min(14, pd.designYaml.split("\n").length + 1);
    yaml.spellcheck = false;
    sec.append(yaml);
    const open = el("button", "drop-design-open", "Open");
    open.type = "button";
    open.addEventListener("click", () => {
      void (async () => {
        if (declarable && yaml.value.trim() && d.engine) {
          await d.engine.add(BROWSER_MOUNT, { [`${OVERLAY_DIR}/${pd.folder}/design.yaml`]: new TextEncoder().encode(yaml.value) });
        }
        dlg.close();
        d.open(mountOf(entry) === BROWSER_MOUNT ? path : entry);
      })().catch((err: unknown) => d.refuse(`Could not open ${path}: ${String(err)}`));
    });
    sec.append(open);
    dlg.append(sec);
  }
  if (r.unread.length) {
    dlg.append(el("h3", "drop-unread-title", "Not read"));
    const ul = el("ul", "drop-unread");
    for (const u of r.unread) ul.append(el("li", "", `${u.path}: ${u.reason}`));
    dlg.append(ul);
  }
  const close = el("button", "drop-close", "Close");
  close.type = "button";
  close.addEventListener("click", () => dlg.close());
  dlg.append(close);
  dlg.showModal();
}
