package service

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// Webhook is a GitHub webhook or a Microsoft Graph subscription a WebhookMaker made or updated
// (agni issues 1726, 1729).
type Webhook struct {
	// Source is the repo or folder the hook watches, such as "github://acme/board".
	Source string
	// ID is GitHub's id for a hook, and 0 for a Graph subscription.
	ID int64
	// SubscriptionID is Graph's id for a subscription, and empty for a GitHub hook. Expires is when
	// Graph ends the subscription unless the server renews it first.
	SubscriptionID string
	Expires        time.Time
	// URL is where the hook sends deliveries, and Events the events it sends.
	URL    string
	Events []string
	// Created is true when the hook is new, and false when one sending to URL was updated.
	Created bool
}

// WebhookSecrets are the secrets the server checks deliveries with: GitHub's for a push webhook
// (--github-webhook-secret), and the clientState Graph echoes in each notification
// (--msgraph-client-state). An empty one means the server takes no deliveries of that kind.
type WebhookSecrets struct {
	GitHub, Graph []byte
}

// WebhookMaker asks a mount's host to tell the server when the mount's source changes (agni issues
// 1726, 1729): a push webhook on a GitHub repo, or a change-notification subscription on a
// SharePoint or OneDrive drive. It classifies its errors with this package's sentinels:
// ErrNotFound for an unknown mount, ErrFailedPrecondition for a mount it can't watch or a secret
// the server lacks, and ErrPermissionDenied for a connection the host won't let manage hooks.
type WebhookMaker interface {
	// CreateWebhook makes a hook or a subscription that sends mount's changes to url, signed with the
	// matching secret, or updates the one already sending to url. It reads through the mount's
	// connection as the caller in ctx.
	CreateWebhook(ctx context.Context, mount, url string, secrets WebhookSecrets) (Webhook, error)
}

// WithWebhooks lets CreateMountWebhook make hooks and subscriptions through m, with secrets. With
// both secrets empty every call is refused, since the server would refuse every delivery.
func (s *WorkspaceService) WithWebhooks(m WebhookMaker, secrets WebhookSecrets) *WorkspaceService {
	s.hooks, s.hookSecrets = m, secrets
	return s
}

// CreateMountWebhook asks GitHub to send a github mount's pushes to the server (agni issue 1726),
// or Graph to send a SharePoint mount's drive changes to it (agni issue 1729).
func (s *WorkspaceService) CreateMountWebhook(ctx context.Context, req *webapi.CreateMountWebhookRequest) (*webapi.MountWebhook, error) {
	if s.hooks == nil || (len(s.hookSecrets.GitHub) == 0 && len(s.hookSecrets.Graph) == 0) {
		return nil, fmt.Errorf("%w: this server takes no webhooks; start it with --github-webhook-secret or --msgraph-client-state", ErrFailedPrecondition)
	}
	u, err := url.Parse(req.GetUrl())
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, fmt.Errorf("%w: url %q must be the server's /hooks/github or /hooks/msgraph as the host reaches it, such as https://agni.example.com/hooks/github", ErrInvalidArgument, req.GetUrl())
	}
	if s.guard != nil {
		if err := s.guard.Mount(ctx, req.GetName()); err != nil {
			return nil, err
		}
	}
	h, err := s.hooks.CreateWebhook(ctx, req.GetName(), u.String(), s.hookSecrets)
	if err != nil {
		return nil, err
	}
	out := &webapi.MountWebhook{Mount: req.GetName(), Source: h.Source, Id: h.ID, Url: h.URL, Events: h.Events, Created: h.Created, SubscriptionId: h.SubscriptionID}
	if !h.Expires.IsZero() {
		out.ExpireTime = timestamppb.New(h.Expires)
	}
	return out, nil
}
