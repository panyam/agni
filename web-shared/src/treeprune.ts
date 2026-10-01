// The pruned-mount note the two file trees share, the viewer's design tree and the datasheets
// workbench's tree. The trees stay separate (the design tree nests sheets under a file and the
// datasheets tree does not), but their note must not diverge. Each tree declares its own `opens`
// kinds beside its own generated FileKind. See docsite/content/architecture/web-app.md#the-pages.

// hiddenNote words the pruned-mount count. An operator configured each mount by hand, so one missing
// from the tree has to be accounted for, or nobody can tell "that folder holds nothing this page
// opens" from "that mount failed to resolve".
//
// `noun` is the plural the page uses for what it opens ("designs", "datasheets").
export function hiddenNote(hidden: number, shown: number, noun: string): string {
  const folders = `${hidden} ${hidden === 1 ? "folder" : "folders"}`;
  return shown === 0 ? `No ${noun} in any of the ${folders} being served` : `${folders} hidden (no ${noun})`;
}
