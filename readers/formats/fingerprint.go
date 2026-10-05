package formats

import (
	"encoding/hex"
	"encoding/json"
	"hash/crc64"
	"io"
	"io/fs"
	"sort"
)

// fingerprintVersion changes whenever what Fingerprint records does, so an older blob reads as a miss.
const fingerprintVersion = 1

var crcTable = crc64.MakeTable(crc64.ECMA)

// Fingerprint is what t recorded, by CONTENT rather than by stamp, so it can be checked in another
// process over another copy of the same files (agni issue 911). A stamp's modification time is the
// moment a browser worker was handed a file, so after a reload nothing would match.
//
// Each name carries whether it existed, and a file its size and CRC-64 while a directory the hash of
// its entry names, as Unchanged already compares. CRC-64 rather than SHA-256 because the check runs on
// every restore and costs 82ms over a 97MB board in wasm against 458ms. It guards against a file that
// changed, not against one built to collide, and the files are the visitor's own.
//
// It reports false when the read cannot be fingerprinted: nothing was recorded, a name was read off the
// host filesystem rather than through a Loader's FS (no other process sees that name space), or a file
// changed between the read and now, so the content hashed would not be what the read saw.
func (t *Touched) Fingerprint() ([]byte, bool) {
	if t == nil {
		return nil, false
	}
	t.mu.Lock()
	seen := make(map[string]touch, len(t.seen))
	for n, v := range t.seen {
		seen[n] = v
	}
	t.mu.Unlock()
	if len(seen) == 0 {
		return nil, false
	}
	fp := fingerprint{V: fingerprintVersion}
	for n, v := range seen {
		if v.fsys == nil || takeStamp(v.fsys, n) != v.s {
			return nil, false
		}
		e, ok := contentOf(v.fsys, n)
		if !ok {
			return nil, false
		}
		fp.Names = append(fp.Names, e)
	}
	sort.Slice(fp.Names, func(i, j int) bool { return fp.Names[i].Name < fp.Names[j].Name })
	b, err := json.Marshal(fp)
	if err != nil {
		return nil, false
	}
	return b, true
}

// CheckFingerprint reports whether every name fp records reads the same in fsys, and if so returns a
// recorder holding each name as it stamps NOW, so the restored result is checked by stamp from then on,
// as a fresh read's is. A blob it cannot decode, or one an older version wrote, does not match.
func CheckFingerprint(fsys fs.FS, fp []byte) (*Touched, bool) {
	var f fingerprint
	if fsys == nil || json.Unmarshal(fp, &f) != nil || f.V != fingerprintVersion || len(f.Names) == 0 {
		return nil, false
	}
	t := NewTouched()
	for _, want := range f.Names {
		got, ok := contentOf(fsys, want.Name)
		if !ok || got != want {
			return nil, false
		}
		t.note(fsys, want.Name)
	}
	return t, true
}

type fingerprint struct {
	V     int            `json:"v"`
	Names []contentStamp `json:"names"`
}

// contentStamp is one name by content. Sum is the file's CRC-64, or a directory's entry-name hash.
type contentStamp struct {
	Name   string `json:"n"`
	Exists bool   `json:"e,omitempty"`
	Dir    bool   `json:"d,omitempty"`
	Size   int64  `json:"s,omitempty"`
	Sum    string `json:"h,omitempty"`
}

func contentOf(fsys fs.FS, name string) (contentStamp, bool) {
	fi, err := fs.Stat(fsys, name)
	if err != nil {
		return contentStamp{Name: name}, true
	}
	if fi.IsDir() {
		h := dirNames(fsys, name)
		return contentStamp{Name: name, Exists: true, Dir: true, Sum: hex.EncodeToString(h[:])}, true
	}
	f, err := fsys.Open(name)
	if err != nil {
		return contentStamp{}, false
	}
	defer f.Close()
	c := crc64.New(crcTable)
	n, err := io.Copy(c, f)
	if err != nil {
		return contentStamp{}, false
	}
	return contentStamp{Name: name, Exists: true, Size: n, Sum: hex.EncodeToString(c.Sum(nil))}, true
}
