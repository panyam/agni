package query

import (
	"reflect"
	"testing"
)

// The builder produces the SAME AST Parse does for the equivalent text — so switching a generator
// from string-building to the builder is behavior-preserving by construction.
func TestBuildMatchesParse(t *testing.T) {
	built := Build(
		[]Rule{Def(Rel("hit", V("n")),
			Pos(Rel("component-on-net", V("r"), V("n"))),
			Pos(Rel("suffix", V("n"), Str("_CS"))),
			Neg(Rel("rail", V("n"))),
			Cmp(V("n"), "!=", Str("GND")))},
		[]Literal{Pos(Rel("hit", V("n")))},
		V("n"))
	parsed := MustParse(`hit(?n) :- component-on-net(?r, ?n), suffix(?n, "_CS"), not rail(?n), ?n != "GND"; hit(?n) => ?n`)
	if !reflect.DeepEqual(built, parsed) {
		t.Fatalf("built AST != parsed AST:\n built  = %#v\n parsed = %#v", built, parsed)
	}
}

// Reads returns only the EDB (fact-base) relations, sorted — not IDB heads (hit) or built-ins (suffix).
func TestReads(t *testing.T) {
	q := MustParse(`hit(?n) :- component-on-net(?r, ?n), net.pin_count(?n, ?c), suffix(?n, "_CS"); hit(?n) => ?n`)
	got := Reads(q)
	want := []string{"component-on-net", "net.pin_count"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Reads = %v, want %v", got, want)
	}
}
