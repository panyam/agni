package param

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

func fixtureBytes(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestLoadSet(t *testing.T) {
	fsys := fstest.MapFS{
		"lm1117.textproto":        {Data: fixtureBytes(t, "lm1117.textproto")},
		"vendor/bss138.textproto": {Data: fixtureBytes(t, "bss138.textproto")},
		"README.md":               {Data: []byte("not a spec")},
	}
	set, err := LoadSet(fsys)
	if err != nil {
		t.Fatalf("LoadSet: %v", err)
	}
	if len(set) != 2 {
		t.Fatalf("want 2 specs (nested dirs walked, non-textproto ignored), got %d", len(set))
	}
	if set.Lookup("LM1117") == nil || set.Lookup("BSS138") == nil {
		t.Errorf("Lookup by exact mpn failed: %v", set)
	}
	if set.Lookup("lm1117") == nil {
		t.Errorf("Lookup must be case-insensitive on mpn (vendor casing varies)")
	}
	if set.Lookup("LM7805") != nil {
		t.Errorf("unseeded mpn must return nil")
	}
}

func TestLoadSetRejectsInvalidSpec(t *testing.T) {
	bad := strings.Replace(string(fixtureBytes(t, "lm1117.textproto")), `mpn: "LM1117"`, `mpn: ""`, 1)
	fsys := fstest.MapFS{"bad.textproto": {Data: []byte(bad)}}
	if _, err := LoadSet(fsys); err == nil || !strings.Contains(err.Error(), "mpn") {
		t.Fatalf("LoadSet must surface Validate failures with the file named, got %v", err)
	}
}

func TestLoadSetRejectsDuplicateMPN(t *testing.T) {
	fsys := fstest.MapFS{
		"a.textproto": {Data: fixtureBytes(t, "lm1117.textproto")},
		"b.textproto": {Data: fixtureBytes(t, "lm1117.textproto")},
	}
	if _, err := LoadSet(fsys); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("LoadSet must reject a corpus with two specs for one mpn, got %v", err)
	}
}
