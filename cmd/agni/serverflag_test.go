package main

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseServerSpecShapes(t *testing.T) {
	for _, tc := range []struct {
		name, in  string
		self      bool
		url       string
		wantError bool
	}{
		{name: "empty mints nothing", in: ""},
		{name: "self", in: "self", self: true},
		{name: "url", in: "http://localhost:8080", url: "http://localhost:8080"},
		{name: "url trailing slash trimmed", in: "http://localhost:8080/", url: "http://localhost:8080"},
		{name: "bare host is not a url", in: "localhost:8080", wantError: true},
		{name: "self with a non-numeric port", in: "self:web", wantError: true},
		{name: "self with an out-of-range port", in: "self:99999", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseServerSpec(tc.in)
			if tc.wantError {
				if err == nil {
					t.Fatalf("parseServerSpec(%q) = %+v, want an error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseServerSpec(%q): %v", tc.in, err)
			}
			defer closeSpec(got)
			if got.self != tc.self {
				t.Errorf("self = %v, want %v", got.self, tc.self)
			}
			if got.url != tc.url {
				t.Errorf("url = %q, want %q", got.url, tc.url)
			}
			if got.self && got.ln == nil {
				t.Error("self parsed without binding a listener, so nothing holds the port")
			}
		})
	}
}

// TestSelfHoldsItsPort is what makes "fails on a taken port" exact rather than likely: the port is
// bound while the flag is parsed, so nothing can take it between the check and the serve. A probe
// that bound and closed would pass this test and still lose the race in the field.
func TestSelfHoldsItsPort(t *testing.T) {
	spec, err := parseServerSpec("self")
	if err != nil {
		t.Fatal(err)
	}
	defer closeSpec(spec)

	addr := spec.ln.Addr().String()
	second, err := net.Listen("tcp", addr)
	if err == nil {
		second.Close()
		t.Fatalf("%s was still bindable, so the spec is not holding it", addr)
	}
	if !strings.HasPrefix(spec.base(), "http://") {
		t.Errorf("base() = %q, want an http address", spec.base())
	}
}

// TestSelfPortInUseFailsBeforeAnyWork covers the choice to fail rather than fall back to a free port.
// An explicit port is an assertion about where the links will point, so binding elsewhere would mint
// links resolving on whatever else is listening.
func TestSelfPortInUseFailsBeforeAnyWork(t *testing.T) {
	held, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	_, port, _ := net.SplitHostPort(held.Addr().String())

	_, err = parseServerSpec("self:" + port)
	if err == nil {
		t.Fatal("a port already in use parsed cleanly")
	}
	if !strings.Contains(err.Error(), "in use") {
		t.Errorf("error does not say the port is in use: %v", err)
	}
}

// TestSelfLinksAMintedMount is the ticket's own complaint, as a test.
//
// A mount minted for one run means nothing on a server that was not started with it, so linkTarget
// refuses to build a link from one. Under `--server self` the premise is gone, because this process
// serves the table it just minted. Red-checks by restoring the unconditional Declared check.
func TestSelfLinksAMintedMount(t *testing.T) {
	ws, err := newCLIWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	const uri = "mount://minted/board.edn"

	if got, why := linkTarget(ws, uri, false); got != "" || why == "" {
		t.Errorf("without a server: got %q (%q), want no link and a reason", got, why)
	}
	got, why := linkTarget(ws, uri, true)
	if got == "" {
		t.Fatalf("under --server self a minted mount still refused to link: %s", why)
	}
	if want := "minted/board.edn"; got != want {
		t.Errorf("link target = %q, want %q", got, want)
	}
}

// The alias is GONE, so a command naming it fails rather than working quietly. That is the whole
// point of removing it: a working alias made stale instructions indistinguishable from current ones
// across five docsite pages and `agni open`'s own printed command (agni issue 636).
func TestURLBaseIsNoLongerAFlag(t *testing.T) {
	root := rootCmd()
	for _, name := range []string{"check", "review", "trace"} {
		sub, _, err := root.Find([]string{name})
		if err != nil {
			t.Fatalf("no %s command: %v", name, err)
		}
		if f := sub.Flags().Lookup("url-base"); f != nil {
			t.Errorf("%s still defines --url-base (hidden=%v)", name, f.Hidden)
		}
	}
}

func closeSpec(s serverSpec) {
	if s.ln != nil {
		s.ln.Close()
	}
}

// webAssetsDir builds a directory that satisfies checkWebAssets, so a test can distinguish "the
// assets are missing" from "this test forgot to create them".
func webAssetsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Every file checkWebAssets looks for. Kept as the full list rather than the two obvious ones,
	// because a fixture that satisfies only part of the check makes the positive control below assert
	// nothing: it would fail for a missing fixture file and read as the guard working.
	for _, f := range []string{
		"templates/ViewerPage.html", "static/app.js",
		"templates/DatasheetsPage.html", "static/datasheets.js",
		"templates/BrowsePage.html", "static/browse.js",
	} {
		p := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestSelfMissingWebDirFailsBeforeAnyWork is the sibling of TestSelfPortInUseFailsBeforeAnyWork, for
// self's other precondition. Both have to hold before the wrapped command writes an artifact, because
// self mints links into that artifact and then serves them.
func TestSelfMissingWebDirFailsBeforeAnyWork(t *testing.T) {
	t.Chdir(t.TempDir()) // no ./web here, which is the whole point
	t.Setenv(envWebDir, "")
	_, err := resolveServer("self")
	if err == nil {
		t.Fatal("--server self resolved cleanly with no web assets, so the run would write links to a server that never binds")
	}
	for _, want := range []string{"cannot serve", "web-dir", envWebDir, "agni.yaml"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q, so the reader cannot act on it: %v", want, err)
		}
	}
}

// TestSelfReleasesItsPortWhenTheAssetCheckFails: parseServerSpec binds before resolveServer validates,
// so a failed validation must hand the port back or the next attempt fails for the wrong reason.
func TestSelfReleasesItsPortWhenTheAssetCheckFails(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv(envWebDir, "")
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(probe.Addr().String())
	probe.Close()

	if _, err := resolveServer("self:" + port); err == nil {
		t.Fatal("expected the asset check to fail")
	}
	again, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", port))
	if err != nil {
		t.Fatalf("port %s is still held after a failed --server self, so the listener leaked: %v", port, err)
	}
	again.Close()
}

// TestSelfSucceedsWhenTheAssetsAreThere is the positive control: without it the two tests above pass
// for a spec that can never resolve, and would keep passing if resolveServer simply always failed.
func TestSelfSucceedsWhenTheAssetsAreThere(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv(envWebDir, webAssetsDir(t))
	spec, err := resolveServer("self")
	if err != nil {
		t.Fatalf("--server self refused a directory that has the assets: %v", err)
	}
	defer spec.Close()
	if !spec.self || spec.ln == nil {
		t.Errorf("spec = %+v, want self holding a listener", spec)
	}
}

// TestResolveWebAssetsStillGuardsTheServePath: serve and open have no --server flag and reach the
// same check through runViewer, so the extraction must not have moved the guard off their path.
func TestResolveWebAssetsStillGuardsTheServePath(t *testing.T) {
	good := webAssetsDir(t)
	if _, _, err := resolveWebAssets(good, func(string) string { return "" }); err != nil {
		t.Errorf("a complete asset dir was rejected: %v", err)
	}
	bare := t.TempDir()
	if _, _, err := resolveWebAssets(bare, func(string) string { return "" }); err == nil {
		t.Error("a directory with no viewer assets was accepted")
	}
	if _, _, err := resolveWebAssets(filepath.Join(bare, "nope"), func(string) string { return "" }); err == nil {
		t.Error("a missing directory was accepted")
	}
}
