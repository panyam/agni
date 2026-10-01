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

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
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
	// The config this project owns. Each is OPTIONAL and defaults to the conventional name beside
	// `project.yaml` (see configNames). Declare one for a shared conventions file, a
	// differently-named checklist, or to opt out with an empty value. Extends names another
	// project whose config this one layers on top of, "projects/{project}".
	Extends     string  `yaml:"extends"`
	Conventions *string `yaml:"conventions"`
	Profiles    *string `yaml:"profiles"`
	Params      *string `yaml:"params"`
	Symbols     *string `yaml:"symbols"`
	Checklist   *string `yaml:"checklist"`
	// Lib is the project's own library of derived relations, a directory of `.dl` modules (agni
	// issue 773).
	Lib *string `yaml:"lib"`
}

type designYAML struct {
	Name       string   `yaml:"name"`
	Title      string   `yaml:"title"`
	Entry      string   `yaml:"entry"`
	Companions []string `yaml:"companions"`
	// Intent is this design's declared architecture, optional and defaulting to `intent.yaml` beside
	// the descriptor. It is per-DESIGN, where conventions and profiles describe the team.
	Intent *string `yaml:"intent"`
	// Symbols is this design's own symbol library, optional and defaulting to `symbols` beside the
	// descriptor.
	Symbols *string `yaml:"symbols"`
}

// The conventional config names used when a descriptor declares none, matching the layout of
// `examples/tutorial-project`. The first six belong to `project.yaml` (symbols to both descriptors),
// intent to `design.yaml`. FSStore composes each tier it finds.
const (
	defaultConventions = "conventions.yaml"
	defaultProfiles    = "profiles"
	defaultParams      = "params"
	defaultSymbols     = "symbols"
	defaultChecklist   = "review.yaml"
	defaultLib         = "lib"
	defaultIntent      = "intent.yaml"
)

// ProjectConfigNames is what a parsed project descriptor says its config is called, before anything
// checks whether those files exist. An empty entry means the project opted OUT of that tier, which is
// distinct from "not declared, so use the default".
type ProjectConfigNames struct {
	Conventions string
	Profiles    string
	Params      string
	Checklist   string
	Symbols     string
	Lib         string
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
		Conventions: pick(y.Conventions, defaultConventions),
		Profiles:    pick(y.Profiles, defaultProfiles),
		Params:      pick(y.Params, defaultParams),
		Checklist:   pick(y.Checklist, defaultChecklist),
		Symbols:     pick(y.Symbols, defaultSymbols),
		Lib:         pick(y.Lib, defaultLib),
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
	names = y.configNames()
	for field, rel := range map[string]string{"conventions": names.Conventions, "profiles": names.Profiles, "params": names.Params, "checklist": names.Checklist, "symbols": names.Symbols, "lib": names.Lib} {
		if rel == "" {
			continue
		}
		if err := validRel(field, rel); err != nil {
			return "", nil, ProjectConfigNames{}, fmt.Errorf("%s: %w", ProjectDescriptor, err)
		}
	}
	// Config is always non-nil, even for a project that declares nothing, so callers need no nil
	// check and an absent tier is an empty field.
	return y.Name, &webapi.Project{
		Title:  orName(y.Title, y.Name),
		Config: &webapi.AnalysisConfig{Extends: strings.TrimSpace(y.Extends)},
	}, names, nil
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
	if y.Intent == nil {
		out.Config.IntentUri = defaultIntent
	} else if clean := CleanRel(*y.Intent); clean != "" {
		if err := validRel("intent", clean); err != nil {
			return "", nil, fmt.Errorf("%s: %w", DesignDescriptor, err)
		}
		out.Config.IntentUri = clean
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
// layout, since the defaults already name `conventions.yaml`, `profiles`, `params` and `review.yaml`.
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
		y.Conventions, y.Profiles = &names.Conventions, &names.Profiles
		y.Params, y.Checklist = &names.Params, &names.Checklist
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
