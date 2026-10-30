package builtin

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestRulesMatchNamesOnlyThroughTheLexicon holds the shipped rules to reading a name through the
// lexicon (Model.IsI2CName, IsRailNet, and the rest) rather than a pattern of their own, which a
// project can neither see nor override (agni 1265). A rule file importing regexp is the shape that
// let I2S data lines read as I2C (agni 1255).
func TestRulesMatchNamesOnlyThroughTheLexicon(t *testing.T) {
	var scanned int
	for _, dir := range []string{".", "../datalog"} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			af, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			scanned++
			for _, imp := range af.Imports {
				if imp.Path.Value == `"regexp"` {
					t.Errorf("%s imports regexp; a name pattern belongs in the lexicon (core/classify defaultLexicon)", f)
				}
			}
		}
	}
	if scanned < 20 {
		t.Fatalf("scanned %d rule files, expected the whole builtin and datalog packages", scanned)
	}
}
