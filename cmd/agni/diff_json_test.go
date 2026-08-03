package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestDiffJSON pins the `diff --format json` wire contract (the DiffDesignsResponse shape the
// viewer's DiffService also serves, WS9-004) over the rev-a/rev-b fixture pair: the report's
// classified changes plus the highlight maps, with a renamed net keyed under both names.
func TestDiffJSON(t *testing.T) {
	cmd := diffCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--format", "json", "testdata/rev-a.edn", "testdata/rev-b.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff --format json: %v", err)
	}

	var got struct {
		Report struct {
			ComponentsAdded []string `json:"componentsAdded"`
			Nets            []struct {
				Kind    string `json:"kind"`
				Name    string `json:"name"`
				OldName string `json:"oldName"`
			} `json:"nets"`
		} `json:"report"`
		ComponentStatus map[string]string `json:"componentStatus"`
		NetStatus       map[string]string `json:"netStatus"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if len(got.Report.ComponentsAdded) != 1 || got.Report.ComponentsAdded[0] != "R4" {
		t.Errorf("componentsAdded = %v, want [R4]", got.Report.ComponentsAdded)
	}
	renamed := false
	for _, nc := range got.Report.Nets {
		if nc.Kind == "renamed" && nc.Name == "DATA" && nc.OldName == "SIG" {
			renamed = true
		}
	}
	if !renamed {
		t.Errorf("nets missing the SIG->DATA rename: %+v", got.Report.Nets)
	}
	if got.ComponentStatus["R4"] != "added" {
		t.Errorf("componentStatus[R4] = %q, want added", got.ComponentStatus["R4"])
	}
	if got.NetStatus["SIG"] != "renamed" || got.NetStatus["DATA"] != "renamed" {
		t.Errorf("netStatus rename keys = %q/%q, want renamed/renamed", got.NetStatus["SIG"], got.NetStatus["DATA"])
	}
	if got.NetStatus["CLK"] != "hard" || got.NetStatus["OLD"] != "deleted" || got.NetStatus["NEW"] != "new" {
		t.Errorf("netStatus = %v, want CLK hard / OLD deleted / NEW new", got.NetStatus)
	}
	if _, ok := got.NetStatus["VCC"]; ok {
		t.Error("unchanged net VCC must not appear in the highlight map")
	}
}

// TestDiffTextDefault: without --format the human text output is unchanged (the summary
// header the existing consumers read).
func TestDiffTextDefault(t *testing.T) {
	cmd := diffCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"testdata/rev-a.edn", "testdata/rev-b.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("Components:")) {
		t.Errorf("text output missing summary header:\n%s", out.String())
	}
}
