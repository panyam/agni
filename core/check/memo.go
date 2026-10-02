package check

import "sync"

// Memoizer is a Model that can keep values derived from it, so work every rule would otherwise
// repeat is done once per model. The model built by NewModel is one.
type Memoizer interface {
	Memo(key any, build func() any) any
}

// Memo returns the value cached on m under key, building it with build the first time it is asked
// for. A model that is not a Memoizer builds every time, which is correct and only slower.
//
// It exists for derived structures several rules read whole, such as the datalog fact base every
// query-backed rule evaluates over (agni issue 810): rebuilding it per rule was most of a check's
// time on a large board. A value cached here must be a pure function of the model and the key, and
// must not be mutated by whoever reads it, because every later caller gets the same one. key should
// be a value of a type private to the caller's package, so two packages cannot collide.
func Memo(m Model, key any, build func() any) any {
	if mz, ok := m.(Memoizer); ok {
		return mz.Memo(key, build)
	}
	return build()
}

type memoEntry struct {
	once sync.Once
	v    any
}

// Memo implements Memoizer. Concurrent callers asking for one key share a single build.
func (m *irModel) Memo(key any, build func() any) any {
	e, _ := m.memo.LoadOrStore(key, &memoEntry{})
	entry := e.(*memoEntry)
	entry.once.Do(func() { entry.v = build() })
	return entry.v
}
