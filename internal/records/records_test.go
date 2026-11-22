package records_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/panyam/agni/internal/records"
	"github.com/panyam/agni/internal/records/recordstest"
)

func TestDirContract(t *testing.T) {
	recordstest.Run(t, func(t *testing.T) records.Store {
		d, err := records.NewDir(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		return d
	})
}

// A write or delete waits out a reader that has the record open. Windows refuses to rename over or
// remove an open file, and Dir's reads take no lock, so without a retry a Put racing a Get failed
// there with "Access is denied". Elsewhere the rename goes through at once.
func TestDirWritesPastAReaderHoldingTheFile(t *testing.T) {
	ctx := context.Background()
	d, err := records.NewDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Put(ctx, "c/x.json", []byte("one")); err != nil {
		t.Fatal(err)
	}
	for _, op := range []struct {
		name string
		do   func() error
	}{
		{"Put", func() error { return d.Put(ctx, "c/x.json", []byte("two")) }},
		{"Delete", func() error { return d.Delete(ctx, "c/x.json") }},
	} {
		f, err := os.Open(filepath.Join(d.Root(), "c", "x.json"))
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- op.do() }()
		time.Sleep(50 * time.Millisecond)
		f.Close()
		if err := <-done; err != nil {
			t.Fatalf("%s while a reader held the file: %v", op.name, err)
		}
	}
}
