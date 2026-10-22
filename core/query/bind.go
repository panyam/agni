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

// Bindings gives goal variables values from the caller, keyed by the variable's name with or
// without its `?`. A variable holding one value answers exactly as that value written into the goal as
// a constant does. One holding several ranges over them, so the goal answers for each, as if it were
// joined with a relation holding exactly those values (agni issue 1121), and one holding none matches
// nothing.
type Bindings map[string][]Value

// One is a Bindings giving each variable a single value.
func One(vals map[string]Value) Bindings {
	if len(vals) == 0 {
		return nil
	}
	out := make(Bindings, len(vals))
	for k, v := range vals {
		out[k] = []Value{v}
	}
	return out
}

// Bind gives goal variables values from the caller, so a query asked about one net or part is the
// same text every time and carries the value as data (agni issue 793). The engine refuses a name the
// goal does not use when the query is evaluated, so a misspelled binding fails rather than leaving
// the variable free.
func Bind(b Bindings) datalog.Option {
	m := make(map[datalog.Var][]ns.Value, len(b))
	for k, vals := range b {
		m[datalog.Var(strings.TrimPrefix(k, "?"))] = vals
	}
	return datalog.Bind(m)
}

// BindEach gives one goal variable several values, so the goal answers for each of them, as a rule
// asked about a few nets binds its subject variable to their names (agni issue 897).
func BindEach(name string, vals []Value) datalog.Option {
	return Bind(Bindings{name: vals})
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

// FormatBindings renders bound values as a query would spell them, `?n = "GND"` or `?v = 3.3`, and
// a variable bound to several values as the list it ranges over, `?n in ["I2C_SCL", "I2C_SDA"]`,
// sorted by name, so a rendered view states the values beside its query.
//
// Text goes between quotes exactly as it is, since a query string takes no escapes: `"U1\."` is the
// four characters U, 1, a backslash and a dot (jaala v0.1.62). Go's quoting doubled that backslash,
// which spells another pattern. A value holding a quote with no backslash before it, or ending in a
// lone backslash, has no spelling in the language; it is shown the same way, and still binds, since
// a binding never travels as text.
func FormatBindings(b Bindings) []string {
	out := make([]string, 0, len(b))
	for k, vals := range b {
		name := "?" + strings.TrimPrefix(k, "?")
		if len(vals) == 1 {
			out = append(out, name+" = "+spell(vals[0]))
			continue
		}
		spelled := make([]string, len(vals))
		for i, v := range vals {
			spelled[i] = spell(v)
		}
		out = append(out, name+" in ["+strings.Join(spelled, ", ")+"]")
	}
	sort.Strings(out)
	return out
}

func spell(v Value) string {
	if v.Num != nil {
		return v.S
	}
	return `"` + v.S + `"`
}

// BindArgs spells bound values as `--bind` takes them, `n=GND`, `v=3.3`, and `t="3"` for text that
// would otherwise read as a number, sorted by name, so a value carried somewhere as text (a viewer
// link, agni issue 634) reads back as the value ParseBinding was given. A variable bound to several
// values is spelled once per value, in its order, which is how `--bind` and a link give a list.
func BindArgs(b Bindings) []string {
	names := make([]string, 0, len(b))
	for k := range b {
		names = append(names, k)
	}
	sort.Strings(names)
	var out []string
	for _, k := range names {
		name := strings.TrimPrefix(k, "?")
		for _, v := range b[k] {
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
	}
	return out
}

// ParseBindings reads `name=value` pairs, as ParseBinding reads one, into the values a query is asked
// with. A name given more than once binds the variable to each of its values, in order.
func ParseBindings(args []string) (Bindings, error) {
	if len(args) == 0 {
		return nil, nil
	}
	out := Bindings{}
	for _, a := range args {
		name, v, err := ParseBinding(a)
		if err != nil {
			return nil, err
		}
		out[name] = append(out[name], v)
	}
	return out, nil
}
