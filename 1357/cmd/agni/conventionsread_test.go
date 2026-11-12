package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/ir"
)

// sigIsARail writes a convention reading the fixture's SIG net as a rail, which the built-ins do not.
func sigIsARail(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "conv.yaml")
	body := "name: sig-rail\nlexicon:\n  net:\n    rail:\n      patterns: [\"^SIG\"]\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func hasRole(n *ir.Net, role ir.Role) bool {
	for _, r := range n.GetRoles() {
		if r.GetRoleKind() == role {
			return true
		}
	}
	return false
}

const conventionFixture = "testdata/conformance/dupname.fires.edn"

// stats reads through both paths, the summary through readDesign and json through a snapshot, and
// diff's text and json formats split the same way, so a convention has to reach both.
func TestConventionsReachThePlainRead(t *testing.T) {
	roleOf := func(t *testing.T) bool {
		d, err := readDesign(conventionFixture)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		rail := false
		for _, n := range d.GetNets() {
			if strings.HasPrefix(n.GetName(), "SIG") {
				seen++
				rail = rail || hasRole(n, ir.Role_ROLE_RAIL)
			}
		}
		if seen == 0 {
			t.Fatal("the fixture has no SIG net, so this test proves nothing")
		}
		return rail
	}
	if roleOf(t) {
		t.Fatal("SIG is a rail with no convention, so the control proves nothing")
	}
	reset, err := withConventions(sigIsARail(t))
	if err != nil {
		t.Fatal(err)
	}
	if !roleOf(t) {
		t.Error("SIG is not a rail under the convention")
	}
	reset()
	if roleOf(t) {
		t.Error("the convention outlived its command")
	}
}

func TestStatsConventionsReachesTheSnapshot(t *testing.T) {
	cmd := statsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--format", "json", "--mask", "design.nets", "--conventions", sigIsARail(t), conventionFixture})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Design struct {
			Nets []struct {
				Name  string `json:"name"`
				Roles []struct {
					RoleKind string `json:"roleKind"`
				} `json:"roles"`
			} `json:"nets"`
		} `json:"design"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	rail := false
	for _, n := range got.Design.Nets {
		if strings.HasPrefix(n.Name, "SIG") {
			for _, r := range n.Roles {
				rail = rail || r.RoleKind == "ROLE_RAIL"
			}
		}
	}
	if !rail {
		t.Errorf("SIG carries no rail role in stats --format json:\n%s", out.String())
	}
}

func TestDiffAndRenderRefuseAnUnreadableConventionsFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "none.yaml")
	for name, run := range map[string]func() error{
		"diff": func() error {
			c := diffCmd()
			c.SetArgs([]string{"--conventions", missing, conventionFixture, conventionFixture})
			return c.Execute()
		},
		"render": func() error {
			c := renderCmd()
			c.SetArgs([]string{"--conventions", missing, conventionFixture})
			return c.Execute()
		},
	} {
		if run() == nil {
			t.Errorf("%s took a conventions file that does not exist", name)
		}
	}
}
