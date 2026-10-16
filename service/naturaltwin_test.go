package service

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

// TestNaturalOrderTwin holds compareNatural to testdata/natural_twin.json, which the query panel's
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
		slices.SortFunc(got, compareNatural)
		if !slices.Equal(got, want) {
			t.Errorf("compareNatural ordered %v, want %v", got, want)
		}
	}
}
