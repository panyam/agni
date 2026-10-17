package wasmengine

import (
	"io/fs"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// collector decides when a browser engine whose automatic collection is off collects (agni issue
// 1010). Go's wasm port runs on one thread with no asynchronous preemption, so a concurrent mark there
// runs beside whatever request is evaluating and slows it for as long as the cycle lasts. On Jetson
// two such cycles cost 3.0 and 2.5 s of a 7.8 s check, while a forced collection over the same heap
// took 20 to 57 ms. So the engine collects only between requests, and after each layer its
// DesignCache loads, and lets a request's garbage stand until it ends.
//
// After a request it collects only when the heap has doubled since the last collection, which is the
// growth GOGC=100 would have allowed, so a run of small requests does not pay a collection each. It
// does the same as each file of a mount opens, so a read that parses many files frees one file's parse
// before the next: opening Jetson built its drawing with the heap at 576 MB and 105 MB live, against
// 285 MB while the automatic cycles ran, and a wasm memory keeps its largest size.
type collector struct {
	heap  func() uint64 // the bytes the heap holds now
	gc    func()
	later func(func()) // runs a collection once the response has gone back

	mu   sync.Mutex
	live uint64 // the heap after the last collection
}

// processCollector is the one collector every engine in the process shares, since the heap it
// measures is the process's and the host rebuilds the engine on every mount change.
var processCollector = &collector{heap: heapBytes, gc: runtime.GC, later: afterTheResponse}

// afterTheResponse runs f on a timer rather than on a goroutine. The host sends a response back only
// once the handler returns, and Go's wasm scheduler runs every runnable goroutine before it yields to
// JavaScript, so a collection on a goroutine would still hold the page's answer. A sleeping one lets
// the event loop deliver it first.
func afterTheResponse(f func()) { time.AfterFunc(time.Millisecond, f) }

func heapBytes() uint64 {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms.HeapAlloc
}

// collect collects now and records what survived. DesignCache.AfterLoad calls it after each layer.
func (c *collector) collect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gc()
	c.live = c.heap()
}

// collectIfGrown collects when the heap has doubled since the last collection.
func (c *collector) collectIfGrown() { c.collectPast(0) }

// fileHeadroom is how far past the live heap a read may grow before a file's opening collects. A
// fixed allowance rather than doubling, since a read's live heap is small beside its garbage: doubling
// a 100 MB live heap collected so often that Jetson's drawing took 2.8 s to build rather than 1.8 s.
// At 128 MB the worker opened Jetson at 318 MB of wasm memory (312 MB with the automatic cycles, 606
// MB collecting only between requests, 388 MB at 256 MB), with its reads no slower.
const fileHeadroom = 128 << 20

// collectPast collects when the heap is more than headroom past the last collection's live heap, or
// has doubled when headroom is 0.
func (c *collector) collectPast(headroom uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	limit := 2 * c.live
	if headroom > 0 {
		limit = c.live + headroom
	}
	if c.heap() > limit {
		c.gc()
		c.live = c.heap()
	}
}

// after serves a request through h and then, once its response has gone back, collects if the heap
// has grown enough to.
func (c *collector) after(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)
		c.later(c.collectIfGrown)
	})
}

// files wraps a mount's files so that opening one first collects if the heap has grown enough to,
// which is the boundary between one file's parse and the next.
func (c *collector) files(fsys fs.FS) fs.FS { return collectingFS{fsys, c} }

type collectingFS struct {
	fs.FS
	c *collector
}

func (f collectingFS) Open(name string) (fs.File, error) {
	f.c.collectPast(fileHeadroom)
	return f.FS.Open(name)
}
