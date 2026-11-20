package mountset

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/panyam/goapplib/storage/mounts"
	"golang.org/x/oauth2"

	"github.com/panyam/agni/service"
)

var _ service.WebhookMaker = (*Table)(nil)

// hookClient calls GitHub's hooks API. A hook call is small, so it gets a short deadline.
var hookClient = &http.Client{Timeout: 30 * time.Second}

// CreateWebhook makes or updates the push webhook on a github mount's repo, for
// service.WebhookMaker (agni issue 1726). It reads through the mount's connection as the caller, so
// a connection that is one person's own works only for that person. GitHub lets a token manage a
// repo's hooks only with admin rights on the repo, or a fine-grained token's webhooks permission.
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
	repo, ok := strings.CutPrefix(m.Source, "github://")
	if !ok || strings.Count(repo, "/") != 1 {
		return service.Webhook{}, fmt.Errorf("%w: the mount %q names %q, not github://owner/repo", service.ErrFailedPrecondition, name, m.Source)
	}
	ts, err := t.svc.TokenSource(viewing(ctx), m.Connection)
	if err != nil {
		return service.Webhook{}, service.ConnectionErr(err)
	}
	gh := hookAPI{base: strings.TrimSuffix(t.githubAPI, "/") + "/repos/" + repo + "/hooks", ts: ts, repo: repo}
	var hooks []githubHook
	if err := gh.call(ctx, http.MethodGet, "?per_page=100", nil, &hooks); err != nil {
		return service.Webhook{}, err
	}
	body := map[string]any{
		"active": true,
		"events": []string{"push"},
		"config": map[string]any{"url": url, "content_type": "json", "secret": string(secret), "insecure_ssl": "0"},
	}
	out := githubHook{}
	created := true
	for _, h := range hooks {
		if h.Config.URL == url {
			created = false
			if err := gh.call(ctx, http.MethodPatch, fmt.Sprintf("/%d", h.ID), body, &out); err != nil {
				return service.Webhook{}, err
			}
			break
		}
	}
	if created {
		body["name"] = "web"
		if err := gh.call(ctx, http.MethodPost, "", body, &out); err != nil {
			return service.Webhook{}, err
		}
	}
	return service.Webhook{Source: m.Source, ID: out.ID, URL: out.Config.URL, Events: out.Events, Created: created}, nil
}

// githubHook is the part of GitHub's hook object CreateWebhook reads.
type githubHook struct {
	ID     int64    `json:"id"`
	Events []string `json:"events"`
	Config struct {
		URL string `json:"url"`
	} `json:"config"`
}

// hookAPI calls one repo's hooks endpoint with a connection's token.
type hookAPI struct {
	base string
	ts   oauth2.TokenSource
	repo string
}

func (a hookAPI) call(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.base+path, body)
	if err != nil {
		return err
	}
	tok, err := a.ts.Token()
	if err != nil {
		return service.ConnectionErr(err)
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := hookClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: asking GitHub for %s's webhooks: %v", service.ErrUnavailable, a.repo, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized:
		// GitHub answers 404 to a token that can read a repo but not manage its hooks.
		return fmt.Errorf("%w: GitHub won't let the mount's connection manage %s's webhooks (%d); its token needs admin rights on the repo, or a fine-grained token's webhooks permission", service.ErrPermissionDenied, a.repo, resp.StatusCode)
	case resp.StatusCode >= 300:
		return fmt.Errorf("%w: GitHub answered %d to %s %s's webhooks: %s", service.ErrUnavailable, resp.StatusCode, method, a.repo, strings.TrimSpace(string(raw)))
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("%w: reading GitHub's answer about %s's webhooks: %v", service.ErrUnavailable, a.repo, err)
		}
	}
	return nil
}
