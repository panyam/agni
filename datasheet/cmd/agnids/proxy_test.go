package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/oneauth/core"

	"github.com/panyam/agni/internal/authgate"
)

// agnids behind a proxy that ends TLS gives a browser that signs in through the proxy a Secure
// session cookie, and ignores X-Forwarded-Proto from any other peer, as agni serve does (agni issue
// 1810).
func TestAgnidsBehindATrustedProxySetsASecureSession(t *testing.T) {
	store := t.TempDir()
	made, err := authgate.Store(store).CreateAPIKey(context.Background(), &core.CreateAPIKeyRequest{Subject: "bob", Name: "bob"})
	if err != nil {
		t.Fatal(err)
	}
	signIn := func(proxies []string) *http.Cookie {
		t.Helper()
		tp, err := authgate.ParseProxies(proxies)
		if err != nil {
			t.Fatal(err)
		}
		srv := httptest.NewServer(guard(http.NotFoundHandler(), authgate.Store(store), tp))
		defer srv.Close()
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/signin", strings.NewReader(url.Values{"token": {made.FullKey}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Forwarded-Proto", "https")
		client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		for _, c := range resp.Cookies() {
			if c.Name == authgate.SessionCookie {
				return c
			}
		}
		t.Fatalf("signing in to agnids = %d with no %s cookie", resp.StatusCode, authgate.SessionCookie)
		return nil
	}
	if c := signIn([]string{"127.0.0.1"}); !c.Secure {
		t.Errorf("a sign-in from the trusted proxy saying https set %+v; want it Secure", c)
	}
	if c := signIn(nil); c.Secure {
		t.Errorf("a sign-in saying https with no trusted proxy set %+v; want it not Secure", c)
	}
}

func TestAgnidsServeRefusesABadTrustedProxyByName(t *testing.T) {
	store := "file://" + filepath.ToSlash(t.TempDir())
	c := serveCmd()
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	c.SetArgs([]string{"--store", store, "--auth", authgate.Tokens, "--addr", "192.0.2.1:0", "--trusted-proxy", "10.0.0.0/33"})
	if err := c.Execute(); err == nil || !strings.Contains(err.Error(), "--trusted-proxy: trusted proxy \"10.0.0.0/33\"") {
		t.Errorf("agnids serve --trusted-proxy 10.0.0.0/33 = %v; want a refusal naming --trusted-proxy", err)
	}
}
