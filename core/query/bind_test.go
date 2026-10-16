package query

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// ParseBinding reads a --bind value as the query language reads a constant (agni issue 793).
func TestParseBindingReadsAConstant(t *testing.T) {
	for _, c := range []struct {
		in, name, s string
		num         bool
	}{
		{"n=GND", "n", "GND", false},
		{"?n=GND", "n", "GND", false},
		{"v=3.3", "v", "3.3", true},
		{`n="3"`, "n", "3", false},
		{"p=(?i)^U", "p", "(?i)^U", false},
		{"n=", "n", "", false},
	} {
		name, v, err := ParseBinding(c.in)
		if err != nil {
			t.Errorf("%s: %v", c.in, err)
			continue
		}
		if name != c.name || v.S != c.s || (v.Num != nil) != c.num {
			t.Errorf("%s = %s %q (number %v), want %s %q (number %v)", c.in, name, v.S, v.Num != nil, c.name, c.s, c.num)
		}
	}
	for _, bad := range []string{"GND", "=GND"} {
		if _, _, err := ParseBinding(bad); err == nil {
			t.Errorf("%q parsed, want an error asking for name=value", bad)
		}
	}
}

func TestFormatBindingsSpellsEachAsTheQueryWould(t *testing.T) {
	got := FormatBindings(map[string]Value{"n": Text("GND"), "?v": Number(3.3)})
	want := []string{`?n = "GND"`, `?v = 3.3`}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A query string takes no escapes, so a pattern's backslash is written once. Doubled, it reads as a
// pattern matching a literal backslash.
func TestFormatBindingsKeepsABackslashAsWritten(t *testing.T) {
	got := FormatBindings(map[string]Value{"p": Text(`^U1\.`)})
	want := []string{`?p = "^U1\."`}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestASetQueryReadsItsBindings(t *testing.T) {
	s, err := ParseQuerySet([]byte("queries:\n  - name: a\n    query: component.net(?r, ?n) => ?n\n    bind: {r: U1, v: 3.3, t: \"3\"}\n"))
	if err != nil {
		t.Fatal(err)
	}
	b := s.Queries[0].Bind
	if b["r"].S != "U1" || b["r"].Num != nil || b["v"].Num == nil || *b["v"].Num != 3.3 || b["t"].Num != nil || b["t"].S != "3" {
		t.Errorf("bindings = %+v, want r text U1, v number 3.3, t text 3", b)
	}
	if _, err := ParseQuerySet([]byte("queries:\n  - name: a\n    query: x(?r)\n    bind: {r: [1, 2]}\n")); err == nil {
		t.Error("a list bound to one variable parsed, want an error")
	}
}

// TestBindingTwin holds FormatBindings to testdata/binding_twin.json, which the viewer's chips are
// tested against too (agni issue 1045).
func TestBindingTwin(t *testing.T) {
	b, err := os.ReadFile("testdata/binding_twin.json")
	if err != nil {
		t.Fatal(err)
	}
	var twin struct {
		Cases []struct {
			Name   string   `json:"name"`
			Text   *string  `json:"text"`
			Number *float64 `json:"number"`
			Want   string   `json:"want"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(b, &twin); err != nil {
		t.Fatal(err)
	}
	if len(twin.Cases) < 4 {
		t.Fatalf("fixture has %d cases", len(twin.Cases))
	}
	for _, c := range twin.Cases {
		v := Value{}
		if c.Number != nil {
			v = Number(*c.Number)
		} else {
			v = Text(*c.Text)
		}
		if got := FormatBindings(map[string]Value{c.Name: v}); len(got) != 1 || got[0] != c.Want {
			t.Errorf("FormatBindings(%s) = %q, want %q", c.Name, got, c.Want)
		}
	}
}
