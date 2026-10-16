// The reader-facing side of "this read left part of the design out" (agni issues 707, 717).
//
// The EDIF reader extracts a hierarchical design's top cell alone, so every count, every check and
// every drawing covers the top level while the source holds more. Nothing on the canvas looks
// wrong: the sheet draws, the counts are plausible, and a check run comes back clean over a design
// that is mostly absent.
//
// The sentence is the server's (GetDesignResponse.hierarchy_note, built by report.HierarchyNote), the
// one the CLI prints on stderr and both HTML reports carry in their header. The viewer renders it as
// sent rather than composing its own copy, so the three surfaces cannot drift apart.

// hierarchyNote is the strip's input: the served sentence, or null when the read left nothing out.
// An empty or absent field is null, so a server that predates the field shows no strip.
export function hierarchyNote(served: string | undefined): string | null {
  return served ? served : null;
}

// hierarchyStrip wraps the notice element, mirroring undrawnStrip: it renders the note and hides the
// element entirely when there is none, so a flat design carries no chrome. A null element is a
// no-op.
export function hierarchyStrip(el: HTMLElement | null): (note: string | null) => void {
  return (note) => {
    if (!el) return;
    if (!note) {
      el.classList.remove("on");
      el.textContent = "";
      return;
    }
    el.textContent = note;
    el.classList.add("on");
  };
}
