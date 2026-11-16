package archivefs

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// job is a small tree of members, shaped like an ODB++ job one folder down, as producers pack them.
var job = map[string]string{
	"job/matrix/matrix":                 "STEP {\nNAME=PCB\n}\n",
	"job/steps/pcb/profile":             "S P 0\n",
	"job/steps/pcb/layers/top/features": "#\nP 1 2 0 P 0 0\n",
}

func tarOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, name := range sortedKeys(files) {
		body := files[name]
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(tw, body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipOf(t *testing.T, files map[string]string, flags uint16) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range sortedKeys(files) {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Flags: flags})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, files[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func gz(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(b); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func xzOf(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := xz.NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(b); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zstdOf(t *testing.T, b []byte) []byte {
	t.Helper()
	w, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	return w.EncodeAll(b, nil)
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// members reads every file of fsys into a map.
func members(t *testing.T, fsys fs.FS) map[string]string {
	t.Helper()
	got := map[string]string{}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(fsys, p)
		got[p] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func sameMembers(t *testing.T, kind string, got, want map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s holds %d members, want %d: %v", kind, len(got), len(want), sortedKeys(got))
	}
	for name, body := range want {
		if got[name] != body {
			t.Errorf("%s: %s = %q, want %q", kind, name, got[name], body)
		}
	}
}

func TestEveryContainerKindOpensToTheSameMembers(t *testing.T) {
	plain := tarOf(t, job)
	bz2, err := os.ReadFile(filepath.Join("testdata", "job.tar.bz2"))
	if err != nil {
		t.Fatal(err)
	}
	for kind, b := range map[string][]byte{
		"zip": zipOf(t, job, 0), "tar": plain, "tar.gz": gz(t, plain), "tar.xz": xzOf(t, plain),
		"tar.zst": zstdOf(t, plain), "tar.bz2": bz2,
	} {
		fsys, err := Open(b)
		if err != nil {
			t.Errorf("%s: %v", kind, err)
			continue
		}
		sameMembers(t, kind, members(t, fsys), job)
	}
}

func TestTheBytesChooseTheDecoderWhateverTheName(t *testing.T) {
	dir := t.TempDir()
	// A gzipped tar named as a zip, as producers name these loosely.
	if err := os.WriteFile(filepath.Join(dir, "job.zip"), gz(t, tarOf(t, job)), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := fs.ReadFile(Expand(os.DirFS(dir)), "job.zip/job/matrix/matrix")
	if err != nil || string(b) != job["job/matrix/matrix"] {
		t.Errorf("job.zip/job/matrix/matrix = %q, %v; want the matrix", b, err)
	}
}

func TestACompressedFileThatHoldsNoTarIsNoArchive(t *testing.T) {
	if _, err := Open(gz(t, []byte("(kicad_pcb (version 20240108))"))); !errors.Is(err, ErrNotArchive) {
		t.Errorf("Open(gzip of one file) = %v, want ErrNotArchive", err)
	}
	if _, err := Open([]byte("(kicad_pcb)")); !errors.Is(err, ErrNotArchive) {
		t.Errorf("Open(text) = %v, want ErrNotArchive", err)
	}
}

// withBounds lowers the bounds for one test.
func withBounds(t *testing.T, expanded, member int64, count int) {
	t.Helper()
	e, m, c := maxExpanded, maxMember, maxMembers
	maxExpanded, maxMember, maxMembers = expanded, member, count
	t.Cleanup(func() { maxExpanded, maxMember, maxMembers = e, m, c })
}

// zeros writes a gzipped tar holding one member of n zero bytes without holding the member, so a
// test can build a bomb whose member is far larger than what it allocates.
func zeros(t *testing.T, n int64) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{Name: "job/steps/pcb/layers/top/features", Mode: 0o644, Size: n, Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	chunk := make([]byte, 1<<20)
	for left := n; left > 0; left -= int64(len(chunk)) {
		if left < int64(len(chunk)) {
			chunk = chunk[:left]
		}
		if _, err := tw.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestABombStopsAtItsBoundAndHoldsNoMoreThanTheBound(t *testing.T) {
	const member = 256 << 20 // what the bomb's one member expands to
	const bound = 4 << 20
	withBounds(t, 64<<20, bound, MaxMembers)
	bomb := zeros(t, member)
	if len(bomb) > member/100 {
		t.Fatalf("the bomb is %d bytes, too large to be one", len(bomb))
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := Open(bomb)
	runtime.ReadMemStats(&after)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Open(bomb) = %v, want ErrTooLarge", err)
	}
	// It allocates the bound and the reader's buffers, never the member. 4x the bound is room for
	// io.ReadAll's doubling.
	if grew := after.TotalAlloc - before.TotalAlloc; grew > 4*bound+(8<<20) {
		t.Errorf("decoding the bomb allocated %d bytes, against a %d-byte bound and a %d-byte member", grew, bound, member)
	}
}

func TestTheWholeArchiveIsBoundedAsWellAsEachMember(t *testing.T) {
	withBounds(t, 30, 1<<20, MaxMembers)
	if _, err := Open(tarOf(t, job)); !errors.Is(err, ErrTooLarge) {
		t.Errorf("tar past the total bound = %v, want ErrTooLarge", err)
	}
	if _, err := Open(zipOf(t, job, 0)); !errors.Is(err, ErrTooLarge) {
		t.Errorf("zip past the total bound = %v, want ErrTooLarge", err)
	}
}

func TestTooManyMembersFailsInEveryKind(t *testing.T) {
	withBounds(t, MaxExpandedBytes, MaxMemberBytes, 2)
	if _, err := Open(gz(t, tarOf(t, job))); !errors.Is(err, ErrTooMany) {
		t.Errorf("tgz with 3 members against 2 = %v, want ErrTooMany", err)
	}
	if _, err := Open(zipOf(t, job, 0)); !errors.Is(err, ErrTooMany) {
		t.Errorf("zip with 3 members against 2 = %v, want ErrTooMany", err)
	}
}

// TestAMemberIsKeptInsideTheArchive keeps a member named ../x inside the archive, and keeps an
// AppleDouble shadow out of it. The cases moved here from readers/odb with the decoder (agni issue
// 1591).
func TestAMemberIsKeptInsideTheArchive(t *testing.T) {
	for _, c := range []struct {
		in   string
		want string
		ok   bool
	}{
		{"job/matrix/matrix", "job/matrix/matrix", true},
		{"../../etc/passwd", "etc/passwd", true},
		{"/abs/matrix/matrix", "abs/matrix/matrix", true},
		{"job/._matrix", "", false},
		{"__MACOSX/job/matrix/matrix", "", false},
	} {
		got, ok := member(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("member(%q) = %q, %v, want %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
	fsys, err := Open(tarOf(t, map[string]string{"../../escape": "x"}))
	if err != nil {
		t.Fatal(err)
	}
	if b, err := fs.ReadFile(fsys, "escape"); err != nil || string(b) != "x" {
		t.Errorf("a tar's ../../escape reads %q, %v at its top, want x", b, err)
	}
}

func TestAnEncryptedZipSaysItIsEncrypted(t *testing.T) {
	if _, err := Open(zipOf(t, job, 0x1)); !errors.Is(err, ErrEncrypted) {
		t.Errorf("Open(encrypted zip) = %v, want ErrEncrypted", err)
	}
}

func TestAnArchiveReadsAsAFolderAndNests(t *testing.T) {
	outer := map[string]string{"designs/readme.txt": "hi"}
	outer["designs/job.tgz"] = string(gz(t, tarOf(t, job)))
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "drop.zip"), zipOf(t, outer, 0), 0o600); err != nil {
		t.Fatal(err)
	}
	fsys := Expand(os.DirFS(dir))
	st, err := fs.Stat(fsys, "drop.zip/designs/job.tgz")
	if err != nil || !st.IsDir() {
		t.Fatalf("stat drop.zip/designs/job.tgz = %v, %v; want a directory", st, err)
	}
	des, err := fs.ReadDir(fsys, "drop.zip/designs")
	if err != nil {
		t.Fatal(err)
	}
	var listed []string
	for _, de := range des {
		listed = append(listed, de.Name()+map[bool]string{true: "/"}[de.IsDir()])
	}
	if strings.Join(listed, " ") != "job.tgz/ readme.txt" {
		t.Errorf("drop.zip/designs lists %v, want job.tgz/ and readme.txt", listed)
	}
	var walked []string
	if err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			walked = append(walked, p)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(walked, " "), "drop.zip/designs/job.tgz/job/steps/pcb/layers/top/features") {
		t.Errorf("a walk reached %v, not the nested job's features", walked)
	}
}

func TestAHostPathRunsThroughAnArchive(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "boards", "job.tar.xz")
	if err := os.MkdirAll(filepath.Dir(archive), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, xzOf(t, tarOf(t, job)), 0o600); err != nil {
		t.Fatal(err)
	}
	matrix := filepath.Join(archive, "job", "matrix", "matrix")
	b, err := OSReadFile(matrix)
	if err != nil || string(b) != job["job/matrix/matrix"] {
		t.Fatalf("OSReadFile(%s) = %q, %v", matrix, b, err)
	}
	if got := Container(matrix); got != archive {
		t.Errorf("Container = %q, want %q", got, archive)
	}
	if got := Container(archive); got != "" {
		t.Errorf("Container(the archive itself) = %q, want none", got)
	}
	if st, err := OSStat(archive); err != nil || !st.IsDir() {
		t.Errorf("OSStat(archive) = %v, %v; want a directory", st, err)
	}
	des, err := OSReadDir(filepath.Dir(archive))
	if err != nil || len(des) != 1 || !des[0].IsDir() {
		t.Errorf("OSReadDir(boards) = %v, %v; want job.tar.xz as a directory", des, err)
	}
	if _, err := OSReadFile(filepath.Join(archive, "job", "nothing")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a missing member = %v, want fs.ErrNotExist", err)
	}
	// A changed archive is read again.
	changed := map[string]string{"job/matrix/matrix": "STEP {\nNAME=OTHER\n}\n"}
	if err := os.WriteFile(archive, xzOf(t, tarOf(t, changed)), 0o600); err != nil {
		t.Fatal(err)
	}
	if b, _ := OSReadFile(matrix); string(b) != changed["job/matrix/matrix"] {
		t.Errorf("after the archive changed, the matrix reads %q", b)
	}
}
