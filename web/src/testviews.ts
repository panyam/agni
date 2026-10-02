// Complete, typed view stubs for presenter tests. A partial literal cast with `as any` hides a
// method ADDED to a view interface until the presenter calls it and the stub throws in an unrelated
// test (agni issues 338, 341, 259). Here tsc checks the stubs, so a new view method is one edit HERE.
// Pass overrides for the ports a test asserts on:
//
//	stubQueryView({ setFindings: vi.fn() })
//
// No vitest import, so src/ carries no test-runner dependency. Defaults are no-ops and a test that
// asserts supplies its own spy.

import type { QueryView } from "./query.js";

// stubQueryView returns a QueryView whose every port is a no-op, with `over` applied on top.
export function stubQueryView(over: Partial<QueryView> = {}): QueryView {
  return {
    setState: () => {},
    setRelations: () => {},
    setExamples: () => {},
    setLocateNote: () => {},
    setQuery: () => {},
    setEntityQueries: () => {},
    setSearch: () => {},
    setSelection: () => {},
    setCurrentSheet: () => {},
    setFindings: () => {},
    entityQuery: () => undefined,
    ...over,
  };
}
