// Saving an attached board to the design's design.yaml (agni issue 1367). The server answers the
// descriptor's text (WorkspaceService.DeclareCompanion) and writes nothing. In the browser's drop
// mount the page writes it, as the Load page writes a proposal. A served design's folder belongs to
// whoever runs the server, so the page shows the text and the reader commits it.

import { OVERLAY_DIR } from "./drops.js";

// Declared is the part of a DeclareCompanionResponse the page reads.
export interface Declared {
  descriptorUri: string;
  designYaml: string;
  changed: boolean;
}

// overlayPath is where the drop store keeps the descriptor at uri, "mount://<mount>/<path>", when
// the page can write it there, and "" otherwise. Only the drop mount is the page's to write.
export function overlayPath(uri: string, dropMount: string): string {
  const prefix = `mount://${dropMount}/`;
  return uri.startsWith(prefix) ? `${OVERLAY_DIR}/${uri.slice(prefix.length)}` : "";
}

// showDeclared opens a dialog with the descriptor's text, for a reader to save into the design's
// folder. It resolves when the reader closes it.
export function showDeclared(doc: Document, d: Declared): Promise<void> {
  return new Promise((resolve) => {
    const el = (tag: string, cls: string, text = ""): HTMLElement => {
      const e = doc.createElement(tag);
      e.className = cls;
      if (text) e.textContent = text;
      return e;
    };
    const backdrop = el("div", "board-picker-backdrop");
    const box = el("div", "board-picker board-declared");
    box.setAttribute("role", "dialog");
    box.setAttribute("aria-label", "save to design.yaml");
    const where = d.descriptorUri.replace(/^mount:\/\//, "");
    const note = d.changed
      ? `Save this as ${where}, and the design reads the board without ?board= in its address.`
      : `${where} declares this board already.`;
    const text = el("textarea", "board-declared-yaml") as HTMLTextAreaElement;
    text.readOnly = true;
    text.value = d.designYaml;
    text.rows = Math.min(16, d.designYaml.split("\n").length + 1);
    const buttons = el("div", "board-declared-buttons");
    const copy = el("button", "mode-btn board-declared-copy", "Copy") as HTMLButtonElement;
    const close = el("button", "mode-btn board-declared-close", "Close") as HTMLButtonElement;
    copy.type = close.type = "button";
    buttons.append(copy, close);
    box.append(el("div", "board-picker-title", "Save to design.yaml"), el("div", "board-picker-where", note), text, buttons);
    backdrop.append(box);
    doc.body.append(backdrop);
    copy.addEventListener("click", () => {
      text.select();
      void doc.defaultView?.navigator.clipboard?.writeText(d.designYaml).catch(() => undefined);
    });
    const done = () => {
      backdrop.remove();
      resolve();
    };
    close.addEventListener("click", done);
    backdrop.addEventListener("click", (e) => {
      if (e.target === backdrop) done();
    });
  });
}
