// Package project parses the two descriptors that give the engine a name for a design and for the
// set of designs a team shares config across: `design.yaml` and `project.yaml` (agni issue 170).
//
// The package holds two things and no more: the descriptor SCHEMA (parse and validate, in this
// file), and one tree-walking implementation of discovery over an fs.FS (store.go). It reaches no
// filesystem of its own and imports nothing of the engine, so the CLI, the service tier, an example,
// and an overlay can all read the same descriptors without any of them agreeing on where files live.
// That is what keeps resolution an INTERFACE rather than a path convention: the service tier's port
// (service.ProjectStore) is satisfied by the tree walk here, and equally by an index or a PLM query
// that never opens a file (C1/C13).
//
// The concepts are deliberately asymmetric, because the on-disk layout already is. A project owns
// what is shared across designs (a naming vocabulary, interface profiles, seeded parameters, a review
// checklist). A design owns what is specific to it: which file is the analysis source, and which
// files are companion VIEWS of that same design rather than independent sources of it (C21).
package project

import (
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Project is a parsed `project.yaml`: the identity of a set of designs that share configuration.
//
// It carries no config of its own yet. The conventions/profiles/params/checklist a project owns are
// still resolved from serve-startup flags, and they move here in the change that wires them, so a
// field never sits in the schema doing nothing.
type Project struct {
	// Name is the project id, and the last segment of the resource name "projects/{name}". It is
	// DECLARED rather than derived from the folder, so renaming the folder does not rename the project
	// and two mounts cannot accidentally agree by sharing a directory name.
	Name string `yaml:"name"`
	// Title is the human-readable label a UI shows. Empty falls back to Name.
	Title string `yaml:"title"`
}

// Design is a parsed `design.yaml`: one design's identity and which of the files beside it is which.
type Design struct {
	// Name is the design id, and the last segment of "projects/{project}/designs/{name}".
	Name string `yaml:"name"`
	// Title is the human-readable label a UI shows. Empty falls back to Name.
	Title string `yaml:"title"`
	// Entry names the file this design's ANALYSIS reads: the netlist the design team produces. It is
	// required, because a design whose entry is unstated is exactly the ambiguity this descriptor
	// exists to remove — a folder holding a netlist and a schematic export gives different component
	// counts depending on which one a tool happens to open, and nothing on disk said which was meant.
	//
	// Design-folder-relative, never absolute and never escaping the folder.
	Entry string `yaml:"entry"`
	// Companions name files that are VIEWS of this same design rather than independent sources of it:
	// a schematic export, a board file, an IPC-2581 (C21). They are geometry to render and to locate
	// findings on, and a tool asked to analyse one reads Entry instead.
	//
	// Listing is opt-in per file, and that is load-bearing. A sibling this list does not mention is
	// nobody's companion — a later revision of the netlist (`board-rev-b.edn`) sits in the same folder
	// and is a legitimate analysis source in its own right, so a rule of "everything beside the entry
	// is a companion" would silently redirect a diff of two revisions into a diff of one against
	// itself.
	//
	// Design-folder-relative, same containment as Entry.
	Companions []string `yaml:"companions"`
}

// idPattern is what a project or design id may look like. It is the AIP-122 resource-id shape
// narrowed to what is safe to put in a resource name and to compare without case folding: a
// lowercase alphanumeric start, then alphanumerics, `-`, `_`, or `.`.
//
// The restriction is not cosmetic. An id becomes a path segment in "projects/{p}/designs/{d}", so an
// id carrying a slash would make one name parse as two different resources, and an id differing only
// by case would make two descriptors that a case-insensitive filesystem cannot both hold look
// distinct to the engine.
var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// LoadProject parses and validates a `project.yaml`.
//
// An unknown field is an ERROR rather than an ignored line. A descriptor is something an operator
// hand-writes and then trusts, so a misspelled key that silently does nothing is the failure mode
// worth spending strictness on: the operator believes they configured something and no message says
// otherwise.
func LoadProject(r io.Reader) (Project, error) {
	var p Project
	if err := decodeStrict(r, &p); err != nil {
		return Project{}, fmt.Errorf("project.yaml: %w", err)
	}
	if err := validID("name", p.Name); err != nil {
		return Project{}, fmt.Errorf("project.yaml: %w", err)
	}
	return p, nil
}

// LoadDesign parses and validates a `design.yaml`, with the same strict-field posture as LoadProject.
//
// Validation covers containment as well as shape: an entry or companion that escapes the design
// folder is rejected here rather than at the point a loader would open it, because the descriptor is
// the thing an operator reads and the error is only actionable while they are looking at it.
func LoadDesign(r io.Reader) (Design, error) {
	var d Design
	if err := decodeStrict(r, &d); err != nil {
		return Design{}, fmt.Errorf("design.yaml: %w", err)
	}
	if err := validID("name", d.Name); err != nil {
		return Design{}, fmt.Errorf("design.yaml: %w", err)
	}
	if d.Entry == "" {
		return Design{}, fmt.Errorf("design.yaml: entry is required (name the file analysis should read; a companion view is declared under companions instead)")
	}
	if err := validRel("entry", d.Entry); err != nil {
		return Design{}, fmt.Errorf("design.yaml: %w", err)
	}
	seen := map[string]bool{CleanRel(d.Entry): true}
	for _, c := range d.Companions {
		if err := validRel("companions", c); err != nil {
			return Design{}, fmt.Errorf("design.yaml: %w", err)
		}
		clean := CleanRel(c)
		if seen[clean] {
			// A file that is both the entry and a companion of the same design would make the redirect
			// point at itself, so the descriptor cannot mean what it says.
			return Design{}, fmt.Errorf("design.yaml: %q is listed twice (a file is either the entry or a companion, not both)", c)
		}
		seen[clean] = true
	}
	return d, nil
}

// IsCompanion reports whether rel — a design-folder-relative file name — is one this design declared
// a companion view of itself.
//
// Comparison is on the CLEANED name, so `./gateway.kicad_pcb` and `gateway.kicad_pcb` are the same
// file. A caller that has an absolute or mount-relative path converts it to a design-folder-relative
// one first; this type deliberately knows nothing about where the design folder is.
func (d Design) IsCompanion(rel string) bool {
	want := CleanRel(rel)
	for _, c := range d.Companions {
		if CleanRel(c) == want {
			return true
		}
	}
	return false
}

// IsEntry reports whether rel names this design's analysis entry, on the same cleaned comparison as
// IsCompanion.
func (d Design) IsEntry(rel string) bool { return CleanRel(d.Entry) == CleanRel(rel) }

// DisplayName is Title when the descriptor gave one, else the id. It exists so every consumer
// degrades the same way instead of each choosing whether an untitled project shows blank.
func (p Project) DisplayName() string { return orName(p.Title, p.Name) }

// DisplayName is Title when the descriptor gave one, else the id.
func (d Design) DisplayName() string { return orName(d.Title, d.Name) }

func orName(title, name string) string {
	if title != "" {
		return title
	}
	return name
}

// CleanRel normalizes a descriptor-relative file name for comparison: forward slashes, no `./`, no
// trailing slash. It is exported because the adapters that turn a mount-relative path into a
// design-relative one have to normalize on exactly the same rule, and two spellings of "clean" is
// how a companion stops being recognised as one.
func CleanRel(rel string) string {
	return path.Clean(strings.ReplaceAll(strings.TrimSpace(rel), "\\", "/"))
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
