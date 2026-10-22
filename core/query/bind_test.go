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
	got := FormatBindings(One(map[string]Value{"n": Text("GND"), "?v": Number(3.3)}))
	want := []string{`?n = "GND"`, `?v = 3.3`}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A query string takes no escapes, so a pattern's backslash is written once. Doubled, it reads as a
// pattern matching a literal backslash.
func TestFormatBindingsKeepsABackslashAsWritten(t *testing.T) {
	got := FormatBindings(One(map[string]Value{"p": Text(`^U1\.`)}))
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
	if len(b["r"]) != 1 || len(b["v"]) != 1 || len(b["t"]) != 1 {
		t.Fatalf("bindings = %+v, want one value each", b)
	}
	r, v, tx := b["r"][0], b["v"][0], b["t"][0]
	if r.S != "U1" || r.Num != nil || v.Num == nil || *v.Num != 3.3 || tx.Num != nil || tx.S != "3" {
		t.Errorf("bindings = %+v, want r text U1, v number 3.3, t text 3", b)
	}
	for _, bad := range []string{"{r: {a: 1}}", "{r: [[1, 2]]}", "{r: [{a: 1}]}"} {
		if _, err := ParseQuerySet([]byte("queries:\n  - name: a\n    query: x(?r)\n    bind: " + bad + "\n")); err == nil {
			t.Errorf("bind: %s parsed, want an error asking for a value or a list of values", bad)
		}
	}
}

// A list under bind: binds its variable to each value, in order, each typed as a scalar is (agni
// issue 1121).
func TestASetQueryBindsAList(t *testing.T) {
	s, err := ParseQuerySet([]byte("queries:\n  - name: a\n    query: x(?n)\n    bind: {n: [I2C_SDA, 3, \"4\"]}\n"))
	if err != nil {
		t.Fatal(err)
	}
	got := s.Queries[0].Bind["n"]
	if len(got) != 3 || got[0].S != "I2C_SDA" || got[0].Num != nil || got[1].Num == nil || *got[1].Num != 3 || got[2].Num != nil || got[2].S != "4" {
		t.Errorf("n = %+v, want I2C_SDA, the number 3 and the text 4", got)
	}
}

// --bind given one name twice binds a list, and BindArgs spells it back the same way, so a viewer link
// carries the list and reads back as it was asked.
func TestRepeatedBindingsAreAList(t *testing.T) {
	b, err := ParseBindings([]string{"n=I2C_SDA", "v=3", "n=I2C_SCL"})
	if err != nil {
		t.Fatal(err)
	}
	if len(b["n"]) != 2 || b["n"][0].S != "I2C_SDA" || b["n"][1].S != "I2C_SCL" || len(b["v"]) != 1 {
		t.Fatalf("bindings = %+v, want n bound to I2C_SDA then I2C_SCL and v to 3", b)
	}
	args := BindArgs(b)
	if want := []string{"n=I2C_SDA", "n=I2C_SCL", "v=3"}; !reflect.DeepEqual(args, want) {
		t.Errorf("BindArgs = %q, want %q", args, want)
	}
	again, err := ParseBindings(args)
	if err != nil || !reflect.DeepEqual(again, b) {
		t.Errorf("read back as %+v (%v), want %+v", again, err, b)
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
			Name   string    `json:"name"`
			Text   *string   `json:"text"`
			Number *float64  `json:"number"`
			List   []twinVal `json:"list"`
			Want   string    `json:"want"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(b, &twin); err != nil {
		t.Fatal(err)
	}
	if len(twin.Cases) < 4 {
		t.Fatalf("fixture has %d cases", len(twin.Cases))
	}
	lists := 0
	for _, c := range twin.Cases {
		var vals []Value
		if c.List != nil {
			lists++
			for _, e := range c.List {
				vals = append(vals, e.value())
			}
		} else {
			vals = []Value{twinVal{Text: c.Text, Number: c.Number}.value()}
		}
		if got := FormatBindings(Bindings{c.Name: vals}); len(got) != 1 || got[0] != c.Want {
			t.Errorf("FormatBindings(%s) = %q, want %q", c.Name, got, c.Want)
		}
	}
	if lists == 0 {
		t.Error("fixture has no list case")
	}
}

type twinVal struct {
	Text   *string  `json:"text"`
	Number *float64 `json:"number"`
}

func (v twinVal) value() Value {
	if v.Number != nil {
		return Number(*v.Number)
	}
	return Text(*v.Text)
}
