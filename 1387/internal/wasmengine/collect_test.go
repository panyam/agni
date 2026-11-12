package wasmengine

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

// fakeHeap is a heap a test grows by hand. A collection shrinks it to what the test says survives.
type fakeHeap struct {
	bytes, survives uint64
	gcs             int
}

func (f *fakeHeap) collector() *collector {
	return &collector{heap: func() uint64 { return f.bytes }, gc: func() { f.gcs++; f.bytes = f.survives }, later: func(g func()) { g() }}
}

// With automatic collection off, a request collects after it ends only once the heap has doubled
// since the last collection (agni issue 1010). GOGC=100 allows the same growth.
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
	// The bar is now 120, the heap that survived that collection, so 240 is under it and 241 past it.
	f.bytes = 240
	ask()
	f.bytes = 241
	ask()
	if f.gcs != 3 || served != 4 {
		t.Fatalf("got %d collections over %d requests, want 3 over 4", f.gcs, served)
	}
}

// A read collects between its files once the heap is a fixed allowance past what last survived. The
// live heap's size does not matter, since a read's garbage dwarfs what it keeps.
func TestOpeningAFileCollectsOnceTheHeapIsPastItsHeadroom(t *testing.T) {
	f := &fakeHeap{bytes: 10 << 20, survives: 10 << 20}
	c := f.collector()
	c.collect()
	files := c.files(fstest.MapFS{"a.kicad_sch": {Data: []byte("x")}})
	open := func() {
		if _, err := fs.ReadFile(files, "a.kicad_sch"); err != nil {
			t.Fatal(err)
		}
	}
	f.bytes = 10<<20 + fileHeadroom // past double the live heap, but not past the allowance
	open()
	if f.gcs != 1 {
		t.Fatalf("a heap within its headroom collected on open (%d collections)", f.gcs)
	}
	f.bytes++
	open()
	if f.gcs != 2 {
		t.Fatalf("a heap past its headroom ran %d collections on open, want 2", f.gcs)
	}
}

// The option turns the per-request collection on. An engine without it serves the mux itself, so a
// native caller spends nothing on collection. Every engine guards against a held-back file (agni
// issue 992), so the test looks for the mux inside that guard.
func TestOnlyAnEngineAskedToCollectWrapsItsHandler(t *testing.T) {
	plain, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := plain.Handler.(*guarded).next.(*http.ServeMux); !ok {
		t.Fatalf("an engine without CollectBetweenRequests serves %T, want the *http.ServeMux", plain.Handler.(*guarded).next)
	}
	collecting, err := NewWith(Options{CollectBetweenRequests: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := collecting.Handler.(*guarded).next.(*http.ServeMux); ok {
		t.Fatal("an engine with CollectBetweenRequests serves the bare mux, so nothing collects after a request")
	}
	rec := httptest.NewRecorder()
	collecting.Handler.ServeHTTP(rec, httptest.NewRequest("GET", "/no-such-path", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("the wrapped handler answered %d for an unknown path, want the mux's 404", rec.Code)
	}
}
