package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/panyam/goapplib/storage/connections/entratest"
	"github.com/panyam/goapplib/storage/drivers/msgraph/graphtest"
)

// jetsonSheets is the Jetson AGX Thor baseboard's schematic, rev 1.1.3, from agni-samples (make
// samples-revisions): every .kicad_sch file, keyed by name.
func jetsonSheets(t *testing.T) map[string][]byte {
	t.Helper()
	dir := filepath.Join("..", "..", "tools", "samples", "revisions", "jetson-agx-thor-baseboard-1.1.3")
	names, err := filepath.Glob(filepath.Join(dir, "*.kicad_sch"))
	if err != nil || len(names) < 10 {
		t.Fatalf("the Jetson 1.1.3 sheets are missing (make samples-revisions): %v, %d files", err, len(names))
	}
	out := map[string][]byte{}
	for _, n := range names {
		b, err := os.ReadFile(n) // #nosec G304 -- the samples corpus
		if err != nil {
			t.Fatal(err)
		}
		out[filepath.Base(n)] = b
	}
	return out
}

// fakeS3 starts gofakes3 with the bucket "designs" and returns its URL and a client to seed it.
func fakeS3(t *testing.T) (string, *awss3.Client) {
	t.Helper()
	srv := httptest.NewServer(gofakes3.New(s3mem.New()).Server())
	t.Cleanup(srv.Close)
	c := awss3.New(awss3.Options{
		Region: "us-east-1", BaseEndpoint: aws.String(srv.URL), UsePathStyle: true,
		Credentials: aws.AnonymousCredentials{},
	})
	if _, err := c.CreateBucket(context.Background(), &awss3.CreateBucketInput{Bucket: aws.String("designs")}); err != nil {
		t.Fatal(err)
	}
	return srv.URL, c
}

func putS3(t *testing.T, c *awss3.Client, key string, body []byte) {
	t.Helper()
	if _, err := c.PutObject(context.Background(), &awss3.PutObjectInput{Bucket: aws.String("designs"), Key: aws.String(key), Body: bytes.NewReader(body)}); err != nil {
		t.Fatal(err)
	}
}

// A design linked from an S3 bucket checks as the same sheets do in a folder, and a changed object
// moves it on the next sync (agni issue 1730). The sheets are the Jetson baseboard's.
func TestADesignLinkedFromAnS3BucketChecksAsItsFolder(t *testing.T) {
	sheets := jetsonSheets(t)
	endpoint, s3c := fakeS3(t)
	folder := t.TempDir()
	for name, b := range sheets {
		putS3(t, s3c, "jetson/"+name, b)
		if err := os.WriteFile(filepath.Join(folder, name), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("AGNI_TEST_AWS_SECRET", "fake-secret")
	dir := t.TempDir()
	url := serveViewer(t, viewerOpts{store: testStore(t, filepath.Join(dir, "store")), uploadsDir: filepath.Join(dir, "uploads"), s3Endpoint: endpoint})
	must := func(out string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return out
	}
	must(runCobra(t, connectionsCmd(), "add", "aws", "--aws", "--access-key-id", "AKIAFAKE", "--key", "env:AGNI_TEST_AWS_SECRET", "--region", "us-east-1", "--remote", url))
	out := must(runMounts(t, "add", "bb", "s3://designs/jetson", "--connection", "aws", "--remote", url))
	if !strings.Contains(out, "bb\ts3\t") {
		t.Fatalf("mounts add = %q; want an s3 mount", out)
	}
	must(runDesigns(t, url, "create", "bb"))
	out = must(runDesigns(t, url, "link", "bb", "schematic", "bb/jetson-agx-thor-baseboard.kicad_sch"))
	if !strings.Contains(out, "files") {
		t.Fatalf("designs link = %q; want the sheets read", out)
	}
	stored := findingKeys(t, "check", "designs/bb", "--remote", url)
	local := findingKeys(t, "check", filepath.Join(folder, "jetson-agx-thor-baseboard.kicad_sch"))
	if len(stored) == 0 || !slices.Equal(stored, local) {
		t.Fatalf("a check on the bucket-linked design gives %d findings, and the folder %d; want the same, and some", len(stored), len(local))
	}

	putS3(t, s3c, "jetson/power.kicad_sch", append(sheets["power.kicad_sch"], '\n'))
	// goapplib's bucket driver downloads every file again on a sync (panyam/goapplib#234), so the
	// count of files fetched isn't checked here.
	out = must(runDesigns(t, url, "sync", "bb"))
	if !strings.Contains(out, "designs/bb/artifacts/schematic\tchanged\t") {
		t.Errorf("designs sync after one object changed = %q; want the schematic changed", out)
	}
	if again := findingKeys(t, "check", "designs/bb", "--remote", url); !slices.Equal(again, local) {
		t.Errorf("after a newline was added to one sheet, the check gives %d findings; want the folder's %d", len(again), len(local))
	}
}

// A gs:// mount over Cloud Storage serves its files, and a design links from it (agni issue 1730).
func TestAGCSMountServesItsFiles(t *testing.T) {
	gcs, err := fakestorage.NewServerWithOptions(fakestorage.Options{Scheme: "http", Host: "127.0.0.1", Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(gcs.Stop)
	gcs.CreateBucketWithOpts(fakestorage.CreateBucketOpts{Name: "designs"})
	for name, body := range map[string]string{"hw/bb.kicad_sch": "(kicad_sch)", "hw/bb.kicad_pro": "{}"} {
		gcs.CreateObject(fakestorage.Object{ObjectAttrs: fakestorage.ObjectAttrs{BucketName: "designs", Name: name}, Content: []byte(body)})
	}
	dir := t.TempDir()
	url := serveViewer(t, viewerOpts{store: testStore(t, filepath.Join(dir, "store")), uploadsDir: filepath.Join(dir, "uploads"), gcsEndpoint: gcs.URL() + "/storage/v1/"})
	out, err := runMounts(t, "add", "boards", "gs://designs/hw", "--remote", url)
	if err != nil || !strings.Contains(out, "boards\tgcs\t") {
		t.Fatalf("mounts add of a gs:// source = %q, %v; want a gcs mount", out, err)
	}
	if _, err := runDesigns(t, url, "create", "bb"); err != nil {
		t.Fatal(err)
	}
	if out, err := runDesigns(t, url, "link", "bb", "schematic", "boards/bb.kicad_sch"); err != nil || !strings.Contains(out, "files") {
		t.Fatalf("designs link from the gcs mount = %q, %v", out, err)
	}
	if out, err := runMounts(t, "add", "pinned", "gs://designs/hw", "--ref", "main", "--remote", url); err == nil || !strings.Contains(err.Error(), "has no refs") {
		t.Errorf("a gs:// mount at a ref = %q, %v; want it refused", out, err)
	}
}

// A SharePoint mount follows its drive through Microsoft Graph's notifications (agni issue 1729).
// agni mounts webhook subscribes the drive, Graph's validation request gets its token back, a
// notification with the server's clientState syncs the linked design, and one with another
// clientState moves nothing.
func TestASharePointMountFollowsGraphNotifications(t *testing.T) {
	ets := entratest.New(t)
	ets.AddSecretApp("contoso", "agni-app", "app-secret")
	graph := graphtest.New(t)
	graph.AcceptToken(ets.ValidToken)
	drive := graph.AddSiteDrive("eng", "d1")
	drive.Put("hw/bb.kicad_sch", "(kicad_sch)")
	drive.Put("hw/bb.kicad_pro", "{}")
	t.Setenv("AGNI_TEST_ENTRA_SECRET", "app-secret")
	t.Setenv("AGNI_TEST_GRAPH_STATE", "graph-client-state")
	dir := t.TempDir()
	url := serveViewer(t, viewerOpts{store: testStore(t, filepath.Join(dir, "store")), uploadsDir: filepath.Join(dir, "uploads"),
		graphAPI: graph.BaseURL(), graphState: []byte("graph-client-state")})
	must := func(out string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return out
	}
	must(runCobra(t, connectionsCmd(), "add", "sp", "--entra-tenant", "contoso", "--entra-client", "agni-app", "--entra-auth", "secret",
		"--key", "env:AGNI_TEST_ENTRA_SECRET", "--base-url", ets.URL, "--remote", url))
	if out, err := runMounts(t, "add", "nosign", "msgraph://sites/eng/drives/d1/root/hw", "--remote", url); err == nil {
		t.Errorf("a msgraph mount with no connection = %q; want it refused", out)
	}
	out := must(runMounts(t, "add", "sp", "msgraph://sites/eng/drives/d1/root/hw", "--connection", "sp", "--remote", url))
	if !strings.Contains(out, "sp\tmsgraph\t") {
		t.Fatalf("mounts add = %q; want a msgraph mount", out)
	}
	must(runDesigns(t, url, "create", "bb"))
	must(runDesigns(t, url, "link", "bb", "schematic", "sp/bb.kicad_sch"))
	before := must(runDesigns(t, url, "artifacts", "bb"))

	// Graph checks a new subscription's URL with a validation request first.
	resp, err := http.Post(url+"/hooks/msgraph?validationToken=check-123", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	echo := new(bytes.Buffer)
	echo.ReadFrom(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || echo.String() != "check-123" || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/plain") {
		t.Fatalf("the validation request = %d %q %q; want 200 with the token as text/plain", resp.StatusCode, echo.String(), resp.Header.Get("Content-Type"))
	}

	out = must(runMounts(t, "webhook", "sp", "--url", url+"/hooks/msgraph", "--remote", url))
	if !strings.Contains(out, "made Graph subscription") {
		t.Fatalf("mounts webhook on a SharePoint mount = %q; want a Graph subscription made", out)
	}
	subs := graph.Subscriptions()
	if len(subs) != 1 {
		t.Fatalf("Graph holds %d subscriptions, want 1", len(subs))
	}
	var subID string
	for id, s := range subs {
		subID = id
		if s.NotificationURL != url+"/hooks/msgraph" || s.ClientState != "graph-client-state" {
			t.Fatalf("the subscription sends to %q with clientState %q; want the server's hook and secret", s.NotificationURL, s.ClientState)
		}
	}
	if out := must(runMounts(t, "webhook", "sp", "--url", url+"/hooks/msgraph", "--remote", url)); !strings.Contains(out, "updated Graph subscription "+subID) || len(graph.Subscriptions()) != 1 {
		t.Errorf("a second mounts webhook = %q with %d subscriptions; want the one renewed", out, len(graph.Subscriptions()))
	}

	drive.Put("hw/bb.kicad_sch", "(kicad_sch (version 2))")
	forged := bytes.ReplaceAll(graph.NotificationBody(subID), []byte("graph-client-state"), []byte("someone-else"))
	if resp, err := http.Post(url+"/hooks/msgraph", "application/json", bytes.NewReader(forged)); err != nil || resp.StatusCode != http.StatusAccepted {
		t.Fatalf("a notification with another clientState = %v, %v; want 202", resp, err)
	}
	time.Sleep(200 * time.Millisecond)
	if now := must(runDesigns(t, url, "artifacts", "bb")); now != before {
		t.Fatalf("a notification with another clientState moved the design:\n%s\nwas\n%s", now, before)
	}
	if resp, err := http.Post(url+"/hooks/msgraph", "application/json", bytes.NewReader(graph.NotificationBody(subID))); err != nil || resp.StatusCode != http.StatusAccepted {
		t.Fatalf("Graph's notification = %v, %v; want 202", resp, err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		now := must(runDesigns(t, url, "artifacts", "bb"))
		if now != before {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the design never moved after Graph's notification:\n%s", now)
		}
		time.Sleep(50 * time.Millisecond)
	}

	must(runDesigns(t, url, "unlink", "bb", "schematic"))
	must(runMounts(t, "rm", "sp", "--remote", url))
	if n := len(graph.Subscriptions()); n != 0 {
		t.Errorf("after mounts rm, Graph holds %d subscriptions; want the mount's ended", n)
	}
}
