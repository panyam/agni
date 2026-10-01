package projects

import (
	"io/fs"
	"path"
	"strings"
	"sync"
)

// The store's cache. Discovery is a bounded directory walk per call, ~20ms on a mount holding a few
// hundred directories, and a browse UI pays it on every listing.
//
// It never TRUSTS itself. Every read revalidates against the filesystem before returning, because an
// operator who edits a descriptor while the server runs must see the edit on the next request (agni
// issue 176). Revalidation stays cheap, since a stat is a fraction of a ReadDir and comparing stats is a
// fraction of parsing YAML.
//
// Two levels, because two things change independently:
//
//   - DISCOVERY (which directories hold a descriptor) is keyed on the mtimes of the directories the
//     walk visited, since a directory's mtime moves when a descriptor is added or removed.
//   - CONTENT (what a descriptor says) is keyed on each read file's mtime and size.
//
// Keying discovery on file content, or content on directory mtime, would each miss half the edits.

// dirStamp is what a directory looked like when the walk visited it.
type dirStamp struct {
	dir     string
	modUnix int64
	missing bool // the directory was gone; a later one appearing is itself a change
}

// fileStamp is what a file looked like when it was parsed.
type fileStamp struct {
	modUnix int64
	size    int64
	missing bool
}

// walkCache memoizes one findDescriptors result against the directories it visited.
type walkCache struct {
	mu      sync.Mutex
	entries map[string]walkEntry
}

type walkEntry struct {
	found   []string
	visited []dirStamp
}

func newWalkCache() *walkCache { return &walkCache{entries: map[string]walkEntry{}} }

// find returns the folders under root holding `name`, walking only when the previous answer can no
// longer be trusted.
func (c *walkCache) find(fsys fs.FS, root, name string) ([]string, error) {
	key := root + "\x00" + name
	c.mu.Lock()
	prev, ok := c.entries[key]
	c.mu.Unlock()
	if ok && stampsHold(fsys, prev.visited) {
		return append([]string(nil), prev.found...), nil
	}
	found, visited, err := findDescriptorsStamped(fsys, root, name)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.entries[key] = walkEntry{found: found, visited: visited}
	c.mu.Unlock()
	return append([]string(nil), found...), nil
}

// stampsHold reports whether every directory the walk visited still looks the way it did. A
// directory's mtime does NOT move when a file's contents change, which is fine because discovery
// only asks WHICH folders hold a descriptor; parseCache answers what one says.
func stampsHold(fsys fs.FS, visited []dirStamp) bool {
	for _, d := range visited {
		info, err := fs.Stat(fsys, walkRoot(d.dir))
		switch {
		case err != nil:
			if !d.missing {
				return false
			}
		case d.missing:
			return false // a directory that was absent now exists, so the walk must run again
		case info.ModTime().UnixNano() != d.modUnix:
			return false
		}
	}
	return true
}

// findDescriptorsStamped returns the tree-relative folders under root (root included) holding a
// descriptor of the given name, to MaxDepth, along with what each visited directory looked like so
// the answer can be revalidated later.
//
// It does NOT descend into a folder that already holds the descriptor, since a project inside a
// project would be ambiguous, and stopping there keeps the walk off a design's own symbol and sheet
// subfolders. Dot-directories are skipped so `.git` or `.venv` never costs a traversal.
//
// The descriptor is looked for AMONG THE ENTRIES ReadDir already returned rather than with a second
// Open, which would cost a syscall per directory.
func findDescriptorsStamped(fsys fs.FS, root, name string) ([]string, []dirStamp, error) {
	var out []string
	var visited []dirStamp
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		info, statErr := fs.Stat(fsys, walkRoot(dir))
		stamp := dirStamp{dir: dir}
		if statErr != nil {
			stamp.missing = true
		} else {
			stamp.modUnix = info.ModTime().UnixNano()
		}
		visited = append(visited, stamp)

		entries, err := fs.ReadDir(fsys, walkRoot(dir))
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() && e.Name() == name {
				out = append(out, dir)
				return
			}
		}
		if depth >= MaxDepth {
			return
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			walk(path.Join(dir, e.Name()), depth+1)
		}
	}
	walk(normalizeDir(root), 0)
	return out, visited, nil
}

// parseCache memoizes a loaded descriptor against every file the load read.
//
// Against EVERY file, not just the descriptor, because loading a project also reads its conventions
// and probes for its profiles, params, and checklist. An entry keyed on `project.yaml` alone would
// survive an edit to `conventions.yaml`, and the rules would keep running under the old vocabulary
// with nothing to show it.
//
// The containing DIRECTORY is a dependency too, which covers the existence probes, since adding or
// removing `params/` moves the directory's mtime.
type parseCache[T any] struct {
	mu      sync.Mutex
	entries map[string]parseEntry[T]
}

type parseEntry[T any] struct {
	deps  map[string]fileStamp
	id    string
	value T
}

func newParseCache[T any]() *parseCache[T] {
	return &parseCache[T]{entries: map[string]parseEntry[T]{}}
}

// get returns the parsed value cached under key, re-parsing only when a file in deps has changed.
//
// `clone` exists because callers mutate the returned pointer (the store fills in resource names and
// rewrites relative refs into URIs), and handing out the cached message would leak one request's
// fill-in into the next.
func (c *parseCache[T]) get(fsys fs.FS, key string, deps []string, parse func() (string, T, error), clone func(T) T) (string, T, error) {
	now := stampAll(fsys, deps)
	c.mu.Lock()
	prev, ok := c.entries[key]
	c.mu.Unlock()
	if ok && sameStamps(prev.deps, now) {
		return prev.id, clone(prev.value), nil
	}
	id, v, err := parse()
	if err != nil {
		var zero T
		return "", zero, err
	}
	c.mu.Lock()
	c.entries[key] = parseEntry[T]{deps: now, id: id, value: clone(v)}
	c.mu.Unlock()
	return id, v, nil
}

func stampAll(fsys fs.FS, deps []string) map[string]fileStamp {
	out := make(map[string]fileStamp, len(deps))
	for _, d := range deps {
		out[d] = stampOf(fsys, d)
	}
	return out
}

func sameStamps(a, b map[string]fileStamp) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func stampOf(fsys fs.FS, name string) fileStamp {
	info, err := fs.Stat(fsys, name)
	if err != nil {
		return fileStamp{missing: true}
	}
	return fileStamp{modUnix: info.ModTime().UnixNano(), size: info.Size()}
}
