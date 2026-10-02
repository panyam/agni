package query

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/panyam/jaala/datalog"
	"github.com/panyam/jaala/ns"
)

// Text is a string Value to bind to a goal variable (agni issue 793), as a quoted constant in a
// query is.
func Text(s string) Value { return ns.S(s) }

// Number is a numeric value, as an unquoted number in a query is.
func Number(f float64) Value { return ns.N(f) }

// Bind gives goal variables values from the caller, so a query asked about one net or part is the
// same text every time and carries the value as data. A bound variable answers exactly as the same
// constant written into the goal. Names may carry their leading `?` or not. The engine refuses a
// name the goal does not use when the query is evaluated, so a misspelled binding fails rather than
// leaving the variable free.
func Bind(vals map[string]Value) datalog.Option {
	m := make(map[datalog.Var]ns.Value, len(vals))
	for k, v := range vals {
		m[datalog.Var(strings.TrimPrefix(k, "?"))] = v
	}
	return datalog.Bind(m)
}

// ParseBinding reads one `name=value` pair, the form `agni query --bind` takes. The value is read
// as the query language reads a constant: a quoted value is text, one that parses as a number is a
// number, and any other bare word is text, so `--bind n=GND` needs no quotes and `--bind 'n="3"'`
// binds the text "3" rather than the number.
func ParseBinding(s string) (string, Value, error) {
	name, val, ok := strings.Cut(s, "=")
	name = strings.TrimPrefix(strings.TrimSpace(name), "?")
	if !ok || name == "" {
		return "", Value{}, fmt.Errorf("binding %q: want name=value", s)
	}
	if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
		return name, Text(val[1 : len(val)-1]), nil
	}
	if f, err := strconv.ParseFloat(val, 64); err == nil {
		// Canonical, as a number bound over the wire is, so `3.0` binds and echoes as 3 on every
		// transport.
		return name, Number(f), nil
	}
	return name, Text(val), nil
}

// FormatBindings renders bound values as a query would spell them, `?n = "GND"` or `?v = 3.3`,
// sorted by name, so a rendered view states the values beside its query.
func FormatBindings(vals map[string]Value) []string {
	out := make([]string, 0, len(vals))
	for k, v := range vals {
		val := strconv.Quote(v.S)
		if v.Num != nil {
			val = v.S
		}
		out = append(out, "?"+strings.TrimPrefix(k, "?")+" = "+val)
	}
	sort.Strings(out)
	return out
}
