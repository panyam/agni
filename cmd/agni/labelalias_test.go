package main

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
)

// A reused sheet's two hierarchical labels, each on its own wire, are not rival labels when the
// parent straps both to GND, which is how a parent configures one instance of a sheet (agni issue
// 1249, Glasgow's INA233 address straps). Two local labels on one wire in the child still are, on
// both instances. strapped_root.kicad_sch is checked against kicad-cli: GND carries R101 pins 1 and 2.
func TestPortsStrappedByTheParentAreNotRivalLabels(t *testing.T) {
	var rule []*check.Rule
	for _, r := range check.BuiltinRules() {
		if r.Name == "label-alias-conflict" {
			rule = append(rule, r)
		}
	}
	if len(rule) != 1 {
		t.Fatal("label-alias-conflict is not in the built-in catalog")
	}
	m, err := readModel(filepath.Join("testdata", "labelalias", "strapped_root.kicad_sch"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range check.RunBackground(m, rule) {
		got = append(got, check.EntityRef(f.Subject)+": "+f.Message[strings.LastIndex(f.Message, ": ")+2:])
	}
	sort.Strings(got)
	want := []string{"/buf1/P: /buf1/P, /buf1/Q", "/buf2/P: /buf2/P, /buf2/Q"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("label-alias-conflict\n got %q\nwant %q", got, want)
	}
}
