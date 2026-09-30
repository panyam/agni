package query

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// A QuerySet is a named list of queries answered over one read of a design (agni issue 729). An
// audit is usually a workbook rather than a question: every pin's net, every part's MPN, test points
// per net, passives grouped by MPN. Each is a query, and a set lets them be versioned together,
// share derived relations, and cost one read of the design instead of one per question.
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
}

// ParseQuerySet reads a query set from YAML and validates it. An unknown key is an error rather than
// ignored, because a misspelled `preamble` would otherwise drop the shared rules silently and every
// query using them would fail for a reason nowhere in its own text.
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
// text, a repeated name, or a preamble that is not rules alone. It does NOT compile the queries: one
// query failing is that query's problem and is reported against its name when the set runs, while
// the rest still answer.
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

// Compile returns the i'th query with the preamble's rules in front of it, ready to evaluate.
func (s QuerySet) Compile(i int) (Query, error) {
	text := s.Queries[i].Query
	if strings.TrimSpace(s.Preamble) != "" {
		text = s.Preamble + ";\n" + text
	}
	return Parse(text)
}
