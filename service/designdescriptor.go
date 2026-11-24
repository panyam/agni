package service

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/stdlib/rules/intent"
	"github.com/panyam/goapplib/storage/mounts"
)

// declared is what a top-level design.yaml among a design's files declares. Push, the viewer's
// upload, add and link all read it here, so a stored design keeps what its folder declares however
// it arrived (agni issue 1887). internal/projects parses a design.yaml strictly where one is read in
// place. This reads loosely, as ProposeDesigns does, and a key that does not decode gives that key's
// zero value. service cannot import internal/projects, since projects imports service.
type declared struct {
	// entry and companions are cleaned paths relative to the folder. entry is "" when none is named.
	entry      string
	companions []string
	// symbols are the symbol folders named under `symbols:`, cleaned. A folder outside the files,
	// such as ../shared/symbols, can't be among them and is left out.
	symbols []string
	// flatten is the flatten_hierarchy the descriptor sets, nil when it has no such line. A stored
	// design keeps its own value when the line is absent, as it keeps its intent.
	flatten *bool
	// intentNode is the raw intent: section. Intent binds it.
	intentNode yaml.Node
}

// readDeclared reads the design.yaml among files. No design.yaml, or one that does not parse, gives
// the zero value. Only a failed read is an error.
func readDeclared(files []string, read func(string) ([]byte, error)) (declared, error) {
	if !slices.Contains(files, DesignDescriptorName) {
		return declared{}, nil
	}
	raw, err := read(DesignDescriptorName)
	if err != nil {
		return declared{}, err
	}
	var y struct {
		Entry      yaml.Node `yaml:"entry"`
		Companions yaml.Node `yaml:"companions"`
		Symbols    yaml.Node `yaml:"symbols"`
		Flatten    yaml.Node `yaml:"flatten_hierarchy"`
		Intent     yaml.Node `yaml:"intent"`
	}
	if yaml.Unmarshal(raw, &y) != nil {
		return declared{}, nil
	}
	clean := func(p string) string { return path.Clean(strings.ReplaceAll(p, "\\", "/")) }
	out := declared{intentNode: y.Intent}
	var entry string
	if y.Entry.Decode(&entry) == nil && entry != "" {
		out.entry = clean(entry)
		var companions []string
		if y.Companions.Decode(&companions) == nil {
			for _, c := range companions {
				out.companions = append(out.companions, clean(c))
			}
		}
	}
	var symbols []string
	if y.Symbols.Decode(&symbols) == nil {
		for _, dir := range symbols {
			if c, err := mounts.CleanPath(dir); err == nil && c != "" {
				out.symbols = append(out.symbols, c)
			}
		}
	}
	var flatten bool
	if !y.Flatten.IsZero() && y.Flatten.Decode(&flatten) == nil {
		out.flatten = &flatten
	}
	return out, nil
}

// Intent binds the descriptor's intent: section, nil when it has none. A section that does not bind
// is INVALID_ARGUMENT, since keeping the design without it would check the board as if it declared
// nothing.
func (d declared) Intent() (*configpb.DesignIntent, error) {
	if d.intentNode.IsZero() || d.intentNode.Tag == "!!null" {
		return nil, nil
	}
	in, err := intent.DecodeSection(&d.intentNode)
	if err != nil {
		return nil, fmt.Errorf("%w: %s's intent: %v", ErrInvalidArgument, DesignDescriptorName, err)
	}
	return in, nil
}

// keep applies what the descriptor declares to a stored design's record. An intent or a
// flatten_hierarchy line it doesn't have leaves the record's own, so a value set with UpdateDesign
// or a bootstrap entry stays.
func (d declared) keep(r *DesignRecord, in *configpb.DesignIntent) {
	if in != nil {
		r.Intent = in
	}
	if d.flatten != nil {
		r.FlattenHierarchy = *d.flatten
	}
}
