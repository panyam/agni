package common

import "testing"

func TestReadSchematicFixture(t *testing.T) {
	g, err := ReadSchematicFixture("demo-schematic.eds")
	if err != nil {
		t.Fatalf("ReadSchematicFixture: %v", err)
	}
	if len(g.Sheets) == 0 {
		t.Errorf("expected at least one sheet, got 0")
	}
}

func TestLoadSchematicDiskPath(t *testing.T) {
	g, err := LoadSchematic("designs/demo-schematic.eds")
	if err != nil {
		t.Fatalf("LoadSchematic(path): %v", err)
	}
	if len(g.Sheets) == 0 {
		t.Errorf("expected at least one sheet, got 0")
	}
}

func TestLoadSchematicFixtureFallback(t *testing.T) {
	// A bare name that is not a file at the cwd falls back to the embedded fixture.
	if _, err := LoadSchematic("demo-schematic.eds"); err != nil {
		t.Fatalf("LoadSchematic(fixture name): %v", err)
	}
}

func TestLoadSchematicMissing(t *testing.T) {
	if _, err := LoadSchematic("no-such-schematic.eds"); err == nil {
		t.Error("LoadSchematic of a missing path and unknown fixture should error")
	}
}
