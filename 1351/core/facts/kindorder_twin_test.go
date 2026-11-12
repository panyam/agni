package facts

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

// TestKindOrderTwin holds KindOrder to testdata/kind_order.json, which the web tier's picker test
// reads too, so a kind renamed or added here fails the viewer's test until it labels it (agni issue
// 1045).
func TestKindOrderTwin(t *testing.T) {
	b, err := os.ReadFile("testdata/kind_order.json")
	if err != nil {
		t.Fatal(err)
	}
	var twin struct {
		Kinds []string `json:"kinds"`
	}
	if err := json.Unmarshal(b, &twin); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(twin.Kinds, KindOrder) {
		t.Errorf("kind_order.json lists %v, KindOrder is %v; update the fixture, then the viewer's RELATION_KINDS", twin.Kinds, KindOrder)
	}
}
