// Package teamca lets the agni CLI trust a team's own certificate authority (agni issue 1800).
//
// A team whose agni serve has a certificate from its own CA hands each client the CA's file. The
// client libraries take it when they connect. The CLI reads it from AGNI_CA_FILE, for every
// --remote call and for agni login. It trusts the system's roots too, so a server with a public
// certificate still verifies. On macOS, Go ignores SSL_CERT_FILE and asks the system's verifier,
// so without this a CLI there could reach a team-CA server only after the CA went into the
// Keychain.
package teamca

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
)

// Env names the CA file, PEM, that the CLI trusts beside the system's roots.
const Env = "AGNI_CA_FILE"

// Transport returns the transport an agni CLI call goes through. With AGNI_CA_FILE unset it is
// http.DefaultTransport. With it set, it is a copy of http.DefaultTransport that trusts the
// system's roots and every certificate in the file. A file that can't be read, or holds no
// certificate, is an error naming it, so a typo doesn't fall back to the system's roots unnoticed.
func Transport() (http.RoundTripper, error) {
	path := os.Getenv(Env)
	if path == "" {
		return http.DefaultTransport, nil
	}
	pem, err := os.ReadFile(path) // #nosec G304 G703 -- the person names their own CA file
	if err != nil {
		return nil, fmt.Errorf("%s: %w", Env, err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("%s=%s holds no PEM certificate", Env, path)
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	return t, nil
}
