package ident

import (
	"encoding/json"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"
)

// TestNaturalOrderTwin holds CompareNatural to testdata/natural_twin.json, which the query panel's
// own sort is tested against too (agni issue 1045).
func TestNaturalOrderTwin(t *testing.T) {
	b, err := os.ReadFile("testdata/natural_twin.json")
	if err != nil {
		t.Fatal(err)
	}
	var twin struct {
		Lists [][]string `json:"lists"`
	}
	if err := json.Unmarshal(b, &twin); err != nil {
		t.Fatal(err)
	}
	if len(twin.Lists) < 5 {
		t.Fatalf("fixture has %d lists", len(twin.Lists))
	}
	for _, want := range twin.Lists {
		got := slices.Clone(want)
		slices.Reverse(got)
		slices.SortFunc(got, CompareNatural)
		if !slices.Equal(got, want) {
			t.Errorf("CompareNatural ordered %v, want %v", got, want)
		}
	}
}

func TestCompareNaturalReadsDigitsAsNumbers(t *testing.T) {
	got := []string{"R10", "r2", "C1", "R1", "U10.12", "U10.3", "R01", ""}
	sort.SliceStable(got, func(i, j int) bool { return CompareNatural(got[i], got[j]) < 0 })
	want := "|C1|R01|R1|r2|R10|U10.3|U10.12"
	if strings.Join(got, "|") != want {
		t.Errorf("natural order = %q, want %q", strings.Join(got, "|"), want)
	}
}
