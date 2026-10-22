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
// longer be trusted. The key carries the mount, since every tree is walked from its own ".".
func (c *walkCache) find(t Tree, root, name string) ([]string, error) {
	fsys := t.FS
	key := t.Mount + "\x00" + root + "\x00" + name
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
// Against EVERY file the load read, not only the descriptor. A descriptor's conventions, checklists
// and intent are sections of it, so the descriptor covers them, but a load also checks that each
// profiles, params, symbols, lib and queries source it names exists, and removing one must revalidate
// it. The parse reports those dependencies, since only reading the descriptor says what they are.
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

// get returns the parsed value cached under key, re-parsing only when a file the last parse depended
// on has changed. parse returns the files it depended on alongside its value.
//
// The clone keeps a cached message from being handed out by pointer, since the store mutates what it
// loads (see cloneProject).
func (c *parseCache[T]) get(fsys fs.FS, key string, parse func() ([]string, string, T, error), clone func(T) T) (string, T, error) {
	c.mu.Lock()
	prev, ok := c.entries[key]
	c.mu.Unlock()
	if ok && sameStamps(prev.deps, stampAll(fsys, keys(prev.deps))) {
		return prev.id, clone(prev.value), nil
	}
	deps, id, v, err := parse()
	if err != nil {
		var zero T
		return "", zero, err
	}
	c.mu.Lock()
	c.entries[key] = parseEntry[T]{deps: stampAll(fsys, deps), id: id, value: clone(v)}
	c.mu.Unlock()
	return id, v, nil
}

func keys(m map[string]fileStamp) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
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
