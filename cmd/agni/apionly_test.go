package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// clearNamedWebDir removes the agni.yaml tier, so with noEnv a test starts from "nothing named a web
// dir".
func clearNamedWebDir(t *testing.T) {
	t.Helper()
	prev := envConfigWebDir
	envConfigWebDir = ""
	t.Cleanup(func() { envConfigWebDir = prev })
}

// An installed binary run outside a checkout has no ./web and nothing naming one. serve then runs
// the API alone rather than refusing to start (agni issue 735).
func TestServeWithNothingNamedAndNoWebDirIsAPIOnly(t *testing.T) {
	clearNamedWebDir(t)
	t.Chdir(t.TempDir())
	got, err := resolveServeAssets("", noEnv)
	if err != nil {
		t.Fatalf("serve refused to start with no web dir anywhere: %v", err)
	}
	if got.viewer {
		t.Errorf("reported a viewer to serve from %q, which does not exist", got.dir)
	}
}

// Absent is a choice and broken is a mistake. Every way of NAMING a web dir still fails loudly when
// the name is wrong, because the operator typed it and wants to hear about the typo.
func TestServeStillRefusesANamedWebDirThatIsWrong(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	for _, tc := range []struct {
		name   string
		flag   string
		file   string
		getenv func(string) string
	}{
		{name: "flag", flag: missing, getenv: noEnv},
		{name: "agni.yaml", file: missing, getenv: noEnv},
		{name: "environment", getenv: func(k string) string {
			if k == envWebDir {
				return missing
			}
			return ""
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearNamedWebDir(t)
			envConfigWebDir = tc.file
			t.Chdir(t.TempDir())
			if _, err := resolveServeAssets(tc.flag, tc.getenv); err == nil {
				t.Errorf("a web dir named by the %s that does not exist was accepted", tc.name)
			}
		})
	}
}

// A ./web that exists without its bundle is a checkout that forgot `make ui`, which is a mistake to
// report rather than a reason to drop the viewer silently.
func TestServeStillRefusesAnIncompleteCheckout(t *testing.T) {
	clearNamedWebDir(t)
	root := t.TempDir()
	touch(t, filepath.Join(root, defaultWebDir, "templates", "ViewerPage.html"))
	t.Chdir(root)
	_, err := resolveServeAssets("", noEnv)
	if err == nil || !strings.Contains(err.Error(), "pnpm build") {
		t.Errorf("a ./web with no bundle should say to build it, got %v", err)
	}
}

// The workbench is optional. A viewer without it still serves, and the reason it is off is kept for
// /datasheets/ to answer with.
func TestServeWithoutTheWorkbenchStillServesTheViewer(t *testing.T) {
	clearNamedWebDir(t)
	dir := webAssetsDir(t) // carries no pdf.worker.js
	got, err := resolveServeAssets(dir, noEnv)
	if err != nil {
		t.Fatalf("a complete viewer without the workbench was refused: %v", err)
	}
	if !got.viewer {
		t.Error("the viewer group is present and was not served")
	}
	if got.datasheetsErr == nil || !strings.Contains(got.datasheetsErr.Error(), "pdf.worker.js") {
		t.Errorf("the missing workbench file should be named, got %v", got.datasheetsErr)
	}
}

// `open` exists to show one design's page, so it keeps refusing without the viewer, before it binds.
func TestOpenStillRequiresTheViewer(t *testing.T) {
	clearNamedWebDir(t)
	t.Setenv(envWebDir, "")
	design, err := filepath.Abs("../../examples/tutorial-project/designs/gateway/gateway.edn")
	if err != nil {
		t.Fatal(err)
	}
	// rootCmd loads the user's own ~/.config/agni/agni.yaml, whose web_dir would satisfy open and
	// start a real server, so the home config is pointed at an empty tree.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	t.Chdir(t.TempDir())
	cmd := rootCmd()
	cmd.SetArgs([]string{"open", design, "--addr", "127.0.0.1:0"})
	cmd.SetErr(new(strings.Builder))
	// A regression here does not return an error, it starts a server that never stops, so the check
	// is bounded rather than left to the suite's timeout.
	done := make(chan error, 1)
	go func() { done <- cmd.Execute() }()
	select {
	case err = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("open started serving with no viewer assets instead of refusing")
	}
	if err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("open should refuse with no viewer assets, got %v", err)
	}
}

func TestAPIOnlyRootSaysWhatIsServed(t *testing.T) {
	h := apiOnlyHandler([]string{"/agni.v1.webapi.QueryService/", "/agni.v1.webapi.CheckService/"})
	for _, tc := range []struct {
		path string
		code int
	}{{"/", http.StatusOK}, {"/designs/tut/board.edn/view", http.StatusNotFound}} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", tc.path, nil))
		if rec.Code != tc.code {
			t.Errorf("GET %s = %d, want %d", tc.path, rec.Code, tc.code)
		}
		body := rec.Body.String()
		for _, want := range []string{"WITHOUT the viewer", "agni.v1.webapi.QueryService", "--web-dir", envWebDir, "make ui"} {
			if !strings.Contains(body, want) {
				t.Errorf("GET %s does not mention %q:\n%s", tc.path, want, body)
			}
		}
	}
}

// With the workbench off, /datasheets/ names why, and the rest of the viewer is unaffected.
func TestPagesWithoutTheWorkbench(t *testing.T) {
	mux := http.NewServeMux()
	why := errors.New(`"web" has no static/pdf.worker.js, so the datasheets workbench is off`)
	registerPages(newPageApp(filepath.Join("..", "..", "web"), &serveApp{}), mux, why)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/datasheets/", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "pdf.worker.js") {
		t.Errorf("GET /datasheets/ = %d %q, want a 404 naming the missing file", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("GET / = %d with the workbench off, want the landing page", rec.Code)
	}
}
