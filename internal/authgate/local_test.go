package authgate

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/panyam/oneauth/core"

	"github.com/panyam/agni/service"
)

const alicePassword = "correct horse battery"

// storeDir is a fresh --store folder. oneauth records a token's use in a goroutine of its own, so
// a write can land in the folder after the test ends. t.TempDir fails the test when that happens,
// and this folder's cleanup tries again instead.
func storeDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "authgate-local-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for i := 0; i < 50 && os.RemoveAll(dir) != nil; i++ {
			time.Sleep(20 * time.Millisecond)
		}
	})
	return dir
}

// localGate is a gate over a fresh --store folder with alice's account in it. Its handler answers
// with the caller's subject. With local false, the gate has no local accounts.
func localGate(t *testing.T, local bool) (http.Handler, Keys, string) {
	t.Helper()
	return localGateWith(t, local, LocalAccounts{})
}

// localGateWith is localGate with cfg's limiters and TTL.
func localGateWith(t *testing.T, local bool, cfg LocalAccounts) (http.Handler, Keys, string) {
	t.Helper()
	dir := storeDir(t)
	keys := Store(dir)
	accts := AccountsStore(dir)
	if _, err := AddUser(context.Background(), accts, "alice", alicePassword); err != nil {
		t.Fatal(err)
	}
	g := New(keys, nil)
	if local {
		var err error
		cfg.Store = accts
		if g, err = g.WithLocalAccounts(cfg); err != nil {
			t.Fatal(err)
		}
	}
	whoami := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := service.CallerFrom(r.Context())
		_, _ = io.WriteString(w, c.Subject)
	})
	return g.Wrap(whoami), keys, dir
}

// postLogin posts the sign-in form from peer and returns the response and the session cookie it set,
// or "" for none.
func postLogin(h http.Handler, peer, user, password string) (*httptest.ResponseRecorder, string) {
	form := url.Values{"username": {user}, "password": {password}, "next": {"/designs"}}
	r := httptest.NewRequest(http.MethodPost, LocalSignInPath, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.RemoteAddr = peer
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	for _, k := range w.Result().Cookies() {
		if k.Name == SessionCookie {
			return w, k.Value
		}
	}
	return w, ""
}

func tokenCount(t *testing.T, keys Keys) int {
	t.Helper()
	all, err := List(context.Background(), keys)
	if err != nil {
		t.Fatal(err)
	}
	return len(all)
}

// A good username and password sets a session holding a token for local:alice, which the gate then
// accepts as alice, and which expires in 30 days like a GitHub or OIDC sign-in's (agni issue 1815).
func TestALocalSignInMintsATokenTheGateAccepts(t *testing.T) {
	h, keys, _ := localGate(t, true)
	w, session := postLogin(h, "192.0.2.1:1000", "Alice", alicePassword)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/designs" || session == "" {
		t.Fatalf("POST %s = %d to %q with session %q; want 303 to /designs with a session", LocalSignInPath, w.Code, w.Header().Get("Location"), session)
	}
	r := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	r.Header.Set("Authorization", "Bearer "+session)
	got := httptest.NewRecorder()
	h.ServeHTTP(got, r)
	if got.Code != http.StatusOK || got.Body.String() != "local:alice" {
		t.Fatalf("GET /whoami with the session = %d %q; want 200 local:alice", got.Code, got.Body.String())
	}
	all, err := List(context.Background(), keys)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].Subject != "local:alice" || all[0].ExpiresAt == nil {
		t.Fatalf("tokens after the sign-in = %+v; want one for local:alice with an expiry", all)
	}
	if left := time.Until(*all[0].ExpiresAt); left < 29*24*time.Hour || left > 31*24*time.Hour {
		t.Errorf("the token expires in %v; want about 30 days", left)
	}
}

// A wrong password and an unknown username both get a 401, the same message and no token, so the
// form doesn't tell anyone which accounts exist.
func TestALocalSignInRefusesAWrongPasswordAndAnUnknownUser(t *testing.T) {
	h, keys, _ := localGate(t, true)
	var bodies []string
	for i, c := range [][2]string{{"alice", "wrong horse battery"}, {"mallory", alicePassword}, {"x", alicePassword}} {
		w, session := postLogin(h, "192.0.2.1:1000", c[0], c[1])
		if w.Code != http.StatusUnauthorized || session != "" {
			t.Errorf("sign-in %d as %s = %d with session %q; want 401 and none", i, c[0], w.Code, session)
		}
		bodies = append(bodies, w.Body.String())
	}
	if n := tokenCount(t, keys); n != 0 {
		t.Errorf("refused sign-ins minted %d tokens; want 0", n)
	}
	if !strings.Contains(bodies[0], "don&#39;t match an account") || bodies[0] != bodies[1] {
		t.Errorf("a wrong password and an unknown user showed different pages:\n%s\n---\n%s", bodies[0], bodies[1])
	}
}

// An unknown username takes a bcrypt comparison to refuse, as a wrong password does, so a caller
// can't time the form to learn which accounts exist.
//
// See: https://cwe.mitre.org/data/definitions/208.html
func TestAnUnknownUserTakesAsLongToRefuse(t *testing.T) {
	h, _, _ := localGate(t, true)
	const bcryptFloor = 15 * time.Millisecond
	start := time.Now()
	w, _ := postLogin(h, "192.0.2.1:1000", "mallory", alicePassword)
	if took := time.Since(start); w.Code != http.StatusUnauthorized || took < bcryptFloor {
		t.Fatalf("an unknown user = %d after %v; want 401 after at least %v", w.Code, took, bcryptFloor)
	}
}

// slowLimits are the default bursts, 5 per username and 20 per address, refilling one an hour, so a
// slow run under -race doesn't refill a bucket between tries.
func slowLimits() LocalAccounts {
	return LocalAccounts{PerUser: core.NewInMemoryRateLimiter(1.0/3600, 5), PerAddress: core.NewInMemoryRateLimiter(1.0/3600, 20)}
}

// With no limiters named, the gate allows 5 tries per username and 20 per address in a burst.
func TestTheDefaultLimitsAre5PerUsernameAnd20PerAddress(t *testing.T) {
	g, err := New(testKeys(t), nil).WithLocalAccounts(LocalAccounts{Store: AccountsStore(t.TempDir())})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name  string
		l     core.RateLimiter
		burst int
	}{{"per username", g.local.cfg.PerUser, 5}, {"per address", g.local.cfg.PerAddress, 20}} {
		n := 0
		for n < 100 && c.l.Allow("k") {
			n++
		}
		if n != c.burst {
			t.Errorf("the default limit %s allowed %d tries in a burst; want %d", c.name, n, c.burst)
		}
	}
}

// After 5 tries for one username the form answers 429 without checking the password, even a right
// one, while another username from another address still signs in.
func TestTheFormLimitsTriesPerUsername(t *testing.T) {
	h, keys, dir := localGateWith(t, true, slowLimits())
	if _, err := AddUser(context.Background(), AccountsStore(dir), "bob", alicePassword); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if w, _ := postLogin(h, "192.0.2.1:1000", "alice", "wrong horse battery"); w.Code != http.StatusUnauthorized {
			t.Fatalf("wrong try %d = %d; want 401", i, w.Code)
		}
	}
	w, session := postLogin(h, "192.0.2.2:1000", "ALICE", alicePassword)
	if w.Code != http.StatusTooManyRequests || session != "" || w.Header().Get("Retry-After") == "" {
		t.Fatalf("the 6th try for alice = %d with session %q and Retry-After %q; want 429, no session, a Retry-After", w.Code, session, w.Header().Get("Retry-After"))
	}
	if n := tokenCount(t, keys); n != 0 {
		t.Fatalf("the limited try minted %d tokens; want 0", n)
	}
	if w, session := postLogin(h, "192.0.2.3:1000", "bob", alicePassword); w.Code != http.StatusSeeOther || session == "" {
		t.Errorf("bob from another address = %d with session %q; want 303 and a session", w.Code, session)
	}
}

// After 20 tries from one address, across usernames, the form answers 429 for any username from it,
// while another address still signs in.
func TestTheFormLimitsTriesPerAddress(t *testing.T) {
	h, keys, _ := localGateWith(t, true, slowLimits())
	for i := 0; i < 20; i++ {
		if w, _ := postLogin(h, "198.51.100.7:1000", "user"+strings.Repeat("x", i%10)+string(rune('a'+i)), "wrong horse battery"); w.Code != http.StatusUnauthorized {
			t.Fatalf("try %d = %d; want 401", i, w.Code)
		}
	}
	if w, session := postLogin(h, "198.51.100.7:2000", "alice", alicePassword); w.Code != http.StatusTooManyRequests || session != "" {
		t.Fatalf("the 21st try from the address = %d with session %q; want 429 and none", w.Code, session)
	}
	if n := tokenCount(t, keys); n != 0 {
		t.Fatalf("the limited try minted %d tokens; want 0", n)
	}
	if w, session := postLogin(h, "198.51.100.8:1000", "alice", alicePassword); w.Code != http.StatusSeeOther || session == "" {
		t.Errorf("alice from another address = %d with session %q; want 303 and a session", w.Code, session)
	}
}

// The store keeps a bcrypt hash of the password, never the password.
func TestTheStoredAccountHoldsNoPlainPassword(t *testing.T) {
	_, _, dir := localGate(t, true)
	files := 0
	err := filepath.WalkDir(filepath.Join(dir, AccountsDir), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p) // #nosec G304 -- a file the test's own store wrote
		if err != nil {
			return err
		}
		files++
		if strings.Contains(string(b), alicePassword) || !strings.Contains(string(b), "$2a$") {
			t.Errorf("%s holds %s; want a bcrypt hash and no password", p, b)
		}
		return nil
	})
	if err != nil || files != 1 {
		t.Fatalf("walking the accounts found %d files, err %v; want 1", files, err)
	}
}

// The sign-in page shows the username and password form only on a gate with local accounts, and
// the form's path answers 404 on a gate without them, as an unoffered provider's does.
func TestTheSignInPageShowsTheFormOnlyWithLocalAccounts(t *testing.T) {
	for _, local := range []bool{false, true} {
		h, _, _ := localGate(t, local)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/signin", nil))
		page := w.Body.String()
		if has := strings.Contains(page, `action="/signin/local"`) && strings.Contains(page, `name="username"`); has != local {
			t.Errorf("local accounts %v: the sign-in page has the form %v", local, has)
		}
		if !strings.Contains(page, `name="token"`) {
			t.Errorf("local accounts %v: the sign-in page lost the token form", local)
		}
		if !local {
			if w, _ := postLogin(h, "192.0.2.1:1000", "alice", alicePassword); w.Code != http.StatusNotFound {
				t.Errorf("POST %s with no local accounts = %d; want 404", LocalSignInPath, w.Code)
			}
		}
	}
}

func TestLocalAccountsRefuseBadNamesShortPasswordsAndDuplicates(t *testing.T) {
	ctx := context.Background()
	accts := AccountsStore(t.TempDir())
	for _, bad := range []string{"", "ab", "has space", "../etc", strings.Repeat("a", 21), "a:b"} {
		if _, err := AddUser(ctx, accts, bad, alicePassword); err == nil {
			t.Errorf("AddUser(%q) = nil; want a refusal", bad)
		}
	}
	if _, err := AddUser(ctx, accts, "carol", "short"); err == nil {
		t.Error("AddUser with a 5-character password = nil; want a refusal")
	}
	if _, err := AddUser(ctx, accts, "carol", alicePassword); err != nil {
		t.Fatal(err)
	}
	if _, err := AddUser(ctx, accts, "Carol", alicePassword); !errors.Is(err, ErrUserExists) {
		t.Errorf("AddUser(Carol) after carol = %v; want ErrUserExists", err)
	}
	if _, err := SetPassword(ctx, accts, "dave", alicePassword); !errors.Is(err, ErrNoSuchUser) {
		t.Errorf("SetPassword(dave) = %v; want ErrNoSuchUser", err)
	}
	if removed, err := RemoveUser(ctx, accts, "dave"); err != nil || removed {
		t.Errorf("RemoveUser(dave) = %v, %v; want false, nil", removed, err)
	}
}
