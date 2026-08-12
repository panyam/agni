package service

import "github.com/panyam/agni/core/classify"

// ReadOptions is the per-read configuration a service passes down to its loader. It exists because
// some inputs belong to the READ rather than to the rule catalog: a naming lexicon resolves net roles
// once at ingestion (WS3-072), so it has to arrive before the design is parsed, not after.
//
// It rides the Design method as variadic options so every existing call site is unchanged while the
// type system still forces each loader implementation to accept them. An optional
// capability-by-type-assertion would have let a loader silently ignore a project's conventions, which
// reads exactly like a design that had none.
type ReadOptions struct {
	// Lexicon is the naming vocabulary to stamp the design with; nil means the engine defaults.
	Lexicon *classify.Lexicon
	// SymbolPaths are directories to search for the schematic's external symbol libraries, ADDED to
	// whatever the loader was built with. They ride the read rather than the catalog because an
	// unresolved symbol changes what the design CONTAINS, not what is checked about it.
	SymbolPaths []string
}

// ReadOption configures one read.
type ReadOption func(*ReadOptions)

// WithLexicon stamps the design being read with a project's naming vocabulary (WS3-106) instead of
// the built-in one, so which nets count as rails and grounds follows the request's conventions.
func WithLexicon(lex *classify.Lexicon) ReadOption {
	return func(o *ReadOptions) { o.Lexicon = lex }
}

// WithSymbolPaths adds a config's symbol search directories to one read, so a design whose project
// declares its libraries resolves them without the caller passing a flag.
func WithSymbolPaths(dirs []string) ReadOption {
	return func(o *ReadOptions) { o.SymbolPaths = append(o.SymbolPaths, dirs...) }
}

// ReadOpts resolves options to a value, for a loader implementation to read. Exported because the
// implementations live at the cmd edge, where the file I/O is (C1/C13).
func ReadOpts(opts ...ReadOption) ReadOptions {
	var o ReadOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
