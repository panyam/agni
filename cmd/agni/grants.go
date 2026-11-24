package main

import (
	"fmt"
	"strings"

	"github.com/panyam/goapplib/storage/connections"
	"gopkg.in/yaml.v3"
)

// parseGrant reads one --grant value, a source and its viewer patterns as
// github://acme/board=local:*,oidc:* (agni issue 1889). It refuses a grant that fails
// SourceGrant.Validate, since goapplib lets a bad pattern match nobody and say nothing.
func parseGrant(v string) (connections.SourceGrant, error) {
	source, viewers, ok := strings.Cut(v, "=")
	if !ok {
		return connections.SourceGrant{}, fmt.Errorf("--grant %q: want source=pattern,pattern, such as github://acme/board=local:*", v)
	}
	g := connections.SourceGrant{Source: source}
	for p := range strings.SplitSeq(viewers, ",") {
		g.Viewers = append(g.Viewers, strings.TrimSpace(p))
	}
	if err := g.Validate(); err != nil {
		return connections.SourceGrant{}, fmt.Errorf("--grant %q: %w", v, err)
	}
	return g, nil
}

// parseGrants reads every --grant value.
func parseGrants(vs []string) ([]connections.SourceGrant, error) {
	var out []connections.SourceGrant
	for _, v := range vs {
		g, err := parseGrant(v)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

// accessEntry is one item of a serve.yaml's access: list.
type accessEntry struct {
	Source string   `yaml:"source"`
	Allow  []string `yaml:"allow"`
}

// accessGrants reads a serve.yaml's access: list and returns each entry as a --grant value. It
// refuses an unknown key, an entry that fails SourceGrant.Validate, and a pattern holding a comma,
// which --grant would split. Each error names the entry by its place in the list.
func accessGrants(n *yaml.Node) ([]string, error) {
	if n.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("line %d: want a list of {source, allow}", n.Line)
	}
	var out []string
	for i, it := range n.Content {
		var e accessEntry
		b, err := yaml.Marshal(it)
		if err == nil {
			dec := yaml.NewDecoder(strings.NewReader(string(b)))
			dec.KnownFields(true)
			err = dec.Decode(&e)
		}
		if err != nil {
			return nil, fmt.Errorf("access[%d] at line %d: want {source, allow}: %w", i, it.Line, err)
		}
		g := connections.SourceGrant{Source: e.Source, Viewers: e.Allow}
		if err := g.Validate(); err != nil {
			return nil, fmt.Errorf("access[%d] at line %d: %w", i, it.Line, err)
		}
		if strings.Contains(e.Source, "=") {
			return nil, fmt.Errorf("access[%d] at line %d: source %q has an '=', which --grant would split", i, it.Line, e.Source)
		}
		for _, p := range e.Allow {
			if strings.Contains(p, ",") {
				return nil, fmt.Errorf("access[%d] at line %d: pattern %q has a comma, which no subject holds", i, it.Line, p)
			}
		}
		out = append(out, e.Source+"="+strings.Join(e.Allow, ","))
	}
	return out, nil
}
