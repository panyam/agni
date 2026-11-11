package formats

import (
	"encoding/hex"
	"encoding/json"
	"hash/crc64"
	"io"
	"io/fs"
	"sort"
)

// fingerprintVersion changes whenever Fingerprint records something new, so an older blob reads as a
// miss.
const fingerprintVersion = 2

var crcTable = crc64.MakeTable(crc64.ECMA)

// Fingerprint encodes what t recorded by CONTENT rather than by stamp. Another process can then check
// it over another copy of the same files (agni issue 911). A stamp's modification time is the moment a
// browser worker was handed a file, so after a reload no stamp would match.
//
// Each name records whether it existed. A file adds its size and CRC-64, or the sum a ContentSummer
// gives for it, and a directory adds the hash of its entry names, as Unchanged already compares. The sum is CRC-64 rather than SHA-256, because the
// check runs on every restore. Over a 97MB board in wasm it costs 82ms, against 458ms for SHA-256. It
// guards against a file that changed, not against one built to collide, and the files are the
// visitor's own.
//
// It reports false when the read cannot be fingerprinted. That happens when nothing was recorded. It
// happens when a name was read off the host filesystem rather than through a Loader's FS, since no
// other process sees that name space. And it happens when a file changed between the read and now,
// since the hashed content would then differ from what the read saw.
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

// CheckFingerprint reports whether every name in fp reads the same in fsys. If so, it returns a
// recorder holding each name as it stamps NOW. From then on the restored result is checked by stamp,
// as a fresh read's is. A blob it cannot decode does not match, and nor does one an older version
// wrote.
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

// ContentSummer is a file system that can name a file's content without reading it, from a hash it
// was given with the file (agni issue 1515). A browser worker is given each design file's listed
// sha256, and holds a placeholder in place of a board it was not sent. It then checks a stored read
// against that hash, rather than reading the bytes or failing on the placeholder.
//
// ContentSum reports the file's sum and size, and ok false when it does not know them. A sum is
// "sha256:<hex>", so it never equals a CRC-64 sum, and a file the summer knows always fingerprints
// the same way on every file system that knows it.
type ContentSummer interface {
	ContentSum(name string) (sum string, size int64, ok bool)
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
	if s, ok := fsys.(ContentSummer); ok {
		if sum, size, ok := s.ContentSum(name); ok {
			return contentStamp{Name: name, Exists: true, Size: size, Sum: sum}, true
		}
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
