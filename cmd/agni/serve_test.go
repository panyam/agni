package main

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckWebAssets asserts serve validates its assets dir upfront: a folder with no viewer
// template points at --mount (the misdirected-arg case), a template without the bundle hints to
// build it, and the repo's own web/ passes.
func TestCheckWebAssets(t *testing.T) {
	dir := t.TempDir()

	// No templates: the "you passed a design folder" case.
	if err := checkWebAssets(dir); err == nil || !strings.Contains(err.Error(), "--mount") {
		t.Errorf("empty dir should error mentioning --mount, got %v", err)
	}

	// Template present but no built bundle: hint to build.
	touch(t, filepath.Join(dir, "templates", "ViewerPage.html"))
	if err := checkWebAssets(dir); err == nil || !strings.Contains(err.Error(), "pnpm build") {
		t.Errorf("missing bundle should hint pnpm build, got %v", err)
	}

	// Viewer template + bundle present, but the datasheets workbench page is missing (WS13-006).
	touch(t, filepath.Join(dir, "static", "app.js"))
	if err := checkWebAssets(dir); err == nil || !strings.Contains(err.Error(), "DatasheetsPage.html") {
		t.Errorf("missing datasheets page should name DatasheetsPage.html, got %v", err)
	}

	// Datasheets template present but its bundle missing: hint to build.
	touch(t, filepath.Join(dir, "templates", "DatasheetsPage.html"))
	if err := checkWebAssets(dir); err == nil || !strings.Contains(err.Error(), "datasheets.js") {
		t.Errorf("missing datasheets bundle should name datasheets.js, got %v", err)
	}

	// Datasheets complete, but the browse page is missing (WS9-049 phase 2).
	touch(t, filepath.Join(dir, "static", "datasheets.js"))
	if err := checkWebAssets(dir); err == nil || !strings.Contains(err.Error(), "BrowsePage.html") {
		t.Errorf("missing browse page should name BrowsePage.html, got %v", err)
	}

	// Browse template present but its bundle missing: hint to build.
	touch(t, filepath.Join(dir, "templates", "BrowsePage.html"))
	if err := checkWebAssets(dir); err == nil || !strings.Contains(err.Error(), "browse.js") {
		t.Errorf("missing browse bundle should name browse.js, got %v", err)
	}

	// All three templates and all three bundles present: valid.
	touch(t, filepath.Join(dir, "static", "browse.js"))
	if err := checkWebAssets(dir); err != nil {
		t.Errorf("a dir with every template and bundle should pass, got %v", err)
	}

	// The repo's web/ dir passes (asserts the marker paths match the real layout).
	if err := checkWebAssets("../../web"); err != nil {
		t.Errorf("repo web/ should pass checkWebAssets, got %v", err)
	}
}

// touch writes an empty file, creating parent directories.
func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestServeRejectsUnknownTheme pins the flag contract: an unknown --theme errors up front
// listing the valid palettes (it used to fall back to the default silently, unlike every
// other validated enum flag).
func TestServeRejectsUnknownTheme(t *testing.T) {
	cmd := rootCmd()
	cmd.SetArgs([]string{"serve", "--theme", "solarized", "--web-dir", "definitely-missing-dir"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), `unknown --theme "solarized"`) || !strings.Contains(err.Error(), "default") {
		t.Fatalf("want an unknown-theme error naming the valid palettes, got %v", err)
	}
}

// TestServeTakesNoPositional pins the compatibility break. The argument used to be the web-assets
// dir, every caller in the tree passed the literal default, and its practical function was to invite
// a DESIGN folder in the one position that would not accept one. Rejecting it is louder than
// accepting and ignoring it, which would have left the mistake silent.
func TestServeTakesNoPositional(t *testing.T) {
	cmd := rootCmd()
	cmd.SetArgs([]string{"serve", "some-folder-of-designs"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("serve must reject a positional argument")
	}
	if !strings.Contains(err.Error(), "some-folder-of-designs") {
		t.Errorf("the error should name what was rejected, got %v", err)
	}
}

// TestServeWebDirErrorSaysWhatItIsNot: the message is load-bearing. It exists because people pass a
// design folder here, so it has to name the flag's real subject and point at --mount for the thing
// they actually wanted.
func TestServeWebDirErrorSaysWhatItIsNot(t *testing.T) {
	dir := t.TempDir() // a real directory with none of the viewer's assets in it
	err := checkWebAssets(dir)
	if err == nil {
		t.Fatal("a directory with no ViewerPage.html is not a web-assets dir")
	}
	for _, want := range []string{"--web-dir", "--mount", "not a folder to browse"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q, got %v", want, err)
		}
	}
}

// TestHealthHandler asserts the probe answers 200 with a body, and that it is registered on the
// exact path rather than as a prefix — a "/healthz/" subtree would quietly swallow page routes.
func TestHealthHandler(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", healthHandler())
	// Stand-in for the page space that registerPages puts at "/", so a prefix-shadowing
	// regression shows up as this handler never being reached.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "page")
	})

	t.Run("200 with a body", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("got %d, want 200", rec.Code)
		}
		if strings.TrimSpace(rec.Body.String()) != "ok" {
			t.Fatalf("got body %q, want ok", rec.Body.String())
		}
	})

	t.Run("does not shadow the page space", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz/sub", nil))
		if rec.Body.String() != "page" {
			t.Fatalf("/healthz/sub reached the probe, so it is registered as a subtree: %q", rec.Body.String())
		}
	})
}

// The startup line is the only instruction most people get for reaching the server, so it has to
// name an address they can actually open. The bug this pins: printing the bind address verbatim
// turns the default ":8080" into "http://:8080/", which no browser resolves.
func TestServeURLs(t *testing.T) {
	fixedIPs := func() []string { return []string{"192.168.1.23"} }
	noIPs := func() []string { return nil }

	cases := []struct {
		name string
		addr string
		ips  func() []string
		want []string
	}{{
		name: "the default wildcard names localhost first, then the network",
		addr: ":8080",
		ips:  fixedIPs,
		want: []string{"http://localhost:8080/", "http://192.168.1.23:8080/"},
	}, {
		name: "0.0.0.0 is the same wildcard spelled out",
		addr: "0.0.0.0:8080",
		ips:  fixedIPs,
		want: []string{"http://localhost:8080/", "http://192.168.1.23:8080/"},
	}, {
		name: "the IPv6 wildcard too",
		addr: "[::]:8080",
		ips:  fixedIPs,
		want: []string{"http://localhost:8080/", "http://192.168.1.23:8080/"},
	}, {
		name: "a wildcard with no reachable interface still gives a usable URL",
		addr: ":8080",
		ips:  noIPs,
		want: []string{"http://localhost:8080/"},
	}, {
		name: "an explicit loopback bind is NOT advertised to the network",
		addr: "127.0.0.1:8080",
		ips:  fixedIPs,
		want: []string{"http://127.0.0.1:8080/"},
	}, {
		name: "an explicit host is left alone",
		addr: "example.internal:9000",
		ips:  fixedIPs,
		want: []string{"http://example.internal:9000/"},
	}, {
		name: "an IPv6 literal keeps its brackets, which separate host from port",
		addr: "[fd00::1]:8080",
		ips:  fixedIPs,
		want: []string{"http://[fd00::1]:8080/"},
	}, {
		name: "an unparseable address is echoed rather than guessed at",
		addr: "8080",
		ips:  fixedIPs,
		want: []string{"http://8080/"},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := serveURLs(tc.addr, tc.ips)
			if len(got) != len(tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %q, want %q", got, tc.want)
				}
			}
		})
	}
}

// lanIPs reads the real machine, so this asserts the property that holds everywhere rather than a
// specific address: whatever it returns must be usable in a URL, and must never be a loopback or a
// self-assigned address, since printing one of those as "on this network" would be a lie.
func TestLanIPsAreRoutable(t *testing.T) {
	for _, s := range lanIPs() {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("lanIPs returned %q, which is not an IP", s)
		}
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.To4() == nil {
			t.Fatalf("lanIPs returned %q, which is not a routable IPv4 address", s)
		}
	}
}

// TestServeNarratesOnlyTheEnvironment: a reader should be told once. applyEnvConfig names the
// agni.yaml it read and the serving line names the resolved directory, so a third line for that case
// is noise; the environment is the only provenance nothing else reports.
func TestServeNarratesOnlyTheEnvironment(t *testing.T) {
	t.Cleanup(func() { envConfigWebDir = "" })

	envConfigWebDir = "/from/file"
	_, source := resolveWebDir("", func(string) string { return "" })
	if source != "agni.yaml" {
		t.Fatalf("source = %q, want agni.yaml", source)
	}
	if source == envWebDir {
		t.Error("the agni.yaml case must not be narrated by serve")
	}

	envConfigWebDir = ""
	_, source = resolveWebDir("", func(k string) string {
		if k == envWebDir {
			return "/from/env"
		}
		return ""
	})
	if source != envWebDir {
		t.Errorf("source = %q, want the environment so serve narrates it", source)
	}
}
