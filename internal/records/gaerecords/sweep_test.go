package gaerecords

import (
	"bytes"
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"cloud.google.com/go/datastore"
)

// Sweep removes parts no head names once they are past grace: parts of a version the head moved off,
// and parts under a head that's gone. It leaves the head's own parts and a part still in its grace
// (agni issue 1823).
func TestSweepRemovesOnlyStrandedParts(t *testing.T) {
	if os.Getenv("DATASTORE_EMULATOR_HOST") == "" {
		t.Skip("DATASTORE_EMULATOR_HOST names no emulator; make test-stores runs this")
	}
	// recordstest imports this package, so the test makes its own client.
	client, err := datastore.NewClient(context.Background(), "agni-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	ns := "t_sweep_" + strconv.FormatInt(time.Now().UnixNano(), 16)
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	s := New(client, ns)
	s.now = func() time.Time { return now }
	ctx := context.Background()
	big := bytes.Repeat([]byte("x"), 2*partSize+5)
	if err := s.Put(ctx, "r/big.json", big); err != nil {
		t.Fatal(err)
	}
	strand := func(head, at string, n int) {
		t.Helper()
		keys := s.partKeys(s.key(head), at, n)
		ents := make([]*partEntity, n)
		for i := range ents {
			ents[i] = &partEntity{Body: []byte("stale")}
		}
		if _, err := client.PutMulti(ctx, keys, ents); err != nil {
			t.Fatal(err)
		}
	}
	old := strconv.FormatInt(now.Add(-2*time.Hour).UnixNano(), 16) + "-dead"
	fresh := strconv.FormatInt(now.Add(-time.Minute).UnixNano(), 16) + "-busy"
	strand("r/big.json", old, 2)  // a version the head never named
	strand("r/gone.json", old, 2) // under a head that doesn't exist
	strand("r/big.json", fresh, 1)
	n, err := s.Sweep(ctx, time.Hour)
	if err != nil || n != 4 {
		t.Fatalf("Sweep = %d, %v; want the 4 stranded parts past grace", n, err)
	}
	if got, err := s.Get(ctx, "r/big.json"); err != nil || !bytes.Equal(got, big) {
		t.Fatalf("after Sweep, the live record reads %d bytes, %v; want its %d", len(got), err, len(big))
	}
	if n, err := s.Sweep(ctx, time.Hour); err != nil || n != 0 {
		t.Fatalf("a second Sweep = %d, %v; want nothing left past grace", n, err)
	}
	if n, err := s.Sweep(ctx, 0); err != nil || n != 1 {
		t.Fatalf("Sweep with no grace = %d, %v; want the one fresh part", n, err)
	}
}
