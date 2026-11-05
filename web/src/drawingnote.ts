// The reader-facing side of "agni laid these sheets out itself".
//
// A netlist, or a board read alone, carries no schematic drawing, so the viewer draws its parts on
// agni's default layout. That drawing looks wrong to anyone who knows the board, and the viewer
// offers no layout picker to explain it. So the server sends a sentence in
// GetDesignResponse.drawing_note naming the file that would draw the sheets as the designer did,
// and this strip shows it on every sheet except the board, whose drawing is its own.

// drawingNote returns the sentence for the strip on the given sheet, or null when there is none to
// show. An empty or absent field is null, so a server that predates the field shows no strip.
export function drawingNote(served: string | undefined, sheetId: string): string | null {
  if (!served || sheetId === "board") return null;
  return served;
}

// drawingStrip wraps the notice element, as hierarchyStrip does. It hides the element entirely
// when there is no note, so a drawn design shows no chrome. A null element is a no-op.
export function drawingStrip(el: HTMLElement | null): (note: string | null) => void {
  return (note) => {
    if (!el) return;
    el.textContent = note ?? "";
    el.classList.toggle("on", !!note);
  };
}
