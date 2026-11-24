// Package grants reads an operator's grants, each a private source and the people who sign in that
// may read it with no GitHub connection of their own (agni issue 1889). agni serve and agnids both
// read them, from --grant, from a serve.yaml's access: list, and from --grants-file (agni issue
// 1895). Every grant is checked with SourceGrant.Validate here, since goapplib lets a bad pattern
// match nobody and say nothing.
package grants

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/panyam/goapplib/storage/connections"
	"gopkg.in/yaml.v3"
)

// Parse reads one --grant value, a source and its viewer patterns as
// github://acme/board=local:*,oidc:*. It refuses a grant that fails SourceGrant.Validate.
func Parse(v string) (connections.SourceGrant, error) {
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

// ParseAll reads every --grant value, in order, and stops at the first it refuses.
func ParseAll(vs []string) ([]connections.SourceGrant, error) {
	var out []connections.SourceGrant
	for _, v := range vs {
		g, err := Parse(v)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

// accessEntry is one item of an access: list.
type accessEntry struct {
	Source string   `yaml:"source"`
	Allow  []string `yaml:"allow"`
}

// Access reads an access: list, as a serve.yaml or a --grants-file holds it, and returns each entry
// as a --grant value. It refuses an unknown key, an entry that fails SourceGrant.Validate, and a
// pattern holding a comma, which --grant would split. Each error names the entry by its place in
// the list.
func Access(n *yaml.Node) ([]string, error) {
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

// grantsFile is a --grants-file: an access: list and nothing else, so the same text pastes into a
// serve.yaml.
type grantsFile struct {
	Access yaml.Node `yaml:"access"`
}

// ReadFile reads the grants in a --grants-file at path. The file holds one key, access:, in a
// serve.yaml's shape. An empty list, or an empty file, grants nothing, so a deployment can ship the
// file before it grants anything. Any other key is refused, as is an entry Access refuses. Each
// error names the file.
func ReadFile(path string) ([]connections.SourceGrant, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- the operator names the file in --grants-file, as they name --tls-cert
	if err != nil {
		return nil, fmt.Errorf("--grants-file: %w", err)
	}
	var f grantsFile
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("--grants-file %s: want only access: [{source, allow}]: %w", path, err)
	}
	if f.Access.Kind == 0 {
		return nil, nil
	}
	vs, err := Access(&f.Access)
	if err != nil {
		return nil, fmt.Errorf("--grants-file %s: %w", path, err)
	}
	return ParseAll(vs)
}

// Patterns returns every viewer pattern across gs, so a GitHub sign-in can check the orgs a
// github:org: pattern names (agni issue 1896).
func Patterns(gs []connections.SourceGrant) []string {
	var out []string
	for _, g := range gs {
		out = append(out, g.Viewers...)
	}
	return out
}
