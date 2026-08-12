package main

import (
	"github.com/panyam/agni/internal/service"
	"github.com/panyam/agni/readers/formats"
)

// readerFor picks the formats.Loader one read should use: the shared one when the read carries no
// options, else a COPY carrying that read's naming lexicon. Copying is what keeps a per-request
// project convention from leaking — the shared loader is never mutated, so two concurrent reads with
// different conventions cannot see each other's (WS3-102, on the WS3-106 value).
//
// A nil base is a supported caller (see formats.Loader's own nil handling), so it stays nil rather
// than being dereferenced; the copy then starts from a zero loader carrying only the lexicon.
func readerFor(base *formats.Loader, opts ...service.ReadOption) *formats.Loader {
	o := service.ReadOpts(opts...)
	if o.Lexicon == nil && len(o.SymbolPaths) == 0 {
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
	return &cp
}
