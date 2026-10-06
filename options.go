package agni

import (
	"io/fs"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/service"
	"github.com/panyam/agni/stdlib/profiles"
)

// datalogSourceName is the catalog source stdlib/rules/datalog registers under.
const datalogSourceName = "dl"

// builder accumulates what the options set, so New composes once from a complete picture rather
// than mutating an Engine per option. As in facts.Registry, collisions are checked once at the end
// instead of depending on the order the caller wrote the options in.
type builder struct {
	profiles    []profiles.Profile
	sources     []check.RuleSource
	factOptions []facts.Option
	store       service.ProjectStore
	config      service.ConfigResolver
	resolver    *service.ProjectResolver
	version     string
	noDatalog   bool
}

// Option configures New. Every option carries a VALUE rather than a path, since reading config is
// the caller's business (C22).
type Option func(*builder)

// WithProfiles composes interface profiles into the catalog. A profile whose Name matches a built-in
// SUPERSEDES that built-in's rules rather than running alongside them, and the review's profile
// index tracks that, so an item scoped to the interface cannot pass on rules no longer in the run.
//
// Load them with profiles.LoadDir for a directory of YAML, or build the values in Go.
func WithProfiles(ps []profiles.Profile) Option {
	return func(b *builder) { b.profiles = append(b.profiles, ps...) }
}

// WithSources composes arbitrary rule sources into the catalog: a house suite built in Go, a naming
// convention's rules, anything satisfying check.RuleSource. They compose after the profile
// sources, and a source implementing check.SupersedingSource replaces what it names.
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
// takes an fs.FS rather than a path because an fs.FS has no parent to climb into, so a resolution
// walk stops at the tree root.
//
// It exposes the default store without making internal/projects public API. A caller that outgrows
// the directory shape implements service.ProjectStore and passes WithProjectStore instead.
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
// with the services this package does not construct (the design, diff, query, check and review
// services all take the same resolver). It is the composed form of WithProjectStore plus WithConfigResolver and wins
// over both, so one run cannot resolve projects two ways.
func WithProjectResolver(r *service.ProjectResolver) Option {
	return func(b *builder) { b.resolver = r }
}

// WithConfigResolver supplies what resolves an analysis config's URIs into engine values: a
// project's interface profiles, its seeded parameters, its symbol paths, a design's intent. This is
// where per-design config enters a run, so a server serving several projects composes each one's own
// rules rather than applying one team's config to every design.
func WithConfigResolver(c service.ConfigResolver) Option {
	return func(b *builder) { b.config = c }
}

// WithProducerVersion stamps the build identity onto every results document the engine writes, a
// stored review run and a saved check report alike, so the document records which engine produced it. An embedder passes its own version string; the CLI passes the
// engine's.
func WithProducerVersion(v string) Option {
	return func(b *builder) { b.version = v }
}

// WithoutDatalogRules declares that shipping with no datalog-authored rule suite is intended, and
// drops the warning New would otherwise record. It changes nothing about the composition. The
// warning exists because the absence is invisible from a report.
func WithoutDatalogRules() Option {
	return func(b *builder) { b.noDatalog = true }
}
