package service

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/core/timing"
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
// on every mount change, which drops its in-memory entries with it, and the parsed design, drawing and
// board are also kept in a BlobStore that outlives the worker (see Persist).
type DesignCache struct {
	mu       sync.Mutex
	max      int
	entries  map[string]*cacheEntry
	tick     uint64
	hits     atomic.Int64
	misses   atomic.Int64
	restored atomic.Int64
	store    *persistTier
}

// A BlobStore keeps bytes across processes, for a DesignCache's persistent tier (agni issue 911). It
// is the shape of goapplib's wasmhost.Cache, which the browser engine hands it, so this package names
// no host. Any error from Get, a miss included, means the read runs again, and an error from Put
// means only that the next process reads again too. A key is 64 lowercase hex digits.
type BlobStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Put(ctx context.Context, key string, b []byte) error
}

// A FingerprintCheck reports whether a stored read's content fingerprint (formats.Touched.Fingerprint)
// still matches the files the host serves now, and if so returns a recorder of those names as they
// stamp now. It is the host's, because only the host can open the files (C13);
// formats.CheckFingerprint over the Loader's FS is the usual one.
type FingerprintCheck func(fp []byte) (*formats.Touched, bool)

// persistTier is where a DesignCache keeps its proto layers past the process.
type persistTier struct {
	store   BlobStore
	check   FingerprintCheck
	version string
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

// Persist keeps the cache's parsed designs, drawings and boards in store as well, so another process
// over the same files restores them rather than reading them again (agni issue 911). The model and
// fact base are rebuilt from a restored design, at about a tenth of a read's cost, because they have
// no serialized form and C8 keeps it that way.
//
// A stored entry carries a content fingerprint of every name its read touched
// (formats.Touched.Fingerprint) and is used only when check says they all read the same now, so a file
// edited, added or removed since reads again, as it does in memory. version names what wrote the entry: an engine that reads a file differently must pass a
// different one, or it restores what the older engine parsed. An empty version persists nothing.
func (c *DesignCache) Persist(store BlobStore, check FingerprintCheck, version string) *DesignCache {
	if c != nil && store != nil && check != nil && version != "" {
		c.store = &persistTier{store: store, check: check, version: version}
	}
	return c
}

// Restored is how many misses were answered from the persistent tier rather than by a read.
func (c *DesignCache) Restored() int64 {
	if c == nil {
		return 0
	}
	return c.restored.Load()
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
	return c.getPersisted(ctx, key, nil, build)
}

// getPersisted is get with a codec for values the persistent tier may keep. On a miss it tries the
// store before building, and stores what it built.
func (c *DesignCache) getPersisted(ctx context.Context, key string, codec *protoCodec, build func(context.Context) (any, error)) (any, error) {
	layer, _, _ := strings.Cut(key, "\x00")
	if c == nil || c.max <= 0 || key == "" {
		timing.Cached(ctx, layer, "uncached")
		defer timing.Begin(ctx, "build."+layer)()
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
				timing.Cached(ctx, layer, "memory")
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
		endRestore := func() {}
		if c.store != nil {
			endRestore = timing.Begin(ctx, "restore."+layer)
		}
		v, t, ok := c.restore(ctx, key, codec)
		endRestore()
		if ok {
			e.val, e.touched = v, t
			c.restored.Add(1)
			timing.Cached(ctx, layer, "store")
		} else {
			timing.Cached(ctx, layer, "built")
			endBuild := timing.Begin(ctx, "build."+layer)
			e.val, e.err = build(withTouched(ctx, e.touched))
			endBuild()
			if e.err == nil {
				c.save(ctx, key, codec, e.val, e.touched)
			}
		}
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

// storeKey is key's name in the persistent tier: the hash of the writer's version and key, so an
// entry another engine version wrote is never found.
func (p *persistTier) storeKey(key string) string {
	h := sha256.Sum256([]byte(p.version + "\x00" + key))
	return hex.EncodeToString(h[:])
}

// restore returns key's value from the persistent tier when its fingerprint still matches the files.
func (c *DesignCache) restore(ctx context.Context, key string, codec *protoCodec) (any, *formats.Touched, bool) {
	if c.store == nil || codec == nil {
		return nil, nil, false
	}
	b, err := c.store.store.Get(ctx, c.store.storeKey(key))
	if err != nil {
		return nil, nil, false
	}
	n, k := binary.Uvarint(b)
	if k <= 0 || uint64(len(b)-k) < n {
		return nil, nil, false
	}
	fp, payload := b[k:k+int(n)], b[k+int(n):]
	t, ok := c.store.check(fp)
	if !ok {
		return nil, nil, false
	}
	v, err := codec.decode(payload)
	if err != nil {
		return nil, nil, false
	}
	return v, t, true
}

// save stores a value the persistent tier may keep. A read that cannot be fingerprinted, or a store
// that refuses, leaves only the in-memory entry.
func (c *DesignCache) save(ctx context.Context, key string, codec *protoCodec, v any, t *formats.Touched) {
	if c.store == nil || codec == nil {
		return
	}
	fp, ok := t.Fingerprint()
	if !ok {
		return
	}
	payload, err := codec.encode(v)
	if err != nil {
		return
	}
	b := binary.AppendUvarint(make([]byte, 0, binary.MaxVarintLen64+len(fp)+len(payload)), uint64(len(fp)))
	b = append(append(b, fp...), payload...)
	_ = c.store.store.Put(ctx, c.store.storeKey(key), b)
}

// protoCodec writes one of the cache's proto layers for the persistent tier. A layer may be absent (a
// design with no board), which is a value worth keeping too, so the first byte says which.
type protoCodec struct{ empty func() proto.Message }

func (pc *protoCodec) encode(v any) ([]byte, error) {
	m, _ := v.(proto.Message)
	if m == nil || !m.ProtoReflect().IsValid() {
		return []byte{0}, nil
	}
	b, err := proto.Marshal(m)
	if err != nil {
		return nil, err
	}
	return append([]byte{1}, b...), nil
}

func (pc *protoCodec) decode(b []byte) (any, error) {
	m := pc.empty()
	if len(b) == 0 || b[0] == 0 {
		// The typed nil each loader method returns for an absent layer.
		return m.ProtoReflect().Type().Zero().Interface(), nil
	}
	if err := proto.Unmarshal(b[1:], m); err != nil {
		return nil, err
	}
	return m, nil
}

var (
	designCodec   = &protoCodec{empty: func() proto.Message { return &ir.Design{} }}
	geometryCodec = &protoCodec{empty: func() proto.Message { return &geom.SchematicGeometry{} }}
	boardCodec    = &protoCodec{empty: func() proto.Message { return &geom.BoardGeometry{} }}
)

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
	v, err := l.cache.getPersisted(ctx, key, designCodec, func(ctx context.Context) (any, error) {
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
	v, err := l.cache.getPersisted(ctx, key, geometryCodec, func(ctx context.Context) (any, error) {
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
	v, err := l.cache.getPersisted(ctx, key, boardCodec, func(ctx context.Context) (any, error) {
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
	return b.baseFor(context.Background(), reg)
}

// baseFor is base, timing the projection as the request's "factbase" stage when it builds one.
func (b *builtModel) baseFor(ctx context.Context, reg *facts.Registry) *query.Base {
	b.mu.Lock()
	defer b.mu.Unlock()
	if qb, ok := b.bases[reg]; ok {
		return qb
	}
	defer timing.Begin(ctx, "factbase")()
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
		return m, func(reg *facts.Registry) *query.Base { return b.baseFor(ctx, reg) }, nil
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
	return b.model, func(reg *facts.Registry) *query.Base { return b.baseFor(ctx, reg) }, nil
}
