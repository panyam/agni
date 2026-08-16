// What the two file trees share about pruning: the kinds each one declares to the server, and the
// line it shows when the server pruned a mount out of its listing.
//
// The trees themselves stay separate (the design tree nests sheets under a file, the datasheets
// tree does not, and they open different pages). What must not diverge is what they TELL the user
// when a configured mount is missing, because that message is the only thing standing between
// "this folder holds nothing I open" and "this mount is broken".

import { FileKind } from "./gen/agni/v1/webapi/workspace_pb.js";

// DESIGN_OPENS / DATASHEET_OPENS are what each tree passes as `opens`. Named constants rather than
// inline arrays so the request and the view filter cannot drift: a tree that asked the server to
// prune by one kind and then filtered rows by another would hide folders it then showed files from.
export const DESIGN_OPENS = [FileKind.DESIGN];
export const DATASHEET_OPENS = [FileKind.DATASHEET];

// hiddenNote words the pruned-mount count. A mount is something an operator configured by hand, so
// one missing from the tree has to be accounted for: without this line there is no way to tell
// "that folder holds nothing this page opens" from "that mount failed to resolve".
//
// `noun` is the plural the page uses for what it opens ("designs", "datasheets"), because the same
// sentence is wrong on the other page.
export function hiddenNote(hidden: number, shown: number, noun: string): string {
  const folders = `${hidden} ${hidden === 1 ? "folder" : "folders"}`;
  return shown === 0 ? `No ${noun} in any of the ${folders} being served` : `${folders} hidden (no ${noun})`;
}
