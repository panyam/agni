package authgate

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
