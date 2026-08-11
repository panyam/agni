// Package projects is the filesystem-backed implementation of service.ProjectStore: it discovers
// the `project.yaml` / `design.yaml` descriptors that name a design and the set of designs a team
// shares config across (agni issue 170).
//
// It is ONE implementation of that port, and everything in here is specific to being one. Tree
// walking, descriptor file names, and design-folder-relative paths are facts about storing projects
// in a directory hierarchy; a store backed by a database, with design files on object storage,
// implements the same port and has none of them. That is why the port lives in `internal/service`
// and this package lives behind it, and why nothing above the port imports this.
//
// The descriptors parse straight into the wire types (`webapi.Project`, `webapi.Design`). There is
// deliberately no third Go shape for a project between the YAML and the proto: the proto is the
// contract (CONSTRAINTS C2), and a parallel struct per layer is how two layers end up disagreeing
// about what a design is.
package projects

import (
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// The two descriptor file names. They are the only path convention in the feature, and they are
// confined to this package for the reason in the package doc.
const (
	ProjectDescriptor = "project.yaml"
	DesignDescriptor  = "design.yaml"
)

// projectYAML and designYAML are the on-disk SHAPES, and exist only to give the YAML decoder
// something with field tags. They are not a model of a project: the moment parsing succeeds the
// values become the proto, and nothing outside this file sees these types.
type projectYAML struct {
	Name  string `yaml:"name"`
	Title string `yaml:"title"`
}

type designYAML struct {
	Name       string   `yaml:"name"`
	Title      string   `yaml:"title"`
	Entry      string   `yaml:"entry"`
	Companions []string `yaml:"companions"`
}

// idPattern is what a project or design id may look like. It is the AIP-122 resource-id shape
// narrowed to what is safe to put in a resource name and to compare without case folding.
//
// The restriction is not cosmetic. An id becomes a path segment in "projects/{p}/designs/{d}", so an
// id carrying a slash would make one name parse as two different resources, and an id differing only
// by case would make two descriptors that a case-insensitive filesystem cannot both hold look
// distinct to the engine.
var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// ParseProject reads a `project.yaml`, returning the declared id and the wire message.
//
// The id comes back SEPARATELY rather than as `Project.name`, because a bare id is not a resource
// name and a message carrying a half-formed one is a message some caller will eventually serve.
// Building the name needs the store, which is the only thing that knows the descriptor was reachable
// and where.
//
// An unknown field is an ERROR rather than an ignored line. A descriptor is something an operator
// hand-writes and then trusts, so a misspelled key that silently does nothing is the failure mode
// worth spending strictness on: the operator believes they configured something and no message says
// otherwise.
func ParseProject(r io.Reader) (id string, p *webapi.Project, err error) {
	var y projectYAML
	if err := decodeStrict(r, &y); err != nil {
		return "", nil, fmt.Errorf("%s: %w", ProjectDescriptor, err)
	}
	if err := validID("name", y.Name); err != nil {
		return "", nil, fmt.Errorf("%s: %w", ProjectDescriptor, err)
	}
	return y.Name, &webapi.Project{Title: orName(y.Title, y.Name)}, nil
}

// ParseDesign reads a `design.yaml`, returning the declared id and the wire message, with the same
// id-separate and strict-field posture as ParseProject.
//
// `entry_uri` and `companion_uris` come back holding DESIGN-FOLDER-RELATIVE NAMES, not URIs. The
// store turns them into URIs when it locates the design, which is the only place that knows where
// the folder sits. Nothing outside this package observes the intermediate state.
//
// Validation covers containment as well as shape: an entry or companion that escapes the design
// folder is rejected here rather than where a loader would open it, because the descriptor is what
// an operator reads and the error is only actionable while they are looking at it.
func ParseDesign(r io.Reader) (id string, d *webapi.Design, err error) {
	var y designYAML
	if err := decodeStrict(r, &y); err != nil {
		return "", nil, fmt.Errorf("%s: %w", DesignDescriptor, err)
	}
	if err := validID("name", y.Name); err != nil {
		return "", nil, fmt.Errorf("%s: %w", DesignDescriptor, err)
	}
	if y.Entry == "" {
		return "", nil, fmt.Errorf("%s: entry is required (name the file analysis should read; a companion view is declared under companions instead)", DesignDescriptor)
	}
	if err := validRel("entry", y.Entry); err != nil {
		return "", nil, fmt.Errorf("%s: %w", DesignDescriptor, err)
	}
	// entry and companions are DESIGN-FOLDER-RELATIVE here, and become URIs when the store
	// locates the design. Parsing knows the names; only the store knows where they live.
	entry := CleanRel(y.Entry)
	out := &webapi.Design{Title: orName(y.Title, y.Name), EntryUri: entry}
	seen := map[string]bool{entry: true}
	for _, c := range y.Companions {
		if err := validRel("companions", c); err != nil {
			return "", nil, fmt.Errorf("%s: %w", DesignDescriptor, err)
		}
		clean := CleanRel(c)
		if seen[clean] {
			// A file that is both the entry and a companion of the same design would make the
			// redirect point at itself, so the descriptor cannot mean what it says.
			return "", nil, fmt.Errorf("%s: %q is listed twice (a file is either the entry or a companion, not both)", DesignDescriptor, c)
		}
		seen[clean] = true
		out.CompanionUris = append(out.CompanionUris, clean)
	}
	return y.Name, out, nil
}

// CleanRel normalizes a descriptor-relative file name for comparison: forward slashes, no `./`, no
// trailing slash. Exported because the store joins on exactly this rule, and two spellings of
// "clean" is how a companion stops being recognised as one.
func CleanRel(rel string) string {
	return path.Clean(strings.ReplaceAll(strings.TrimSpace(rel), "\\", "/"))
}

func orName(title, name string) string {
	if title != "" {
		return title
	}
	return name
}

// decodeStrict decodes YAML with unknown fields rejected.
func decodeStrict(r io.Reader, out any) error {
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		if err == io.EOF {
			return fmt.Errorf("is empty")
		}
		return err
	}
	return nil
}

// validID checks one declared id against idPattern, with a message that says what is allowed rather
// than echoing the regexp.
func validID(field, id string) error {
	if id == "" {
		return fmt.Errorf("%s is required", field)
	}
	if !idPattern.MatchString(id) {
		return fmt.Errorf("%s %q is not a valid id: lowercase letters, digits, '-', '_' and '.', starting with a letter or digit", field, id)
	}
	return nil
}

// validRel checks that a descriptor's file reference stays inside the descriptor's own folder. An
// absolute path or one climbing out with `..` is rejected: a descriptor is read on behalf of whoever
// mounted the folder, so a reference reaching outside it would let a checked-in file name a path its
// author never had access to.
func validRel(field, rel string) error {
	clean := CleanRel(rel)
	switch {
	case rel == "" || clean == "." || clean == "":
		return fmt.Errorf("%s is empty", field)
	case path.IsAbs(clean) || strings.HasPrefix(rel, "/"):
		return fmt.Errorf("%s %q must be relative to the design folder, not absolute", field, rel)
	case clean == ".." || strings.HasPrefix(clean, "../"):
		return fmt.Errorf("%s %q must stay inside the design folder", field, rel)
	}
	return nil
}
