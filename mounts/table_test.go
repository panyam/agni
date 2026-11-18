package mounts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const commit = "0123456789abcdef0123456789abcdef01234567"

func TestATableSeesARecordWrittenAfterItStarted(t *testing.T) {
	records, uploads := t.TempDir(), t.TempDir()
	table, err := NewTable(Mount{Name: "boards", Root: t.TempDir()}).WithRecords(records, uploads)
	if err != nil {
		t.Fatal(err)
	}
	table.every = 0
	if n := len(table.List()); n != 1 {
		t.Fatalf("before any record the table serves %d mounts; want 1", n)
	}
	// Another process, agni serve, writes the record. The folder's mtime moves, and a later List
	// reads it. The sleep makes the mtime differ on a filesystem with coarse stamps.
	time.Sleep(10 * time.Millisecond)
	if err := WriteRecord(records, Record{Name: "jetson", Kind: KindGitHub, Source: "github://antmicro/jetson", Commit: commit}); err != nil {
		t.Fatal(err)
	}
	m, ok := table.Find("jetson")
	if !ok || !m.Runtime || m.Root != SnapshotView(uploads, "jetson", commit) || m.Kind != KindGitHub {
		t.Fatalf("Find(jetson) after the record = %+v, %v; want the runtime mount at its view", m, ok)
	}
	if err := RemoveRecord(records, "jetson"); err != nil {
		t.Fatal(err)
	}
	if _, ok := table.Find("jetson"); ok {
		t.Fatal("the table still serves a mount whose record is gone")
	}
}

func TestAFixedMountWinsOverARecordOfTheSameName(t *testing.T) {
	records := t.TempDir()
	if err := WriteRecord(records, Record{Name: "boards", Kind: KindGitHub, Commit: commit}); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	table, err := NewTable(Mount{Name: "boards", Root: root}).WithRecords(records, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ms := table.List()
	if len(ms) != 1 || ms[0].Root != root || ms[0].Runtime {
		t.Fatalf("List = %+v; want the one fixed mount", ms)
	}
}

func TestReadRecordsRefusesARecordThatWouldServeTheWrongFolder(t *testing.T) {
	for _, tc := range []struct {
		file, body, want string
	}{
		{"a.yaml", "name: b\ncommit: " + commit + "\n", `names the mount "b"`},
		{"a.yaml", "name: a\ncommit: ../../etc\n", "not a full hex commit"},
		{"A.yaml", "name: A\ncommit: " + commit + "\n", "may hold only"},
	} {
		dir := t.TempDir()
		if err := writeFile(dir, tc.file, tc.body); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadRecords(dir); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("ReadRecords over %s %q = %v; want an error naming %q", tc.file, tc.body, err, tc.want)
		}
	}
}

func writeFile(dir, name, body string) error {
	return os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600)
}
