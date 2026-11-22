package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/panyam/agni/internal/storeurl"
)

// A design kept with --uploads s3:// lives in the bucket. A second server over the same store, with
// an empty local cache, opens it and checks it as the first did, since it rebuilds the view from
// blobs it fetches from the bucket (agni issue 1730). The design is the Jetson baseboard's 1.1.3
// schematic from agni-samples.
func TestUploadsInAnS3BucketOutliveTheLocalCopy(t *testing.T) {
	sheets := jetsonSheets(t)
	endpoint, s3c := fakeS3(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAFAKE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "fake-secret")
	t.Setenv("AWS_REGION", "us-east-1")
	folder := filepath.Join(t.TempDir(), "jetson")
	if err := os.MkdirAll(folder, 0o750); err != nil {
		t.Fatal(err)
	}
	// The corpus also carries sheets the root never references, and a put of those reads as more
	// designs. Copy the root and the sheets it names.
	const root = "jetson-agx-thor-baseboard.kicad_sch"
	for name, b := range sheets {
		if name != root && !strings.Contains(string(sheets[root]), `"Sheetfile" "`+name+`"`) {
			delete(sheets, name)
			continue
		}
		if err := os.WriteFile(filepath.Join(folder, name), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if len(sheets) < 10 {
		t.Fatalf("the root references %d sheets; want the baseboard's 14 and the root", len(sheets)-1)
	}
	dir := t.TempDir()
	serve := func(cache string) string {
		t.Helper()
		st, up, err := storeurl.Open(testStoreURL(t, filepath.Join(dir, "store")), "s3://designs/agni",
			storeurl.WithUploadsCache(cache), storeurl.WithEndpoints(endpoint, ""))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { st.Close() })
		if up != cache {
			t.Fatalf("the uploads folder is %q, want the cache %q", up, cache)
		}
		return serveViewer(t, viewerOpts{store: st, uploadsDir: up, s3Endpoint: endpoint, uploadMaxBytes: 1 << 26})
	}
	first := serve(filepath.Join(dir, "cache-1"))
	if out, err := runDesigns(t, first, "create", "bb"); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if out, err := runDesigns(t, first, "put", "bb", "schematic", folder); err != nil {
		t.Fatalf("designs put: %v\n%s", err, out)
	}
	want := findingKeys(t, "check", "designs/bb", "--remote", first)
	if len(want) == 0 {
		t.Fatal("the design has no findings to compare")
	}
	objs, err := s3c.ListObjectsV2(context.Background(), &awss3.ListObjectsV2Input{Bucket: aws.String("designs"), Prefix: aws.String("agni/")})
	if err != nil || len(objs.Contents) < len(sheets) {
		t.Fatalf("the bucket holds %d objects under agni/, %v; want every sheet's blob", len(objs.Contents), err)
	}

	// A second server, as on a fresh machine, with nothing in its cache.
	second := serve(filepath.Join(dir, "cache-2"))
	if got := findingKeys(t, "check", "designs/bb", "--remote", second); !slices.Equal(got, want) {
		t.Fatalf("a server with an empty cache checks designs/bb with %d findings; want the first's %d", len(got), len(want))
	}
	ents, err := os.ReadDir(filepath.Join(dir, "cache-2", "designs", "bb"))
	if err != nil || len(ents) == 0 {
		t.Errorf("the second server's cache holds no view of bb: %v", err)
	}
}

// --uploads refuses an s3:// URL with no bucket, and takes a cache folder as a path or a file:// URL.
func TestUploadsNamesABucket(t *testing.T) {
	dir := t.TempDir()
	store := testStoreURL(t, filepath.Join(dir, "store"))
	if _, _, err := storeurl.Open(store, "s3://"); err == nil || !strings.Contains(err.Error(), "names no bucket") {
		t.Errorf("--uploads s3:// = %v; want it refused for naming no bucket", err)
	}
	cache := filepath.Join(dir, "cache")
	st, up, err := storeurl.Open(store, "gs://exports/agni", storeurl.WithUploadsCache("file:///"+strings.TrimPrefix(filepath.ToSlash(cache), "/")))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if up != cache || st.UploadsRemote != "gs://exports/agni" {
		t.Errorf("--uploads gs:// = folder %q, remote %q; want %q and the bucket", up, st.UploadsRemote, cache)
	}
}
