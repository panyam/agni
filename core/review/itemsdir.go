package review

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/panyam/jaala/ns"
	"gopkg.in/yaml.v3"

	"github.com/panyam/agni/core/ident"
)

// itemFile is one item file's frontmatter: what an inline item carries, plus the area it sits in.
type itemFile struct {
	Area string `yaml:"area"`
	Item `yaml:",inline"`
}

// areaPrefix is the ordering number a folder name starts with, such as "01-" in "01-power".
var areaPrefix = regexp.MustCompile(`^[0-9]+[-_ .]+`)

// ItemsDir reads a checklist kept as a folder of item files (agni issues 1812, 1836) into a manifest
// titled title. label names the checklist in errors. list returns every file under dir, as
// slash-separated paths in the same space as dir, and read returns one file's contents. It returns
// every path it read, files and folders, so a cache can watch them.
//
// Every .md file under the folder is one item, except a README.md. Its YAML frontmatter carries the
// item's fields and its body is ignored. An item's area is its frontmatter area, or else the name of
// the folder it sits in under dir, without an ordering number such as "01-". Areas come in the order
// of their first file's path, and items within an area in the natural order of their id, so 16 comes
// before 100 (agni issue 1888). A key the item doesn't know is ignored as the team's own metadata. A
// file with no frontmatter, a key that looks like a typo of a known one, no area, or an item the
// review would refuse fails the read and names the file, as does an id two files share.
//
// It reads every item file before parsing any. A caller that lacks several files' contents then
// learns of them all from one call, joined, as a server asking a client to upload them does.
func ItemsDir(label, title, dir string, list func(dir string) ([]string, error), read func(string) ([]byte, error)) (Manifest, []string, error) {
	files, err := list(dir)
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("checklist %q: items_dir %s: %w", label, dir, err)
	}
	slices.Sort(files)
	isItem := func(f string) bool { return path.Ext(f) == ".md" && !strings.EqualFold(path.Base(f), "README.md") }
	contents := map[string][]byte{}
	var failed []error
	for _, f := range files {
		if !isItem(f) {
			continue
		}
		raw, err := read(f)
		if err != nil {
			failed = append(failed, fmt.Errorf("checklist %q: %s: %w", label, f, err))
			continue
		}
		contents[f] = raw
	}
	if len(failed) > 0 {
		return Manifest{}, nil, errors.Join(failed...)
	}
	var deps []string
	man := Manifest{Name: title}
	at := map[string]int{}
	ids := map[string]string{}
	for _, f := range files {
		rel := strings.TrimPrefix(f, dir+"/")
		for p := path.Dir(rel); p != "."; p = path.Dir(p) {
			if q := path.Join(dir, p); !slices.Contains(deps, q) {
				deps = append(deps, q)
			}
		}
		raw, ok := contents[f]
		if !isItem(f) || !ok {
			continue
		}
		deps = append(deps, f)
		it, err := parseItemFile(raw)
		if err != nil {
			return Manifest{}, nil, fmt.Errorf("checklist %q: %s: %w", label, f, err)
		}
		area := strings.TrimSpace(it.Area)
		if area == "" {
			folder, _, nested := strings.Cut(rel, "/")
			if !nested {
				return Manifest{}, nil, fmt.Errorf("checklist %q: %s: an item at the top of items_dir needs area: in its frontmatter, or a folder of its own", label, f)
			}
			area = areaPrefix.ReplaceAllString(folder, "")
		}
		if err := ValidateStructure(Manifest{Name: label, Areas: []Area{{Name: area, Items: []Item{it.Item}}}}); err != nil {
			return Manifest{}, nil, fmt.Errorf("checklist %q: %s: %w", label, f, err)
		}
		if prev, dup := ids[it.ID]; dup {
			return Manifest{}, nil, fmt.Errorf("checklist %q: %s and %s both hold item %q", label, prev, f, it.ID)
		}
		ids[it.ID] = f
		i, ok := at[area]
		if !ok {
			i = len(man.Areas)
			at[area] = i
			man.Areas = append(man.Areas, Area{Name: area})
		}
		man.Areas[i].Items = append(man.Areas[i].Items, it.Item)
	}
	if len(man.Areas) == 0 {
		return Manifest{}, nil, fmt.Errorf("checklist %q: items_dir %s holds no item files (.md)", label, dir)
	}
	for i := range man.Areas {
		slices.SortStableFunc(man.Areas[i].Items, func(a, b Item) int { return ident.CompareNatural(a.ID, b.ID) })
	}
	if err := ValidateStructure(man); err != nil {
		return Manifest{}, nil, fmt.Errorf("checklist %q (items_dir %s): %w", label, dir, err)
	}
	return man, append(deps, dir), nil
}

// LoadFS reads the checklist name names in fsys (agni issue 1836). name may be a manifest file, a
// manifest file whose items_dir names a folder of item files, or such a folder itself, which reads as
// a checklist titled with the folder's name. An items_dir is relative to the manifest file and must
// sit under the file's folder, as a descriptor's paths must. A manifest naming items_dir holds a name
// beside it and nothing else, so one written with areas too is refused rather than read half.
func LoadFS(fsys fs.FS, name string) (Manifest, error) {
	fi, err := fs.Stat(fsys, name)
	if err != nil {
		return Manifest{}, err
	}
	if fi.IsDir() {
		title := path.Base(name)
		man, _, err := ItemsDir(title, title, name, walkFiles(fsys), func(f string) ([]byte, error) { return fs.ReadFile(fsys, f) })
		return man, err
	}
	raw, err := fs.ReadFile(fsys, name)
	if err != nil {
		return Manifest{}, err
	}
	var head map[string]yaml.Node
	if yaml.Unmarshal(raw, &head) == nil {
		if dirNode, ok := head["items_dir"]; ok {
			for k := range head {
				if k != "name" && k != "items_dir" {
					return Manifest{}, fmt.Errorf("review manifest %s names items_dir, so its items come from that folder; remove %s, or drop items_dir and list the areas here", name, k)
				}
			}
			var title string
			if n, ok := head["name"]; ok {
				title = n.Value
			}
			rel := path.Clean(strings.ReplaceAll(strings.TrimSpace(dirNode.Value), "\\", "/"))
			if dirNode.Value == "" || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") || path.IsAbs(rel) {
				return Manifest{}, fmt.Errorf("review manifest %s: items_dir %q must name a folder under the manifest's own folder", name, dirNode.Value)
			}
			label := title
			if label == "" {
				label = name
			}
			man, _, err := ItemsDir(label, title, path.Join(path.Dir(name), rel), walkFiles(fsys), func(f string) ([]byte, error) { return fs.ReadFile(fsys, f) })
			return man, err
		}
	}
	return Load(bytes.NewReader(raw))
}

// walkFiles lists every file under a folder of fsys.
func walkFiles(fsys fs.FS) func(string) ([]string, error) {
	return func(dir string) ([]string, error) {
		var out []string
		err := fs.WalkDir(fsys, dir, func(p string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !e.IsDir() {
				out = append(out, p)
			}
			return nil
		})
		return out, err
	}
}

// parseItemFile reads an item file's frontmatter, the YAML between a leading "---" line and the next
// "---" line. A key the item doesn't know is the team's own metadata, such as owner: or needs:, and
// the review ignores it (agni issue 1888). A key that looks like a typo of one it knows, such as rul:
// for rule:, still fails and names the field, so a misspelling never drops a binding quietly.
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
	var keys map[string]yaml.Node
	if err := yaml.Unmarshal([]byte(front), &keys); err != nil {
		return itemFile{}, fmt.Errorf("frontmatter: %w", err)
	}
	known := itemFileKeys()
	var typos []string
	for k := range keys {
		if slices.Contains(known, k) {
			continue
		}
		if hint := ns.DidYouMeanValue(known, k); hint != "" {
			typos = append(typos, fmt.Sprintf("%s:%s", k, hint))
		}
	}
	if len(typos) > 0 {
		slices.Sort(typos)
		return itemFile{}, fmt.Errorf("frontmatter: unknown field %s", strings.Join(typos, "; unknown field "))
	}
	var it itemFile
	if err := yaml.Unmarshal([]byte(front), &it); err != nil {
		return itemFile{}, fmt.Errorf("frontmatter: %w", err)
	}
	return it, nil
}

// itemFileKeys lists the frontmatter keys an item file reads: area, and every field an Item takes,
// its inlined Binding's included.
func itemFileKeys() []string {
	return append([]string{"area"}, yamlKeys(reflect.TypeOf(Item{}))...)
}

// yamlKeys lists the YAML keys a struct type decodes, walking into fields tagged inline.
func yamlKeys(t reflect.Type) []string {
	var keys []string
	for i := range t.NumField() {
		f := t.Field(i)
		name, opts, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		switch {
		case name == "-":
		case strings.Contains(opts, "inline"):
			keys = append(keys, yamlKeys(f.Type)...)
		case name != "":
			keys = append(keys, name)
		}
	}
	return keys
}
