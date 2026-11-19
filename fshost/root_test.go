package fshost

import (
	"testing"
	"testing/fstest"
)

type withSums struct{ fstest.MapFS }

func (w withSums) ContentSum(name string) (string, int64, bool) {
	if name != "a.txt" {
		return "", 0, false
	}
	return "sha256:aa", 1, true
}

// Root answers ContentSum from the mount that holds a file, so the wasm engine fingerprints a stored
// design's files by their manifest's sums through the composed tree (agni issues 1515, 1761).
func TestRootAsksTheHoldingMountForAContentSum(t *testing.T) {
	root := Root([]Mount{{Name: "m", FS: withSums{fstest.MapFS{"a.txt": {Data: []byte("a")}}}}})
	s, ok := root.(interface {
		ContentSum(string) (string, int64, bool)
	})
	if !ok {
		t.Fatal("Root's fs.FS has no ContentSum")
	}
	if sum, size, ok := s.ContentSum("m/a.txt"); !ok || sum != "sha256:aa" || size != 1 {
		t.Errorf("ContentSum(m/a.txt) = %q, %d, %v; want the mount's sum", sum, size, ok)
	}
	if _, _, ok := s.ContentSum("m/b.txt"); ok {
		t.Errorf("ContentSum of a file the mount doesn't know = ok; want false")
	}
}
