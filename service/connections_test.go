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

// The server keeps AWS, GCP and Entra connections for bucket and SharePoint mounts, sends their
// fields back, and leaves an Entra one's base_url empty rather than the server's GitHub API (agni
// issues 1729, 1730). None can be a person's, and an Entra one names how it signs in.
func TestTheServerKeepsCloudConnections(t *testing.T) {
	store, err := fsstore.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc := service.NewConnectionService(store, nil).WithBaseURL("https://ghe.example.com/api/v3")
	ctx := context.Background()
	for _, c := range []struct {
		id   string
		in   *webapi.Connection
		want func(*webapi.Connection) bool
	}{
		{"aws", &webapi.Connection{Kind: webapi.ConnectionKind_CONNECTION_KIND_AWS, AccessKeyId: "AKIAFAKE", KeySource: "env:AWS_SECRET", RoleArn: "arn:aws:iam::1:role/r", Region: "us-east-1"},
			func(o *webapi.Connection) bool {
				return o.GetKind() == webapi.ConnectionKind_CONNECTION_KIND_AWS && o.GetAccessKeyId() == "AKIAFAKE" && o.GetRoleArn() == "arn:aws:iam::1:role/r" && o.GetRegion() == "us-east-1" && o.GetBaseUrl() == ""
			}},
		{"aws-chain", &webapi.Connection{Kind: webapi.ConnectionKind_CONNECTION_KIND_AWS},
			func(o *webapi.Connection) bool {
				return o.GetKind() == webapi.ConnectionKind_CONNECTION_KIND_AWS && o.GetKeySource() == ""
			}},
		{"gcs", &webapi.Connection{Kind: webapi.ConnectionKind_CONNECTION_KIND_GCP, KeySource: "file:/srv/sa.json"},
			func(o *webapi.Connection) bool {
				return o.GetKind() == webapi.ConnectionKind_CONNECTION_KIND_GCP && o.GetKeySource() == "file:/srv/sa.json"
			}},
		{"sp", &webapi.Connection{Kind: webapi.ConnectionKind_CONNECTION_KIND_ENTRA, TenantId: "contoso", ClientId: "app", EntraAuth: webapi.EntraAuth_ENTRA_AUTH_SECRET, KeySource: "env:ENTRA"},
			func(o *webapi.Connection) bool {
				return o.GetKind() == webapi.ConnectionKind_CONNECTION_KIND_ENTRA && o.GetTenantId() == "contoso" && o.GetClientId() == "app" &&
					o.GetEntraAuth() == webapi.EntraAuth_ENTRA_AUTH_SECRET && o.GetBaseUrl() == ""
			}},
		{"sp-mi", &webapi.Connection{Kind: webapi.ConnectionKind_CONNECTION_KIND_ENTRA, EntraAuth: webapi.EntraAuth_ENTRA_AUTH_MANAGED_IDENTITY},
			func(o *webapi.Connection) bool {
				return o.GetEntraAuth() == webapi.EntraAuth_ENTRA_AUTH_MANAGED_IDENTITY
			}},
	} {
		out, err := svc.CreateConnection(ctx, &webapi.CreateConnectionRequest{ConnectionId: c.id, Connection: c.in})
		if err != nil || !c.want(out) {
			t.Errorf("CreateConnection %s = %v, %v", c.id, out, err)
			continue
		}
		got, err := svc.GetConnection(ctx, &webapi.GetConnectionRequest{Name: "connections/" + c.id})
		if err != nil || !c.want(got) {
			t.Errorf("GetConnection %s = %v, %v; want what was kept", c.id, got, err)
		}
	}
	for _, c := range []struct {
		name string
		in   *webapi.Connection
	}{
		{"an Entra connection with no entra_auth", &webapi.Connection{Kind: webapi.ConnectionKind_CONNECTION_KIND_ENTRA, TenantId: "contoso", ClientId: "app", KeySource: "env:E"}},
		{"an AWS access key with no secret", &webapi.Connection{Kind: webapi.ConnectionKind_CONNECTION_KIND_AWS, AccessKeyId: "AKIAFAKE"}},
		{"a person's AWS connection", &webapi.Connection{Kind: webapi.ConnectionKind_CONNECTION_KIND_AWS, Scope: webapi.ConnectionScope_CONNECTION_SCOPE_PERSON}},
	} {
		if _, err := svc.CreateConnection(ctx, &webapi.CreateConnectionRequest{ConnectionId: "bad", Connection: c.in}); !errors.Is(err, service.ErrInvalidArgument) {
			t.Errorf("%s = %v; want INVALID_ARGUMENT", c.name, err)
		}
	}
}
