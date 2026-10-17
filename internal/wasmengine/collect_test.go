package wasmengine

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeHeap is a heap a test grows by hand, and a collection shrinks to what it says survives.
type fakeHeap struct {
	bytes, survives uint64
	gcs             int
}

func (f *fakeHeap) collector() *collector {
	return &collector{heap: func() uint64 { return f.bytes }, gc: func() { f.gcs++; f.bytes = f.survives }, later: func(g func()) { g() }}
}

// agni issue 1010: with automatic collection off, a request collects after it ends only once the heap
// has doubled since the last collection, the growth GOGC=100 allows.
func TestCollectorCollectsAfterARequestOnlyOnceTheHeapDoubled(t *testing.T) {
	f := &fakeHeap{bytes: 100, survives: 100}
	c := f.collector()
	c.collect()
	if f.gcs != 1 {
		t.Fatalf("collect ran %d collections, want 1", f.gcs)
	}
	served := 0
	h := c.after(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { served++ }))
	ask := func() { h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/", nil)) }

	f.bytes = 200 // doubled exactly, not past it
	ask()
	if f.gcs != 1 {
		t.Fatalf("a heap at twice its live size collected (%d collections), want none until it is past that", f.gcs)
	}
	f.bytes, f.survives = 201, 120
	ask()
	if f.gcs != 2 {
		t.Fatalf("a heap past twice its live size ran %d collections, want 2", f.gcs)
	}
	// The bar is now what survived that collection, 120, so 240 is under it and 241 past it.
	f.bytes = 240
	ask()
	f.bytes = 241
	ask()
	if f.gcs != 3 || served != 4 {
		t.Fatalf("got %d collections over %d requests, want 3 over 4", f.gcs, served)
	}
}

// The option is what turns the per-request collection on, and an engine without it serves the mux
// itself, so a native caller pays nothing.
func TestOnlyAnEngineAskedToCollectWrapsItsHandler(t *testing.T) {
	plain, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := plain.Handler.(*http.ServeMux); !ok {
		t.Fatalf("an engine without CollectBetweenRequests serves %T, want the *http.ServeMux", plain.Handler)
	}
	collecting, err := NewWith(Options{CollectBetweenRequests: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := collecting.Handler.(*http.ServeMux); ok {
		t.Fatal("an engine with CollectBetweenRequests serves the bare mux, so nothing collects after a request")
	}
	rec := httptest.NewRecorder()
	collecting.Handler.ServeHTTP(rec, httptest.NewRequest("GET", "/no-such-path", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("the wrapped handler answered %d for an unknown path, want the mux's 404", rec.Code)
	}
}
