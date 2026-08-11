package project

import (
	"strings"
	"testing"
)

func TestLoadDesign(t *testing.T) {
	d, err := LoadDesign(strings.NewReader(`
name: gateway
title: Gateway ECU
entry: gateway.edn
companions:
  - gateway.kicad_sch
  - ./gateway.kicad_pcb
`))
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "gateway" || d.Entry != "gateway.edn" {
		t.Fatalf("design = %+v", d)
	}
	if d.DisplayName() != "Gateway ECU" {
		t.Errorf("DisplayName = %q, want the title", d.DisplayName())
	}
	if !d.IsEntry("./gateway.edn") {
		t.Error("IsEntry should compare on the cleaned name")
	}
	// The `./` spelling in the descriptor must not make the companion unrecognisable, since the
	// caller's path never carries it.
	if !d.IsCompanion("gateway.kicad_pcb") {
		t.Error("IsCompanion should compare on the cleaned name")
	}
	if d.IsCompanion("gateway-rev-b.edn") {
		t.Error("an undeclared sibling is not a companion")
	}
}

// TestLoadDesignUntitledFallsBackToID: every consumer degrades the same way, so an untitled design
// never renders blank.
func TestLoadDesignUntitledFallsBackToID(t *testing.T) {
	d, err := LoadDesign(strings.NewReader("name: gateway\nentry: gateway.edn\n"))
	if err != nil {
		t.Fatal(err)
	}
	if d.DisplayName() != "gateway" {
		t.Errorf("DisplayName = %q, want the id", d.DisplayName())
	}
}

func TestLoadDesignRejects(t *testing.T) {
	cases := []struct{ name, yaml, want string }{
		{"no name", "entry: a.edn\n", "name is required"},
		{"no entry", "name: gateway\n", "entry is required"},
		{"upper id", "name: Gateway\nentry: a.edn\n", "not a valid id"},
		{"slash in id", "name: a/b\nentry: a.edn\n", "not a valid id"},
		{"absolute entry", "name: g\nentry: /etc/passwd\n", "not absolute"},
		{"escaping entry", "name: g\nentry: ../../secrets.edn\n", "stay inside"},
		{"escaping companion", "name: g\nentry: a.edn\ncompanions: ['../b.kicad_pcb']\n", "stay inside"},
		{"entry also companion", "name: g\nentry: a.edn\ncompanions: ['./a.edn']\n", "listed twice"},
		// A misspelled key that silently does nothing is the failure this strictness is for: the
		// operator believes they declared companions and nothing says otherwise.
		{"unknown field", "name: g\nentry: a.edn\ncompanion: [b.kicad_pcb]\n", "field companion not found"},
		{"empty", "", "is empty"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := LoadDesign(strings.NewReader(c.yaml))
			if err == nil {
				t.Fatalf("want an error mentioning %q, got none", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %q, want it to mention %q", err, c.want)
			}
		})
	}
}

func TestLoadProject(t *testing.T) {
	p, err := LoadProject(strings.NewReader("name: gateway\ntitle: Gateway program\n"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "gateway" || p.DisplayName() != "Gateway program" {
		t.Fatalf("project = %+v", p)
	}
	if _, err := LoadProject(strings.NewReader("name: Gateway\n")); err == nil {
		t.Error("an id that is not a valid resource-name segment should be rejected")
	}
	if _, err := LoadProject(strings.NewReader("naem: gateway\n")); err == nil {
		t.Error("an unknown field should be rejected")
	}
}
