package param

import (
	"slices"
	"testing"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

func TestAudience(t *testing.T) {
	mk := func(v string) *parampb.PartSpec {
		return &parampb.PartSpec{Attributes: map[string]string{AudienceKey: v}}
	}
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"single", "powertrain", []string{"powertrain"}},
		{"multi", "powertrain, chassis", []string{"powertrain", "chassis"}},
		{"whitespace and empty entries dropped", " a ,, b ", []string{"a", "b"}},
		{"blank", "   ", nil},
	}
	for _, tc := range cases {
		if got := Audience(mk(tc.in)); !slices.Equal(got, tc.want) {
			t.Errorf("%s: Audience(%q) = %v, want %v", tc.name, tc.in, got, tc.want)
		}
	}
	// Unset means "not annotated", not "no one": nil, distinct from an empty result the caller
	// could mistake for a deny. A nil spec and a spec with no attributes both yield nil.
	if got := Audience(nil); got != nil {
		t.Errorf("nil spec: want nil, got %v", got)
	}
	if got := Audience(&parampb.PartSpec{}); got != nil {
		t.Errorf("no attributes: want nil, got %v", got)
	}
}
