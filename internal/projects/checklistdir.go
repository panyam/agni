package projects

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/panyam/agni/core/review"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// ChecklistDir is a checklist a descriptor declares as a folder of item files, with items_dir in
// place of areas (agni issue 1812). Index is the checklist's place in the config's Checklists, and
// Dir is the folder relative to the descriptor.
type ChecklistDir struct {
	Index int
	Name  string
	Dir   string
}

// itemFile is one item file's frontmatter: what an inline item carries, plus the area it sits in.
type itemFile struct {
	Area        string `yaml:"area"`
	review.Item `yaml:",inline"`
}

// areaPrefix is the ordering number a folder name starts with, such as "01-" in "01-power".
var areaPrefix = regexp.MustCompile(`^[0-9]+[-_ .]+`)

// ResolveChecklistDirs reads each folder checklist src declares into cfg's Checklists (agni issue
// 1812). list returns every file under a folder, as slash-separated paths relative to the
// descriptor's folder, and read returns one file's contents. It returns every path it read, files
// and folders, so a cache can watch them.
//
// Every .md file under the folder is one item, except a README.md. Its YAML frontmatter carries the
// item's fields and its body is ignored. An item's area is its frontmatter area, or else the name of
// the folder it sits in under items_dir, without an ordering number such as "01-". Areas come in the
// order of their first file's path, and items within an area in the order of their id. A file with
// no frontmatter, an unknown field, no area, or an item the review would refuse fails the read and
// names the file, as does an id two files share.
func ResolveChecklistDirs(cfg *webapi.AnalysisConfig, dirs []ChecklistDir, list func(dir string) ([]string, error), read func(string) ([]byte, error)) ([]string, error) {
	var deps []string
	for _, d := range dirs {
		files, err := list(d.Dir)
		if err != nil {
			return nil, fmt.Errorf("checklist %q: items_dir %s: %w", d.Name, d.Dir, err)
		}
		slices.Sort(files)
		// Read every item file before parsing any, so a caller that lacks several contents learns of
		// them all from one call, as a server asking a client to upload does.
		contents := map[string][]byte{}
		var failed []error
		for _, f := range files {
			if path.Ext(f) != ".md" || strings.EqualFold(path.Base(f), "README.md") {
				continue
			}
			raw, err := read(f)
			if err != nil {
				failed = append(failed, fmt.Errorf("checklist %q: %s: %w", d.Name, f, err))
				continue
			}
			contents[f] = raw
		}
		if len(failed) > 0 {
			return nil, errors.Join(failed...)
		}
		man := review.Manifest{Name: cfg.Checklists[d.Index].GetManifest().GetName()}
		at := map[string]int{}
		ids := map[string]string{}
		for _, f := range files {
			rel := strings.TrimPrefix(f, d.Dir+"/")
			if folder := path.Dir(rel); folder != "." {
				for p := folder; p != "."; p = path.Dir(p) {
					if q := path.Join(d.Dir, p); !slices.Contains(deps, q) {
						deps = append(deps, q)
					}
				}
			}
			if path.Ext(f) != ".md" || strings.EqualFold(path.Base(f), "README.md") {
				continue
			}
			raw, ok := contents[f]
			if !ok {
				continue
			}
			deps = append(deps, f)
			it, err := parseItemFile(raw)
			if err != nil {
				return nil, fmt.Errorf("checklist %q: %s: %w", d.Name, f, err)
			}
			area := strings.TrimSpace(it.Area)
			if area == "" {
				folder, _, nested := strings.Cut(rel, "/")
				if !nested {
					return nil, fmt.Errorf("checklist %q: %s: an item at the top of items_dir needs area: in its frontmatter, or a folder of its own", d.Name, f)
				}
				area = areaPrefix.ReplaceAllString(folder, "")
			}
			if err := review.ValidateStructure(review.Manifest{Name: d.Name, Areas: []review.Area{{Name: area, Items: []review.Item{it.Item}}}}); err != nil {
				return nil, fmt.Errorf("checklist %q: %s: %w", d.Name, f, err)
			}
			if prev, dup := ids[it.ID]; dup {
				return nil, fmt.Errorf("checklist %q: %s and %s both hold item %q", d.Name, prev, f, it.ID)
			}
			ids[it.ID] = f
			i, ok := at[area]
			if !ok {
				i = len(man.Areas)
				at[area] = i
				man.Areas = append(man.Areas, review.Area{Name: area})
			}
			man.Areas[i].Items = append(man.Areas[i].Items, it.Item)
		}
		if len(man.Areas) == 0 {
			return nil, fmt.Errorf("checklist %q: items_dir %s holds no item files (.md)", d.Name, d.Dir)
		}
		for i := range man.Areas {
			slices.SortStableFunc(man.Areas[i].Items, func(a, b review.Item) int { return strings.Compare(a.ID, b.ID) })
		}
		if err := review.ValidateStructure(man); err != nil {
			return nil, fmt.Errorf("checklist %q (items_dir %s): %w", d.Name, d.Dir, err)
		}
		cfg.Checklists[d.Index].Manifest = service.ManifestProto(man)
		deps = append(deps, d.Dir)
	}
	return deps, nil
}

// parseItemFile reads an item file's frontmatter, the YAML between a leading "---" line and the next
// "---" line. An unknown field is an error, as in a descriptor.
func parseItemFile(raw []byte) (itemFile, error) {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return itemFile{}, fmt.Errorf("has no frontmatter; start the file with a --- line, the item's fields, and another --- line")
	}
	front, _, ok := strings.Cut(rest, "\n---")
	if !ok {
		return itemFile{}, fmt.Errorf("its frontmatter has no closing --- line")
	}
	var it itemFile
	dec := yaml.NewDecoder(bytes.NewReader([]byte(front)))
	dec.KnownFields(true)
	if err := dec.Decode(&it); err != nil {
		return itemFile{}, fmt.Errorf("frontmatter: %w", err)
	}
	return it, nil
}
