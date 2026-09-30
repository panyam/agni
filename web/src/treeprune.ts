// The pruning parts the two file trees share, namely the kinds each one declares to the server and
// the note it shows when the server pruned a mount out of its listing. The trees stay separate (the
// design tree nests sheets under a file and the datasheets tree does not), but their pruned-mount
// note must not diverge. See docsite/content/architecture/web-app.md#the-pages.

import { FileKind } from "./gen/agni/v1/webapi/workspace_pb.js";

// DESIGN_OPENS / DATASHEET_OPENS are what each tree passes as `opens`. Named constants so the request
// and the view filter cannot drift, since a tree that asked the server to prune by one kind and then
// filtered rows by another would hide folders it then showed files from.
export const DESIGN_OPENS = [FileKind.DESIGN];
export const DATASHEET_OPENS = [FileKind.DATASHEET];

// hiddenNote words the pruned-mount count. An operator configured each mount by hand, so one missing
// from the tree has to be accounted for, or nobody can tell "that folder holds nothing this page
// opens" from "that mount failed to resolve".
//
// `noun` is the plural the page uses for what it opens ("designs", "datasheets").
export function hiddenNote(hidden: number, shown: number, noun: string): string {
  const folders = `${hidden} ${hidden === 1 ? "folder" : "folders"}`;
  return shown === 0 ? `No ${noun} in any of the ${folders} being served` : `${folders} hidden (no ${noun})`;
}
