package main

import "testing"

// TestResolveSymbolPaths covers the case the container depends on. The image sets
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

	// Ambient configuration must never widen an explicit request. An operator who named their
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

	// An operator's hand-edited compose file often has a trailing colon or a stray space. An
	// empty entry would become a search of the process working directory.
	t.Run("skips empty and whitespace entries", func(t *testing.T) {
		symbolPaths = nil
		got := resolveSymbolPaths(env("/a::  : /b :"))
		if len(got) != 2 || got[0] != "/a" || got[1] != "/b" {
			t.Fatalf("got %v, want [/a /b]", got)
		}
	})
}

// TestResolveModelPaths covers --model-path's fallbacks (agni 1652). The flag wins outright, then
// AGNI_MODEL_PATH, then the newest of KiCad's own library variables that is set.
func TestResolveModelPaths(t *testing.T) {
	env := func(kv map[string]string) func(string) string {
		return func(k string) string { return kv[k] }
	}
	defer func() { modelPaths = nil }()
	for _, c := range []struct {
		name string
		flag []string
		env  map[string]string
		want []string
	}{
		{"flag wins", []string{"/flag"}, map[string]string{envModelPath: "/a", "KICAD9_3DMODEL_DIR": "/k9"}, []string{"/flag"}},
		{"agni's variable next", nil, map[string]string{envModelPath: "/a:/b", "KICAD9_3DMODEL_DIR": "/k9"}, []string{"/a", "/b"}},
		{"newest KiCad variable", nil, map[string]string{"KICAD8_3DMODEL_DIR": "/k8", "KICAD9_3DMODEL_DIR": "/k9"}, []string{"/k9"}},
		{"nothing set", nil, nil, nil},
	} {
		modelPaths = c.flag
		got := resolveModelPaths(env(c.env))
		if len(got) != len(c.want) || (len(got) > 0 && got[len(got)-1] != c.want[len(c.want)-1]) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// TestResolveModelVars covers --model-var (agni 1687). The flag wins over AGNI_MODEL_VARS, and a pair
// with no folder is an error rather than a variable that never resolves.
func TestResolveModelVars(t *testing.T) {
	env := func(v string) func(string) string {
		return func(k string) string {
			if k == envModelVars {
				return v
			}
			return ""
		}
	}
	defer func() { modelVars = nil }()
	modelVars = nil
	got, err := resolveModelVars(env("ANT3DMDL=/models/ant:OTHER=/x"))
	if err != nil || len(got) != 2 || got["ANT3DMDL"] != "/models/ant" || got["OTHER"] != "/x" {
		t.Errorf("from the environment: %v, %v", got, err)
	}
	modelVars = []string{"ANT3DMDL=/flag"}
	got, err = resolveModelVars(env("ANT3DMDL=/env"))
	if err != nil || len(got) != 1 || got["ANT3DMDL"] != "/flag" {
		t.Errorf("flag over environment: %v, %v", got, err)
	}
	for _, bad := range []string{"ANT3DMDL", "=/x", "ANT3DMDL="} {
		modelVars = []string{bad}
		if _, err := resolveModelVars(env("")); err == nil {
			t.Errorf("--model-var %q: no error", bad)
		}
	}
	modelVars = nil
	if got, err := resolveModelVars(env("")); err != nil || got != nil {
		t.Errorf("nothing set: %v, %v", got, err)
	}
}
