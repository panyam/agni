// The reader-facing side of "this link was computed against different bytes" (agni issue 392).
//
// `agni check --verdicts --url-base` mints a URL carrying both the verdict id and the content hash of
// the design the run actually read. The viewer resolving that id has no idea whether the file behind
// the mount is still that revision, and an id is derived from a rule name and a subject ref, so a
// design edited since the run will happily resolve the SAME id against a different net. The proof then
// draws, confidently, on the wrong pins. That is the exact false-confidence failure the verdict layer
// exists to remove, relocated into the browser.
//
// THREE STATES, NOT TWO. A link whose hash cannot be checked is not a link whose hash matched, and
// collapsing them is how "absent" comes to look like "fine". The server reports an empty hash when it
// could not hash the file, and that case gets its own note rather than the benefit of the doubt.

// LinkTrust is what the viewer could establish about the link it was opened with.
export type LinkTrust =
  // The link named a revision and this server could not hash the file to compare against it.
  | "unverifiable"
  // The link named a revision and this server read different bytes.
  | "mismatch";

// StaleLinkNote is what the strip renders. Null when there is nothing to say, which covers both the
// link that named no revision (nothing was claimed) and the link that matched (the claim held).
export interface StaleLinkNote {
  trust: LinkTrust;
  // claimed and served are the SHORT forms, for a reader comparing two digests by eye. The full
  // values are in the URL and in the report the link came from; forty hex characters twice over in a
  // banner is not something anyone reads.
  claimed: string;
  served: string;
}

// shortHash trims "sha256:" and keeps enough hex to distinguish revisions by eye without filling the
// strip. Twelve characters is what the datasheet revision note uses, so the two read alike.
export function shortHash(h: string): string {
  const hex = h.startsWith("sha256:") ? h.slice(7) : h;
  return hex.slice(0, 12);
}

// staleLinkNote compares the revision a link claimed against the one this server read.
//
// claimed is the URL's ?hash= ("" when the link named none, which is what the CLI emits for a design
// it could not hash). served is GetDesignResponse.content_hash ("" when the server could not hash).
export function staleLinkNote(claimed: string, served: string): StaleLinkNote | null {
  if (!claimed) return null; // the link made no claim, so there is none to check
  if (!served) return { trust: "unverifiable", claimed: shortHash(claimed), served: "" };
  if (claimed === served) return null; // the claim held
  return { trust: "mismatch", claimed: shortHash(claimed), served: shortHash(served) };
}

// staleLinkStrip wraps the notice element, mirroring undrawnStrip: it renders the note and hides the
// element entirely when there is none, so an ordinary open carries no chrome. A null element is a
// no-op.
//
// The two states get different severity, because they ask the reader for different things. A mismatch
// means what is on screen may be about a different net and the reader should re-run the check; an
// unverifiable link is a link that is probably fine and cannot be proven so.
export function staleLinkStrip(el: HTMLElement | null): (note: StaleLinkNote | null) => void {
  return (note) => {
    if (!el) return;
    el.classList.remove("on", "warn");
    if (!note) {
      el.textContent = "";
      return;
    }
    if (note.trust === "mismatch") {
      el.textContent =
        `This link was computed against a different revision of the design: it names ${note.claimed}, ` +
        `and this server read ${note.served}. Anything highlighted may be about a different net. ` +
        `Re-run the checks to see this revision's answer.`;
      el.classList.add("on", "warn");
      return;
    }
    el.textContent =
      `This link names revision ${note.claimed} and this server could not hash the file to compare. ` +
      `The proof below is drawn against whatever is on disk now, unverified.`;
    el.classList.add("on");
  };
}
