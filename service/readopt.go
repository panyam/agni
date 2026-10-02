package service

import (
	"github.com/panyam/agni/core/classify"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
)

// ReadOptions is the per-read configuration a service passes down to its loader, for inputs that
// belong to the READ rather than to the rule catalog. A naming lexicon resolves net roles once at
// ingestion (WS3-072), so it has to arrive before the design is parsed.
//
// The options are variadic on the Design method, so the type system forces every loader to accept
// them. An optional capability found by type assertion would let a loader silently ignore a
// project's conventions, which reads like a design that had none.
type ReadOptions struct {
	// Lexicon is the naming vocabulary to stamp the design with; nil means the engine defaults.
	Lexicon *classify.Lexicon
	// SymbolPaths are directories to search for the schematic's external symbol libraries, ADDED to
	// whatever the loader was built with. They belong to the read because an unresolved symbol changes
	// what the design CONTAINS, not what is checked about it.
	SymbolPaths []string
	// DeviceClassFor answers a datasheet's device_class for an MPN, nil when this run has no corpus.
	// Like the lexicon, the class is stamped into the IR at ingestion. A model built later re-runs the
	// same additive, idempotent pass, so a corpus arriving after the read still gets the class, but
	// only a class stamped here reaches the DRAWING (agni issue 710).
	DeviceClassFor func(mpn string) string
	// Intent is the design's declared intent, for the MODEL built from this read rather than for the
	// read itself: BuildModel hands it to check.WithIntent so the exposure rules know which connectors
	// the design declares internal (agni issue 831). It rides the read options because every surface
	// that builds a model already passes Overlay.ReadOptions, so none can miss it.
	Intent *configpb.DesignIntent
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

// WithDeviceClasses supplies the datasheet device-class lookup for one read, so a design read inside
// a project that declares a params tier carries the classes only its corpus can establish.
func WithDeviceClasses(f func(mpn string) string) ReadOption {
	return func(o *ReadOptions) { o.DeviceClassFor = f }
}

// WithDesignIntent carries a design's declared intent to the model BuildModel builds (see
// ReadOptions.Intent).
func WithDesignIntent(di *configpb.DesignIntent) ReadOption {
	return func(o *ReadOptions) { o.Intent = di }
}

// ReadOpts resolves options to a value, for a loader implementation to read. Exported because the
// loaders live in cmd/agni (C1/C13).
func ReadOpts(opts ...ReadOption) ReadOptions {
	var o ReadOptions
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
