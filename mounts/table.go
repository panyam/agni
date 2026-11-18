package mounts

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/panyam/agni/artifact"
)

// Table is a server's live mount list (agni issue 1735). The mounts from its flags are fixed. A
// table with a records folder also serves the mounts CreateMount made, one record each in --store,
// and rereads the folder when it changes. So agni serve and agnids, sharing one --store, list the
// same runtime mounts. Every OS-backed adapter resolves through a Table rather than a slice taken
// at startup, so a mount made at runtime is reachable at once.
type Table struct {
	mu      sync.Mutex
	fixed   []Mount
	records string
	uploads string
	runtime []Mount
	// stamp is what the records folder held at the last read, by stampOf.
	stamp   string
	checked time.Time
	// lastErr is the last reload's error, kept so a bad record does not empty the list.
	lastErr error
	// every is how often List looks at the records folder, recheck unless a test sets it.
	every time.Duration
}

// recheck bounds how often List looks at the records folder, since every Resolve calls List.
const recheck = time.Second

// NewTable returns a table serving fixed and nothing else.
func NewTable(fixed ...Mount) *Table {
	return &Table{fixed: append([]Mount(nil), fixed...), every: recheck}
}

// WithRecords makes the table serve the runtime mounts recorded in records, with their files under
// uploads. It reads them now, and a record it cannot read is an error. Later reads keep the last
// good list.
func (t *Table) WithRecords(records, uploads string) (*Table, error) {
	t.mu.Lock()
	t.records, t.uploads = records, uploads
	t.mu.Unlock()
	return t, t.Reload()
}

// List returns every mount the table serves, fixed ones first in configuration order, then runtime
// ones by name. The caller owns the slice.
func (t *Table) List() []Mount {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.records != "" && time.Since(t.checked) >= t.every {
		t.checked = time.Now()
		if stampOf(t.records) != t.stamp {
			_ = t.reloadLocked()
		}
	}
	out := make([]Mount, 0, len(t.fixed)+len(t.runtime))
	return append(append(out, t.fixed...), t.runtime...)
}

// Find returns the mount named name.
func (t *Table) Find(name string) (Mount, bool) { return Find(t.List(), name) }

// Resolve maps uri to a host path inside its mount, as the package's Resolve does.
func (t *Table) Resolve(uri artifact.URI) (string, error) { return Resolve(t.List(), uri) }

// Fixed reports whether name is a mount from the flags.
func (t *Table) Fixed(name string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := Find(t.fixed, name)
	return ok
}

// Reload rereads the records folder now. A process that just wrote a record calls it, so its own
// change shows at once rather than at the next recheck.
func (t *Table) Reload() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.reloadLocked()
}

func (t *Table) reloadLocked() error {
	if t.records == "" {
		return nil
	}
	stamp := stampOf(t.records)
	recs, err := ReadRecords(t.records)
	if err != nil {
		t.lastErr = err
		return err
	}
	var runtime []Mount
	for _, r := range recs {
		if _, clash := Find(t.fixed, r.Name); clash {
			continue
		}
		runtime = append(runtime, r.Mount(t.uploads))
	}
	t.runtime, t.lastErr, t.stamp = runtime, nil, stamp
	t.checked = time.Now()
	return nil
}

// stampOf describes the records folder by each record's name, size and modification time. A folder's
// own modification time is not enough, since Windows does not always move it when a file is renamed
// into the folder. A missing folder stamps as "".
func stampOf(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "%s\x00%d\x00%d\n", e.Name(), fi.Size(), fi.ModTime().UnixNano())
	}
	return b.String()
}

// Record is a runtime mount as --store keeps it, in <store>/mounts/<name>.yaml. Files lists every
// file in the snapshot with its hash, as a design's record does, so the uploads sweep keeps the
// blobs and a restart can rebuild a view that went missing. It holds no file bytes.
type Record struct {
	Name    string       `yaml:"name"`
	Kind    Kind         `yaml:"kind"`
	Source  string       `yaml:"source"`
	Ref     string       `yaml:"ref,omitempty"`
	Commit  string       `yaml:"commit"`
	Created time.Time    `yaml:"created"`
	Files   []RecordFile `yaml:"files,omitempty"`
}

// RecordFile is one file of a snapshot, by its path in the mount and its "sha256:<hex>" hash.
type RecordFile struct {
	Path   string `yaml:"path"`
	Sha256 string `yaml:"sha256"`
}

// RecordsDir is the folder under --store that holds one record per runtime mount.
const RecordsDir = "mounts"

// SnapshotsDir is the folder under --uploads that holds the views of runtime mounts. A view is
// <uploads>/mounts/<name>/<commit>/, hard links to blobs laid out as the source had them.
const SnapshotsDir = "mounts"

// SnapshotView is the folder a runtime mount at commit serves from.
func SnapshotView(uploads, name, commit string) string {
	return filepath.Join(uploads, SnapshotsDir, name, commit)
}

// Mount is the record as the table serves it, rooted at its view under uploads.
func (r Record) Mount(uploads string) Mount {
	return Mount{Name: r.Name, Root: SnapshotView(uploads, r.Name, r.Commit), Kind: r.Kind, Source: r.Source, Ref: r.Ref, Commit: r.Commit, Runtime: true}
}

// ReadRecords reads every record in dir, sorted by name. A missing dir holds none. A record whose
// file name and name disagree, or whose name cannot be a mount, is an error, since serving it would
// put a mount under a handle the store never meant.
func ReadRecords(dir string) ([]Record, error) {
	r, err := os.OpenRoot(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer r.Close()
	entries, err := fs.ReadDir(r.FS(), ".")
	if err != nil {
		return nil, err
	}
	var out []Record
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".yaml")
		if !ok || e.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		b, err := r.ReadFile(e.Name())
		if err != nil {
			return nil, err
		}
		var rec Record
		if err := yaml.Unmarshal(b, &rec); err != nil {
			return nil, fmt.Errorf("mount record %s: %w", e.Name(), err)
		}
		if rec.Name != name {
			return nil, fmt.Errorf("mount record %s names the mount %q", e.Name(), rec.Name)
		}
		if err := ValidName(rec.Name); err != nil {
			return nil, fmt.Errorf("mount record %s: %w", e.Name(), err)
		}
		if !commitPattern(rec.Commit) {
			return nil, fmt.Errorf("mount record %s: commit %q is not a full hex commit", e.Name(), rec.Commit)
		}
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// WriteRecord writes rec into dir through a temp file and a rename, so a reader sees the whole
// record or none.
func WriteRecord(dir string, rec Record) error {
	if err := ValidName(rec.Name); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	b, err := yaml.Marshal(rec)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".record-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, rec.Name+".yaml"))
}

// RemoveRecord deletes name's record. A missing one is fs.ErrNotExist.
func RemoveRecord(dir, name string) error {
	if err := ValidName(name); err != nil {
		return err
	}
	return os.Remove(filepath.Join(dir, name+".yaml"))
}

// ValidName reports whether name can be a runtime mount's handle: what a mount:// authority takes,
// and also safe as a file name in the records and uploads folders.
func ValidName(name string) error {
	if name == "" || len(name) > 64 {
		return fmt.Errorf("mount name %q must be 1 to 64 characters", name)
	}
	for i, c := range name {
		ok := c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || (i > 0 && (c == '-' || c == '_' || c == '.'))
		if !ok {
			return fmt.Errorf("mount name %q may hold only a-z, 0-9, and - _ . after the first character", name)
		}
	}
	if _, err := artifact.New(name, ""); err != nil {
		return fmt.Errorf("mount name %q: %w", name, err)
	}
	return nil
}

// commitPattern reports whether s is a full 40-hex git commit, so a record cannot name a view
// outside its mount's folder.
func commitPattern(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// ValidCommit reports whether s is a full 40-hex git commit.
func ValidCommit(s string) bool { return commitPattern(s) }
