// Package expect loads a test design's expected findings from a sidecar file, so one artifact
// drives both the conformance harness (this repo) and, later, the web viewer's expectations panel
// (roadmap WS9-018). It is the committed-side reader of the WS6-004 `expect` shape.
//
// A sidecar is `<design>.expect.yaml`:
//
//	fires:                    # rules that MUST fire, with the exact subjects
//	  duplicate-ref-des: [U1]
//	  single-pin-net: [STUB]
//	  decoupling-present:     # long form: subjects plus the fixture's narration (WS6-008)
//	    subjects: [VCC1]
//	    why: "VCC1 has no cap; VCC2 is the control with C1"
//	pending:                  # rules staged before they exist: rendered, never asserted
//	  net-naming-convention: [BAD_NAME]
//
// `fires` is a hard expectation in both directions, so a listed rule must fire those subjects and no
// others. `pending` stages an expectation ahead of its rule. A consumer may show it, but the harness
// neither requires nor forbids it, so it lands without breaking CI. Move the entry to `fires` once the
// rule is implemented.
package expect

import (
	"fmt"
	"os"

	yaml "gopkg.in/yaml.v3"
)

// Entry is one expected rule, the exact subjects it must fire on plus an optional Why. Why is the
// fixture's own narration of what is wrong and what the control case is, shown with a red
// expectation in a harness failure or a viewer panel.
type Entry struct {
	Subjects []string
	Why      string
}

// UnmarshalYAML accepts both sidecar entry forms: the short bare subject list
// (`rule: [A, B]`) and the long mapping (`rule: {subjects: [A], why: "..."}`).
func (e *Entry) UnmarshalYAML(n *yaml.Node) error {
	switch n.Kind {
	case yaml.SequenceNode:
		return n.Decode(&e.Subjects)
	case yaml.MappingNode:
		var long struct {
			Subjects []string `yaml:"subjects"`
			Why      string   `yaml:"why"`
		}
		if err := n.Decode(&long); err != nil {
			return err
		}
		e.Subjects, e.Why = long.Subjects, long.Why
		return nil
	}
	return fmt.Errorf("expectation entry must be a subject list or a {subjects, why} mapping (line %d)", n.Line)
}

// Expectations is a design's expected findings, keyed by rule name.
type Expectations struct {
	Fires   map[string]Entry `yaml:"fires"`
	Pending map[string]Entry `yaml:"pending"`
}

// Load reads and parses a `<design>.expect.yaml` sidecar. A missing file is an error, since the
// caller decides which designs should have one. An empty file is a valid "no findings" expectation.
func Load(path string) (*Expectations, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var e Expectations
	if err := yaml.Unmarshal(b, &e); err != nil {
		return nil, err
	}
	if e.Fires == nil {
		e.Fires = map[string]Entry{}
	}
	return &e, nil
}
