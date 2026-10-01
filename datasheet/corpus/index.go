// Package corpus keeps a published PartSpec corpus: promotion of a workbench draft into it, and the
// index the datasheet service maintains beside it (agni issue 749).
//
// The files are the source of truth and the index is derived from them, so an index can always be
// rebuilt and a rebuild is the answer to any doubt about one. What the index adds is what files
// cannot give cheaply: which file seeds an MPN without parsing every file, the content hash of what
// was validated, and a generation that advances whenever the corpus changes, which is how a reader
// will learn that what it cached is stale.
//
// The engine never reads this package (C34). It reads PartSpecs through core/param, the contract, and
// LoadSet stays the way a project's own params/ is read. The index is the datasheet service's, and a
// reader outside the service will reach it through a lookup in the contract rather than through this
// file format.
//
// This package does no I/O of its own. It reads a corpus through an fs.FS and returns bytes for its
// caller to write.
package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/panyam/agni/core/param"
)

// IndexFile is the index's name inside the corpus directory. LoadSet reads only *.textproto, so the
// index never reaches a check as if it were a spec.
const IndexFile = "corpus.index.json"

// ErrNoIndex is what Read reports for a corpus that has never been indexed.
var ErrNoIndex = errors.New("the corpus has no index")

// Index maps every seeded MPN to the file that seeds it. Entries are sorted by upper-cased MPN, the
// key LoadSet matches on, so the file is stable in git and two builds write it identically.
type Index struct {
	// Generation advances by one on every change to the entries, through promotion or a rebuild that
	// found the files had moved on. It never goes backwards, so a reader can compare two.
	Generation uint64  `json:"generation"`
	Entries    []Entry `json:"entries"`
}

// Entry is one seeded spec.
type Entry struct {
	MPN  string `json:"mpn"`
	File string `json:"file"`
	// Hash is the sha256 of the file's bytes as they were validated, so a file edited by hand after it
	// was indexed no longer matches.
	Hash string `json:"hash"`
}

// Build walks the corpus with param.LoadCorpus, the walk LoadSet makes, and indexes what it finds. It
// is all-or-nothing for the same reason LoadSet is: a file that does not parse or Validate, or two
// files claiming one MPN, fail the build with the file named, rather than leaving a spec out of the
// index. The result has generation zero; Refresh decides the generation.
func Build(corpus fs.FS) (*Index, error) {
	files, err := param.LoadCorpus(corpus)
	if err != nil {
		return nil, err
	}
	ix := &Index{Entries: make([]Entry, 0, len(files))}
	for _, f := range files {
		ix.Entries = append(ix.Entries, Entry{MPN: f.Spec.GetMpn(), File: f.File, Hash: hashOf(f.Data)})
	}
	ix.sort()
	return ix, nil
}

// Refresh rebuilds the index from the corpus and carries the generation on from prev: unchanged when
// the entries are the same, one more when they are not. prev may be nil, for a corpus never indexed,
// and then any corpus is a change. It reports whether the entries changed.
func Refresh(corpus fs.FS, prev *Index) (*Index, bool, error) {
	ix, err := Build(corpus)
	if err != nil {
		return nil, false, err
	}
	if prev != nil && sameEntries(prev.Entries, ix.Entries) {
		ix.Generation = prev.Generation
		return ix, false, nil
	}
	ix.Generation = prev.next()
	return ix, true, nil
}

// Read loads the corpus's index, or ErrNoIndex when it has none.
func Read(corpus fs.FS) (*Index, error) {
	b, err := fs.ReadFile(corpus, IndexFile)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNoIndex
	}
	if err != nil {
		return nil, err
	}
	ix := &Index{}
	if err := json.Unmarshal(b, ix); err != nil {
		return nil, fmt.Errorf("%s: %w", IndexFile, err)
	}
	return ix, nil
}

// Marshal renders the index as it is written to IndexFile.
func (ix *Index) Marshal() ([]byte, error) {
	b, err := json.MarshalIndent(ix, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Lookup returns the entry seeding an MPN, matched case-insensitively as LoadSet matches.
func (ix *Index) Lookup(mpn string) (Entry, bool) {
	key := strings.ToUpper(mpn)
	i := sort.Search(len(ix.Entries), func(i int) bool { return strings.ToUpper(ix.Entries[i].MPN) >= key })
	if i < len(ix.Entries) && strings.ToUpper(ix.Entries[i].MPN) == key {
		return ix.Entries[i], true
	}
	return Entry{}, false
}

// Diff lists how next differs from prev, one line per MPN added, removed or changed, for a caller
// explaining why an index is stale.
func Diff(prev, next *Index) []string {
	var out []string
	old := map[string]Entry{}
	if prev != nil {
		for _, e := range prev.Entries {
			old[strings.ToUpper(e.MPN)] = e
		}
	}
	for _, e := range next.Entries {
		k := strings.ToUpper(e.MPN)
		was, ok := old[k]
		switch {
		case !ok:
			out = append(out, fmt.Sprintf("added   %s (%s)", e.MPN, e.File))
		case was != e:
			out = append(out, fmt.Sprintf("changed %s (%s)", e.MPN, e.File))
		}
		delete(old, k)
	}
	gone := make([]string, 0, len(old))
	for _, e := range old {
		gone = append(gone, fmt.Sprintf("removed %s (%s)", e.MPN, e.File))
	}
	sort.Strings(gone)
	return append(out, gone...)
}

// next is the generation after ix, treating a nil index as generation zero.
func (ix *Index) next() uint64 {
	if ix == nil {
		return 1
	}
	return ix.Generation + 1
}

func (ix *Index) sort() {
	sort.Slice(ix.Entries, func(i, j int) bool {
		return strings.ToUpper(ix.Entries[i].MPN) < strings.ToUpper(ix.Entries[j].MPN)
	})
}

func sameEntries(a, b []Entry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func hashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}
