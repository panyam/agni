package main

import "testing"

// TestResolveSymbolPaths covers the container's load-bearing case: the image sets
// AGNI_SYMBOL_PATH so the symbol libraries reach every subcommand, because `docker run <image>
// check ...` replaces CMD and would otherwise drop them. A symbol-short read does not error, it
// just reports fewer findings, so losing this silently is the expensive failure.
func TestResolveSymbolPaths(t *testing.T) {
	env := func(v string) func(string) string {
		return func(k string) string {
			if k == envSymbolPath {
				return v
			}
			return ""
		}
	}

	t.Run("env supplies the paths when the flag is absent", func(t *testing.T) {
		symbolPaths = nil
		got := resolveSymbolPaths(env("/a:/b:/c"))
		if len(got) != 3 || got[0] != "/a" || got[2] != "/c" {
			t.Fatalf("got %v, want the three env paths in order", got)
		}
	})

	// Ambient configuration must never widen an explicit request: an operator who named their
	// own library is asserting which symbols resolve, and quietly appending the image's would
	// change what the design reads as.
	t.Run("flag wins outright over env", func(t *testing.T) {
		symbolPaths = []string{"/flag"}
		defer func() { symbolPaths = nil }()
		got := resolveSymbolPaths(env("/a:/b"))
		if len(got) != 1 || got[0] != "/flag" {
			t.Fatalf("got %v, want only the flag value", got)
		}
	})

	t.Run("unset env yields no paths", func(t *testing.T) {
		symbolPaths = nil
		if got := resolveSymbolPaths(env("")); len(got) != 0 {
			t.Fatalf("got %v, want none", got)
		}
	})

	// A trailing colon or a stray space is what an operator's hand-edited compose file looks
	// like; an empty entry would become a search of the process working directory.
	t.Run("skips empty and whitespace entries", func(t *testing.T) {
		symbolPaths = nil
		got := resolveSymbolPaths(env("/a::  : /b :"))
		if len(got) != 2 || got[0] != "/a" || got[1] != "/b" {
			t.Fatalf("got %v, want [/a /b]", got)
		}
	})
}
