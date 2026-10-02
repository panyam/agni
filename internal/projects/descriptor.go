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

// projectYAML and designYAML are the on-disk SHAPES, giving the YAML decoder field tags. They are
// not a model of a project, since parsed values become the proto and nothing outside this file sees
// these types.
type projectYAML struct {
	Name  string `yaml:"name"`
	Title string `yaml:"title"`
	// Extends names another project whose config this one layers on top of, "projects/{project}".
	Extends string `yaml:"extends,omitempty"`
	// Conventions is the team's naming vocabulary and Checklists its review checklists by name, both
	// written inline (agni issue 828). Each is bound to its proto, config.NamingConvention and
	// checks.ReviewManifest, so the file and the wire share one schema.
	Conventions yaml.Node `yaml:"conventions,omitempty"`
	Checklists  yaml.Node `yaml:"checklists,omitempty"`
	// Checklist is the key that named a checklist FILE before agni issue 828. It is decoded only so a
	// descriptor still carrying it is refused with a message saying where the checklist goes now.
	Checklist *string `yaml:"checklist,omitempty"`
	// The directory tiers this project owns. Each is OPTIONAL and defaults to the conventional name
	// beside `project.yaml` (see configNames). Declare one for a differently-named directory, or opt
	// out with an empty value.
	Profiles *string `yaml:"profiles,omitempty"`
	Params   *string `yaml:"params,omitempty"`
	Symbols  *string `yaml:"symbols,omitempty"`
	// Lib is the project's own library of derived relations, a directory of `.dl` modules (agni
	// issue 773).
	Lib *string `yaml:"lib,omitempty"`
}

type designYAML struct {
	Name       string   `yaml:"name"`
	Title      string   `yaml:"title"`
	Entry      string   `yaml:"entry"`
	Companions []string `yaml:"companions,omitempty"`
	// Intent is this design's declared architecture, written inline (agni issue 824). It is per-DESIGN,
	// where conventions and profiles describe the team. Only its presence is read here; the intent
	// package parses it, from this same file, when a run composes the design's config.
	Intent yaml.Node `yaml:"intent,omitempty"`
	// Symbols is this design's own symbol library, optional and defaulting to `symbols` beside the
	// descriptor.
	Symbols *string `yaml:"symbols,omitempty"`
}

// The conventional directory names used when a descriptor declares none, matching the layout of
// `examples/tutorial-project`. They belong to `project.yaml` (symbols to both descriptors). FSStore
// composes each tier it finds. Conventions, checklists and a design's intent are not among them,
// since each is a section of a descriptor.
const (
	defaultProfiles = "profiles"
	defaultParams   = "params"
	defaultSymbols  = "symbols"
	defaultLib      = "lib"
)

// formerIntentFile is where a design's intent lived before it moved into design.yaml, and
// formerProjectFiles where a project's conventions and checklist lived before they moved into
// project.yaml. A folder still holding one is refused rather than read without it (agni issues 824,
// 828), because a read that skipped it would quietly drop a whole tier.
const formerIntentFile = "intent.yaml"

var formerProjectFiles = map[string]string{
	"conventions.yaml": "conventions:",
	"review.yaml":      "checklists: {review: ...}",
}

// ProjectConfigNames is what a parsed project descriptor says its config is called, before anything
// checks whether those files exist. An empty entry means the project opted OUT of that tier, which is
// distinct from "not declared, so use the default".
type ProjectConfigNames struct {
	Profiles string
	Params   string
	Symbols  string
	Lib      string
}

// configNames resolves a project descriptor's declarations against the defaults.
func (y projectYAML) configNames() ProjectConfigNames {
	pick := func(declared *string, fallback string) string {
		if declared == nil {
			return fallback
		}
		// An explicitly EMPTY declaration opts a tier out, so it skips CleanRel, which would turn ""
		// into "." and fail validation as "empty".
		if strings.TrimSpace(*declared) == "" {
			return ""
		}
		return CleanRel(*declared)
	}
	return ProjectConfigNames{
		Profiles: pick(y.Profiles, defaultProfiles),
		Params:   pick(y.Params, defaultParams),
		Symbols:  pick(y.Symbols, defaultSymbols),
		Lib:      pick(y.Lib, defaultLib),
	}
}

// idPattern is what a project or design id may look like, the AIP-122 resource-id shape narrowed
// to what is safe in a resource name and comparable without case folding. An id is a path segment in
// "projects/{p}/designs/{d}", so a slash would make one name parse as two resources, and ids
// differing only by case would collide on a case-insensitive filesystem.
var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// ParseProject reads a `project.yaml`, returning the declared id and the wire message.
//
// The id comes back SEPARATELY rather than as `Project.name`, because a bare id is not a resource
// name and only the store knows where the descriptor was reached.
//
// An unknown field is an ERROR, so a misspelled key in a hand-written descriptor fails loudly rather
// than silently configuring nothing.
func ParseProject(r io.Reader) (id string, p *webapi.Project, names ProjectConfigNames, err error) {
	var y projectYAML
	if err := decodeStrict(r, &y); err != nil {
		return "", nil, ProjectConfigNames{}, fmt.Errorf("%s: %w", ProjectDescriptor, err)
	}
	if err := validID("name", y.Name); err != nil {
		return "", nil, ProjectConfigNames{}, fmt.Errorf("%s: %w", ProjectDescriptor, err)
	}
	if y.Checklist != nil {
		return "", nil, ProjectConfigNames{}, fmt.Errorf("%s: checklist names a file (%q); a project's checklists are now written inline under checklists:, by name (agni issue 828)", ProjectDescriptor, *y.Checklist)
	}
	cfg := &webapi.AnalysisConfig{Extends: strings.TrimSpace(y.Extends)}
	conv, err := parseConventions(&y.Conventions)
	if err != nil {
		return "", nil, ProjectConfigNames{}, fmt.Errorf("%s: %w", ProjectDescriptor, err)
	}
	cfg.Conventions = conv
	if cfg.Checklists, err = parseChecklists(&y.Checklists); err != nil {
		return "", nil, ProjectConfigNames{}, fmt.Errorf("%s: %w", ProjectDescriptor, err)
	}
	names = y.configNames()
	for field, rel := range map[string]string{"profiles": names.Profiles, "params": names.Params, "symbols": names.Symbols, "lib": names.Lib} {
		if rel == "" {
			continue
		}
		if err := validRel(field, rel); err != nil {
			return "", nil, ProjectConfigNames{}, fmt.Errorf("%s: %w", ProjectDescriptor, err)
		}
	}
	// Config is always non-nil, even for a project that declares nothing, so callers need no nil
	// check and an absent tier is an empty field.
	return y.Name, &webapi.Project{Title: orName(y.Title, y.Name), Config: cfg}, names, nil
}

// parseConventions binds a project's conventions section to its proto. An absent section is nil,
// meaning the engine's vocabulary. A scalar is the earlier form that named a file, refused with
// where the content goes now.
func parseConventions(n *yaml.Node) (*configpb.NamingConvention, error) {
	switch {
	case n.IsZero() || n.Tag == "!!null":
		return nil, nil
	case n.Kind == yaml.ScalarNode:
		return nil, fmt.Errorf("conventions names a file (%q); a project's conventions are now written inline under conventions: (agni issue 828)", n.Value)
	}
	conv := &configpb.NamingConvention{}
	if err := yamlpb.Decode(n, conv); err != nil {
		return nil, fmt.Errorf("conventions: %w", err)
	}
	return conv, nil
}

// parseChecklists reads a project's checklists section, a mapping from a name to a review manifest,
// in the order it is written, since the first is the project's default. Each manifest is validated
// here, so a malformed checklist fails the project's load and names the checklist.
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
			return nil, fmt.Errorf("line %d: checklist %q must be a checklist (name, areas), not a file name", v.Line, name)
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

// ParseDesign reads a `design.yaml`, returning the declared id and the wire message, with the same
// id-separate and strict-field posture as ParseProject.
//
// `entry_uri` and `companion_uris` come back holding DESIGN-FOLDER-RELATIVE NAMES, not URIs. The
// store turns them into URIs when it locates the design, and nothing outside this package observes
// the intermediate state.
//
// An entry or companion that escapes the design folder is rejected here rather than where a loader
// would open it, so the error names the descriptor the operator has to fix.
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
	entry := CleanRel(y.Entry)
	out := &webapi.Design{Title: orName(y.Title, y.Name), EntryUri: entry}
	seen := map[string]bool{entry: true}
	for _, c := range y.Companions {
		if err := validRel("companions", c); err != nil {
			return "", nil, fmt.Errorf("%s: %w", DesignDescriptor, err)
		}
		clean := CleanRel(c)
		if seen[clean] {
			// A file that is both entry and companion would make the redirect point at itself.
			return "", nil, fmt.Errorf("%s: %q is listed twice (a file is either the entry or a companion, not both)", DesignDescriptor, c)
		}
		seen[clean] = true
		out.CompanionUris = append(out.CompanionUris, clean)
	}
	// A design's config carries only its intent and symbols (see Design.config). Conventions,
	// profiles and parameters describe the team and live on the Project.
	out.Config = &webapi.AnalysisConfig{}
	if y.Symbols == nil {
		out.Config.SymbolPathUris = []string{defaultSymbols}
	} else if clean := CleanRel(*y.Symbols); clean != "" {
		if err := validRel("symbols", clean); err != nil {
			return "", nil, fmt.Errorf("%s: %w", DesignDescriptor, err)
		}
		out.Config.SymbolPathUris = []string{clean}
	}
	// The intent section is bound to its proto here and compiled where a run composes its config, so
	// the store hands it on as a value and no reader of the design opens this file again.
	if !y.Intent.IsZero() && y.Intent.Tag != "!!null" {
		di, err := intent.DecodeSection(&y.Intent)
		if err != nil {
			return "", nil, fmt.Errorf("%s: %w", DesignDescriptor, err)
		}
		out.Config.Intent = di
	}
	return y.Name, out, nil
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

// WriteProject writes a `project.yaml`, the write half of ParseProject. It lives here so the
// descriptor SHAPE has one definition, and TestDescriptorRoundTrip pins the two halves together.
//
// names is optional. Passing nil declares nothing, which suits a scaffolder writing the conventional
// layout, since the defaults already name `profiles`, `params`, `symbols` and `lib`. The conventions
// and checklists sections are a scaffolder's to append, since their content is prose an operator
// edits rather than a value this writer could own.
//
// header is prose written as a YAML comment above the document, "" for none.
func WriteProject(w io.Writer, header, id, title string, names *ProjectConfigNames) error {
	return WriteProjectExtending(w, header, id, title, "", names)
}

// WriteProjectExtending is WriteProject plus a declared `extends`, for a scaffolder writing a project
// that inherits shared config. An empty extends writes no key at all.
func WriteProjectExtending(w io.Writer, header, id, title, extends string, names *ProjectConfigNames) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("project id %q must match %s", id, idPattern)
	}
	y := projectYAML{Name: id, Title: title, Extends: extends}
	if names != nil {
		y.Profiles, y.Params = &names.Profiles, &names.Params
		y.Symbols, y.Lib = &names.Symbols, &names.Lib
	}
	return writeDescriptor(w, header, y)
}

// WriteDesign writes a `design.yaml`, the write half of ParseDesign.
//
// entry and companions are descriptor-relative names, cleaned on the way in on the same terms
// ParseDesign cleans them, so a caller cannot write a descriptor its own parser would reject.
func WriteDesign(w io.Writer, header, id, title, entry string, companions []string) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("design id %q must match %s", id, idPattern)
	}
	if entry == "" {
		return fmt.Errorf("design %q needs an entry", id)
	}
	y := designYAML{Name: id, Title: title, Entry: CleanRel(entry)}
	for _, c := range companions {
		y.Companions = append(y.Companions, CleanRel(c))
	}
	return writeDescriptor(w, header, y)
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
