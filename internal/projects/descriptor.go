// Package projects is the filesystem-backed implementation of service.ProjectStore. It discovers
// the `project.yaml` / `design.yaml` descriptors that name a design and the set of designs a team
// shares config across (agni issue 170).
//
// It is ONE implementation of that port. Tree walking, descriptor file names, and
// design-folder-relative paths are facts about storing projects in directories, which a
// database-backed store would not share, so the port lives in `service/` and nothing above it
// imports this package.
//
// The descriptors parse straight into the wire types (`webapi.Project`, `webapi.Design`), with no
// third Go shape between the YAML and the proto, because the proto is the contract (CONSTRAINTS
// C2).
package projects

import (
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/review"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/yamlpb"
	"github.com/panyam/agni/service"
	"github.com/panyam/agni/stdlib/rules/intent"
)

// The two descriptor file names, confined to this package for the reason in the package doc.
const (
	ProjectDescriptor = "project.yaml"
	DesignDescriptor  = "design.yaml"
)

// descriptorYAML is the on-disk SHAPE of both descriptors, giving the YAML decoder field tags. A
// project and a design share one schema (agni issue 1136), and the file's name says which role it
// plays: a design names the files it reads (entry, companions, revisions) and a project names none.
// It is not a model of either, since parsed values become the proto and nothing outside this file
// sees this type.
type descriptorYAML struct {
	Name  string `yaml:"name"`
	Title string `yaml:"title"`
	// Extends names the project descriptor this one layers on, as a path relative to this descriptor
	// or a mount:// URI. A design names its project and a project names shared config.
	Extends string `yaml:"extends,omitempty"`
	// Conventions, Checklists and Intent are written inline, each bound to its proto
	// (config.NamingConvention, checks.ReviewManifest, config.DesignIntent), so the file and the wire
	// share one schema.
	Conventions yaml.Node `yaml:"conventions,omitempty"`
	Checklists  yaml.Node `yaml:"checklists,omitempty"`
	Intent      yaml.Node `yaml:"intent,omitempty"`
	// The file tiers, each a list of files or directories, as paths relative to this descriptor or
	// mount:// URIs. Nothing is found by its folder's name, so a tier this list does not name is not
	// in the run.
	Profiles []string `yaml:"profiles,omitempty"`
	Params   []string `yaml:"params,omitempty"`
	Symbols  []string `yaml:"symbols,omitempty"`
	Lib      []string `yaml:"lib,omitempty"`
	Queries  []string `yaml:"queries,omitempty"`
	// The design's own files, relative to its folder. The top-level entry is the current revision.
	// Revisions are its others, each its own entry with its own companions, so a later netlist is read
	// with its own board (agni issue 848). A file belongs to one revision at most.
	Entry      string         `yaml:"entry,omitempty"`
	Companions []string       `yaml:"companions,omitempty"`
	Revisions  []revisionYAML `yaml:"revisions,omitempty"`
	// FlattenHierarchy reads the design's netlist whole, walking into every hierarchical block
	// (webapi.Design.flatten_hierarchy, agni issue 1102).
	FlattenHierarchy bool `yaml:"flatten_hierarchy,omitempty"`
}

type revisionYAML struct {
	Entry      string   `yaml:"entry"`
	Companions []string `yaml:"companions,omitempty"`
}

// Sources is what a descriptor names outside itself, as written: the descriptor it extends and the
// files of each tier, each a descriptor-relative path or a mount:// URI. The store resolves them,
// since only it knows where the descriptor sits, and a source naming nothing is an error there.
type Sources struct {
	Extends  string
	Profiles []string
	Params   []string
	Symbols  []string
	Lib      []string
	Queries  []string
}

func (y descriptorYAML) sources() Sources {
	return Sources{
		Extends: strings.TrimSpace(y.Extends), Profiles: y.Profiles, Params: y.Params,
		Symbols: y.Symbols, Lib: y.Lib, Queries: y.Queries,
	}
}

// idPattern is what a project or design id may look like, the AIP-122 resource-id shape narrowed
// to what is safe in a resource name and comparable without case folding. An id is a path segment in
// "projects/{p}/designs/{d}", so a slash would make one name parse as two resources, and ids
// differing only by case would collide on a case-insensitive filesystem.
var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// ParseProject reads a `project.yaml`, returning the declared id, the wire message and the sources it
// names, which the store resolves.
//
// The id comes back SEPARATELY rather than as `Project.name`, because a bare id is not a resource
// name and only the store knows where the descriptor was reached.
//
// An unknown field is an ERROR, so a misspelled key in a hand-written descriptor fails loudly rather
// than silently configuring nothing. A design's own keys are refused too, naming where they go.
func ParseProject(r io.Reader) (id string, p *webapi.Project, src Sources, err error) {
	y, cfg, src, err := parseDescriptor(r, ProjectDescriptor)
	if err != nil {
		return "", nil, Sources{}, err
	}
	for key, set := range map[string]bool{"entry": y.Entry != "", "companions": len(y.Companions) > 0, "revisions": len(y.Revisions) > 0, "flatten_hierarchy": y.FlattenHierarchy} {
		if set {
			return "", nil, Sources{}, fmt.Errorf("%s: %s names a design's files, so it belongs in that design's %s", ProjectDescriptor, key, DesignDescriptor)
		}
	}
	// Config is always non-nil, even for a project that declares nothing, so callers need no nil
	// check and an absent tier is an empty field.
	return y.Name, &webapi.Project{Title: orName(y.Title, y.Name), Config: cfg}, src, nil
}

// ParseDesign reads a `design.yaml`, returning the declared id, the wire message and the sources it
// names, with the same id-separate and strict-field posture as ParseProject.
//
// `entry_uri` and `companion_uris` come back holding DESIGN-FOLDER-RELATIVE NAMES, not URIs. The
// store turns them into URIs when it locates the design, and nothing outside this package observes
// the intermediate state.
//
// An entry or companion that escapes the design folder is rejected here rather than where a loader
// would open it, so the error names the descriptor the operator has to fix.
func ParseDesign(r io.Reader) (id string, d *webapi.Design, src Sources, err error) {
	y, cfg, src, err := parseDescriptor(r, DesignDescriptor)
	if err != nil {
		return "", nil, Sources{}, err
	}
	fail := func(err error) (string, *webapi.Design, Sources, error) {
		return "", nil, Sources{}, fmt.Errorf("%s: %w", DesignDescriptor, err)
	}
	if y.Entry == "" {
		return fail(fmt.Errorf("entry is required (name the file analysis should read; a companion view is declared under companions instead)"))
	}
	if err := validRel("entry", y.Entry); err != nil {
		return fail(err)
	}
	entry := CleanRel(y.Entry)
	out := &webapi.Design{Title: orName(y.Title, y.Name), EntryUri: entry, FlattenHierarchy: y.FlattenHierarchy, Config: cfg}
	seen := map[string]bool{entry: true}
	for _, c := range y.Companions {
		if err := validRel("companions", c); err != nil {
			return fail(err)
		}
		clean := CleanRel(c)
		if seen[clean] {
			// A file that is both entry and companion would make the redirect point at itself.
			return fail(fmt.Errorf("%q is listed twice (a file is either the entry or a companion, not both)", c))
		}
		seen[clean] = true
		out.CompanionUris = append(out.CompanionUris, clean)
	}
	// Each revision is its own entry and views, checked against every file already declared, so no
	// file is two revisions' at once and a board is never borrowed by a netlist it was not drawn for.
	for i, r := range y.Revisions {
		where := fmt.Sprintf("revisions[%d]", i)
		if r.Entry == "" {
			return fail(fmt.Errorf("%s: entry is required (name the revision's netlist; its views go under companions)", where))
		}
		rev := &webapi.DesignRevision{}
		for j, f := range append([]string{r.Entry}, r.Companions...) {
			field := where + ".entry"
			if j > 0 {
				field = where + ".companions"
			}
			if err := validRel(field, f); err != nil {
				return fail(err)
			}
			clean := CleanRel(f)
			if seen[clean] {
				return fail(fmt.Errorf("%q is listed twice (a file belongs to one revision, as its entry or one of its companions)", f))
			}
			seen[clean] = true
			if j == 0 {
				rev.EntryUri = clean
			} else {
				rev.CompanionUris = append(rev.CompanionUris, clean)
			}
		}
		out.Revisions = append(out.Revisions, rev)
	}
	return y.Name, out, src, nil
}

// parseDescriptor decodes either descriptor and reads the sections both may carry. The inline
// sections are bound to their protos here, so the store hands them on as values and no reader of the
// design opens this file again. The sources are checked for shape only, since whether one names
// anything depends on where the descriptor sits.
func parseDescriptor(r io.Reader, file string) (descriptorYAML, *webapi.AnalysisConfig, Sources, error) {
	var y descriptorYAML
	fail := func(err error) (descriptorYAML, *webapi.AnalysisConfig, Sources, error) {
		return descriptorYAML{}, nil, Sources{}, fmt.Errorf("%s: %w", file, err)
	}
	if err := decodeStrict(r, &y); err != nil {
		return fail(err)
	}
	if err := validID("name", y.Name); err != nil {
		return fail(err)
	}
	cfg := &webapi.AnalysisConfig{}
	var err error
	if cfg.Conventions, err = parseConventions(&y.Conventions); err != nil {
		return fail(err)
	}
	if cfg.Checklists, err = parseChecklists(&y.Checklists); err != nil {
		return fail(err)
	}
	if !y.Intent.IsZero() && y.Intent.Tag != "!!null" {
		if cfg.Intent, err = intent.DecodeSection(&y.Intent); err != nil {
			return fail(err)
		}
	}
	src := y.sources()
	if src.Extends != "" {
		if err := validSource("extends", src.Extends); err != nil {
			return fail(err)
		}
		if path.Base(CleanRel(src.Extends)) != ProjectDescriptor {
			return fail(fmt.Errorf("extends %q must name a %s, the descriptor this one layers on", src.Extends, ProjectDescriptor))
		}
	}
	for field, refs := range map[string][]string{"profiles": src.Profiles, "params": src.Params, "symbols": src.Symbols, "lib": src.Lib, "queries": src.Queries} {
		for _, ref := range refs {
			if err := validSource(field, ref); err != nil {
				return fail(err)
			}
		}
	}
	return y, cfg, src, nil
}

// parseConventions binds a descriptor's conventions section to its proto. An absent section is nil,
// meaning the vocabulary the descriptor inherits, or the engine's.
func parseConventions(n *yaml.Node) (*configpb.NamingConvention, error) {
	if n.IsZero() || n.Tag == "!!null" {
		return nil, nil
	}
	conv := &configpb.NamingConvention{}
	if err := yamlpb.Decode(n, conv); err != nil {
		return nil, fmt.Errorf("conventions: %w", err)
	}
	return conv, nil
}

// parseChecklists reads a descriptor's checklists section, a mapping from a name to a review
// manifest, in the order it is written, since the first is the default. Each manifest is validated
// here, so a malformed checklist fails the load and names the checklist.
//
// A manifest's YAML is the review package's authoring form, which spells an item's binding flat where
// the proto nests it, so it is read by review.Load and converted, rather than bound to the proto.
func parseChecklists(n *yaml.Node) ([]*webapi.NamedChecklist, error) {
	if n.IsZero() || n.Tag == "!!null" {
		return nil, nil
	}
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("line %d: checklists must map a name to a checklist, as checklists: {review: {areas: [...]}}", n.Line)
	}
	var out []*webapi.NamedChecklist
	seen := map[string]bool{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		name := strings.TrimSpace(k.Value)
		if k.Kind != yaml.ScalarNode || name == "" {
			return nil, fmt.Errorf("line %d: a checklist needs a name", k.Line)
		}
		if seen[name] {
			return nil, fmt.Errorf("line %d: checklist %q is declared twice", k.Line, name)
		}
		seen[name] = true
		if v.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("line %d: checklist %q must be a checklist (name, areas)", v.Line, name)
		}
		b, err := yaml.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("checklist %q: %w", name, err)
		}
		man, err := review.Load(strings.NewReader(string(b)))
		if err != nil {
			return nil, fmt.Errorf("checklist %q (line %d): %w", name, v.Line, err)
		}
		out = append(out, &webapi.NamedChecklist{Name: name, Manifest: service.ManifestProto(man)})
	}
	return out, nil
}

// CleanRel normalizes a descriptor-relative file name for comparison (forward slashes, no `./`, no
// trailing slash). Exported because the store joins on this rule, and a second spelling of "clean"
// would stop a companion being recognised as one.
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
// absolute path or one climbing out with `..` is rejected, because a descriptor is read on behalf of
// whoever mounted the folder and must not name a path its author never had access to.
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

// validSource checks the shape of a source a descriptor names: a mount:// URI, or a path relative to
// the descriptor. A relative path may climb with `..`, since the containment boundary is the mount and
// artifact.URI.Join refuses a path that leaves it.
func validSource(field, ref string) error {
	ref = strings.TrimSpace(ref)
	switch {
	case ref == "":
		return fmt.Errorf("%s names an empty path", field)
	case strings.HasPrefix(ref, artifact.Scheme+"://"):
		if _, err := artifact.Parse(ref); err != nil {
			return fmt.Errorf("%s %q: %w", field, ref, err)
		}
		return nil
	case strings.Contains(ref, "://"):
		return fmt.Errorf("%s %q: a source is a path relative to the descriptor or a %s:// URI", field, ref, artifact.Scheme)
	case path.IsAbs(CleanRel(ref)) || strings.HasPrefix(ref, "/"):
		return fmt.Errorf("%s %q must be relative to the descriptor, not absolute", field, ref)
	}
	return nil
}

// WriteProject writes a `project.yaml`, the write half of ParseProject. It lives here so the
// descriptor SHAPE has one definition, and TestDescriptorRoundTrip pins the two halves together.
//
// src names the project's parent and its file tiers, as ParseProject returns them. The conventions and
// checklists sections are a scaffolder's to append, since their content is prose an operator edits
// rather than a value this writer could own.
//
// header is prose written as a YAML comment above the document, "" for none.
func WriteProject(w io.Writer, header, id, title string, src Sources) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("project id %q must match %s", id, idPattern)
	}
	y := descriptorYAML{Name: id, Title: title}
	y.setSources(src)
	return writeDescriptor(w, header, y)
}

// WriteDesign writes a `design.yaml`, the write half of ParseDesign.
//
// entry and companions are descriptor-relative names, cleaned on the way in on the same terms
// ParseDesign cleans them, so a caller cannot write a descriptor its own parser would reject. src.Extends
// names the design's project, "" for a design in none.
func WriteDesign(w io.Writer, header, id, title, entry string, companions []string, src Sources) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("design id %q must match %s", id, idPattern)
	}
	if entry == "" {
		return fmt.Errorf("design %q needs an entry", id)
	}
	y := descriptorYAML{Name: id, Title: title, Entry: CleanRel(entry)}
	for _, c := range companions {
		y.Companions = append(y.Companions, CleanRel(c))
	}
	y.setSources(src)
	return writeDescriptor(w, header, y)
}

func (y *descriptorYAML) setSources(src Sources) {
	y.Extends = src.Extends
	y.Profiles, y.Params, y.Symbols, y.Lib, y.Queries = src.Profiles, src.Params, src.Symbols, src.Lib, src.Queries
}

// writeDescriptor emits the optional comment header then the marshalled document.
func writeDescriptor(w io.Writer, header string, doc any) error {
	if header != "" {
		for line := range strings.SplitSeq(strings.TrimRight(header, "\n"), "\n") {
			var err error
			if line == "" {
				_, err = fmt.Fprintln(w, "#")
			} else {
				_, err = fmt.Fprintln(w, "# "+line)
			}
			if err != nil {
				return err
			}
		}
	}
	b, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}
