// The compare picker is a modal file chooser for "compare the open design against this one"
// (WS9-049 phase 3). It carries no armed mode and does not need the Files dock panel open.
//
// The picker reports a CHOSEN DESIGN, not "side B". The viewer holds one comparison, and a callback
// meaning "a design to compare against" still fits if it grows to hold several (design A against
// successive versions), where "set side B" would not.
import type { EventBus } from "@panyam/tsappkit";
import type { SolidIsland } from "@panyam/tsappkit-solid";
import { fileTreeIsland } from "./filetree.js";

// CompareTarget is a design the user chose to compare against.
export interface CompareTarget {
  mount: string;
  path: string;
}

// ComparePicker is the modal's control surface. It is plain DOM chrome, like the panels menu, since
// the tree island owns which files exist and whether the modal is showing is not presenter state.
export interface ComparePicker {
  // open shows the picker. exclude is the design already open (side A), greyed out in the list so
  // the user cannot start a comparison of a design against itself.
  open(exclude: CompareTarget | null): void;
  // close hides the picker without choosing. Idempotent.
  close(): void;
  // isOpen reports visibility, so the host can avoid stacking a second open.
  isOpen(): boolean;
}

// comparePickerIsland builds the modal over a server-rendered hole, which must contain a backdrop
// element and a tree host (see ViewerPage.html). The island mounts once at boot (C11) and opening is
// a CSS toggle, so the file listing is already loaded the first time the user asks for it.
export function comparePickerIsland(
  host: HTMLElement,
  treeEl: HTMLElement,
  eventBus: EventBus | null,
  onPick: (target: CompareTarget) => void,
): { island: SolidIsland; picker: ComparePicker } {
  const doc = host.ownerDocument;
  let open = false;
  let excluded: CompareTarget | null = null;

  const setOpen = (on: boolean): void => {
    open = on;
    host.classList.toggle("on", on);
    // aria-hidden rather than display:none on the host, because the tree island inside must stay
    // mounted and measurable. The .on class drives visibility.
    host.setAttribute("aria-hidden", on ? "false" : "true");
  };

  const tree = fileTreeIsland(treeEl, eventBus, {
    onFileSelect: (mount, path) => {
      // Choosing side A again is ignored rather than closing, so the click reads as "that one is
      // already open".
      if (excluded && excluded.mount === mount && excluded.path === path) return;
      setOpen(false);
      onPick({ mount, path });
    },
    // A folder click only expands. The picker addresses no URL and opens no design.
    onDirSelect: () => {},
    // Nothing pushes sheet state into this tree, so no sheet node is ever rendered.
    onSheetSelect: () => {},
  });

  host.addEventListener("click", (e) => {
    // A click on the backdrop (the host itself, not the dialog inside it) dismisses.
    if (e.target === host) setOpen(false);
  });
  doc.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && open) setOpen(false);
  });
  setOpen(false);

  return {
    island: tree.island,
    picker: {
      open: (exclude) => {
        excluded = exclude;
        // Mark the open design as unavailable. The tree highlights its "active" file from the sheet
        // state it is pushed, the same channel the work page uses.
        tree.view.setState({ mount: exclude?.mount ?? "", path: exclude?.path ?? "", sheets: [], activeId: "" });
        setOpen(true);
      },
      close: () => setOpen(false),
      isOpen: () => open,
    },
  };
}
