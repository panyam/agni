package mountset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/panyam/goapplib/storage/mounts"

	"github.com/panyam/agni/service"
)

// subscriptionLife is how long the server asks Graph to keep a drive subscription. Graph caps one
// on a driveItem at 42300 minutes, a little over 29 days.
const subscriptionLife = 29 * 24 * time.Hour

// subscriptionRecord is what the server keeps for one SharePoint mount's subscription, as
// <mount>.json under Config.Subscriptions.
type subscriptionRecord struct {
	Mount   string    `json:"mount"`
	ID      string    `json:"id"`
	URL     string    `json:"url"`
	Expires time.Time `json:"expires"`
}

// credentials gives m's connection's credentials, as the registry does for a read.
func (t *Table) credentials(ctx context.Context, m mounts.Mount) (mounts.Credentials, error) {
	if t.svc == nil || m.Connection == "" {
		return nil, fmt.Errorf("%w: the mount %q reads through no connection the server keeps", service.ErrFailedPrecondition, m.Name)
	}
	return t.svc.Credentials(ctx, m.ConnectionOwner, m.Connection)
}

// subscribe asks Graph to send m's drive changes to url with clientState, or renews the
// subscription the server already keeps for m at url (agni issue 1729). Graph checks url before it
// answers, so the server's /hooks/msgraph must be serving there already.
func (t *Table) subscribe(ctx context.Context, m mounts.Mount, url string, clientState []byte) (service.Webhook, error) {
	switch {
	case len(clientState) == 0:
		return service.Webhook{}, fmt.Errorf("%w: this server takes no Graph notifications; start it with --msgraph-client-state", service.ErrFailedPrecondition)
	case t.subs == nil || t.graph == nil:
		return service.Webhook{}, fmt.Errorf("%w: this server keeps no subscriptions; start it with --store and --uploads", service.ErrFailedPrecondition)
	}
	creds, err := t.credentials(viewing(ctx), m)
	if err != nil {
		return service.Webhook{}, service.ConnectionErr(err)
	}
	expires := time.Now().Add(subscriptionLife)
	if rec, err := t.subscription(ctx, m.Name); err == nil && rec.URL == url {
		if sub, err := t.graph.RenewSubscription(ctx, creds, rec.ID, expires); err == nil {
			rec.Expires = sub.Expires
			if err := t.keepSubscription(ctx, rec); err != nil {
				return service.Webhook{}, err
			}
			return webhookOf(m, rec, false), nil
		}
		// Graph forgot it, so a new one replaces it.
	}
	sub, err := t.graph.CreateSubscription(ctx, m, creds, url, string(clientState), expires)
	if err != nil {
		return service.Webhook{}, service.ConnectionErr(Err(err))
	}
	if old, err := t.subscription(ctx, m.Name); err == nil && old.ID != sub.ID {
		_ = t.graph.DeleteSubscription(ctx, creds, old.ID)
	}
	rec := subscriptionRecord{Mount: m.Name, ID: sub.ID, URL: url, Expires: sub.Expires}
	if err := t.keepSubscription(ctx, rec); err != nil {
		return service.Webhook{}, err
	}
	return webhookOf(m, rec, true), nil
}

func webhookOf(m mounts.Mount, rec subscriptionRecord, created bool) service.Webhook {
	return service.Webhook{Source: m.Source, SubscriptionID: rec.ID, URL: rec.URL, Events: []string{"updated"}, Created: created, Expires: rec.Expires}
}

func (t *Table) subscription(ctx context.Context, mount string) (subscriptionRecord, error) {
	b, err := t.subs.Get(ctx, mount+".json")
	if err != nil {
		return subscriptionRecord{}, err
	}
	var rec subscriptionRecord
	return rec, json.Unmarshal(b, &rec)
}

func (t *Table) keepSubscription(ctx context.Context, rec subscriptionRecord) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return t.subs.Put(ctx, rec.Mount+".json", b)
}

// RenewSubscriptions renews every Graph subscription the server keeps that ends within before, and
// returns how many it renewed (agni issue 1729). One Graph no longer knows is made again at the same
// URL. A mount that's gone takes its record with it. It logs each subscription it can't renew to
// log and goes on to the next.
func (t *Table) RenewSubscriptions(ctx context.Context, before time.Duration, clientState []byte, log io.Writer) (int, error) {
	if t.subs == nil || t.graph == nil {
		return 0, nil
	}
	ents, err := t.subs.List(ctx, "")
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range ents {
		if e.Dir {
			continue
		}
		var rec subscriptionRecord
		b, err := t.subs.Get(ctx, e.Name)
		if err == nil {
			err = json.Unmarshal(b, &rec)
		}
		if err != nil || time.Until(rec.Expires) > before {
			continue
		}
		m, err := t.reg.Get(ctx, rec.Mount)
		if errors.Is(err, mounts.ErrNotFound) {
			_ = t.subs.Delete(ctx, e.Name)
			continue
		}
		if err == nil {
			_, err = t.subscribe(ctx, m, rec.URL, clientState)
		}
		if err != nil {
			fmt.Fprintf(log, "note: renewing the Graph subscription on %s: %v\n", rec.Mount, err)
			continue
		}
		n++
	}
	return n, nil
}

// unsubscribe ends mount's Graph subscription, if the server keeps one, and forgets it. A
// subscription Graph can't end now runs out on its own within 30 days.
func (t *Table) unsubscribe(ctx context.Context, m mounts.Mount) {
	if t.subs == nil || t.graph == nil {
		return
	}
	rec, err := t.subscription(ctx, m.Name)
	if err != nil {
		return
	}
	if creds, err := t.credentials(viewing(ctx), m); err == nil {
		_ = t.graph.DeleteSubscription(ctx, creds, rec.ID)
	}
	_ = t.subs.Delete(ctx, m.Name+".json")
}
