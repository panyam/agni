package opscmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A server that ends TLS itself answers the probe only over HTTPS, so the image's HEALTHCHECK can
// run against agni in the enterprise bundle with --tls (agni issue 1802).
func TestHealthcheckProbesAServerOnTLSWithTheFlag(t *testing.T) {
	healthy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, "ok")
	})
	tlsSrv := httptest.NewTLSServer(healthy)
	defer tlsSrv.Close()
	plain := httptest.NewServer(healthy)
	defer plain.Close()
	run := func(args ...string) error {
		c := Healthcheck("agni", "localhost:8080")
		c.SetOut(io.Discard)
		c.SetErr(io.Discard)
		c.SetArgs(args)
		return c.Execute()
	}
	host := func(u string) string { return strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://") }

	if err := run("--addr", host(tlsSrv.URL), "--tls"); err != nil {
		t.Errorf("healthcheck --tls against a TLS server = %v; want ok", err)
	}
	if err := run("--addr", host(tlsSrv.URL)); err == nil {
		t.Errorf("healthcheck without --tls against a TLS server = ok; want a failure")
	}
	if err := run("--addr", host(plain.URL)); err != nil {
		t.Errorf("healthcheck against a plain server = %v; want ok", err)
	}
}
