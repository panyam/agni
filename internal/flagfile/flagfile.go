// Package flagfile reads a YAML file that sets a command's flags (agni issue 1853). Each key names
// a flag. Sections nest, and a key path joins with dashes, so `github: {client_id: x}` and a flat
// `github_client_id: x` both set --github-client-id. Underscores in a key read as dashes.
//
// A flag typed on the command line wins over the file outright, as a flag wins over agni.yaml. A
// value is handed to the flag as text, so a `file:` or `env:` reference stays a reference for the
// flag to resolve.
package flagfile

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

// Entry is one flag the file sets.
type Entry struct {
	// Key is the key path as the file writes it, such as github.client_id, for errors.
	Key string
	// Flag is the flag the key names, such as github-client-id.
	Flag string
	// Values holds a scalar's one value, or a list's items.
	Values []string
	// List reports whether the file gave a list.
	List bool
	// Raw is the value's node, for a flag Parse was asked to keep whole. Values is empty then.
	Raw *yaml.Node
}

// Parse reads data into the flags it sets, in file order. A key whose flag is named in raw keeps
// its value as a node, whatever its shape, so a caller can read a section such as an inline
// bootstrap itself. Parse refuses two spellings of one flag, a list of anything but scalars, and a
// key with no value. An empty file sets nothing.
func Parse(data []byte, raw ...string) ([]Entry, error) {
	var doc yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, err
	}
	if len(doc.Content) == 0 {
		return nil, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("line %d: want a mapping of flag names to values", root.Line)
	}
	p := parser{raw: map[string]bool{}, seen: map[string]string{}}
	for _, r := range raw {
		p.raw[r] = true
	}
	if err := p.mapping(root, "", ""); err != nil {
		return nil, err
	}
	return p.out, nil
}

type parser struct {
	raw  map[string]bool
	seen map[string]string
	out  []Entry
}

func (p *parser) mapping(n *yaml.Node, keyPrefix, flagPrefix string) error {
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if k.Kind != yaml.ScalarNode || k.Value == "" {
			return fmt.Errorf("line %d: a key must be a flag name", k.Line)
		}
		key, flag := keyPrefix+k.Value, flagPrefix+strings.ReplaceAll(k.Value, "_", "-")
		if p.raw[flag] {
			if err := p.add(Entry{Key: key, Flag: flag, Raw: v}, k.Line); err != nil {
				return err
			}
			continue
		}
		switch v.Kind {
		case yaml.MappingNode:
			if err := p.mapping(v, key+".", flag+"-"); err != nil {
				return err
			}
		case yaml.SequenceNode:
			e := Entry{Key: key, Flag: flag, List: true}
			for _, it := range v.Content {
				if it.Kind != yaml.ScalarNode {
					return fmt.Errorf("line %d: %s: want a list of values", it.Line, key)
				}
				e.Values = append(e.Values, it.Value)
			}
			if err := p.add(e, k.Line); err != nil {
				return err
			}
		case yaml.ScalarNode:
			if v.Tag == "!!null" {
				return fmt.Errorf("line %d: %s has no value", k.Line, key)
			}
			if err := p.add(Entry{Key: key, Flag: flag, Values: []string{v.Value}}, k.Line); err != nil {
				return err
			}
		default:
			return fmt.Errorf("line %d: %s: want a value, a list or a section", k.Line, key)
		}
	}
	return nil
}

func (p *parser) add(e Entry, line int) error {
	if first, ok := p.seen[e.Flag]; ok {
		return fmt.Errorf("line %d: %s and %s both set --%s", line, first, e.Key, e.Flag)
	}
	p.seen[e.Flag] = e.Key
	p.out = append(p.out, e)
	return nil
}

// Apply sets each entry's flag in fs, skipping a flag already set on the command line and any
// entry kept Raw. A list replaces a list flag's value, and a list for a flag that takes one value is
// refused. An entry naming no flag in fs is refused by its key, as is a value the flag refuses.
func Apply(fs *pflag.FlagSet, entries []Entry) error {
	typed := map[string]bool{}
	// Changed, not Visit, since a FlagSet built from another's flags, as cobra's LocalFlags is,
	// shares their Changed but visits none of them.
	fs.VisitAll(func(f *pflag.Flag) { typed[f.Name] = f.Changed })
	for _, e := range entries {
		f := fs.Lookup(e.Flag)
		if f == nil {
			return fmt.Errorf("%s: there is no flag --%s", e.Key, e.Flag)
		}
		if typed[e.Flag] || e.Raw != nil {
			continue
		}
		if !e.List {
			if err := fs.Set(e.Flag, e.Values[0]); err != nil {
				return fmt.Errorf("%s: %w", e.Key, err)
			}
			continue
		}
		sv, ok := f.Value.(pflag.SliceValue)
		if !ok {
			return fmt.Errorf("%s: --%s takes one value, not a list", e.Key, e.Flag)
		}
		if err := sv.Replace(e.Values); err != nil {
			return fmt.Errorf("%s: %w", e.Key, err)
		}
		f.Changed = true
	}
	return nil
}
