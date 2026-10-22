package projects

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseDesign(t *testing.T) {
	id, d, _, err := ParseDesign(strings.NewReader(`
name: gateway
title: Sample Board
entry: gateway.edn
companions:
  - gateway.kicad_sch
  - ./gateway.kicad_pcb
`))
	if err != nil {
		t.Fatal(err)
	}
	if id != "gateway" {
		t.Fatalf("id = %q", id)
	}
	if d.GetTitle() != "Sample Board" {
		t.Errorf("title = %q", d.GetTitle())
	}
	// Parsing leaves the refs DESIGN-FOLDER-RELATIVE; the store rewrites them once it knows where the
	// folder sits. The `./` spelling is normalized here so a companion is recognisable by comparison.
	if d.GetEntryUri() != "gateway.edn" {
		t.Errorf("entry ref = %q, want it design-relative", d.GetEntryUri())
	}
	want := []string{"gateway.kicad_sch", "gateway.kicad_pcb"}
	got := d.GetCompanionUris()
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("companion refs = %v, want %v normalized and in declared order", got, want)
	}
	// The name is NOT set, because a bare id is not a resource name, and only the store knows the
	// parent.
	if d.GetName() != "" {
		t.Errorf("name = %q, want it left to the store", d.GetName())
	}
}

// TestParseDesignUntitledFallsBackToID pins that every consumer degrades the same way, so an
// untitled design never renders blank.
func TestParseDesignUntitledFallsBackToID(t *testing.T) {
	_, d, _, err := ParseDesign(strings.NewReader("name: gateway\nentry: gateway.edn\n"))
	if err != nil {
		t.Fatal(err)
	}
	if d.GetTitle() != "gateway" {
		t.Errorf("title = %q, want the id", d.GetTitle())
	}
}

func TestParseDesignReadsFlattenHierarchy(t *testing.T) {
	for src, want := range map[string]bool{
		"name: b\nentry: b.edn\nflatten_hierarchy: true\n": true,
		"name: b\nentry: b.edn\n":                          false,
	} {
		_, d, _, err := ParseDesign(strings.NewReader(src))
		if err != nil {
			t.Fatal(err)
		}
		if d.GetFlattenHierarchy() != want {
			t.Errorf("%q: flatten_hierarchy = %v, want %v", src, d.GetFlattenHierarchy(), want)
		}
	}
}

func TestParseDesignRejects(t *testing.T) {
	cases := []struct{ name, yaml, want string }{
		{"no name", "entry: a.edn\n", "name is required"},
		{"no entry", "name: gateway\n", "entry is required"},
		{"upper id", "name: Gateway\nentry: a.edn\n", "not a valid id"},
		{"slash in id", "name: a/b\nentry: a.edn\n", "not a valid id"},
		{"absolute entry", "name: g\nentry: /etc/passwd\n", "not absolute"},
		{"escaping entry", "name: g\nentry: ../../secrets.edn\n", "stay inside"},
		{"escaping companion", "name: g\nentry: a.edn\ncompanions: ['../b.kicad_pcb']\n", "stay inside"},
		{"entry also companion", "name: g\nentry: a.edn\ncompanions: ['./a.edn']\n", "listed twice"},
		// A misspelled key that silently does nothing is the failure this strictness is for. The
		// operator believes they declared companions and nothing says otherwise.
		{"unknown field", "name: g\nentry: a.edn\ncompanion: [b.kicad_pcb]\n", "field companion not found"},
		{"empty", "", "is empty"},
		// A revision is declared file by file, and a file belongs to one revision at most, so a board
		// is never shared between two netlists that differ (agni issue 848).
		{"revision without entry", "name: g\nentry: a.edn\nrevisions:\n  - companions: [b.kicad_pcb]\n", "revisions[0]: entry is required"},
		{"revision reuses the entry", "name: g\nentry: a.edn\nrevisions:\n  - entry: a.edn\n", "listed twice"},
		{"companion shared across revisions", "name: g\nentry: a.edn\ncompanions: [a.kicad_pcb]\nrevisions:\n  - entry: b.edn\n    companions: [a.kicad_pcb]\n", "listed twice"},
		{"escaping revision", "name: g\nentry: a.edn\nrevisions:\n  - entry: ../b.edn\n", "stay inside"},
		{"unknown revision field", "name: g\nentry: a.edn\nrevisions:\n  - entry: b.edn\n    companion: [c.kicad_pcb]\n", "field companion not found"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, _, err := ParseDesign(strings.NewReader(c.yaml))
			if err == nil {
				t.Fatalf("want an error mentioning %q, got none", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %q, want it to mention %q", err, c.want)
			}
		})
	}
}

func TestParseDesignReadsRevisions(t *testing.T) {
	_, d, _, err := ParseDesign(strings.NewReader("name: g\nentry: a.edn\ncompanions: [a.kicad_pcb]\nrevisions:\n  - entry: ./b.edn\n    companions: [b.kicad_pcb]\n  - entry: c.edn\n"))
	if err != nil {
		t.Fatal(err)
	}
	revs := d.GetRevisions()
	if len(revs) != 2 || revs[0].GetEntryUri() != "b.edn" || len(revs[0].GetCompanionUris()) != 1 || revs[0].GetCompanionUris()[0] != "b.kicad_pcb" ||
		revs[1].GetEntryUri() != "c.edn" || len(revs[1].GetCompanionUris()) != 0 {
		t.Errorf("revisions = %v, want b.edn with b.kicad_pcb, then c.edn alone", revs)
	}
}

func TestParseProject(t *testing.T) {
	id, p, _, err := ParseProject(strings.NewReader("name: gateway\ntitle: Gateway program\n"))
	if err != nil {
		t.Fatal(err)
	}
	if id != "gateway" || p.GetTitle() != "Gateway program" {
		t.Fatalf("id = %q, project = %+v", id, p)
	}
	if p.GetName() != "" {
		t.Errorf("name = %q, want it left to the store", p.GetName())
	}
	if _, _, _, err := ParseProject(strings.NewReader("name: Gateway\n")); err == nil {
		t.Error("an id that is not a valid resource-name segment should be rejected")
	}
	if _, _, _, err := ParseProject(strings.NewReader("naem: gateway\n")); err == nil {
		t.Error("an unknown field should be rejected")
	}
}

// One schema serves both descriptors (agni issue 1136), and the file's name says which role it plays,
// so a key naming a design's own files is refused in a project with where it goes.
func TestParseProjectRefusesADesignsKeys(t *testing.T) {
	for _, doc := range []string{
		"name: p\nentry: a.edn\n",
		"name: p\ncompanions: [a.kicad_pcb]\n",
		"name: p\nrevisions:\n  - entry: b.edn\n",
		"name: p\nflatten_hierarchy: true\n",
	} {
		_, _, _, err := ParseProject(strings.NewReader(doc))
		if err == nil || !strings.Contains(err.Error(), "belongs in that design's design.yaml") {
			t.Errorf("%q: error = %v, want it refused as a design's key", doc, err)
		}
	}
}

// A design declares every section a project can, so a board can override its team's naming or add a
// checklist of its own.
func TestParseDesignReadsEverySection(t *testing.T) {
	_, d, src, err := ParseDesign(strings.NewReader(`
name: b
extends: ../../project.yaml
entry: b.edn
conventions: {name: legacy}
checklists:
  extra: {name: Extra, areas: [{name: A, items: [{id: X1, title: t, note: by hand}]}]}
intent: {modules: [{name: regs, class: regulator, count: 1}]}
profiles: [profiles/can.yaml]
params: [../shared/params]
symbols: [symbols]
lib: [mount://shared/lib]
queries: [queries]
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := d.GetConfig()
	if cfg.GetConventions().GetName() != "legacy" || len(cfg.GetChecklists()) != 1 || len(cfg.GetIntent().GetModules()) != 1 {
		t.Errorf("config = %v, want the design's conventions, its checklist and its intent", cfg)
	}
	want := Sources{Extends: "../../project.yaml", Profiles: []string{"profiles/can.yaml"}, Params: []string{"../shared/params"},
		Symbols: []string{"symbols"}, Lib: []string{"mount://shared/lib"}, Queries: []string{"queries"}}
	if !reflect.DeepEqual(src, want) {
		t.Errorf("sources = %+v, want %+v as written, for the store to resolve", src, want)
	}
}

func TestParseRejectsAMalformedSource(t *testing.T) {
	for doc, want := range map[string]string{
		"name: p\nextends: ../house\n":                    "must name a project.yaml",
		"name: p\nextends: ../house/design.yaml\n":        "must name a project.yaml",
		"name: p\nprofiles: [/etc/profiles]\n":            "not absolute",
		"name: p\nprofiles: ['']\n":                       "empty path",
		"name: p\nparams: [https://example.com/p.yaml]\n": "a source is a path relative to the descriptor or a mount:// URI",
		"name: p\nlib: ['mount://shared/../../etc']\n":    "escapes its mount",
		"name: p\nprofiles: profiles\n":                   "cannot unmarshal",
	} {
		_, _, _, err := ParseProject(strings.NewReader(doc))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: error = %v, want it to mention %q", doc, err, want)
		}
	}
}
