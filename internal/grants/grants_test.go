package grants

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/panyam/goapplib/storage/connections"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "access.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// A --grants-file reads to the same grants as the --grant values a serve.yaml's access: sets, and
// an empty list or an empty file grants nothing (agni issue 1895).
func TestReadFileMatchesAccess(t *testing.T) {
	got, err := ReadFile(write(t, "access:\n  - {source: github://acme/board, allow: [local:*, oidc:*]}\n  - {source: github://acme/other, allow: [\"*\"]}\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []connections.SourceGrant{
		{Source: "github://acme/board", Viewers: []string{"local:*", "oidc:*"}},
		{Source: "github://acme/other", Viewers: []string{"*"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadFile = %v; want %v", got, want)
	}
	for _, body := range []string{"access: []\n", "", "# nothing granted yet\n"} {
		if got, err := ReadFile(write(t, body)); err != nil || len(got) != 0 {
			t.Errorf("ReadFile of %q = %v, %v; want no grants", body, got, err)
		}
	}
}

func TestReadFileRefusesAnythingButAccess(t *testing.T) {
	for _, c := range []struct{ body, want string }{
		{"grant: [\"github://acme/board=*\"]\n", "want only access:"},
		{"access:\n  - {source: github://acme/board, allow: [\"lo*cal\"]}\n", `access[0] at line 2: connections: invalid source grant: viewer pattern "lo*cal"`},
		{"access: github://acme/board\n", "want a list of {source, allow}"},
	} {
		p := write(t, c.body)
		if _, err := ReadFile(p); err == nil || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), p) {
			t.Errorf("ReadFile of %q = %v; want one naming the file and saying %q", c.body, err, c.want)
		}
	}
	if _, err := ReadFile(filepath.Join(t.TempDir(), "missing.yaml")); err == nil || !strings.Contains(err.Error(), "--grants-file") {
		t.Errorf("ReadFile of a missing file = %v; want a --grants-file error", err)
	}
}
