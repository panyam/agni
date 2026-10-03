package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// reboundCANProject writes a project whose CAN bus is named _BUSH/_BUSL/_BTX, with its own CAN profile
// re-binding the built-in's signals to those suffixes, and returns the design folder. Against the
// built-in CAN profile the bus is not in use at all, so a surface reading the built-in describes a
// different interface from the one the project's rules check.
func reboundCANProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile("testdata/review/can-broken.edn")
	if err != nil {
		t.Fatal(err)
	}
	design := strings.NewReplacer("CAN_CANH", "CAN_BUSH", "CAN_CANL", "CAN_BUSL", "CAN_TXD", "CAN_BTX").Replace(string(b))
	write("designs/d/board.edn", design)
	write("designs/d/design.yaml", "name: d\ntitle: D\nentry: board.edn\n")
	write("profiles/can.yaml", `name: CAN
signals:
  - {name: CANH, suffix: _BUSH, anchor: true}
  - {name: CANL, suffix: _BUSL}
  - {name: TXD,  suffix: _BTX}
requirements:
  - {type: signal-missing}
  - {type: signal-dangling}
`)
	write("project.yaml", `name: rebound
title: Rebound
checklists:
  review:
    name: rebound review
    areas:
      - name: CAN
        items:
          - {id: "c1", title: bus signals present, profile: CAN}
`)
	return filepath.Join(root, "designs", "d")
}

// The coverage panel walks the profiles the run's rules came from, so a project's own CAN profile is
// the one it describes, and the bus it re-binds shows its nets (agni issue 833). Against the built-in
// CAN profile the panel listed no CAN interface on this board at all.
func TestCoverageReadsTheProjectsProfiles(t *testing.T) {
	design := reboundCANProject(t)
	uri, err := cliArgURI(design)
	if err != nil {
		t.Fatal(err)
	}
	svc := service.NewCheckService(&localLoader{loader: newLoader()}, check.DefaultCatalog(), nil, "", nil, cliProjects())
	resp, err := svc.GetInterfaceCoverage(context.Background(), &webapi.GetInterfaceCoverageRequest{Uri: uri})
	if err != nil {
		t.Fatal(err)
	}
	var can *webapi.InterfaceCoverage
	for _, ic := range resp.GetInterfaces() {
		if ic.GetProfile() == "CAN" {
			can = ic
		}
	}
	if can == nil {
		t.Fatalf("the project's CAN profile should be detected on its re-bound bus, got %v", resp.GetInterfaces())
	}
	nets := map[string]string{}
	for _, s := range can.GetSignals() {
		nets[s.GetName()] = s.GetNet()
	}
	if nets["CANH"] != "CAN_BUSH" || nets["TXD"] != "CAN_BTX" {
		t.Errorf("coverage should match the project's suffixes, got %v", nets)
	}
}

// A review's interface-presence gate asks about the project's own profile, so an item bound to a
// re-bound interface is scored rather than read not-applicable against the built-in's suffixes
// (agni issue 833).
func TestReviewPresenceGateReadsTheProjectsProfiles(t *testing.T) {
	design := reboundCANProject(t)
	out, errOut, err := runReviewCapturing(t, design, "--format", "json")
	if err != nil {
		t.Fatalf("review: %v\n%s", err, errOut)
	}
	// protojson varies the spaces after a colon between runs, so match on the fields alone.
	compact := strings.Join(strings.Fields(out), "")
	i := strings.Index(compact, `"id":"c1"`)
	if i < 0 {
		t.Fatalf("item c1 missing from the review:\n%s", out)
	}
	item := compact[i:min(len(compact), i+300)]
	if !strings.Contains(item, `"outcome":"fail"`) || !strings.Contains(item, "rebound-profiles/can-signal-dangling") {
		t.Errorf("the project's CAN profile is in use and its bus dangles, so c1 must fail on the project's rule:\n%s", item)
	}
}
