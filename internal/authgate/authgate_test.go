package authgate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/panyam/oneauth/core"
)

func TestCheckAuthFlags(t *testing.T) {
	for _, c := range []struct {
		auth, store, cert, key, addr string
		want                         string
	}{
		{"", "", "", "", ":8080", ""},
		{Tokens, "/s", "", "", "localhost:8080", ""},
		{Tokens, "/s", "", "", "127.0.0.1:0", ""},
		{Tokens, "/s", "c", "k", ":8080", ""},
		{Tokens, "", "", "", "localhost:8080", "needs --store"},
		{Tokens, "/s", "", "", ":8080", "in the clear"},
		{Tokens, "/s", "", "", "10.0.0.5:8080", "in the clear"},
		{"", "", "c", "", ":8080", "go together"},
		{"oauth", "/s", "", "", "localhost:8080", "the one kind"},
	} {
		err := CheckFlags(c.auth, c.store, c.cert, c.key, c.addr)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("CheckFlags(%q, %q, %q, %q, %q) = %v, want nil", c.auth, c.store, c.cert, c.key, c.addr, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("CheckFlags(%q, %q, %q, %q, %q) = %v, want %q", c.auth, c.store, c.cert, c.key, c.addr, err, c.want)
		}
	}
}

func TestSafeNext(t *testing.T) {
	for in, want := range map[string]string{
		"/designs/x?y=1":       "/designs/x?y=1",
		"":                     "/",
		"//evil.example/":      "/",
		"/\\evil.example":      "/",
		"https://evil.example": "/",
		"designs":              "/",
	} {
		if got := safeNext(in); got != want {
			t.Errorf("safeNext(%q) = %q, want %q", in, got, want)
		}
	}
}

// GitHub sends a push webhook with no token, so the gate lets that one path through to the handler,
// which checks the delivery's signature, and still refuses its neighbors (agni issue 1726).
func TestTheGateLetsAWebhookThroughWithNoToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) })
	h := New(Store(t.TempDir()), false).Wrap(next)
	for path, want := range map[string]int{"/hooks/github": http.StatusAccepted, "/hooks/gitlab": http.StatusUnauthorized, "/hooks/github/x": http.StatusUnauthorized} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}")))
		if w.Code != want {
			t.Errorf("POST %s with no token = %d; want %d", path, w.Code, want)
		}
	}
}

// Prune deletes the tokens that have expired, and keeps one with no expiry, one still valid and a
// revoked one (agni issue 1778).
func TestPruneDeletesOnlyExpiredTokens(t *testing.T) {
	ctx := context.Background()
	store := Store(t.TempDir())
	now := time.Now()
	mint := func(subject string, expires *time.Time) string {
		t.Helper()
		resp, err := store.CreateAPIKey(ctx, &core.CreateAPIKeyRequest{Subject: subject, Name: subject, ExpiresAt: expires})
		if err != nil {
			t.Fatal(err)
		}
		return resp.APIKey.KeyID
	}
	past, future := now.Add(-time.Hour), now.Add(time.Hour)
	mint("github:old", &past)
	mint("github:fresh", &future)
	mint("ops", nil)
	revoked := mint("gone", nil)
	if _, err := store.RevokeAPIKey(ctx, &core.RevokeAPIKeyRequest{KeyID: revoked}); err != nil {
		t.Fatal(err)
	}

	n, err := Prune(ctx, store, now)
	if err != nil || n != 1 {
		t.Fatalf("Prune = %d, %v; want the one expired token deleted", n, err)
	}
	keys, err := List(ctx, store)
	if err != nil {
		t.Fatal(err)
	}
	var left []string
	for _, k := range keys {
		left = append(left, k.Subject)
	}
	sort.Strings(left)
	if got := strings.Join(left, ","); got != "github:fresh,gone,ops" {
		t.Errorf("tokens left = %s; want github:fresh, the revoked gone, and ops", got)
	}
	if n, err := Prune(ctx, store, now); err != nil || n != 0 {
		t.Errorf("a second Prune = %d, %v; want nothing left to delete", n, err)
	}
}
