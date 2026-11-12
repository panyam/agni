// The reader-facing side of "this read left part of the design out" (agni issues 707, 717).
//
// The EDIF reader extracts a hierarchical design's top cell alone, so every count, every check and
// every drawing covers the top level while the source holds more. Nothing on the canvas looks
// wrong. The sheet draws, the counts are plausible, and a check run comes back clean over a design
// that is mostly absent.
//
// The server sends the sentence in GetDesignResponse.hierarchy_note, and report.HierarchyNote builds
// it. The CLI prints the same sentence on stderr, and both HTML reports show it in their header. The
// viewer renders it as sent rather than composing its own copy, so the three surfaces cannot drift
// apart.

// hierarchyNote returns the served sentence for the strip, or null when the read left nothing out.
// An empty or absent field is null, so a server that predates the field shows no strip.
export function hierarchyNote(served: string | undefined): string | null {
  return served ? served : null;
}

// hierarchyStrip wraps the notice element, as undrawnStrip does. It renders the note and hides the
// element entirely when there is none, so a flat design shows no chrome. A null element is a no-op.
//
// readAll, when given, runs when the reader presses the strip's "Read every block" button. The
// button is the browser's form of the CLI's --flatten-hierarchy hint, and it appears where the reader
// learns the read was partial (agni issue 1102). A page already reading designs whole passes none.
// Any note it still shows lists blocks that place themselves, and reading again would not reach them.
export function hierarchyStrip(el: HTMLElement | null, readAll?: () => void): (note: string | null) => void {
  return (note) => {
    if (!el) return;
    if (!note) {
      el.classList.remove("on");
      el.replaceChildren();
      return;
    }
    const text = document.createElement("span");
    text.textContent = note;
    el.replaceChildren(text);
    if (readAll) {
      const b = document.createElement("button");
      b.type = "button";
      b.className = "hierarchy-read-all";
      b.textContent = "Read every block";
      b.addEventListener("click", readAll);
      el.append(" ", b);
    }
    el.classList.add("on");
  };
}
