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
	if err == nil || !strings.Contains(err.Error(), "is not a directory") || !strings.Contains(err.Error(), "releases") {
		t.Fatalf("open should refuse with no viewer assets and say where a build with one comes from, got %v", err)
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

// stubBundle stands in for the viewer an installed binary carries or downloads, and counts the calls.
func stubBundle(t *testing.T, dir string, err error) *int {
	t.Helper()
	calls := new(int)
	prev := viewerBundle
	viewerBundle = func() (string, string, error) {
		*calls++
		return dir, "embedded", err
	}
	t.Cleanup(func() { viewerBundle = prev })
	return calls
}

// An installed binary outside a checkout serves the viewer it carries rather than the API alone.
func TestServeWithNothingNamedTakesTheInstalledViewer(t *testing.T) {
	clearNamedWebDir(t)
	bundle := t.TempDir()
	stubBundle(t, bundle, nil)
	t.Chdir(t.TempDir())
	got, err := resolveServeAssets("", noEnv)
	if err != nil || !got.viewer || got.dir != bundle || got.source != "embedded" {
		t.Fatalf("resolveServeAssets = %+v, %v, want the bundle at %q", got, err, bundle)
	}
	dir, source, err := resolveWebAssets("", noEnv)
	if err != nil || dir != bundle || source != "embedded" {
		t.Errorf("resolveWebAssets = %q, %q, %v, want the bundle", dir, source, err)
	}
}

// A failed download leaves serve on the API alone, saying why, where open refuses with the same reason.
func TestAFailedBundleSaysWhy(t *testing.T) {
	clearNamedWebDir(t)
	stubBundle(t, "", errors.New("downloading the viewer: 503 Service Unavailable"))
	t.Chdir(t.TempDir())
	got, err := resolveServeAssets("", noEnv)
	if err != nil || got.viewer || !strings.Contains(got.why, "503") {
		t.Errorf("resolveServeAssets = %+v, %v, want API-only naming the 503", got, err)
	}
	if _, _, err := resolveWebAssets("", noEnv); err == nil || !strings.Contains(err.Error(), "503") {
		t.Errorf("open's refusal should name the failed download, got %v", err)
	}
}

// Naming a web dir, or a checkout's ./web, never reaches for the installed viewer.
func TestANamedOrCheckoutWebDirNeverTakesTheBundle(t *testing.T) {
	clearNamedWebDir(t)
	calls := stubBundle(t, t.TempDir(), nil)
	root := t.TempDir()
	web := filepath.Join(root, defaultWebDir)
	for _, f := range []string{"templates/ViewerPage.html", "templates/BrowsePage.html", "static/app.js", "static/browse.js"} {
		touch(t, filepath.Join(web, f))
	}
	t.Chdir(root)
	if _, err := resolveServeAssets("", noEnv); err != nil {
		t.Fatal(err)
	}
	resolveServeAssets(filepath.Join(root, "typo"), noEnv)
	resolveWebAssets(filepath.Join(root, "typo"), noEnv)
	if *calls != 0 {
		t.Errorf("the installed viewer was consulted %d times with a web dir present or named", *calls)
	}
}
