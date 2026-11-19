package main

import (
	"runtime"
	"testing"
)

// TestRooted holds a path from a root apart from a path from a folder, on every platform, and on
// Windows also a drive-relative path and one from the current drive's root (agni issue 1595).
func TestRooted(t *testing.T) {
	cases := map[string]bool{"": false, "a": false, "a/b": false, "./a": false, "../a": false, "/srv": true}
	if runtime.GOOS == "windows" {
		for p, want := range map[string]bool{`\srv`: true, `C:\srv`: true, `C:srv`: true, `\\host\share\x`: true, `a\b`: false} {
			cases[p] = want
		}
	}
	for p, want := range cases {
		if got := rooted(p); got != want {
			t.Errorf("Rooted(%q) = %v, want %v", p, got, want)
		}
	}
}
