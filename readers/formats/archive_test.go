package formats

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/panyam/agni/internal/archivefs"
	"github.com/ulikunitz/xz"
	"google.golang.org/protobuf/proto"
)

// archiveJob is the ODB++ job the archive tests pack, read from its folder as the oracle.
const archiveJobDir = "../odb/testdata/board"

// packJob packs a job folder one folder down, as producers wrap it, into kind.
func packJob(t *testing.T, dir, kind string) []byte {
	t.Helper()
	files := map[string][]byte{}
	var names []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		b, err := os.ReadFile(p)
		name := path.Join("board", filepath.ToSlash(rel))
		files[name], names = b, append(names, name)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if kind == "zip" {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		for _, n := range names {
			w, err := zw.Create(n)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write(files[n]); err != nil {
				t.Fatal(err)
			}
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	for _, n := range names {
		if err := tw.WriteHeader(&tar.Header{Name: n, Mode: 0o644, Size: int64(len(files[n])), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(files[n]); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	switch kind {
	case "tar.gz":
		w := gzip.NewWriter(&out)
		_, _ = w.Write(raw.Bytes())
		_ = w.Close()
	case "tar.xz":
		w, err := xz.NewWriter(&out)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(raw.Bytes())
		_ = w.Close()
	case "tar.zst":
		w, err := zstd.NewWriter(nil)
		if err != nil {
			t.Fatal(err)
		}
		return w.EncodeAll(raw.Bytes(), nil)
	default:
		t.Fatalf("no packer for %s", kind)
	}
	return out.Bytes()
}

// oneName gives every read the same provenance name, so two reads of one job compare as IR.
func oneName(string) string { return "board" }

// TestAJobInAnyArchiveReadsAsItsFolder reads the board job from its folder, then from each kind of
// archive, both by the archive's own path and by a path through the archive to its matrix file. Each
// read must equal the folder's (agni issue 1591).
func TestAJobInAnyArchiveReadsAsItsFolder(t *testing.T) {
	want := &Loader{SourceName: oneName}
	wantD, err := want.ReadDesign(filepath.Join(archiveJobDir, "matrix", "matrix"))
	if err != nil {
		t.Fatal(err)
	}
	wantB, err := want.BoardGeometry(filepath.Join(archiveJobDir, "matrix", "matrix"))
	if err != nil {
		t.Fatal(err)
	}
	if len(wantD.GetComponents()) == 0 || len(wantD.GetNets()) == 0 || len(wantB.GetLayers()) == 0 {
		t.Fatalf("the board job reads %d components, %d nets and %d layers", len(wantD.GetComponents()), len(wantD.GetNets()), len(wantB.GetLayers()))
	}
	archives := map[string][]byte{}
	for _, kind := range []string{"zip", "tar.gz", "tar.xz", "tar.zst"} {
		archives["board.odb."+kind] = packJob(t, archiveJobDir, kind)
	}
	for _, f := range sevenZipJobs {
		b, err := os.ReadFile(filepath.Join("testdata", f))
		if err != nil {
			t.Fatal(err)
		}
		archives[f] = b
	}
	dir := t.TempDir()
	for name, b := range archives {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, b, 0o600); err != nil {
			t.Fatal(err)
		}
		for _, entry := range []string{p, filepath.Join(p, "board", "matrix", "matrix")} {
			l := &Loader{SourceName: oneName}
			d, err := l.ReadDesign(entry)
			if err != nil {
				t.Errorf("%s: %v", entry, err)
				continue
			}
			b, err := l.BoardGeometry(entry)
			if err != nil {
				t.Errorf("%s board: %v", entry, err)
				continue
			}
			if !proto.Equal(d, wantD) {
				t.Errorf("%s reads %d components and %d nets, differently from the folder's %d and %d", entry, len(d.GetComponents()), len(d.GetNets()), len(wantD.GetComponents()), len(wantD.GetNets()))
			}
			if !proto.Equal(b, wantB) {
				t.Errorf("%s reads %d layers and %d placements, differently from the folder's %d and %d", entry, len(b.GetLayers()), len(b.GetPlacements()), len(wantB.GetLayers()), len(wantB.GetPlacements()))
			}
		}
	}
}

// sevenZipJobs are the board job packed by 7-Zip itself, solid, so the decoder is held to the tool's
// own output rather than to a writer of ours (testdata/README.md says how they were made).
var sevenZipJobs = []string{"board.odb.7z"}

// TestAnEncryptedArchiveSaysItIsEncrypted reads a 7z that 7-Zip encrypted, headers and all.
func TestAnEncryptedArchiveSaysItIsEncrypted(t *testing.T) {
	l := &Loader{}
	_, err := l.ReadDesign(filepath.Join("testdata", "board-encrypted.odb.7z"))
	if !errors.Is(err, archivefs.ErrEncrypted) {
		t.Errorf("ReadDesign(encrypted 7z) = %v, want archivefs.ErrEncrypted", err)
	}
}

// TestAReadThroughAnArchiveIsStampedByTheArchive records the archive file for a read through it, so
// a cache sees the read go stale when the archive changes.
func TestAReadThroughAnArchiveIsStampedByTheArchive(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "board.odb.tar.xz")
	if err := os.WriteFile(p, packJob(t, archiveJobDir, "tar.xz"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []string{p, filepath.Join(p, "board", "matrix", "matrix")} {
		touched := NewTouched()
		l := &Loader{Touched: touched}
		if _, err := l.ReadDesign(entry); err != nil {
			t.Fatal(err)
		}
		if !touched.Unchanged() {
			t.Fatalf("%s: the read is stale before anything changed", entry)
		}
		if err := os.WriteFile(p, packJob(t, archiveJobDir, "tar.gz"), 0o600); err != nil {
			t.Fatal(err)
		}
		if touched.Unchanged() {
			t.Errorf("%s: the archive changed and the read still reads as current", entry)
		}
		if err := os.WriteFile(p, packJob(t, archiveJobDir, "tar.xz"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
