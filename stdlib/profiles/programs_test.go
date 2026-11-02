package profiles

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/panyam/agni/core/query"
	"google.golang.org/protobuf/encoding/protojson"
)

const programsGolden = "testdata/programs.golden"

// TestBuiltinProgramsAreUnchanged holds every built-in profile to the datalog programs it compiles
// to, one line per finding, domain and evidence query, so a change to a requirement compiler shows
// which interfaces' rules it moved (agni issue 1210). A profile with no instance key must compile as
// it did before one existed, and this is where that is held.
//
// UPDATE_GOLDEN=1 rewrites the file. Read the diff before committing it, since a line that changed
// for a profile the change was not about is the regression this test is for.
func TestBuiltinProgramsAreUnchanged(t *testing.T) {
	var lines []string
	for _, p := range Profiles {
		onCompile = func(fq query.FindingQuery) {
			add := func(kind string, q query.Query) {
				lines = append(lines, p.Name+" "+fq.Rule.Name+" "+kind+" "+canonicalJSON(t, q))
			}
			add("query", fq.Query)
			if fq.Domain != nil {
				add("domain", fq.Domain.Query)
				if fq.Domain.Evidence != nil {
					add("evidence", *fq.Domain.Evidence)
				}
			}
		}
		Compile(p)
	}
	onCompile = nil
	if len(lines) < len(Profiles) {
		t.Fatalf("recorded %d programs for %d profiles, so the hook saw nothing", len(lines), len(Profiles))
	}
	got := strings.Join(lines, "\n") + "\n"
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(programsGolden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(programsGolden)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		wl := strings.Split(string(want), "\n")
		for i, l := range strings.Split(got, "\n") {
			if i >= len(wl) || l != wl[i] {
				t.Fatalf("built-in profile programs changed, first at line %d:\n got %.300s\nwant %.300s\n(UPDATE_GOLDEN=1 rewrites %s)", i+1, l, at(wl, i), programsGolden)
			}
		}
		t.Fatalf("built-in profile programs changed: %d lines, golden has %d", len(lines), len(wl)-1)
	}
}

func at(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return "<end of file>"
}

// canonicalJSON is a query's wire form with its keys sorted, since protojson's whitespace varies
// between builds on purpose.
func canonicalJSON(t *testing.T, q query.Query) string {
	t.Helper()
	b, err := protojson.Marshal(query.QueryProto(q))
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
