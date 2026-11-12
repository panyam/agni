package formats

import (
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"sort"
	"sync"
	"time"
)

// Touched records every name a Loader opened, read, walked or looked for and did not find. It also
// records what each looked like at the time. A caller holding a read's result can later ask whether
// anything the read depended on has changed (agni issue 895). A host uses it to keep a parsed design
// in memory without hashing the files again on every request. Re-stamping the names a read touched
// costs a stat each, and hashing an 81MB board costs 90ms.
//
// A name that was looked for and missing is recorded too. Say a sibling or a symbol library appears
// later. That changes the read as surely as an edit does. A directory a walk visited records its
// entry names. A file added to a symbol library shows in no file's stamp, and the entry names show
// it.
//
// The zero value is unusable, and NewTouched makes one. It is safe for concurrent use, so one
// recorder can follow a design read and its board read running at once.
type Touched struct {
	mu   sync.Mutex
	seen map[string]touch
}

// touch is one recorded name. It holds the name space the name was read in, and its stamp then. The
// name space is the Loader's FS, or nil for the host filesystem. It is a value rather than part of
// the map key, because an fs.FS need not be comparable, and an fstest.MapFS is a map. One read
// resolves every name in one Loader's name space anyway.
type touch struct {
	fsys fs.FS
	s    stamp
}

type stamp struct {
	exists bool
	dir    bool
	size   int64
	mod    time.Time
	names  [32]byte // a directory's entry names, hashed
}

// NewTouched returns an empty recorder.
func NewTouched() *Touched { return &Touched{seen: map[string]touch{}} }

// Len returns how many names have been recorded. A read that recorded nothing went around the
// Loader, so a caller must not treat its result as checkable.
func (t *Touched) Len() int {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.seen)
}

// Merge adds everything other recorded. A result built from another cached result uses Merge to
// inherit what that one depended on.
func (t *Touched) Merge(other *Touched) {
	if t == nil || other == nil || t == other {
		return
	}
	other.mu.Lock()
	cp := make(map[string]touch, len(other.seen))
	for k, v := range other.seen {
		cp[k] = v
	}
	other.mu.Unlock()
	t.mu.Lock()
	defer t.mu.Unlock()
	for k, v := range cp {
		if _, ok := t.seen[k]; !ok {
			t.seen[k] = v
		}
	}
}

// Unchanged reports whether every recorded name still stamps as it did. An empty recorder is never
// unchanged, for the reason Len gives.
func (t *Touched) Unchanged() bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	names := make([]string, 0, len(t.seen))
	want := make([]touch, 0, len(t.seen))
	for n, v := range t.seen {
		names = append(names, n)
		want = append(want, v)
	}
	t.mu.Unlock()
	if len(names) == 0 {
		return false
	}
	for i, n := range names {
		if takeStamp(want[i].fsys, n) != want[i].s {
			return false
		}
	}
	return true
}

// note records name as it stamps now. A name keeps the first stamp it got. Say a file is edited
// between two opens within one read. The next check then reads it as changed, rather than as
// whatever it was last.
func (t *Touched) note(fsys fs.FS, name string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	_, ok := t.seen[name]
	t.mu.Unlock()
	if ok {
		return
	}
	s := takeStamp(fsys, name)
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.seen[name]; !ok {
		t.seen[name] = touch{fsys, s}
	}
}

func takeStamp(fsys fs.FS, name string) stamp {
	var fi fs.FileInfo
	var err error
	if fsys == nil {
		fi, err = os.Stat(name)
	} else {
		fi, err = fs.Stat(fsys, name)
	}
	if err != nil {
		// Missing and unreadable stamp alike. Either way the read could not use the name. If a later
		// stamp can use it, that is a change.
		return stamp{}
	}
	s := stamp{exists: true, dir: fi.IsDir(), size: fi.Size(), mod: fi.ModTime()}
	if s.dir {
		s.size, s.mod = 0, time.Time{} // the entry names say what matters, and a dir's own size and time vary by platform
		s.names = dirNames(fsys, name)
	}
	return s
}

func dirNames(fsys fs.FS, name string) [32]byte {
	var names []string
	if fsys == nil {
		es, err := os.ReadDir(name)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return [32]byte{1}
		}
		for _, e := range es {
			names = append(names, e.Name())
		}
	} else {
		es, err := fs.ReadDir(fsys, name)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return [32]byte{1}
		}
		for _, e := range es {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	h := sha256.New()
	for _, n := range names {
		h.Write([]byte(n))
		h.Write([]byte{0})
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}
