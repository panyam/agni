import type { SheetRef } from "./gen/agni/v1/webapi/design_pb.js";

// SheetsState is the current design's sheets plus which one is active. It carries the file
// (mount+path) so a view keyed by file (the tree) can attach the sheets under the right node.
export interface SheetsState {
  mount: string;
  path: string;
  sheets: SheetRef[];
  activeId: string;
}

// SheetsView is a navigation surface the presenter pushes SheetsState to (the top tabs and
// the file tree both implement it), so every sheet UI stays in sync from one source.
export interface SheetsView {
  setState(s: SheetsState): void;
}
