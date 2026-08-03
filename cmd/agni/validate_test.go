package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/readers/formats"
)

func TestBuildValidateReport(t *testing.T) {
	l := &formats.Loader{}
	rep, err := buildValidateReport(l, []string{
		"../../readers/edif/testdata/basic.edn",  // netlist-only, healthy
		"../../readers/edif/testdata/sample.eds", // EDIF schematic: netlist + geometry, healthy
		"testdata/conformance/nets_from_wires.fires.kicad_sch", // both tiers
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Failed != 0 || rep.Passed != 3 {
		t.Fatalf("passed/failed = %d/%d, want 3/0: %+v", rep.Passed, rep.Failed, rep.Files)
	}
	byBase := map[string]*webapi.FileValidation{}
	for _, fv := range rep.Files {
		byBase[filepath.Base(fv.Path)] = fv
	}
	// A netlist-only format reports netlist counts and no geometry; an EDIF schematic (.eds)
	// carries BOTH tiers (its schematic view has explicit netlist connectivity).
	if fv := byBase["basic.edn"]; fv.GetNetlist().GetComponents() == 0 || fv.GetGeometry() != nil {
		t.Errorf("basic.edn tiers wrong: %+v", fv)
	}
	if fv := byBase["sample.eds"]; fv.GetNetlist().GetComponents() == 0 || fv.GetGeometry().GetSheets() == 0 {
		t.Errorf("sample.eds tiers wrong (want both netlist + geometry): %+v", fv)
	}
	// A KiCad schematic carries both tiers, and every placement resolves.
	if fv := byBase["nets_from_wires.fires.kicad_sch"]; fv.GetNetlist() == nil || fv.GetGeometry() == nil ||
		fv.GetGeometry().GetResolved() != fv.GetGeometry().GetPlacements() {
		t.Errorf("project.kicad_pro tiers wrong: %+v", fv)
	}
}

func TestValidateExplicitVsWalkedUnknownExtension(t *testing.T) {
	l := &formats.Loader{}
	// Explicitly named: a failure.
	rep, err := buildValidateReport(l, []string{"../../go.mod"})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Failed != 1 || !strings.Contains(rep.Files[0].Problems[0], "no reader") {
		t.Fatalf("explicit unknown ext = %+v, want a no-reader failure", rep.Files[0])
	}
	// Walked: skipped, not failed (conformance dir mixes .edn/.kicad_sch with .yaml sidecars).
	rep, err = buildValidateReport(l, []string{"testdata/conformance"})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Skipped == 0 {
		t.Errorf("walk skipped %d, want the .expect.yaml sidecars skipped", rep.Skipped)
	}
	for _, fv := range rep.Files {
		if strings.HasSuffix(fv.Path, ".yaml") {
			t.Errorf("sidecar validated in a walk: %s", fv.Path)
		}
	}
}

// TestValidateCmdJSONAndExit drives the cobra command end to end: --format json emits the
// canonical ValidateReport protojson, and a failing file makes the command error (non-zero
// exit at main).
func TestValidateCmdJSONAndExit(t *testing.T) {
	cmd := rootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"validate", "--format", "json", "../../readers/edif/testdata/basic.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var rep webapi.ValidateReport
	if err := protojson.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("output is not ValidateReport protojson: %v\n%s", err, out.String())
	}
	if rep.Passed != 1 || len(rep.Files) != 1 || rep.Files[0].GetFormat() != "edif" {
		t.Errorf("report = %+v", &rep)
	}

	// hier.edn has no nets (hierarchy-only fixture), so validate must fail it and error.
	cmd = rootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"validate", "../../readers/edif/testdata/hier.edn"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "1 of 1 file(s) failed") {
		t.Fatalf("failing file must error the command, got %v", err)
	}
}
