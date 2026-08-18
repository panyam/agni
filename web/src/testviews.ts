// Complete view stubs for presenter tests.
//
// The presenter pushes to a dozen view ports and a test cares about two of them, so every test
// built its own partial literal and reached for `as any` to make the other ten go away. That cast
// is the problem: it puts the stub outside tsc's reach, so a method ADDED to a view interface is
// invisible until the presenter calls it and the stub throws, in a test file that has nothing to do
// with the change. It happened three times in three consecutive PRs (agni issues 338, 341, 259).
//
// These builders are complete and typed, so the compiler checks them, and a new method on a view
// interface is one edit HERE rather than a runtime failure somewhere else. Pass overrides for the
// handful a given test asserts on:
//
//	stubQueryView({ setFindings: vi.fn() })
//
// They deliberately do not import vitest. A stub whose default is a no-op works for the ports a
// test ignores, and a test that wants to assert supplies its own spy, which keeps this file free of
// a test-runner dependency inside src/.

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
    entityQuery: () => "",
    ...over,
  };
}
