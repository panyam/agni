package deploy

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/panyam/agni/service"
)

const guide = "../docsite/content/guide/deploying.md"

// The enterprise guide names the bundle's .env lines and the chart's values an admin sets. Each must
// still exist, so a renamed variable or value fails here rather than in an admin's install (agni
// issue 1804).
func TestTheEnterpriseGuideNamesSettingsThatExist(t *testing.T) {
	page, err := os.ReadFile(guide)
	if err != nil {
		t.Fatal(err)
	}
	env, err := os.ReadFile("enterprise/env.example")
	if err != nil {
		t.Fatal(err)
	}
	vars := regexp.MustCompile(`\bAGNI_[A-Z_]+\b`).FindAllString(string(page), -1)
	if len(vars) < 10 {
		t.Fatalf("the guide names %d AGNI_ variables; want the bundle's settings, so this test proves something", len(vars))
	}
	for _, v := range vars {
		if !regexp.MustCompile(`(?m)^#?\s*` + v + `=`).Match(env) {
			t.Errorf("the guide names %s, which deploy/enterprise/env.example doesn't set", v)
		}
	}

	raw, err := os.ReadFile("helm/agni/values.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]any
	if err := yaml.Unmarshal(raw, &values); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, m := range regexp.MustCompile(`--set '?([a-zA-Z][a-zA-Z.]*)=`).FindAllStringSubmatch(string(page), -1) {
		keys = append(keys, m[1])
	}
	for _, m := range regexp.MustCompile("`((?:oidc|github|datasheets|ingress|persistence)\\.[a-zA-Z.]+)`").FindAllStringSubmatch(string(page), -1) {
		keys = append(keys, m[1])
	}
	for _, m := range regexp.MustCompile("`(trustedProxies|connectionSecrets|caConfigMap|secretName|store|uploads|publicURL|bootstrap|serve|localAccounts)`").FindAllStringSubmatch(string(page), -1) {
		keys = append(keys, m[1])
	}
	if len(keys) < 10 {
		t.Fatalf("the guide names %d chart values; want the chart's settings, so this test proves something", len(keys))
	}
	for _, k := range keys {
		if !hasKey(values, k) {
			t.Errorf("the guide names the chart value %s, which deploy/helm/agni/values.yaml doesn't hold", k)
		}
	}
}

// hasKey reports whether the dotted key k is in values, holding any value, empty included.
func hasKey(values map[string]any, k string) bool {
	parts := strings.Split(k, ".")
	var at any = values
	for i, part := range parts {
		m, ok := at.(map[string]any)
		if !ok {
			return false
		}
		v, ok := m[part]
		if !ok {
			return false
		}
		if i == len(parts)-1 {
			return true
		}
		at = v
	}
	return false
}

// The guide's bootstrap file parses as the plan agni serve --bootstrap reads (agni issue 1808).
func TestTheGuidesBootstrapFileParses(t *testing.T) {
	page, err := os.ReadFile(guide)
	if err != nil {
		t.Fatal(err)
	}
	_, section, ok := strings.Cut(string(page), "## Declare what the server holds\n")
	if !ok {
		t.Fatal("the guide has no section \"Declare what the server holds\"")
	}
	_, block, ok := strings.Cut(section, "```yaml\n")
	block, _, closed := strings.Cut(block, "```")
	if !ok || !closed {
		t.Fatal("the section has no yaml block")
	}
	plan, err := service.ParseBootstrap([]byte(block))
	if err != nil {
		t.Fatalf("the guide's bootstrap file: %v", err)
	}
	if len(plan.Connections) != 1 || len(plan.Mounts) != 2 || len(plan.Packs) != 1 || len(plan.Designs) != 1 || len(plan.Projects) != 1 {
		t.Errorf("the guide's bootstrap file = %+v; want one connection, two mounts, a pack, a design and a project", plan)
	}
}
