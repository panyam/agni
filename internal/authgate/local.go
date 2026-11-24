package authgate

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/panyam/oneauth/accounts"
	"github.com/panyam/oneauth/core"
	"github.com/panyam/oneauth/localauth"
	oafs "github.com/panyam/oneauth/stores/fs"
	oagae "github.com/panyam/oneauth/stores/gae"
	oagorm "github.com/panyam/oneauth/stores/gorm"
	"gorm.io/gorm"
)

// LocalSignInPath is where the sign-in page posts a local username and password (agni issue 1815).
const LocalSignInPath = "/signin/local"

// AccountsDir holds the server's local accounts under a file:// --store, each as a bcrypt hash of
// its password and never the password itself.
const AccountsDir = "accounts"

// localProvider is the oneauth channel provider a local account is kept under.
const localProvider = "local"

// Accounts is the store of the server's local accounts: oneauth's channel store, with the listing
// and the delete of its ChannelAdmin that ListUsers and RemoveUser use (panyam/oneauth#574). Each
// account is one channel with provider "local" and identity key "username:<name>", holding a
// bcrypt hash of its password.
type Accounts interface {
	accounts.ChannelStore
	accounts.ChannelAdmin
}

// AccountsStore is the store of the server's local accounts, as files under the --store folder.
func AccountsStore(storeDir string) Accounts {
	return oafs.NewFSChannelStore(filepath.Join(storeDir, AccountsDir))
}

// SQLAccounts is the store of the server's local accounts in db, creating oneauth's channels table
// if absent.
func SQLAccounts(db *gorm.DB) (Accounts, error) {
	if err := db.AutoMigrate(&oagorm.ChannelModel{}); err != nil {
		return nil, fmt.Errorf("accounts: %w", err)
	}
	return oagorm.NewChannelStore(db), nil
}

// GAEAccounts is the store of the server's local accounts in Datastore, in namespace.
func GAEAccounts(client *datastore.Client, namespace string) Accounts {
	return oagae.NewChannelStore(client, namespace)
}

// ErrNoSuchUser is what SetPassword returns for a name with no account.
var ErrNoSuchUser = errors.New("no such user")

// ErrUserExists is what AddUser returns for a name that already has an account.
var ErrUserExists = errors.New("that user already exists")

// CanonicalUsername lowercases name and checks it against oneauth's username pattern, 3 to 20
// letters, digits, dashes or underscores. A username is case-insensitive, so "Alice" and "alice"
// name one account, and the lowercased form is the one stored and the one in the subject.
func CanonicalUsername(name string) (string, error) {
	n := strings.ToLower(strings.TrimSpace(name))
	if !localauth.DefaultSignupPolicy().GetUsernamePattern().MatchString(n) {
		return "", fmt.Errorf("username %q: want 3 to 20 letters, digits, dashes or underscores", name)
	}
	return n, nil
}

// checkNewPassword refuses a password shorter than oneauth's signup minimum.
func checkNewPassword(password string) error {
	if min := localauth.DefaultSignupPolicy().GetMinPasswordLength(); len(password) < min {
		return fmt.Errorf("the password needs at least %d characters", min)
	}
	return nil
}

func usernameKey(name string) string { return accounts.IdentityKey("username", name) }

// readAccount returns name's channel, or nil when it has none. A store error other than "absent"
// is returned as an error, so a broken store isn't read as a missing account.
func readAccount(ctx context.Context, store Accounts, name string) (*accounts.Channel, error) {
	resp, err := store.GetChannel(ctx, &accounts.GetChannelRequest{Provider: localProvider, IdentityKey: usernameKey(name)})
	if err != nil {
		// oneauth's stores report an absent channel as a plain "channel not found" error.
		if strings.Contains(err.Error(), "channel not found") {
			return nil, nil
		}
		return nil, err
	}
	return resp.Channel, nil
}

// AddUser makes a local account for name with password, and returns the canonical username. It
// returns ErrUserExists for a name that has one, and refuses a password shorter than 8 characters.
// The check and the write are two steps, so two operators adding one name at once can both succeed,
// and the second password wins.
func AddUser(ctx context.Context, store Accounts, name, password string) (string, error) {
	n, err := CanonicalUsername(name)
	if err != nil {
		return "", err
	}
	if err := checkNewPassword(password); err != nil {
		return "", err
	}
	if ch, err := readAccount(ctx, store, n); err != nil {
		return "", err
	} else if ch != nil {
		return "", fmt.Errorf("%s: %w", n, ErrUserExists)
	}
	return n, savePassword(ctx, store, n, password)
}

// SetPassword replaces name's password. It returns ErrNoSuchUser for a name with no account. A
// token minted at an earlier sign-in keeps working until it expires or is revoked.
func SetPassword(ctx context.Context, store Accounts, name, password string) (string, error) {
	n, err := CanonicalUsername(name)
	if err != nil {
		return "", err
	}
	if err := checkNewPassword(password); err != nil {
		return "", err
	}
	if ch, err := readAccount(ctx, store, n); err != nil {
		return "", err
	} else if ch == nil {
		return "", fmt.Errorf("%s: %w", n, ErrNoSuchUser)
	}
	return n, savePassword(ctx, store, n, password)
}

func savePassword(ctx context.Context, store Accounts, name, password string) error {
	hash, err := localauth.HashPassword(password)
	if err != nil {
		return err
	}
	_, err = store.SaveChannel(ctx, &accounts.SaveChannelRequest{Channel: &accounts.Channel{
		Provider: localProvider, IdentityKey: usernameKey(name),
		Credentials: map[string]any{"password_hash": hash},
		Profile:     map[string]any{"username": name},
	}})
	return err
}

// RemoveUser deletes name's account, and reports whether it had one. Removing a name with no
// account is false and no error. A token minted at an earlier sign-in keeps working until it
// expires or `agni token revoke` revokes it.
func RemoveUser(ctx context.Context, store Accounts, name string) (bool, error) {
	n, err := CanonicalUsername(name)
	if err != nil {
		return false, err
	}
	resp, err := store.DeleteChannel(ctx, &accounts.DeleteChannelRequest{Provider: localProvider, IdentityKey: usernameKey(n)})
	if err != nil {
		return false, err
	}
	return resp.Deleted, nil
}

// User is one local account as ListUsers shows it, with no password hash.
type User struct {
	Name      string
	CreatedAt time.Time
}

// ListUsers returns every local account, sorted by name.
func ListUsers(ctx context.Context, store Accounts) ([]User, error) {
	resp, err := store.ListChannels(ctx, &accounts.ListChannelsRequest{Provider: localProvider})
	if err != nil {
		return nil, err
	}
	var out []User
	for _, c := range resp.Channels {
		if name, ok := strings.CutPrefix(c.IdentityKey, "username:"); ok {
			out = append(out, User{Name: name, CreatedAt: c.CreatedAt})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// LocalAccounts signs people in with a username and password an operator made with `agni users
// add` (agni issue 1815). A good sign-in mints an agni token for the subject "local:<username>",
// the same kind of token a GitHub or OIDC sign-in mints. There is no self-signup.
type LocalAccounts struct {
	// Store holds the accounts, in the server's --store.
	Store Accounts
	// TTL is how long a minted agni token works. Zero is 30 days.
	TTL time.Duration
	// PerUser and PerAddress limit sign-in attempts for one username and from one client address.
	// Nil allows 5 attempts per username and 20 per address in a burst, each refilling one a minute
	// and one every 6 seconds. The address is the client's as the trusted proxies report it.
	PerUser, PerAddress core.RateLimiter
}

// localSignIns is the local accounts the gate signs people in with.
type localSignIns struct {
	cfg  LocalAccounts
	keys core.APIKeyStore
	// mu guards swept, when the limiters last dropped the buckets of keys unused for an hour.
	mu    sync.Mutex
	swept time.Time
}

// sweep drops the limiters' idle buckets at most every ten minutes, so a stream of made-up
// usernames or addresses doesn't grow them for good. A limiter that isn't oneauth's in-memory one
// is left alone.
func (s *localSignIns) sweep(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if now.Sub(s.swept) < 10*time.Minute {
		return
	}
	s.swept = now
	for _, l := range []core.RateLimiter{s.cfg.PerUser, s.cfg.PerAddress} {
		if m, ok := l.(*core.InMemoryRateLimiter); ok {
			m.CleanupStale(time.Hour)
		}
	}
}

// WithLocalAccounts lets people sign in with a local username and password, and returns g. It
// refuses a cfg with no Store.
func (g *Gate) WithLocalAccounts(cfg LocalAccounts) (*Gate, error) {
	if cfg.Store == nil {
		return nil, errors.New("signing in with local accounts needs --store, where agni users add keeps them")
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 30 * 24 * time.Hour
	}
	if cfg.PerUser == nil {
		cfg.PerUser = core.NewInMemoryRateLimiter(1.0/60, 5)
	}
	if cfg.PerAddress == nil {
		cfg.PerAddress = core.NewInMemoryRateLimiter(1.0/6, 20)
	}
	g.local = &localSignIns{cfg: cfg, keys: g.store}
	return g, nil
}

// errBadLogin is the one error a refused sign-in shows, for an unknown user and a wrong password
// alike, so the page doesn't say which accounts exist.
var errBadLogin = errors.New("that username and password don't match an account on this server")

// admit checks name and password and mints an agni token for "local:<name>". An unknown name runs
// the same bcrypt comparison a wrong password does, so the two take as long to refuse.
func (s *localSignIns) admit(ctx context.Context, name, password string) (string, string, error) {
	n, nameErr := CanonicalUsername(name)
	hash := ""
	if nameErr == nil {
		ch, err := readAccount(ctx, s.cfg.Store, n)
		if err != nil {
			return "", "", fmt.Errorf("reading the accounts: %w", err)
		}
		if ch != nil {
			hash, _ = ch.Credentials["password_hash"].(string)
		}
	}
	if !localauth.CheckPassword(hash, password) || nameErr != nil {
		return "", "", errBadLogin
	}
	subject := "local:" + n
	expires := time.Now().Add(s.cfg.TTL)
	resp, err := s.keys.CreateAPIKey(ctx, &core.CreateAPIKeyRequest{Subject: subject, Name: "local sign-in " + n, ExpiresAt: &expires})
	if err != nil {
		return "", "", fmt.Errorf("minting a token for %s: %w", n, err)
	}
	return resp.FullKey, subject, nil
}

// serve answers a post of the sign-in page's username and password form. A good sign-in sets the
// session cookie and goes on to next. A bad one shows the page again with a 401, and too many
// attempts for the username or from the address get a 429 before any password is checked.
func (s *localSignIns) serve(g *Gate, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	name := r.PostFormValue("username")
	password := r.PostFormValue("password")
	next := safeNext(r.PostFormValue("next"))
	page := g.signInView(next)
	status := http.StatusUnauthorized
	key := strings.ToLower(strings.TrimSpace(name))
	s.sweep(time.Now())
	if !s.cfg.PerAddress.Allow("addr:"+g.proxies.ClientIP(r)) || !s.cfg.PerUser.Allow("user:"+key) {
		w.Header().Set("Retry-After", "60")
		status = http.StatusTooManyRequests
		page.Error = "Too many sign-in attempts. Wait a minute and try again."
	} else {
		full, _, err := s.admit(r.Context(), name, password)
		if err == nil {
			http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: full, Path: "/", HttpOnly: true, Secure: g.https(r), SameSite: http.SameSiteStrictMode}) // #nosec G124 -- Secure follows HTTPS, as in signIn
			http.Redirect(w, r, next, http.StatusSeeOther)                                                                                                      // #nosec G710 -- next came through safeNext
			return
		}
		if errors.Is(err, errBadLogin) {
			page.Error = "That username and password don't match an account on this server."
		} else {
			status = http.StatusInternalServerError
			page.Error = "Signing in didn't work: " + err.Error()
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = signInPage.Execute(w, page)
}
