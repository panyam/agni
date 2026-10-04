package service

import (
	"context"

	"github.com/panyam/agni/readers/formats"
)

// LoaderFor picks the formats.Loader one read should use. A read with no options gets the shared
// loader, and any other read gets a COPY carrying its options. The shared loader is never mutated, so
// two concurrent reads with different project conventions cannot see each other's (WS3-102, on the
// WS3-106 value).
//
// A nil base is supported (see formats.Loader's own nil handling) and is never dereferenced. The copy
// then starts from a zero loader carrying only the read's options.
//
// Every loader adapter calls this, the CLI's, the server's and the in-memory one the wasm engine
// uses, so a read option reaches the read the same way whichever host made it.
func LoaderFor(base *formats.Loader, opts ...ReadOption) *formats.Loader {
	o := ReadOpts(opts...)
	if o.Lexicon == nil && len(o.SymbolPaths) == 0 && o.DeviceClassFor == nil {
		return base
	}
	cp := formats.Loader{}
	if base != nil {
		cp = *base
	}
	if o.Lexicon != nil {
		cp.Lexicon = o.Lexicon
	}
	// ADDED to whatever the loader was built with rather than replacing it, so an operator's
	// --symbol-path and a project's declared library both resolve. The flag stays the escape hatch for
	// a library the project does not know about.
	if len(o.SymbolPaths) > 0 {
		cp.SymbolPaths = append(append([]string{}, cp.SymbolPaths...), o.SymbolPaths...)
	}
	// The read's datasheet corpus, so the classes only a spec can establish are stamped into the IR
	// the DRAWING is built from and not only into a check model (agni issue 710). It replaces rather
	// than composes, since two corpora for one read would give two answers to one question.
	if o.DeviceClassFor != nil {
		cp.DeviceClassFor = o.DeviceClassFor
	}
	return &cp
}

type touchedKey struct{}

// withTouched carries a recorder for the reads made under ctx (see LoaderIn).
func withTouched(ctx context.Context, t *formats.Touched) context.Context {
	return context.WithValue(ctx, touchedKey{}, t)
}

// touchedFrom is the recorder ctx carries, or nil.
func touchedFrom(ctx context.Context) *formats.Touched {
	t, _ := ctx.Value(touchedKey{}).(*formats.Touched)
	return t
}

// LoaderIn is LoaderFor for a read made on behalf of ctx. When a DesignCache is building an entry,
// ctx carries the recorder the read's files go into, and this hands it to the Loader. A host's
// Design, Geometry, Report and Board reads go through this, and a read that does not is simply never
// cached.
func LoaderIn(ctx context.Context, base *formats.Loader, opts ...ReadOption) *formats.Loader {
	l := LoaderFor(base, opts...)
	t := touchedFrom(ctx)
	if t == nil {
		return l
	}
	cp := formats.Loader{}
	if l != nil {
		cp = *l
	}
	cp.Touched = t
	return &cp
}
