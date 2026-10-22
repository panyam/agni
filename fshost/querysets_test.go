package fshost

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// A project's queries/ directory is read as one set per .yaml or .yml file, named by its stem and
// in name order, and anything else in the directory is left alone (agni issue 745).
func TestConfigResolverReadsAProjectsQuerySets(t *testing.T) {
	c := &ConfigResolver{Mounts: []Mount{{Name: "m", FS: MemFS(map[string][]byte{
		"queries/power.yaml": []byte("title: Power\nqueries:\n  - name: Rails\n    query: net.rail(?n) => ?n\n"),
		"queries/audit.yml":  []byte("queries:\n  - name: Parts\n    query: component.mpn(?r, ?m) => ?r, ?m\n"),
		"queries/README.md":  []byte("not a set"),
	})}}}
	got, err := c.ResolveConfig(context.Background(), &webapi.AnalysisConfig{QuerySetUris: []string{"mount://m/queries"}}, "p")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range got.QuerySets {
		names = append(names, s.Name+"="+s.Source)
	}
	if strings.Join(names, " ") != "audit=mount://m/queries/audit.yml power=mount://m/queries/power.yaml" {
		t.Errorf("sets = %v, want audit and power, by stem, with their files", names)
	}
	if got.QuerySets[1].Set.Title != "Power" || got.QuerySets[1].Set.Queries[0].Name != "Rails" {
		t.Errorf("power set = %+v, want its title and its one query", got.QuerySets[1].Set)
	}
}

// A set file that does not parse fails the listing and names the file, rather than leaving the set
// out, which would read as the project having no such set.
func TestConfigResolverRefusesASetThatDoesNotParse(t *testing.T) {
	c := &ConfigResolver{Mounts: []Mount{{Name: "m", FS: MemFS(map[string][]byte{
		"queries/bad.yaml": []byte("queries:\n  - name: Rails\n    qeury: net.rail(?n) => ?n\n"),
	})}}}
	_, err := c.ResolveConfig(context.Background(), &webapi.AnalysisConfig{QuerySetUris: []string{"mount://m/queries"}}, "p")
	if err == nil || !strings.Contains(err.Error(), "bad.yaml") {
		t.Errorf("err = %v, want one naming bad.yaml", err)
	}
}

// A descriptor can name one file as a source as well as a directory (agni issue 1136). The file is
// read alone, beside files the descriptor did not name, and its source is the file itself.
func TestConfigResolverReadsASourceNamingOneFile(t *testing.T) {
	can, err := os.ReadFile(filepath.Join("..", "examples", "tutorial-project", "profiles", "can.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	c := &ConfigResolver{Mounts: []Mount{{Name: "m", FS: MemFS(map[string][]byte{
		"queries/power.yaml":   []byte("queries:\n  - name: Rails\n    query: net.rail(?n) => ?n\n"),
		"queries/audit.yaml":   []byte("queries:\n  - name: Parts\n    query: component.mpn(?r, ?m) => ?r, ?m\n"),
		"profiles/can.yaml":    can,
		"profiles/broken.yaml": []byte("not: a profile\n"),
	})}}}
	got, err := c.ResolveConfig(context.Background(), &webapi.AnalysisConfig{
		QuerySetUris: []string{"mount://m/queries/power.yaml"},
		ProfileUris:  []string{"mount://m/profiles/can.yaml"},
	}, "projects/p")
	if err != nil {
		t.Fatalf("the broken profile beside the named one must not be read: %v", err)
	}
	if len(got.QuerySets) != 1 || got.QuerySets[0].Name+"="+got.QuerySets[0].Source != "power=mount://m/queries/power.yaml" {
		t.Errorf("sets = %+v, want power alone, sourced from its file", got.QuerySets)
	}
	if len(got.InterfaceProfiles) != 1 || got.InterfaceProfiles[0].Name != "CAN" {
		t.Errorf("profiles = %+v, want the CAN profile alone", got.InterfaceProfiles)
	}
}

// oneFile answers as an fs.FS whose root holds the one file, which is what every tier's loader walks.
func TestOneFileIsAnFS(t *testing.T) {
	inner := MemFS(map[string][]byte{"dir/x.yaml": []byte("a: 1\n"), "dir/y.yaml": []byte("b: 2\n")})
	info, err := fs.Stat(inner, "dir/x.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := fstest.TestFS(oneFile{fsys: inner, name: "dir/x.yaml", info: info}, "x.yaml"); err != nil {
		t.Fatal(err)
	}
}
