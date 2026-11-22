package service_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/panyam/goapplib/storage/connections"
	"github.com/panyam/goapplib/storage/connections/fsstore"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// fakeKeeper is a service.UserTokenKeeper holding tokens in memory, by connection and owner.
type fakeKeeper struct {
	store  connections.Store
	tokens map[string]string
	fail   error
}

func (k *fakeKeeper) SaveUserToken(_ context.Context, owner, id, token string) error {
	if k.fail != nil {
		return k.fail
	}
	k.tokens[id+"/"+owner] = token
	return nil
}

func (k *fakeKeeper) Disconnect(ctx context.Context, owner, id string) error {
	if _, err := k.store.GetConnection(ctx, owner, id); err != nil {
		return err
	}
	delete(k.tokens, id+"/"+owner)
	return k.store.DeleteConnection(ctx, owner, id)
}

func (k *fakeKeeper) SaveServerSecret(ctx context.Context, id, secret string) error {
	if k.fail != nil {
		return k.fail
	}
	c, err := k.store.GetConnection(ctx, "", id)
	if err != nil {
		return err
	}
	if !c.Sealed {
		return connections.ErrInvalid
	}
	k.tokens[id+"/"] = secret
	return nil
}

func connectionIDs(resp *webapi.ListConnectionsResponse) []string {
	var out []string
	for _, c := range resp.GetConnections() {
		out = append(out, service.ConnectionID(c.GetName()))
	}
	return out
}

// A person's connection takes their token, seals it through the keeper, and belongs to them alone:
// nobody else lists, reads or deletes it, and it never names a server secret (agni issue 1728).
func TestAPersonsConnectionIsTheirsAlone(t *testing.T) {
	store, err := fsstore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	keeper := &fakeKeeper{store: store, tokens: map[string]string{}}
	svc := service.NewConnectionService(store, nil).WithUserTokens(keeper).WithBaseURL("https://ghe.example.com/api/v3")
	alice, bob := as("alice"), as("bob")
	mine := func(ctx context.Context, id, token string) (*webapi.Connection, error) {
		return svc.CreateConnection(ctx, &webapi.CreateConnectionRequest{ConnectionId: id, UserToken: token,
			Connection: &webapi.Connection{Kind: webapi.ConnectionKind_CONNECTION_KIND_GITHUB_USER, Scope: webapi.ConnectionScope_CONNECTION_SCOPE_PERSON}})
	}

	c, err := mine(alice, "alice-gh", "ghp_alice")
	if err != nil {
		t.Fatalf("alice keeps her own connection: %v", err)
	}
	if c.GetOwner() != "alice" || c.GetScope() != webapi.ConnectionScope_CONNECTION_SCOPE_PERSON || c.GetTokenSource() != "" {
		t.Errorf("alice's connection = %v; want hers, a person's, naming no secret", c)
	}
	if c.GetBaseUrl() != "https://ghe.example.com/api/v3" {
		t.Errorf("alice's connection reads through %q; want the server's GitHub API, since she named none", c.GetBaseUrl())
	}
	if keeper.tokens["alice-gh/alice"] != "ghp_alice" {
		t.Errorf("sealed tokens = %v; want alice's under alice-gh", keeper.tokens)
	}
	if _, err := svc.CreateConnection(alice, &webapi.CreateConnectionRequest{ConnectionId: "shared", Connection: &webapi.Connection{Kind: webapi.ConnectionKind_CONNECTION_KIND_GITHUB_TOKEN, TokenSource: "env:SERVER_TOKEN"}}); err != nil {
		t.Fatal(err)
	}

	if l, err := svc.ListConnections(alice, &webapi.ListConnectionsRequest{}); err != nil || !slices.Equal(connectionIDs(l), []string{"alice-gh", "shared"}) {
		t.Errorf("alice lists %q, %v; want hers and the server's", connectionIDs(l), err)
	}
	if l, err := svc.ListConnections(bob, &webapi.ListConnectionsRequest{}); err != nil || !slices.Equal(connectionIDs(l), []string{"shared"}) {
		t.Errorf("bob lists %q, %v; want the server's alone", connectionIDs(l), err)
	}
	if _, err := svc.GetConnection(bob, &webapi.GetConnectionRequest{Name: "connections/alice-gh"}); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("bob reads alice's connection: %v; want not found", err)
	}
	if err := svc.DeleteConnection(bob, &webapi.DeleteConnectionRequest{Name: "connections/alice-gh"}); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("bob deletes alice's connection: %v; want not found", err)
	}

	for _, c := range []struct {
		why string
		ctx context.Context
		req *webapi.CreateConnectionRequest
		err error
	}{
		{"a person's token kind naming a server secret", bob, &webapi.CreateConnectionRequest{ConnectionId: "bob-gh", Connection: &webapi.Connection{Kind: webapi.ConnectionKind_CONNECTION_KIND_GITHUB_TOKEN, Scope: webapi.ConnectionScope_CONNECTION_SCOPE_PERSON, TokenSource: "env:SERVER_TOKEN"}}, service.ErrInvalidArgument},
		{"a person's connection with no token", bob, &webapi.CreateConnectionRequest{ConnectionId: "bob-gh", Connection: &webapi.Connection{Kind: webapi.ConnectionKind_CONNECTION_KIND_GITHUB_USER}}, service.ErrInvalidArgument},
		{"a server connection carrying a token", bob, &webapi.CreateConnectionRequest{ConnectionId: "bob-gh", UserToken: "ghp_bob", Connection: &webapi.Connection{Kind: webapi.ConnectionKind_CONNECTION_KIND_GITHUB_TOKEN, TokenSource: "env:X"}}, service.ErrInvalidArgument},
		{"a person's connection with no caller", context.Background(), &webapi.CreateConnectionRequest{ConnectionId: "anon-gh", UserToken: "ghp_x", Connection: &webapi.Connection{Kind: webapi.ConnectionKind_CONNECTION_KIND_GITHUB_USER}}, service.ErrFailedPrecondition},
	} {
		if _, err := svc.CreateConnection(c.ctx, c.req); !errors.Is(err, c.err) {
			t.Errorf("%s: %v; want %v", c.why, err, c.err)
		}
	}

	keeper.fail = errors.New("no key to seal with")
	if _, err := mine(bob, "bob-gh", "ghp_bob"); err == nil {
		t.Error("a token the keeper can't seal made a connection")
	}
	if _, err := svc.GetConnection(bob, &webapi.GetConnectionRequest{Name: "connections/bob-gh"}); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("after a failed seal, bob's connection = %v; want none left behind", err)
	}
	keeper.fail = nil

	if err := svc.DeleteConnection(alice, &webapi.DeleteConnectionRequest{Name: "connections/alice-gh"}); err != nil {
		t.Fatalf("alice deletes her connection: %v", err)
	}
	if _, ok := keeper.tokens["alice-gh/alice"]; ok {
		t.Error("alice's sealed token outlived her connection")
	}

	if _, err := service.NewConnectionService(store, nil).CreateConnection(alice, &webapi.CreateConnectionRequest{ConnectionId: "alice-gh", UserToken: "ghp_alice", Connection: &webapi.Connection{Kind: webapi.ConnectionKind_CONNECTION_KIND_GITHUB_USER}}); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("a person's connection on a server with no --connections-key: %v; want failed precondition", err)
	}
}

// Each person's ids are their own (agni issue 1778). alice and bob can both keep an alice-gh, and
// each reads, lists and deletes only their own. A person can't take a server connection's id, and
// when a server connection takes a person's id later, that person's own still answers for them.
func TestTwoPeopleKeepConnectionsByOneID(t *testing.T) {
	store, err := fsstore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	keeper := &fakeKeeper{store: store, tokens: map[string]string{}}
	svc := service.NewConnectionService(store, nil).WithUserTokens(keeper)
	alice, bob, ops := as("alice"), as("bob"), as("ops")
	mine := func(ctx context.Context, id, token string) (*webapi.Connection, error) {
		return svc.CreateConnection(ctx, &webapi.CreateConnectionRequest{ConnectionId: id, UserToken: token,
			Connection: &webapi.Connection{Kind: webapi.ConnectionKind_CONNECTION_KIND_GITHUB_USER}})
	}
	server := func(id string) error {
		_, err := svc.CreateConnection(ops, &webapi.CreateConnectionRequest{ConnectionId: id, Connection: &webapi.Connection{Kind: webapi.ConnectionKind_CONNECTION_KIND_GITHUB_TOKEN, TokenSource: "env:SERVER_TOKEN"}})
		return err
	}

	if _, err := mine(alice, "alice-gh", "ghp_alice"); err != nil {
		t.Fatal(err)
	}
	if c, err := mine(bob, "alice-gh", "ghp_bob"); err != nil || c.GetOwner() != "bob" {
		t.Fatalf("bob keeps his own alice-gh = %v, %v; want his", c, err)
	}
	if keeper.tokens["alice-gh/alice"] != "ghp_alice" || keeper.tokens["alice-gh/bob"] != "ghp_bob" {
		t.Errorf("sealed tokens = %v; want alice's and bob's apart", keeper.tokens)
	}
	if _, err := mine(bob, "alice-gh", "ghp_bob2"); !errors.Is(err, service.ErrAlreadyExists) {
		t.Errorf("bob making alice-gh a second time: %v; want already exists", err)
	}
	for who, ctx := range map[string]context.Context{"alice": alice, "bob": bob} {
		c, err := svc.GetConnection(ctx, &webapi.GetConnectionRequest{Name: "connections/alice-gh"})
		if err != nil || c.GetOwner() != who {
			t.Errorf("%s reads connections/alice-gh = %v, %v; want their own", who, c, err)
		}
		if l, err := svc.ListConnections(ctx, &webapi.ListConnectionsRequest{}); err != nil || len(l.GetConnections()) != 1 || l.GetConnections()[0].GetOwner() != who {
			t.Errorf("%s lists %v, %v; want their own alice-gh alone", who, l.GetConnections(), err)
		}
	}

	if err := server("shared"); err != nil {
		t.Fatal(err)
	}
	if _, err := mine(alice, "shared", "ghp_alice"); !errors.Is(err, service.ErrAlreadyExists) {
		t.Errorf("alice taking the server connection's id: %v; want already exists", err)
	}
	if err := server("alice-gh"); err != nil {
		t.Fatalf("a server connection taking an id people keep: %v", err)
	}
	if c, err := svc.GetConnection(alice, &webapi.GetConnectionRequest{Name: "connections/alice-gh"}); err != nil || c.GetOwner() != "alice" {
		t.Errorf("alice reads connections/alice-gh beside the server's = %v, %v; want her own", c, err)
	}
	if c, err := svc.GetConnection(ops, &webapi.GetConnectionRequest{Name: "connections/alice-gh"}); err != nil || c.GetOwner() != "" {
		t.Errorf("ops reads connections/alice-gh = %v, %v; want the server's", c, err)
	}

	if err := svc.DeleteConnection(bob, &webapi.DeleteConnectionRequest{Name: "connections/alice-gh"}); err != nil {
		t.Fatalf("bob deletes his alice-gh: %v", err)
	}
	if _, ok := keeper.tokens["alice-gh/bob"]; ok {
		t.Error("bob's sealed token outlived his connection")
	}
	if c, err := svc.GetConnection(alice, &webapi.GetConnectionRequest{Name: "connections/alice-gh"}); err != nil || c.GetOwner() != "alice" || keeper.tokens["alice-gh/alice"] != "ghp_alice" {
		t.Errorf("after bob deletes his, alice's alice-gh = %v, %v, sealed %q; want hers, untouched", c, err, keeper.tokens["alice-gh/alice"])
	}
	if c, err := svc.GetConnection(bob, &webapi.GetConnectionRequest{Name: "connections/alice-gh"}); err != nil || c.GetOwner() != "" {
		t.Errorf("after deleting his own, bob reads connections/alice-gh = %v, %v; want the server's", c, err)
	}
}

// A server connection can send its secret to be sealed rather than name where it is, for a server
// in a container whose environment the caller can't reach (agni issue 1802). The connection names
// no reference and says it is sealed, rotating replaces the secret, and deleting it deletes the
// secret too. A reference beside a secret, a person's connection with one, and a server with no
// key to seal with are each refused.
func TestAServerConnectionKeepsItsSecretSealed(t *testing.T) {
	store, err := fsstore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	keeper := &fakeKeeper{store: store, tokens: map[string]string{}}
	svc := service.NewConnectionService(store, nil).WithUserTokens(keeper)
	ops := as("ops")
	token := func(id, ref, secret string) (*webapi.Connection, error) {
		return svc.CreateConnection(ops, &webapi.CreateConnectionRequest{ConnectionId: id, ServerSecret: secret,
			Connection: &webapi.Connection{Kind: webapi.ConnectionKind_CONNECTION_KIND_GITHUB_TOKEN, TokenSource: ref}})
	}

	c, err := token("hw", "", "ghp_server")
	if err != nil {
		t.Fatalf("a sealed server connection: %v", err)
	}
	if !c.GetSealed() || c.GetTokenSource() != "" || c.GetScope() != webapi.ConnectionScope_CONNECTION_SCOPE_SERVER || c.GetOwner() != "" {
		t.Errorf("the sealed connection = %v; want a server connection, sealed, naming no token source", c)
	}
	if keeper.tokens["hw/"] != "ghp_server" {
		t.Errorf("the keeper holds %q for hw; want the secret sealed through it", keeper.tokens["hw/"])
	}
	if got, err := svc.GetConnection(as("alice"), &webapi.GetConnectionRequest{Name: "connections/hw"}); err != nil || !got.GetSealed() {
		t.Errorf("another caller's GetConnection hw = %v, %v; want it listed as sealed", got, err)
	}

	if _, err := svc.RotateConnection(ops, &webapi.RotateConnectionRequest{Name: "connections/hw", ServerSecret: "ghp_next"}); err != nil || keeper.tokens["hw/"] != "ghp_next" {
		t.Errorf("rotating hw = %v, and the keeper holds %q; want the new secret", err, keeper.tokens["hw/"])
	}
	if _, err := svc.RotateConnection(ops, &webapi.RotateConnectionRequest{Name: "connections/hw"}); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("rotating with no secret = %v; want ErrInvalidArgument", err)
	}
	if _, err := token("byref", "env:HW_TOKEN", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RotateConnection(ops, &webapi.RotateConnectionRequest{Name: "connections/byref", ServerSecret: "x"}); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("rotating a connection that names its secret = %v; want ErrFailedPrecondition", err)
	}

	if _, err := token("both", "env:HW_TOKEN", "ghp_x"); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("a reference and a sealed secret = %v; want ErrInvalidArgument", err)
	}
	if _, err := svc.CreateConnection(ops, &webapi.CreateConnectionRequest{ConnectionId: "mine", ServerSecret: "ghp_x", UserToken: "ghp_y",
		Connection: &webapi.Connection{Kind: webapi.ConnectionKind_CONNECTION_KIND_GITHUB_USER, Scope: webapi.ConnectionScope_CONNECTION_SCOPE_PERSON}}); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("a person's connection with server_secret = %v; want ErrInvalidArgument", err)
	}
	noKey := service.NewConnectionService(store, nil)
	if _, err := noKey.CreateConnection(ops, &webapi.CreateConnectionRequest{ConnectionId: "nokey", ServerSecret: "ghp_x",
		Connection: &webapi.Connection{Kind: webapi.ConnectionKind_CONNECTION_KIND_GITHUB_TOKEN}}); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("a sealed connection on a server with no connections key = %v; want ErrFailedPrecondition", err)
	}

	if err := svc.DeleteConnection(ops, &webapi.DeleteConnectionRequest{Name: "connections/hw"}); err != nil {
		t.Fatal(err)
	}
	if _, kept := keeper.tokens["hw/"]; kept {
		t.Errorf("deleting hw left its sealed secret in the keeper")
	}
}
