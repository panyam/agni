package main

import (
	"bytes"
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/panyam/goapplib/storage/connections"

	"github.com/panyam/agni/internal/records"
	"github.com/panyam/agni/mountset"
	"github.com/panyam/agni/service"
)

// tour holds a mount table the way `agni serve --store --uploads` builds one (cmd/agni/serve.go),
// over gofakes3, an S3 in the process.
type tour struct {
	table *mountset.Table
	s3    *awss3.Client
	// netlist is the bundled board the bucket and the drive hold.
	netlist []byte
	closers []func()
}

func newTour(root, netlistPath string) (*tour, error) {
	netlist, err := os.ReadFile(netlistPath) // #nosec G304 -- the bundled fixture
	if err != nil {
		return nil, err
	}
	t := &tour{netlist: netlist}
	srv := httptest.NewServer(gofakes3.New(s3mem.New()).Server())
	t.closers = append(t.closers, srv.Close)
	t.s3 = awss3.New(awss3.Options{Region: "us-east-1", BaseEndpoint: aws.String(srv.URL), UsePathStyle: true, Credentials: aws.AnonymousCredentials{}})
	ctx := context.Background()
	if _, err := t.s3.CreateBucket(ctx, &awss3.CreateBucketInput{Bucket: aws.String("designs")}); err != nil {
		return nil, err
	}
	if err := t.putObject("lab/mixer.edn", netlist); err != nil {
		return nil, err
	}

	os.Setenv("AGNI_EXAMPLE_AWS_SECRET", "fake")

	subs, err := records.NewDir(filepath.Join(root, "subscriptions"))
	if err != nil {
		return nil, err
	}
	t.table, err = mountset.Open(mountset.Config{Uploads: filepath.Join(root, "uploads"), Records: filepath.Join(root, "mounts"),
		Connections: filepath.Join(root, "connections"), S3Endpoint: srv.URL, Subscriptions: subs})
	if err != nil {
		return nil, err
	}
	for _, c := range []connections.Connection{
		{ID: "aws", Kind: connections.KindAWS, Scope: connections.ScopeServer, AccessKeyID: "AKIAEXAMPLE", KeyRef: "env:AGNI_EXAMPLE_AWS_SECRET", Region: "us-east-1", Status: connections.StatusOK},
	} {
		if err := t.table.Connections().PutConnection(ctx, c); err != nil {
			return nil, err
		}
	}
	return t, nil
}

func (t *tour) close() {
	for i := len(t.closers) - 1; i >= 0; i-- {
		t.closers[i]()
	}
}

func (t *tour) putObject(key string, body []byte) error {
	_, err := t.s3.PutObject(context.Background(), &awss3.PutObjectInput{Bucket: aws.String("designs"), Key: aws.String(key), Body: bytes.NewReader(body)})
	return err
}

// bucket mounts s3://designs/lab and returns the mount and the files in its root.
func (t *tour) bucket(ctx context.Context) (service.MountInfo, []string, error) {
	m, err := t.table.CreateMount(ctx, "lab", "s3://designs/lab", "", "aws")
	if err != nil {
		return m, nil, err
	}
	ents, err := os.ReadDir(m.Root)
	if err != nil {
		return m, nil, err
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return m, names, nil
}

// pinned overwrites the netlist in the bucket and reads it back through the mount, which still
// serves the version it pinned.
func (t *tour) pinned(m service.MountInfo) (same bool, err error) {
	if err := t.putObject("lab/mixer.edn", []byte("(edif overwritten)")); err != nil {
		return false, err
	}
	b, err := os.ReadFile(filepath.Join(m.Root, "mixer.edn")) // #nosec G304 -- the mount's own root
	if err != nil {
		return false, err
	}
	return bytes.Equal(b, t.netlist), nil
}
