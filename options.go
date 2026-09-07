package agni

import (
	"io/fs"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/service"
	"github.com/panyam/agni/stdlib/profiles"
	"github.com/panyam/agni/stdlib/rules/intent"
)

// datalogSourceName is the catalog source stdlib/rules/datalog registers under.
const datalogSourceName = "dl"

// builder accumulates what the options set, so New composes once from a complete picture rather
// than mutating an Engine as each option arrives. It is the shape facts.Registry uses and for the
// same reason: composing in a known order checks collisions once, at the end, instead of depending
// on the order the caller happened to write the options in.
type builder struct {
	profiles    []profiles.Profile
	intent      *intent.Declaration
	sources     []check.RuleSource
	factOptions []facts.Option
	store       service.ProjectStore
	config      service.ConfigResolver
	resolver    *service.ProjectResolver
	version     string
	noDatalog   bool
}

// Option configures New. Every option carries a VALUE rather than a path, because reading config is
// the caller's business and configuration travels as a value (C22).
type Option func(*builder)

// WithProfiles composes interface profiles into the catalog. A profile whose Name matches a built-in
// SUPERSEDES that built-in's rules rather than running alongside them, and the review's profile
// index tracks that replacement, so an item scoped to the interface cannot score a clean pass on a
// profile whose rules are no longer in the run.
//
// Load them with profiles.LoadDir for a directory of YAML, or build the values in Go. Either way the
// file reading happens in the caller.
func WithProfiles(ps []profiles.Profile) Option {
	return func(b *builder) { b.profiles = append(b.profiles, ps...) }
}

// WithIntent composes a design-intent declaration into the catalog, which is what flips an
// intent-bound review item from needs-design-intent to a real verdict. Intent is per-DESIGN, so an
// Engine composed with one is scoped to that design; a server serving many resolves intent per
// design through the project config instead.
//
// Load it with intent.LoadFile for YAML, or build the Declaration in Go.
func WithIntent(d intent.Declaration) Option {
	return func(b *builder) { b.intent = &d }
}

// WithSources composes arbitrary rule sources into the catalog: a house suite built in Go, a naming
// convention's rules, anything satisfying check.RuleSource. They compose after the profile and
// intent sources, and a source implementing check.SupersedingSource replaces what it names.
func WithSources(srcs ...check.RuleSource) Option {
	return func(b *builder) { b.sources = append(b.sources, srcs...) }
}

// WithRelations composes extra fact relations into the registry, for an overlay that projects its
// own tuples out of the Model. The built-in relation catalog arrives through the blank import of
// stdlib/relations rather than through this option, because it installs as one bulk payload.
func WithRelations(opts ...facts.Option) Option {
	return func(b *builder) { b.factOptions = append(b.factOptions, opts...) }
}

// WithProjectStore supplies the store that answers which projects and designs exist and which design
// an artifact belongs to. A deployment backed by a PLM system, an index, or a database implements
// service.ProjectStore and passes it here; WithFSProjectStore is the shipped directory-walking one.
func WithProjectStore(s service.ProjectStore) Option {
	return func(b *builder) { b.store = s }
}

// Tree is one named filesystem WithFSProjectStore searches for project and design descriptors. Mount
// is the name a mount:// URI addresses the tree by, and FS is its root.
type Tree struct {
	Mount string
	FS    fs.FS
}

// WithFSProjectStore supplies the shipped project store, which walks each tree for descriptors. It
// takes an fs.FS rather than a path so containment is structural: an fs.FS has no parent to climb
// into, so a resolution walk stops at the tree root.
//
// This is how the default store reaches a caller without the package that implements it becoming
// public API. Everything true only of storing projects in DIRECTORIES stays behind
// service.ProjectStore, which is the contract; a caller that outgrows the directory shape implements
// the port and passes WithProjectStore instead.
func WithFSProjectStore(trees ...Tree) Option {
	return func(b *builder) {
		ts := make([]projects.Tree, 0, len(trees))
		for _, t := range trees {
			ts = append(ts, projects.Tree{Mount: t.Mount, FS: t.FS})
		}
		b.store = projects.NewFSStore(ts...)
	}
}

// WithProjectResolver supplies an already-composed resolver, for a caller that built one to share
// with the services this package does not construct (the design, diff and query services all take
// the same resolver). It is the composed form of WithProjectStore plus WithConfigResolver and wins
// over both, so one run cannot resolve projects two ways.
func WithProjectResolver(r *service.ProjectResolver) Option {
	return func(b *builder) { b.resolver = r }
}

// WithConfigResolver supplies what resolves an analysis config's URIs into engine values: a
// project's interface profiles, its seeded parameters, its symbol paths, a design's intent. It is
// the seam where per-design config enters a run, so a server serving several projects gives each
// one its own composed rules rather than applying one team's config to every design it reads.
func WithConfigResolver(c service.ConfigResolver) Option {
	return func(b *builder) { b.config = c }
}

// WithProducerVersion stamps the build identity onto a review's results document, so a stored run
// records which engine produced it. An embedder passes its own version string; the CLI passes the
// engine's.
func WithProducerVersion(v string) Option {
	return func(b *builder) { b.version = v }
}

// WithoutDatalogRules declares that shipping with no datalog-authored rule suite is deliberate, and
// drops the warning New would otherwise record. It changes nothing about the composition. The
// warning exists because the absence is invisible from a report, and this option is how a caller who
// meant it says so once instead of filtering the string.
func WithoutDatalogRules() Option {
	return func(b *builder) { b.noDatalog = true }
}
