package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// Each listed rule links to the published page that explains it, a family rule to its family's page,
// and a project's own rule to none.
func TestListRulesLinksEachRuleToItsPage(t *testing.T) {
	project := check.NewSource("acme", []*check.Rule{{Name: "house-rule", Severity: "warning", Detail: "ours", Eval: func(context.Context, check.Model) []check.Verdict { return nil }}})
	catalog, err := check.DefaultCatalog().With(project)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewCheckService(fakeLoader{}, catalog, nil, "", nil, nil)
	resp, err := svc.ListRules(context.Background(), &webapi.ListRulesRequest{Snapshot: snapOf(t, svc.Snapshots(), "mount://m/d.pdf")})
	if err != nil {
		t.Fatal(err)
	}
	url := map[string]string{}
	var signalMissing string
	for _, r := range resp.GetRules() {
		url[r.GetName()] = r.GetDocUrl()
		if strings.HasPrefix(r.GetName(), "profile/") && strings.HasSuffix(r.GetName(), "-signal-missing") {
			signalMissing = r.GetName()
		}
	}
	if got, want := url["i2c-pull-up"], "https://panyam.github.io/agni/reference/rules/i2c-pull-up/"; got != want {
		t.Errorf("i2c-pull-up links to %q, want %q", got, want)
	}
	if signalMissing == "" {
		t.Fatal("no built-in profile's signal-missing rule is listed")
	}
	if got, want := url[signalMissing], RuleDocsBase+"profile-signal-missing/"; got != want {
		t.Errorf("%s links to %q, want the requirement's page %q", signalMissing, got, want)
	}
	if got, ok := url["acme/house-rule"]; !ok || got != "" {
		t.Errorf("the project's rule: listed %v, linked to %q; want listed with no link", ok, got)
	}
}

// Every link the catalog serves names a page the docsite has, so a renamed or dropped page fails here
// rather than as a 404 in the viewer.
func TestEveryRuleDocURLNamesAPage(t *testing.T) {
	linked := 0
	for _, r := range check.DefaultCatalog().Rules() {
		u := RuleDocURL(r)
		if u == "" {
			continue
		}
		linked++
		page := strings.TrimSuffix(strings.TrimPrefix(u, RuleDocsBase), "/")
		if _, err := os.Stat(filepath.Join("..", "docsite", "content", "reference", "rules", page+".md")); err != nil {
			t.Errorf("%s links to %s, which the docsite has no page for: %v", r.Name, u, err)
		}
	}
	if linked < 50 {
		t.Errorf("only %d rules link to a page; the built-ins alone have more", linked)
	}
}
