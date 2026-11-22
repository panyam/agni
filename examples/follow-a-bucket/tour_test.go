package main

import (
	"context"
	"slices"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

func TestTheTourMountsABucketAndKeepsWhatItRead(t *testing.T) {
	tr, err := newTour(t.TempDir(), netlist)
	if err != nil {
		t.Fatal(err)
	}
	defer tr.close()
	m, files, err := tr.bucket(context.Background())
	if err != nil || m.Kind != webapi.MountKind_MOUNT_KIND_S3 || !slices.Equal(files, []string{"mixer.edn"}) {
		t.Fatalf("bucket = %+v, %v, %v; want an s3 mount holding mixer.edn", m, files, err)
	}
	if same, err := tr.pinned(m); err != nil || !same {
		t.Errorf("after the object was overwritten, the mount serves the netlist it read = %v, %v; want true", same, err)
	}
}
