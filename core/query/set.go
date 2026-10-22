package query

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// A QuerySet is a named list of queries answered over one read of a design (agni issue 729), so an
// audit workbook is versioned together, shares derived relations, and costs one read of the design
// instead of one per question.
//
// Preamble holds rules every query may use, the derived relations the set shares. It holds rules
// only; each query brings its own goal.
type QuerySet struct {
	Title    string       `yaml:"title"`
	Preamble string       `yaml:"preamble"`
	Queries  []NamedQuery `yaml:"queries"`
}

// A NamedQuery is one entry of a QuerySet. Name identifies it in the answer and must be unique in
// its set; Description is carried into the rendered report for the reader.
type NamedQuery struct {
	Name        string `yaml:"name"`
	Query       string `yaml:"query"`
	Description string `yaml:"description"`
	// Bind gives this query's goal variables values (agni issue 793), as RunQueryRequest's bindings
	// do. YAML types each value, so `3.3` binds a number and `"3"` or `GND` binds text, and a list
	// binds the variable to each of its values (agni issue 1121).
	Bind Bindings `yaml:"-"`
}

// UnmarshalYAML reads a query's `bind:` map into typed values. A value is a scalar or a list of
// scalars; a map, or a list holding anything but scalars, is refused.
//
// A key it does not know is refused with its line, as ParseQuerySet refuses one at the top. A custom
// decoder does not inherit the decoder's KnownFields, so a misspelled `descripton:` used to drop the
// description with no error.
func (q *NamedQuery) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			switch k := n.Content[i]; k.Value {
			case "name", "query", "description", "bind":
			default:
				return fmt.Errorf("line %d: field %s not found in a query (want name, query, description, bind)", k.Line, k.Value)
			}
		}
	}
	type plain NamedQuery
	var raw struct {
		plain `yaml:",inline"`
		Bind  map[string]yaml.Node `yaml:"bind"`
	}
	if err := n.Decode(&raw); err != nil {
		return err
	}
	*q = NamedQuery(raw.plain)
	for name, v := range raw.Bind {
		items := []*yaml.Node{&v}
		if v.Kind == yaml.SequenceNode {
			items = v.Content
		}
		vals := make([]Value, 0, len(items))
		for _, it := range items {
			if it.Kind != yaml.ScalarNode {
				return fmt.Errorf("query %q: bind %s: want a value or a list of values", q.Name, name)
			}
			var f float64
			if (it.Tag == "!!int" || it.Tag == "!!float") && it.Decode(&f) == nil {
				vals = append(vals, Number(f))
			} else {
				vals = append(vals, Text(it.Value))
			}
		}
		if q.Bind == nil {
			q.Bind = Bindings{}
		}
		q.Bind[name] = vals
	}
	return nil
}

// ParseQuerySet reads a query set from YAML and validates it. An unknown key is an error, because a
// misspelled `preamble` would otherwise drop the shared rules silently and every query using them
// would fail for a reason nowhere in its own text.
func ParseQuerySet(b []byte) (QuerySet, error) {
	var s QuerySet
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		if errors.Is(err, io.EOF) {
			return QuerySet{}, fmt.Errorf("query set: empty file")
		}
		return QuerySet{}, fmt.Errorf("query set: %w", err)
	}
	return s, s.Validate()
}

// preambleProbe is the goal Validate appends to a preamble so it parses as a query. A preamble that
// already holds a goal then parses as a query with two, which the parser refuses.
const preambleProbe = "probe_(?_)"

// Validate reports what makes a set unusable as a whole: no queries, a query with no name or no
// text, a repeated name, or a preamble that is not rules alone. It does NOT compile the queries, so
// one bad query is reported against its name when the set runs while the rest still answer.
func (s QuerySet) Validate() error {
	if len(s.Queries) == 0 {
		return fmt.Errorf("query set: no queries")
	}
	seen := map[string]bool{}
	for i, q := range s.Queries {
		name := strings.TrimSpace(q.Name)
		if name == "" {
			return fmt.Errorf("query set: query %d has no name", i+1)
		}
		if strings.TrimSpace(q.Query) == "" {
			return fmt.Errorf("query set: %q has no query", name)
		}
		if seen[name] {
			return fmt.Errorf("query set: %q is named twice; each query's name identifies its answer", name)
		}
		seen[name] = true
	}
	if strings.TrimSpace(s.Preamble) != "" {
		if _, err := Parse(s.Preamble + ";\n" + preambleProbe); err != nil {
			return fmt.Errorf("query set: preamble must hold rules only (each clause with `:-`): %w", err)
		}
	}
	return nil
}

// Compile returns the i'th query with the preamble's rules in front of it, ready to evaluate. A
// syntax error's position is within the i'th query's own text.
func (s QuerySet) Compile(i int) (Query, error) {
	prefix := s.prefix()
	q, err := Parse(prefix + s.Queries[i].Query)
	// A syntax error points into the query as its author wrote it, not into the text with the
	// preamble in front.
	return q, shifted(err, prefix, s.Queries[i].Query)
}

// Text is the i'th query as Compile parses it, with the preamble's rules in front, so a caller that
// sends the query somewhere a set does not travel (a viewer link, agni issue 634) sends what the set
// would have asked.
func (s QuerySet) Text(i int) string { return s.prefix() + s.Queries[i].Query }

// prefix is the preamble as the text in front of each query, "" for a set with none.
func (s QuerySet) prefix() string {
	if strings.TrimSpace(s.Preamble) != "" {
		return s.Preamble + ";\n"
	}
	return ""
}
