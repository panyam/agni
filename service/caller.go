package service

import "context"

// Caller is who made a request, as the server's authentication found it (agni issue 1627). The
// transport puts it in the request's context, so this package reads it without importing an auth
// library (C13).
type Caller struct {
	// Subject is the authenticated principal, such as the name a token was created for. It is
	// empty on a server with no authentication.
	Subject string
}

type callerKey struct{}

// WithCaller returns ctx carrying c.
func WithCaller(ctx context.Context, c Caller) context.Context {
	return context.WithValue(ctx, callerKey{}, c)
}

// CallerFrom returns the caller ctx carries. A server with no authentication puts none there, and
// the zero Caller comes back.
func CallerFrom(ctx context.Context) Caller {
	c, _ := ctx.Value(callerKey{}).(Caller)
	return c
}
