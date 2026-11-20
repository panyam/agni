package mountset

import (
	"context"
	"fmt"
	"strconv"

	"github.com/panyam/goapplib/storage/mounts"

	"github.com/panyam/agni/service"
)

var _ service.WebhookMaker = (*Table)(nil)

// CreateWebhook makes or updates the push webhook on a github mount's repo, for
// service.WebhookMaker (agni issue 1726), through goapplib's Registry.EnsureWebhook. It reads
// through the mount's connection as the caller, so a connection that is one person's own works only
// for that person. GitHub lets a token manage a repo's hooks only with admin rights on the repo, or
// a fine-grained token's webhooks permission, and answers any other token with PERMISSION_DENIED.
func (t *Table) CreateWebhook(ctx context.Context, name, url string, secret []byte) (service.Webhook, error) {
	m, err := t.reg.Get(ctx, name)
	if err != nil {
		return service.Webhook{}, Err(err)
	}
	if m.Kind != mounts.KindGitHub {
		return service.Webhook{}, fmt.Errorf("%w: the mount %q isn't a GitHub repo, so it has no webhooks", service.ErrFailedPrecondition, name)
	}
	if m.Connection == "" || t.svc == nil {
		return service.Webhook{}, fmt.Errorf("%w: the mount %q reads with no connection, and a webhook needs a token allowed to manage the repo's hooks; mount it again with --connection", service.ErrFailedPrecondition, name)
	}
	h, err := t.reg.EnsureWebhook(viewing(ctx), name, mounts.WebhookRequest{URL: url, Secret: secret, Events: []string{"push"}})
	if err != nil {
		return service.Webhook{}, service.ConnectionErr(Err(err))
	}
	id, err := strconv.ParseInt(h.ID, 10, 64)
	if err != nil {
		return service.Webhook{}, fmt.Errorf("%w: GitHub named webhook %q, not a number", service.ErrUnavailable, h.ID)
	}
	return service.Webhook{Source: m.Source, ID: id, URL: h.URL, Events: h.Events, Created: h.Created}, nil
}
