package query

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/panyam/jaala/datalog"
)

// Position is where in a query's text a syntax error points. Line and Column count from 1, and
// Column counts characters rather than bytes, as an editor does.
type Position struct {
	Offset int // bytes from the start of the text
	Line   int
	Column int
}

// SyntaxError is a query that would not parse, with where in its text the parser stopped. Its
// message is the engine's, unchanged, so a caller printing the error prints what it always has; a
// caller that can show the text as well asks ErrorPosition for where to point.
type SyntaxError struct {
	At  Position
	err error
}

func (e *SyntaxError) Error() string { return e.err.Error() }

// Unwrap returns the engine's own parse error.
func (e *SyntaxError) Unwrap() error { return e.err }

// syntaxError wraps a parse failure with its position, or returns err as it is when the engine gave
// none.
func syntaxError(err error) error {
	var pe *datalog.ParseError
	if !errors.As(err, &pe) {
		return err
	}
	return &SyntaxError{At: Position{Offset: pe.Offset, Line: pe.Line, Column: pe.Column}, err: err}
}

// ErrorPosition reports where in its query's text err points, when err is a syntax error. An error
// about a query that parsed, such as a relation the vocabulary lacks, has no position.
func ErrorPosition(err error) (Position, bool) {
	var se *SyntaxError
	if errors.As(err, &se) {
		return se.At, true
	}
	return Position{}, false
}

// Caret renders the line of text p points into with a caret under its column, for a terminal or a
// fixed-width box. A tab before the column is kept as a tab, so the caret lines up however wide the
// reader's tabs are. It returns "" when p is not inside text.
func Caret(text string, p Position) string {
	lines := strings.Split(text, "\n")
	if p.Line < 1 || p.Line > len(lines) || p.Column < 1 {
		return ""
	}
	line := strings.TrimRight(lines[p.Line-1], "\r")
	if p.Column-1 > utf8.RuneCountInString(line) {
		return ""
	}
	var pad strings.Builder
	n := 0
	for _, r := range line {
		if n == p.Column-1 {
			break
		}
		if r == '\t' {
			pad.WriteRune('\t')
		} else {
			pad.WriteByte(' ')
		}
		n++
	}
	return line + "\n" + pad.String() + "^"
}

// shifted moves err's position back by the n bytes of text a caller put in front of the query and
// the lines that text spanned, so it points into the query as its author wrote it. A position
// inside the prefix is dropped, since it is not in that query's text.
func shifted(err error, prefix string) error {
	se, ok := err.(*SyntaxError)
	if !ok || prefix == "" {
		return err
	}
	if se.At.Offset < len(prefix) {
		return se.err
	}
	at := se.At
	at.Offset -= len(prefix)
	at.Line -= strings.Count(prefix, "\n")
	return &SyntaxError{At: at, err: se.err}
}
