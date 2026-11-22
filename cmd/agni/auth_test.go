package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/panyam/oneauth/core"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/authgate"
)

// selfSigned writes a certificate and key for 127.0.0.1 into dir, and returns their paths and a
// pool that trusts the certificate.
func selfSigned(t *testing.T, dir string) (certFile, keyFile string, pool *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "agni test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, DNSNames: []string{"localhost"},
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	kder, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certFile, keyFile = filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kder}), 0o600); err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(der)
	pool = x509.NewCertPool()
	pool.AddCert(cert)
	return certFile, keyFile, pool
}

// serveTLS starts runViewer over TLS on a free port and returns its https URL once it answers.
func serveTLS(t *testing.T, o viewerOpts, client *http.Client) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	o.listener = ln
	cmd := rootCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	done := make(chan error, 1)
	go func() { done <- runViewer(cmd, o) }()
	t.Cleanup(func() { ln.Close(); <-done })
	base := "https://" + ln.Addr().String()
	for range 200 {
		select {
		case err := <-done:
			done <- err
			t.Fatalf("runViewer stopped before answering: %v", err)
		default:
		}
		if resp, err := client.Get(base + "/healthz"); err == nil {
			resp.Body.Close()
			return base
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("%s never answered", base)
	return ""
}

// bearer sends token on every call, or nothing for "".
func bearer(token string) connect.Option {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if token != "" {
				req.Header().Set("Authorization", "Bearer "+token)
			}
			return next(ctx, req)
		}
	}))
}

// A shared server over TLS answers a call with a good token in Connect, gRPC and gRPC-Web, refuses
// one with no token or a bad one as unauthenticated, and stops answering a revoked token (agni
// issue 1627).
func TestASharedServerChecksTokensOverTLS(t *testing.T) {
	old := authgate.CacheTTL
	authgate.CacheTTL = time.Millisecond
	t.Cleanup(func() { authgate.CacheTTL = old })

	dir := t.TempDir()
	certFile, keyFile, pool := selfSigned(t, dir)
	store := filepath.Join(dir, "store")
	storeURL := testStoreURL(t, store)
	token, err := runCobra(t, tokenCmd(), "create", "alice", "--store", storeURL)
	if err != nil {
		t.Fatalf("agni token create: %v", err)
	}
	token, _, _ = strings.Cut(strings.TrimSpace(token), "\n")
	if !strings.HasPrefix(token, "oa_") {
		t.Fatalf("agni token create printed %q, want the token", token)
	}

	client := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}, ForceAttemptHTTP2: true}}
	work := filepath.Join(dir, "boards")
	if err := os.MkdirAll(work, 0o750); err != nil {
		t.Fatal(err)
	}
	url := serveTLS(t, viewerOpts{store: testStore(t, store), auth: authgate.Tokens, tlsCert: certFile, tlsKey: keyFile, extraMounts: nil, mountRoot: dir}, client)

	protocols := []struct {
		name string
		opt  connect.ClientOption
	}{{"connect", nil}, {"grpc", connect.WithGRPC()}, {"grpc-web", connect.WithGRPCWeb()}}
	for _, p := range protocols {
		opts := func(tok string) []connect.ClientOption {
			o := []connect.ClientOption{bearer(tok)}
			if p.opt != nil {
				o = append(o, p.opt)
			}
			return o
		}
		info, err := webapiconnect.NewServerServiceClient(client, url, opts(token)...).GetServerInfo(context.Background(), connect.NewRequest(&webapi.GetServerInfoRequest{}))
		if err != nil {
			t.Fatalf("%s: GetServerInfo with alice's token: %v", p.name, err)
		}
		if !slices.Contains(info.Msg.GetFeatures(), "auth") {
			t.Errorf("%s: features %v, want auth among them", p.name, info.Msg.GetFeatures())
		}
		for _, bad := range []string{"", "oa_0123_not-a-token"} {
			_, err := webapiconnect.NewServerServiceClient(client, url, opts(bad)...).GetServerInfo(context.Background(), connect.NewRequest(&webapi.GetServerInfoRequest{}))
			if connect.CodeOf(err) != connect.CodeUnauthenticated {
				t.Errorf("%s: GetServerInfo with token %q = %v, want unauthenticated", p.name, bad, err)
			}
		}
	}

	mounts, err := webapiconnect.NewWorkspaceServiceClient(client, url, bearer(token)).ListMounts(context.Background(), connect.NewRequest(&webapi.ListMountsRequest{}))
	if err != nil {
		t.Fatalf("ListMounts: %v", err)
	}
	if len(mounts.Msg.GetMounts()) == 0 {
		t.Fatal("ListMounts answered no mounts, so the root check below proves nothing")
	}
	for _, m := range mounts.Msg.GetMounts() {
		if m.GetRoot() != "" {
			t.Errorf("mount %s answered root %q to a caller on a shared server, want it empty", m.GetName(), m.GetRoot())
		}
	}

	id, _, _ := strings.Cut(strings.TrimPrefix(token, "oa_"), "_")
	if _, err := runCobra(t, tokenCmd(), "revoke", "oa_"+id, "--store", storeURL); err != nil {
		t.Fatalf("agni token revoke: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	_, err = webapiconnect.NewServerServiceClient(client, url, bearer(token)).GetServerInfo(context.Background(), connect.NewRequest(&webapi.GetServerInfoRequest{}))
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("GetServerInfo with a revoked token = %v, want unauthenticated", err)
	}
}

// A browser with no session goes to /signin, which keeps a good token in a cookie, and a page call
// with that cookie answers (agni issue 1627).
func TestABrowserSignsInWithAToken(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	storeURL := testStoreURL(t, store)
	out, err := runCobra(t, tokenCmd(), "create", "bob", "--store", storeURL)
	if err != nil {
		t.Fatal(err)
	}
	token, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	url := serveViewer(t, viewerOpts{store: testStore(t, store), auth: authgate.Tokens})
	noFollow := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	req, _ := http.NewRequest(http.MethodGet, url+"/mounts/?x=1", nil)
	req.Header.Set("Accept", "text/html")
	resp, err := noFollow.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/signin?next=%2Fmounts%2F%3Fx%3D1" {
		t.Fatalf("a page with no session = %d to %q, want 303 to /signin with next", resp.StatusCode, resp.Header.Get("Location"))
	}

	resp, err = noFollow.PostForm(url+"/signin", map[string][]string{"token": {"oa_0_wrong"}, "next": {"/mounts/"}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized || len(resp.Cookies()) != 0 {
		t.Errorf("signing in with a wrong token = %d with %d cookies, want 401 and none", resp.StatusCode, len(resp.Cookies()))
	}

	resp, err = noFollow.PostForm(url+"/signin", map[string][]string{"token": {token}, "next": {"//evil.example/"}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/" {
		t.Errorf("signing in with next //evil.example/ = %d to %q, want 303 to /", resp.StatusCode, resp.Header.Get("Location"))
	}
	var session *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == authgate.SessionCookie {
			session = c
		}
	}
	if session == nil || !session.HttpOnly || session.SameSite != http.SameSiteStrictMode {
		t.Fatalf("signing in set %+v, want an HttpOnly SameSite=Strict %s cookie", session, authgate.SessionCookie)
	}

	jar := &cookieClient{cookie: session}
	info, err := webapiconnect.NewServerServiceClient(jar, url).GetServerInfo(context.Background(), connect.NewRequest(&webapi.GetServerInfoRequest{}))
	if err != nil {
		t.Fatalf("a call with the session cookie: %v", err)
	}
	if !slices.Contains(info.Msg.GetFeatures(), "auth") {
		t.Errorf("features %v, want auth", info.Msg.GetFeatures())
	}
}

// cookieClient sends one cookie on every request, as a signed-in page does.
type cookieClient struct{ cookie *http.Cookie }

func (c *cookieClient) Do(r *http.Request) (*http.Response, error) {
	r.AddCookie(c.cookie)
	return http.DefaultClient.Do(r)
}

// The CLI's --remote sends the token in AGNI_TOKEN, and without one a shared server refuses it.
func TestTheCLIReachesASharedServerWithAGNI_TOKEN(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	out, err := runCobra(t, tokenCmd(), "create", "ci", "--store", testStoreURL(t, store))
	if err != nil {
		t.Fatal(err)
	}
	token, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	url := serveViewer(t, viewerOpts{store: testStore(t, store), auth: authgate.Tokens})
	t.Setenv(envToken, "")
	if out, err := runMounts(t, "ls", "--remote", url); err == nil || !strings.Contains(err.Error(), "unauthenticated") {
		t.Errorf("agni mounts ls --remote with no token = %q, %v; want unauthenticated", out, err)
	}
	t.Setenv(envToken, token)
	if out, err := runMounts(t, "ls", "--remote", url); err != nil {
		t.Errorf("agni mounts ls --remote with AGNI_TOKEN = %q, %v", out, err)
	}
}

// A localhost server with no --auth answers a call with no token, says nothing of auth, and still
// answers each mount's root (agni issue 1627).
func TestALocalServerWithoutAuthIsUnchanged(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "boards"), 0o750); err != nil {
		t.Fatal(err)
	}
	url := serveViewer(t, viewerOpts{mountRoot: dir, store: testStore(t, filepath.Join(dir, "store"))})
	info, err := webapiconnect.NewServerServiceClient(http.DefaultClient, url).GetServerInfo(context.Background(), connect.NewRequest(&webapi.GetServerInfoRequest{}))
	if err != nil {
		t.Fatalf("GetServerInfo with no token: %v", err)
	}
	if slices.Contains(info.Msg.GetFeatures(), "auth") {
		t.Errorf("features %v on a server with no --auth, want no auth", info.Msg.GetFeatures())
	}
	mounts, err := webapiconnect.NewWorkspaceServiceClient(http.DefaultClient, url).ListMounts(context.Background(), connect.NewRequest(&webapi.ListMountsRequest{}))
	if err != nil {
		t.Fatalf("ListMounts with no token: %v", err)
	}
	if len(mounts.Msg.GetMounts()) == 0 {
		t.Fatal("ListMounts answered no mounts, so the root check below proves nothing")
	}
	for _, m := range mounts.Msg.GetMounts() {
		if m.GetRoot() == "" {
			t.Errorf("mount %s answered no root on a local server, want its folder", m.GetName())
		}
	}
}

// agni token lists what it made, marks a revoked one, and never prints a hash.
func TestTokenCommandListsAndRevokes(t *testing.T) {
	storeURL := testStoreURL(t, filepath.Join(t.TempDir(), "store"))
	for _, who := range []string{"alice", "ci"} {
		if _, err := runCobra(t, tokenCmd(), "create", who, "--store", storeURL); err != nil {
			t.Fatal(err)
		}
	}
	out, err := runCobra(t, tokenCmd(), "ls", "--store", storeURL)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("agni token ls = %q, want two tokens", out)
	}
	id := strings.Fields(lines[0])[0]
	if _, err := runCobra(t, tokenCmd(), "revoke", id, "--store", storeURL); err != nil {
		t.Fatal(err)
	}
	out, _ = runCobra(t, tokenCmd(), "ls", "--store", storeURL)
	if !strings.Contains(out, id) || !strings.Contains(strings.Split(out, "\n")[0], "revoked") || strings.Count(out, "active") != 1 {
		t.Errorf("after revoking %s, agni token ls = %q", id, out)
	}
	if strings.Contains(out, "$2a$") {
		t.Errorf("agni token ls printed a bcrypt hash: %q", out)
	}
	if _, err := runCobra(t, tokenCmd(), "ls"); err == nil {
		t.Error("agni token ls with no --store answered, want a refusal")
	}
}

// A server with --auth deletes the tokens in its store that have expired when it starts, and keeps
// the rest (agni issue 1778).
func TestServeDeletesExpiredTokens(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	keys := testTokens(t, store)
	past := time.Now().Add(-time.Hour)
	if _, err := keys.CreateAPIKey(context.Background(), &core.CreateAPIKeyRequest{Subject: "github:old", Name: "github sign-in", ExpiresAt: &past}); err != nil {
		t.Fatal(err)
	}
	if _, err := keys.CreateAPIKey(context.Background(), &core.CreateAPIKeyRequest{Subject: "ops", Name: "ops"}); err != nil {
		t.Fatal(err)
	}
	serveViewer(t, viewerOpts{store: testStore(t, store), uploadsDir: filepath.Join(dir, "uploads"), auth: authgate.Tokens})
	deadline := time.Now().Add(10 * time.Second)
	for {
		left, err := authgate.List(context.Background(), keys)
		if err != nil {
			t.Fatal(err)
		}
		if len(left) == 1 && left[0].Subject == "ops" {
			return
		}
		if time.Now().After(deadline) {
			var subjects []string
			for _, k := range left {
				subjects = append(subjects, k.Subject)
			}
			t.Fatalf("tokens after serve started = %v; want only ops, the expired one deleted", subjects)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
