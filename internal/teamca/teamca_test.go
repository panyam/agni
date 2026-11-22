package teamca

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func call(t *testing.T, url string) error {
	t.Helper()
	tr, err := Transport()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := (&http.Client{Transport: tr}).Get(url)
	if err == nil {
		_ = resp.Body.Close()
	}
	return err
}

// A server with a certificate from a CA no system trusts verifies once AGNI_CA_FILE names that CA,
// and not before.
func TestTheCLITrustsTheCANamedInAGNI_CA_FILE(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()

	t.Setenv(Env, "")
	if err := call(t, srv.URL); err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("with no CA file, a call to a team-CA server = %v, want a certificate error", err)
	}

	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(Env, ca)
	if err := call(t, srv.URL); err != nil {
		t.Fatalf("with %s naming the server's CA, the call failed: %v", Env, err)
	}
}

// A CA file with no certificate in it is refused by name, rather than falling back to the system's
// roots.
func TestACAFileWithNoCertificateIsRefused(t *testing.T) {
	p := filepath.Join(t.TempDir(), "empty.pem")
	if err := os.WriteFile(p, []byte("not a certificate\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(Env, p)
	if _, err := Transport(); err == nil || !strings.Contains(err.Error(), p) {
		t.Fatalf("Transport() with a CA file holding no certificate = %v, want an error naming %s", err, p)
	}
}
