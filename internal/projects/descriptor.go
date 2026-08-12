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
	// The config this project owns. Each is OPTIONAL and defaults to the conventional name beside
	// `project.yaml` (see defaultProjectConfig), so the layout a review project already takes needs
	// no declaration at all. Declaring is for the cases convention cannot express: a conventions file
	// shared by several projects, a differently-named checklist, or opting out with an empty value.
	// Extends names another project whose config this one layers on top of, "projects/{project}".
	Extends     string  `yaml:"extends"`
	Conventions *string `yaml:"conventions"`
	Profiles    *string `yaml:"profiles"`
	Params      *string `yaml:"params"`
	Checklist   *string `yaml:"checklist"`
}

type designYAML struct {
	Name       string   `yaml:"name"`
	Title      string   `yaml:"title"`
	Entry      string   `yaml:"entry"`
	Companions []string `yaml:"companions"`
	// Intent is this design's declared architecture, optional and defaulting to `intent.yaml` beside
	// the descriptor. It is per-DESIGN because each board has its own intended architecture, where
	// conventions and profiles describe the team.
	Intent *string `yaml:"intent"`
}

// The conventional names a project's config takes when `project.yaml` declares none. They are the
// layout `examples/tutorial-project` already uses, so convention covers the ordinary case and
// declaration is only needed to depart from it.
const (
	defaultConventions = "conventions.yaml"
	defaultProfiles    = "profiles"
	defaultParams      = "params"
	defaultChecklist   = "review.yaml"
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
}

// ConfigNames resolves a project descriptor's declarations against the defaults.
func (y projectYAML) configNames() ProjectConfigNames {
	pick := func(declared *string, fallback string) string {
		if declared == nil {
			return fallback
		}
		return CleanRel(*declared)
	}
	return ProjectConfigNames{
		Conventions: pick(y.Conventions, defaultConventions),
		Profiles:    pick(y.Profiles, defaultProfiles),
		Params:      pick(y.Params, defaultParams),
		Checklist:   pick(y.Checklist, defaultChecklist),
	}
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
func ParseProject(r io.Reader) (id string, p *webapi.Project, names ProjectConfigNames, err error) {
	var y projectYAML
	if err := decodeStrict(r, &y); err != nil {
		return "", nil, ProjectConfigNames{}, fmt.Errorf("%s: %w", ProjectDescriptor, err)
	}
	if err := validID("name", y.Name); err != nil {
		return "", nil, ProjectConfigNames{}, fmt.Errorf("%s: %w", ProjectDescriptor, err)
	}
	names = y.configNames()
	for field, rel := range map[string]string{"conventions": names.Conventions, "profiles": names.Profiles, "params": names.Params, "checklist": names.Checklist} {
		if rel == "" {
			continue
		}
		if err := validRel(field, rel); err != nil {
			return "", nil, ProjectConfigNames{}, fmt.Errorf("%s: %w", ProjectDescriptor, err)
		}
	}
	// Config is always non-nil, even for a project that declares nothing, so every caller that fills
	// or reads a tier can do so without a nil check and an absent tier is an empty field rather than
	// an absent message.
	return y.Name, &webapi.Project{
		Title:  orName(y.Title, y.Name),
		Config: &webapi.AnalysisConfig{Extends: strings.TrimSpace(y.Extends)},
	}, names, nil
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
	// A design's config carries only its intent (see Design.config): conventions, profiles and
	// parameters describe the team and live on the Project.
	out.Config = &webapi.AnalysisConfig{}
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

// WriteProject writes a `project.yaml`. It is the write half of ParseProject, and it lives here for
// the reason the package doc gives for the parse half: the descriptor SHAPE is this package's, and a
// scaffolder that marshalled its own struct would be the second definition of it that the "no third
// Go shape" rule exists to prevent. A round-trip test pins the two against each other.
//
// names is optional. Passing nil declares nothing, which is what a scaffolder writing the conventional
// layout wants: the defaults already name `conventions.yaml`, `profiles`, `params` and `review.yaml`,
// so declaring them would be noise that then has to be kept in step with the files beside it.
//
// header is prose written as a YAML comment above the document, "" for none. It is a plain string
// rather than per-field comments because a generated descriptor needs to say ONE thing — that it was
// generated and can be edited — and threading yaml.Node comments through every field to say it would
// be a lot of machinery for a paragraph.
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
