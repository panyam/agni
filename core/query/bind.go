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
	m := make(map[datalog.Var][]ns.Value, len(vals))
	for k, v := range vals {
		m[datalog.Var(strings.TrimPrefix(k, "?"))] = []ns.Value{v}
	}
	return datalog.Bind(m)
}

// BindEach gives one goal variable several values, so the goal answers for each of them, as a rule
// asked about a few nets binds its subject variable to their names (agni issue 897).
func BindEach(name string, vals []Value) datalog.Option {
	return datalog.Bind(map[datalog.Var][]ns.Value{datalog.Var(strings.TrimPrefix(name, "?")): vals})
}

// ParseBinding reads one `name=value` pair, the form `agni query --bind` takes. The value is read
// as the query language reads a constant: a quoted value is text, one that parses as a number is a
// number, and any other bare word is text, so `--bind n=GND` needs no quotes. In a position the
// schema types, the engine then reads the value as that type either way (panyam/jaala#65), so the
// quoting matters only for a variable nothing types, where `--bind 'n="3"'` binds the text.
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
//
// Text goes between quotes exactly as it is, since a query string takes no escapes: `"U1\."` is the
// four characters U, 1, a backslash and a dot (jaala v0.1.62). Go's quoting doubled that backslash,
// which spells another pattern. A value holding a quote with no backslash before it, or ending in a
// lone backslash, has no spelling in the language; it is shown the same way, and still binds, since
// a binding never travels as text.
func FormatBindings(vals map[string]Value) []string {
	out := make([]string, 0, len(vals))
	for k, v := range vals {
		val := `"` + v.S + `"`
		if v.Num != nil {
			val = v.S
		}
		out = append(out, "?"+strings.TrimPrefix(k, "?")+" = "+val)
	}
	sort.Strings(out)
	return out
}

// BindArgs spells bound values as `--bind` takes them, `n=GND`, `v=3.3`, and `t="3"` for text that
// would otherwise read as a number, sorted by name, so a value carried somewhere as text (a viewer
// link, agni issue 634) reads back as the value ParseBinding was given.
func BindArgs(vals map[string]Value) []string {
	out := make([]string, 0, len(vals))
	for k, v := range vals {
		name := strings.TrimPrefix(k, "?")
		switch {
		case v.Num != nil:
			out = append(out, name+"="+v.S)
		default:
			if _, err := strconv.ParseFloat(v.S, 64); err == nil {
				out = append(out, name+`="`+v.S+`"`)
			} else {
				out = append(out, name+"="+v.S)
			}
		}
	}
	sort.Strings(out)
	return out
}
