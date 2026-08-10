// The compare picker (WS9-049 phase 3): a modal file chooser for "compare the open design against
// this one". It replaces the arm-then-click-in-the-tree flow, which had two problems — arming was
// invisible mode state, and picking the other design required the Files dock panel to be open, so
// the panel could not be retired while that flow existed.
//
// The picker reports a CHOSEN DESIGN, not "side B". The distinction matters for what comes next:
// the viewer holds one comparison today, but the intent is several at once (design A against
// successive versions). A callback that means "here is a design to compare against" needs no
// change when the presenter layer grows to hold a set; one that means "set side B" does.
import type { EventBus } from "@panyam/tsappkit";
import type { SolidIsland } from "@panyam/tsappkit-solid";
import { fileTreeIsland } from "./filetree.js";

// CompareTarget is a design the user chose to compare against.
export interface CompareTarget {
  mount: string;
  path: string;
}

// ComparePicker is the modal's control surface. It is plain DOM chrome, like the panels menu:
// which files exist is the tree island's business, and whether the modal is showing is not
// presenter state.
export interface ComparePicker {
  // open shows the picker. exclude is the design already open (side A), greyed out in the list so
  // the user cannot start a comparison of a design against itself.
  open(exclude: CompareTarget | null): void;
  // close hides the picker without choosing. Idempotent.
  close(): void;
  // isOpen reports visibility, so the host can avoid stacking a second open.
  isOpen(): boolean;
}

// comparePickerIsland builds the modal over a server-rendered hole. The hole must contain a
// backdrop element and a tree host (see ViewerPage.html); the island mounts once at boot like
// every other island (C11), and opening is a CSS toggle rather than a lazy mount, so the file
// listing is already warm the first time the user asks for it.
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
    // aria-hidden rather than display:none on the host itself: the tree island lives inside and
    // must stay mounted and measurable, and the .on class is what actually drives visibility.
    host.setAttribute("aria-hidden", on ? "false" : "true");
  };

  const tree = fileTreeIsland(treeEl, eventBus, {
    onFileSelect: (mount, path) => {
      // Choosing side A again is not a comparison. Ignore rather than close, so the click reads as
      // "that one is already open" instead of silently dismissing the picker.
      if (excluded && excluded.mount === mount && excluded.path === path) return;
      setOpen(false);
      onPick({ mount, path });
    },
    // A folder click in the picker only expands; the picker addresses no URL and opens no design.
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
        // Mark the open design so it reads as unavailable. The tree highlights its "active" file
        // from the sheet state it is pushed, which is the same channel the work page uses.
        tree.view.setState({ mount: exclude?.mount ?? "", path: exclude?.path ?? "", sheets: [], activeId: "" });
        setOpen(true);
      },
      close: () => setOpen(false),
      isOpen: () => open,
    },
  };
}
