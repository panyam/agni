package service_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/panyam/goapplib/storage/connections/fsstore"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// fakeKeeper is a service.UserTokenKeeper holding tokens in memory, by connection and owner.
type fakeKeeper struct {
	tokens map[string]string
	fail   error
}

func (k *fakeKeeper) SaveUserToken(_ context.Context, id, owner, token string) error {
	if k.fail != nil {
		return k.fail
	}
	k.tokens[id+"/"+owner] = token
	return nil
}

func (k *fakeKeeper) ForgetUserToken(_ context.Context, id, owner string) error {
	delete(k.tokens, id+"/"+owner)
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
	keeper := &fakeKeeper{tokens: map[string]string{}}
	svc := service.NewConnectionService(store, nil).WithUserTokens(keeper)
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
