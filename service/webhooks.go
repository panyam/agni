package service

import (
	"context"
	"fmt"
	"net/url"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// Webhook is a GitHub webhook a WebhookMaker made or updated (agni issue 1726).
type Webhook struct {
	// Source is the repo the hook is on, such as "github://acme/board".
	Source string
	// ID is GitHub's id for the hook.
	ID int64
	// URL is where the hook sends deliveries, and Events the events it sends.
	URL    string
	Events []string
	// Created is true when the hook is new, and false when one sending to URL was updated.
	Created bool
}

// WebhookMaker asks a mount's host to send the mount's pushes to a URL (agni issue 1726). It
// classifies its errors with this package's sentinels: ErrNotFound for an unknown mount,
// ErrFailedPrecondition for a mount it can't make a hook for, and ErrPermissionDenied for a token
// the host won't let manage hooks.
type WebhookMaker interface {
	// CreateWebhook makes a hook on mount's repo that sends pushes to url, signed with secret, or
	// updates the hook already sending to url. It reads through the mount's connection as the caller
	// in ctx.
	CreateWebhook(ctx context.Context, mount, url string, secret []byte) (Webhook, error)
}

// WithWebhooks lets CreateMountWebhook make hooks through m, signed with secret, the server's
// --github-webhook-secret. An empty secret refuses every call, since the server would refuse every
// delivery.
func (s *WorkspaceService) WithWebhooks(m WebhookMaker, secret []byte) *WorkspaceService {
	s.hooks, s.hookSecret = m, secret
	return s
}

// CreateMountWebhook asks GitHub to send a github mount's pushes to the server (agni issue 1726).
func (s *WorkspaceService) CreateMountWebhook(ctx context.Context, req *webapi.CreateMountWebhookRequest) (*webapi.MountWebhook, error) {
	if s.hooks == nil || len(s.hookSecret) == 0 {
		return nil, fmt.Errorf("%w: this server takes no webhooks; start it with --github-webhook-secret", ErrFailedPrecondition)
	}
	u, err := url.Parse(req.GetUrl())
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, fmt.Errorf("%w: url %q must be the server's /hooks/github as GitHub reaches it, such as https://agni.example.com/hooks/github", ErrInvalidArgument, req.GetUrl())
	}
	if s.guard != nil {
		if err := s.guard.Mount(ctx, req.GetName()); err != nil {
			return nil, err
		}
	}
	h, err := s.hooks.CreateWebhook(ctx, req.GetName(), u.String(), s.hookSecret)
	if err != nil {
		return nil, err
	}
	return &webapi.MountWebhook{Mount: req.GetName(), Source: h.Source, Id: h.ID, Url: h.URL, Events: h.Events, Created: h.Created}, nil
}
