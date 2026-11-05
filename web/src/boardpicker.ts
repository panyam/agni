// The board picker (agni issue 1367). A reader whose design declares no board, or the wrong one,
// picks a board file or an ODB++ job folder from the design's mount, and the page opens again with
// it attached (?board=). The server marks what can be a board (ListDirRequest.mark_boards), so the
// picker keeps no list of board formats of its own.

// PickerEntry is the part of a WorkspaceService.ListDir entry the picker reads. boardUri is what to
// attach when the entry is a board, and "" otherwise. An ODB++ job's folder answers its matrix entry.
export interface PickerEntry {
  name: string;
  uri: string;
  isDir: boolean;
  boardUri: string;
}

// pickBoard opens the picker at the folder `start` ("mount://<mount>/<dir>") and resolves to the
// chosen board's mount:// URI, or null when the reader cancels. list answers one folder's entries.
// A folder that is a board, an ODB++ job, offers both entering it and using it.
export function pickBoard(doc: Document, start: string, list: (uri: string) => Promise<PickerEntry[]>): Promise<string | null> {
  return new Promise((resolve) => {
    const el = (tag: string, cls: string, text = ""): HTMLElement => {
      const e = doc.createElement(tag);
      e.className = cls;
      if (text) e.textContent = text;
      return e;
    };
    const backdrop = el("div", "board-picker-backdrop");
    const box = el("div", "board-picker");
    box.setAttribute("role", "dialog");
    box.setAttribute("aria-label", "use a board");
    const title = el("div", "board-picker-title", "Use a board");
    const where = el("div", "board-picker-where");
    const rows = el("ul", "board-picker-rows");
    const cancel = el("button", "mode-btn board-picker-cancel", "Cancel") as HTMLButtonElement;
    cancel.type = "button";
    box.append(title, where, rows, cancel);
    backdrop.append(box);
    doc.body.append(backdrop);
    const done = (uri: string | null) => {
      backdrop.remove();
      resolve(uri);
    };
    cancel.addEventListener("click", () => done(null));
    backdrop.addEventListener("click", (e) => {
      if (e.target === backdrop) done(null);
    });

    const button = (cls: string, text: string, act: () => void): HTMLButtonElement => {
      const b = el("button", cls, text) as HTMLButtonElement;
      b.type = "button";
      b.addEventListener("click", act);
      return b;
    };
    const open = async (uri: string): Promise<void> => {
      where.textContent = uri.replace(/^mount:\/\//, "");
      rows.replaceChildren(el("li", "board-picker-note", "listing…"));
      let entries: PickerEntry[];
      try {
        entries = await list(uri);
      } catch (e) {
        rows.replaceChildren(el("li", "board-picker-note", `could not list this folder: ${String(e)}`));
        return;
      }
      rows.replaceChildren();
      const parent = parentOf(uri);
      if (parent) {
        const li = el("li", "board-picker-row");
        li.append(button("board-picker-enter", "..", () => void open(parent)));
        rows.append(li);
      }
      for (const e of entries) {
        if (!e.isDir && !e.boardUri) continue;
        const li = el("li", "board-picker-row");
        li.dataset.uri = e.uri;
        if (e.isDir) li.append(button("board-picker-enter", `${e.name}/`, () => void open(e.uri)));
        else li.append(el("span", "board-picker-name", e.name));
        if (e.boardUri) li.append(button("mode-btn board-picker-use", "Use", () => done(e.boardUri)));
        rows.append(li);
      }
      if (rows.children.length === (parent ? 1 : 0)) rows.append(el("li", "board-picker-note", "no boards or folders here"));
    };
    void open(start);
  });
}

// parentOf is the folder above uri, or "" at a mount's root.
export function parentOf(uri: string): string {
  const m = /^(mount:\/\/[^/]+)(?:\/(.*))?$/.exec(uri);
  if (!m || !m[2]) return "";
  const parts = m[2].split("/").filter((p) => p !== "");
  parts.pop();
  return parts.length ? `${m[1]}/${parts.join("/")}` : m[1];
}
