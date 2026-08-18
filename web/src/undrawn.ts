// The reader-facing side of "this drawing is incomplete" (agni issue 354).
//
// A render that could not resolve a symbol does not look broken. The placement contributes no shapes,
// so it disappears along with the entity keys that make it pickable, while the annotation pass still
// draws its reference designator. The sheet then shows every ref des, every wire and the title block,
// and every component on it is silently unclickable. Nothing on screen says so, and the honest reading
// of a sheet showing C1 that will not respond to a click is "agni knows nothing about C1".

// UndrawnPlacement is the client shape of geom.UndrawnPlacement: the placement, and the symbol it
// asked for and did not get.
export interface UndrawnPlacement {
  refDes: string;
  cellRef: string;
  libraryRef: string;
  sheetId: string;
}

// UndrawnNote is what the strip renders: how many placements were lost, and which libraries cost
// them. Null when the drawing is complete.
export interface UndrawnNote {
  count: number;
  // libraries are the missing symbol references with a count each, worst first then alphabetical, so
  // the one costing the most parts leads.
  libraries: { name: string; count: number }[];
}

// undrawnNote summarises the placements a render lost, or null when it lost none.
//
// Grouped by the missing symbol reference rather than listed per placement, because one missing
// library commonly costs every part drawn from it and forty identical entries bury that single cause.
// The count is the blast radius, which is what tells a reader whether the drawing in front of them is
// worth reading at all.
export function undrawnNote(undrawn: UndrawnPlacement[] | undefined): UndrawnNote | null {
  if (!undrawn || undrawn.length === 0) return null;
  const byRef = new Map<string, number>();
  for (const u of undrawn) {
    const name = u.libraryRef ? `${u.libraryRef}:${u.cellRef}` : u.cellRef;
    byRef.set(name, (byRef.get(name) ?? 0) + 1);
  }
  const libraries = [...byRef.entries()]
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => (b.count !== a.count ? b.count - a.count : a.name < b.name ? -1 : 1));
  return { count: undrawn.length, libraries };
}

// undrawnStrip wraps the notice element, mirroring expectationCaptionStrip: it renders the note and
// hides the element entirely when there is none, so a complete drawing carries no chrome. A null
// element is a no-op.
export function undrawnStrip(el: HTMLElement | null): (note: UndrawnNote | null) => void {
  return (note) => {
    if (!el) return;
    if (!note) {
      el.classList.remove("on");
      el.textContent = "";
      return;
    }
    const parts = note.libraries.map((l) => `${l.name} (${l.count})`);
    el.textContent =
      `${note.count} placement${note.count === 1 ? "" : "s"} could not be drawn: no symbol for ` +
      `${parts.join(", ")}. Their reference designators still appear, and they cannot be clicked.`;
    el.classList.add("on");
  };
}
