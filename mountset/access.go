package mountset

import (
	"context"
	"errors"
	"fmt"

	"github.com/panyam/goapplib/storage/connections"
	"github.com/panyam/goapplib/storage/mounts"
	"golang.org/x/oauth2"

	"github.com/panyam/agni/service"
)

// UserTokensDir is the folder under --store that holds people's own tokens, each sealed under
// --connections-key (agni issue 1728).
const UserTokensDir = "usertokens"

// Guarding returns the private source a mount reads, its github:// source when it reads through a
// connection, and "" for any other mount (agni issue 1728). A mount that needs no connection reads
// a public repo or a folder on the server, which every caller the server lets in may read.
func (t *Table) Guarding(mount string) string {
	m, ok := t.Find(mount)
	if !ok || m.Kind != mounts.KindGitHub || m.Connection == "" {
		return ""
	}
	return m.Source
}

// privateMount reports whether the registry's mount name reads through a connection, whether or not
// List shows it.
func (t *Table) privateMount(name string) bool {
	m, err := t.reg.Get(context.Background(), name)
	return err == nil && m.Kind == mounts.KindGitHub && m.Connection != ""
}

// CanRead asks GitHub, through the caller's own connections, whether the caller can read source. A
// server connection never answers for a person, since it can read what the server can.
func (t *Table) CanRead(ctx context.Context, source string) error {
	subject := service.CallerFrom(ctx).Subject
	if subject == "" {
		return fmt.Errorf("%w: %s is private, and this request has no caller to check it for", service.ErrPermissionDenied, source)
	}
	if t.svc == nil {
		return fmt.Errorf("%w: %s is private, and this server keeps no connection of yours to check it with", service.ErrPermissionDenied, source)
	}
	_, err := t.svc.CanRead(connections.WithViewer(ctx, subject), source)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, connections.ErrNoAccess):
		return fmt.Errorf("%w: %s is private, and no connection of yours can read it; add your own with agni connections add <id> --mine --github-token env:NAME", service.ErrPermissionDenied, source)
	case errors.Is(err, connections.ErrReconnect):
		return fmt.Errorf("%w: %s is private, and your connection's token no longer works; add it again", service.ErrPermissionDenied, source)
	}
	return fmt.Errorf("%w: asking whether you can read %s: %v", service.ErrUnavailable, source, err)
}

var _ service.SourceAccess = (*Table)(nil)

// SaveUserToken seals token as owner's own connection id, for service.UserTokenKeeper. The person
// pasted it, so it is kept as pasted, and a disconnect deletes it without asking GitHub to revoke
// it. The server needs --connections-key to seal it.
func (t *Table) SaveUserToken(ctx context.Context, id, owner, token string) error {
	if t.svc == nil || t.users == nil {
		return fmt.Errorf("%w: this server keeps no person's token; start it with --connections-key env:NAME", service.ErrFailedPrecondition)
	}
	return service.ConnectionErr(t.svc.SavePastedToken(connections.WithViewer(ctx, owner), id, &oauth2.Token{AccessToken: token, TokenType: "bearer"}))
}

// Disconnect removes connection id and its owner's sealed token, for service.UserTokenKeeper,
// through goapplib's Service.Disconnect (agni issue 1776). A pasted token is deleted with no call to
// GitHub, since the person made it there and revokes it there.
func (t *Table) Disconnect(ctx context.Context, id string) error {
	if t.svc == nil {
		return fmt.Errorf("%w: this server keeps no connections", service.ErrFailedPrecondition)
	}
	return t.svc.Disconnect(ctx, id)
}

var _ service.UserTokenKeeper = (*Table)(nil)

// viewing is ctx carrying the caller as goapplib's viewer, so a person's own connection gives its
// token to its owner's requests.
func viewing(ctx context.Context) context.Context {
	return connections.WithViewer(ctx, service.CallerFrom(ctx).Subject)
}
