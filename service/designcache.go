package service

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/core/query"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/readers/formats"
	"google.golang.org/protobuf/proto"
)

// A DesignCache keeps what reading a design produced, so a second request for the same design does
// not read it again (agni issue 895). It holds two layers. CachingLoader keeps the parsed design, its
// drawings and its board, which every surface reads through. BuildModelCached keeps the check model
// and its fact base on top, which a query or a check needs and building costs a further 100ms on a
// large board.
//
// An entry is keyed by what was asked (the URI, the layout, the read options' identity) and checked
// on every hit against what the read touched: every file it opened, walked or looked for, as a
// formats.Touched. A hit whose files stamp differently is read again. So the key never has to name a
// file, which is what lets a hierarchical schematic's sub-sheets, a declared companion and a symbol
// library all invalidate it without anyone listing them.
//
// Two rules keep it from serving a wrong answer. A read whose options carry no identity is never
// cached (see WithIdentity), and neither is a read that recorded no files, since that read went
// around the formats.Loader and nothing could tell when it went stale.
//
// It is the same code in `agni serve` and the browser engine. In the browser the engine is rebuilt
// on every mount change, which drops its cache with it.
type DesignCache struct {
	mu      sync.Mutex
	max     int
	entries map[string]*cacheEntry
	tick    uint64
	hits    atomic.Int64
	misses  atomic.Int64
}

type cacheEntry struct {
	ready   chan struct{}
	val     any
	err     error
	touched *formats.Touched
	used    uint64
}

// NewDesignCache keeps at most max entries, evicting the least recently used. Each layer's entry
// counts once, so a design asked for by both a query and a drawing holds about four. max <= 0 means
// a cache that keeps nothing.
func NewDesignCache(max int) *DesignCache {
	return &DesignCache{max: max, entries: map[string]*cacheEntry{}}
}

// Stats reports hits and misses since the cache was made, for a log line or a test.
func (c *DesignCache) Stats() (hits, misses int64) {
	if c == nil {
		return 0, 0
	}
	return c.hits.Load(), c.misses.Load()
}

// get returns key's value, building it on a miss. Concurrent callers for one key share one build.
// What the value's read touched is merged into the caller's recorder (see withTouched), so a value
// built from another cached value stays checkable against both reads' files.
func (c *DesignCache) get(ctx context.Context, key string, build func(context.Context) (any, error)) (any, error) {
	if c == nil || c.max <= 0 || key == "" {
		return build(ctx)
	}
	outer := touchedFrom(ctx)
	for {
		c.mu.Lock()
		e, ok := c.entries[key]
		if ok {
			c.tick++
			e.used = c.tick
			c.mu.Unlock()
			select {
			case <-e.ready:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			if e.err == nil && e.touched.Unchanged() {
				c.hits.Add(1)
				outer.Merge(e.touched)
				return e.val, nil
			}
			// Stale, or the build failed: drop it, unless someone already replaced it, and go again.
			c.mu.Lock()
			if c.entries[key] == e {
				delete(c.entries, key)
			}
			c.mu.Unlock()
			if e.err != nil {
				// A failed build is not retried here, so one bad read is not run twice for one request.
				return nil, e.err
			}
			continue
		}
		c.misses.Add(1)
		e = &cacheEntry{ready: make(chan struct{}), touched: formats.NewTouched()}
		c.tick++
		e.used = c.tick
		c.entries[key] = e
		c.evictLocked()
		c.mu.Unlock()

		// The build runs under the caller's context, so a cancelled request stops it, and the entry
		// it leaves is an error that is never served (the next caller builds again).
		e.val, e.err = build(withTouched(ctx, e.touched))
		if e.err == nil && e.touched.Len() == 0 {
			// Nothing recorded, so nothing could say when this went stale. Serve it to this caller
			// alone.
			c.mu.Lock()
			if c.entries[key] == e {
				delete(c.entries, key)
			}
			c.mu.Unlock()
		}
		if e.err != nil {
			c.mu.Lock()
			if c.entries[key] == e {
				delete(c.entries, key)
			}
			c.mu.Unlock()
		}
		close(e.ready)
		outer.Merge(e.touched)
		return e.val, e.err
	}
}

func (c *DesignCache) evictLocked() {
	for len(c.entries) > c.max {
		var oldest string
		var at uint64
		for k, e := range c.entries {
			if oldest == "" || e.used < at {
				oldest, at = k, e.used
			}
		}
		delete(c.entries, oldest)
	}
}

// cacheKey joins a key's parts. The separator is a byte no URI or identity carries.
func cacheKey(parts ...string) string { return strings.Join(parts, "\x00") }

// optsKey is the identity of a read's options, and false when they have none (see WithIdentity).
// No options at all is an identity of its own: the loader's defaults, fixed for the process.
func optsKey(opts []ReadOption) (string, bool) {
	if len(opts) == 0 {
		return "-", true
	}
	o := ReadOpts(opts...)
	if o.Identity == "" || o.identityCovers != o.fields {
		return "", false
	}
	return o.Identity, true
}

// CacheableLoader is what a host hands its services: the read ports a CachingLoader wraps.
type CacheableLoader interface {
	Loader
	ReviewLoader
	ConventionLoader
}

// CachingLoader is a host's loader with its design, drawing and board reads kept in a DesignCache.
// Every other port passes through. Each answer is a copy of what the cache holds, because callers
// stamp classes into a design they were handed (check.NewModel's datasheet pass), and a copy of a
// large board's design and drawing costs about 10ms against a read of over a second.
type CachingLoader struct {
	CacheableLoader
	cache *DesignCache
}

// NewCachingLoader wraps l. A nil cache passes every read through.
func NewCachingLoader(l CacheableLoader, c *DesignCache) *CachingLoader {
	return &CachingLoader{CacheableLoader: l, cache: c}
}

// Cache is the DesignCache this loader reads through, for the services that keep a model on top.
func (l *CachingLoader) Cache() *DesignCache { return l.cache }

// Design returns a copy of the design's IR, read once per identity while its files are unchanged.
func (l *CachingLoader) Design(ctx context.Context, uri artifact.URI, opts ...ReadOption) (*ir.Design, error) {
	ok, key := l.key("design", uri.String(), opts)
	if !ok {
		return l.CacheableLoader.Design(ctx, uri, opts...)
	}
	v, err := l.cache.get(ctx, key, func(ctx context.Context) (any, error) {
		return l.CacheableLoader.Design(ctx, uri, opts...)
	})
	if err != nil {
		return nil, err
	}
	return proto.Clone(v.(*ir.Design)).(*ir.Design), nil
}

// Geometry returns a copy of the drawing, read once per layout, symbol source and identity.
func (l *CachingLoader) Geometry(ctx context.Context, uri artifact.URI, layout string, faithfulSymbols bool, opts ...ReadOption) (*geom.SchematicGeometry, error) {
	sym := "glyph"
	if faithfulSymbols {
		sym = "faithful"
	}
	ok, key := l.key("geometry", cacheKey(uri.String(), layout, sym), opts)
	if !ok {
		return l.CacheableLoader.Geometry(ctx, uri, layout, faithfulSymbols, opts...)
	}
	v, err := l.cache.get(ctx, key, func(ctx context.Context) (any, error) {
		return l.CacheableLoader.Geometry(ctx, uri, layout, faithfulSymbols, opts...)
	})
	if err != nil || v == nil {
		return nil, err
	}
	g, _ := v.(*geom.SchematicGeometry)
	if g == nil {
		return nil, nil
	}
	return proto.Clone(g).(*geom.SchematicGeometry), nil
}

// Board returns a copy of the board sidecar. A board read takes no options, so its key is the URI.
func (l *CachingLoader) Board(ctx context.Context, uri artifact.URI) (*geom.BoardGeometry, error) {
	ok, key := l.key("board", uri.String(), nil)
	if !ok {
		return l.CacheableLoader.Board(ctx, uri)
	}
	v, err := l.cache.get(ctx, key, func(ctx context.Context) (any, error) {
		return l.CacheableLoader.Board(ctx, uri)
	})
	if err != nil || v == nil {
		return nil, err
	}
	b, _ := v.(*geom.BoardGeometry)
	if b == nil {
		return nil, nil
	}
	return proto.Clone(b).(*geom.BoardGeometry), nil
}

func (l *CachingLoader) key(kind, what string, opts []ReadOption) (bool, string) {
	if l.cache == nil {
		return false, ""
	}
	id, ok := optsKey(opts)
	if !ok {
		return false, ""
	}
	return true, cacheKey(kind, what, id)
}

// cacheOf is the DesignCache a service's loader reads through, or nil.
func cacheOf(l any) *DesignCache {
	if c, ok := l.(interface{ Cache() *DesignCache }); ok {
		return c.Cache()
	}
	return nil
}

// builtModel is a model and the fact bases built over it, kept together because a query asks for a
// base and a check for the model, and each is the slow half of the other's request.
type builtModel struct {
	model check.Model
	mu    sync.Mutex
	bases map[*facts.Registry]*query.Base
}

// base returns the fact base over reg, built once. The registry is the overlay's, and the overlay is
// in the model's key, so one model rarely sees more than one.
func (b *builtModel) base(reg *facts.Registry) *query.Base {
	b.mu.Lock()
	defer b.mu.Unlock()
	if qb, ok := b.bases[reg]; ok {
		return qb
	}
	qb := query.NewBaseFrom(reg, b.model)
	b.bases[reg] = qb
	return qb
}

// BuildModelCached is BuildModel kept in the loader's DesignCache, keyed on the tiers and the
// overlay's identity. The model is SHARED between requests rather than copied, which is safe because
// rules only read it and its memo (check.Memo) is built for concurrent use. A request whose overlay
// cannot identify itself, or whose datasheet corpus is fetched over the network (where nothing here
// can tell that it changed), builds its own.
//
// It returns the fact base too, built on first use, so a query does not pay 100ms to project every
// relation again.
func BuildModelCached(ctx context.Context, loader ModelLoader, uri, boardURI artifact.URI, ov Overlay, serverSpecs param.ParamProvider) (check.Model, func(*facts.Registry) *query.Base, error) {
	specs := ov.SpecsOver(serverSpecs)
	opts := ov.ReadOptions()
	fresh := func() (check.Model, func(*facts.Registry) *query.Base, error) {
		m, err := BuildModel(ctx, loader, uri, boardURI, specs, opts...)
		if err != nil {
			return nil, nil, err
		}
		b := &builtModel{model: m, bases: map[*facts.Registry]*query.Base{}}
		return m, b.base, nil
	}
	c := cacheOf(loader)
	id, ok := optsKey(opts)
	if c == nil || !ok || param.Fetches(specs) {
		return fresh()
	}
	key := cacheKey("model", uri.String(), boardURI.String(), id)
	v, err := c.get(ctx, key, func(ctx context.Context) (any, error) {
		m, err := BuildModel(ctx, loader, uri, boardURI, specs, opts...)
		if err != nil {
			return nil, err
		}
		return &builtModel{model: m, bases: map[*facts.Registry]*query.Base{}}, nil
	})
	if err != nil {
		return nil, nil, err
	}
	b := v.(*builtModel)
	return b.model, b.base, nil
}
