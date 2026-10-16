package query

import (
	"errors"
	"fmt"
	"testing"

	"github.com/panyam/jaala/datalog"
)

// A query that would not parse says where, and its message is still the engine's own.
func TestASyntaxErrorSaysWhere(t *testing.T) {
	text := "component.net(?r, ?n),\n  net.pin_count(?n ?c) => ?r"
	_, err := Parse(text)
	if err == nil {
		t.Fatal("the malformed query parsed")
	}
	at, ok := ErrorPosition(err)
	if !ok {
		t.Fatalf("%v carries no position", err)
	}
	if at.Line != 2 || at.Column != 17 {
		t.Errorf("position line %d column %d, want line 2 column 17 (the ?n ?c)", at.Line, at.Column)
	}
	if _, engineErr := datalog.Parse(text); engineErr.Error() != err.Error() {
		t.Errorf("message %q, want the engine's %q unchanged", err, engineErr)
	}
	want := "  net.pin_count(?n ?c) => ?r\n                ^"
	if got := Caret(text, at); got != want {
		t.Errorf("caret\n%s\nwant\n%s", got, want)
	}
}

// An error about a query that parsed has no place in the text to point at.
func TestAnErrorThatIsNotSyntaxHasNoPosition(t *testing.T) {
	if _, ok := ErrorPosition(fmt.Errorf("query: %w", errors.New(`unknown module "compnent"`))); ok {
		t.Error("an error that is not a parse failure was given a position")
	}
}

// A set's query is parsed with the preamble in front, and its error still points into the query as
// its author wrote it.
func TestASetQueryErrorPointsIntoItsOwnText(t *testing.T) {
	s := QuerySet{
		Preamble: "on(?r) :- component.net(?r, ?_);\nalso(?r) :- on(?r)",
		Queries:  []NamedQuery{{Name: "a", Query: "on(?r) => ?r ?x"}},
	}
	_, err := s.Compile(0)
	at, ok := ErrorPosition(err)
	if !ok {
		t.Fatalf("%v carries no position", err)
	}
	if at.Line != 1 || at.Column != 11 || at.Offset != 10 {
		t.Errorf("position %+v, want line 1, column 11, offset 10 (the ?r ?x) within the query's own text", at)
	}
}

func TestCaretKeepsTabsAndRefusesAPlaceOutsideTheText(t *testing.T) {
	if got := Caret("\tx(?a ?b)", Position{Line: 1, Column: 6}); got != "\tx(?a ?b)\n\t    ^" {
		t.Errorf("caret %q", got)
	}
	for _, p := range []Position{{Line: 0, Column: 1}, {Line: 3, Column: 1}, {Line: 1, Column: 40}} {
		if got := Caret("x(?a)", p); got != "" {
			t.Errorf("Caret at %+v = %q, want none", p, got)
		}
	}
}
