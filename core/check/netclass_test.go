package check

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func TestNetClassCascade(t *testing.T) {
	def := func(name string, params map[string]string) *ir.Constraint {
		return &ir.Constraint{Name: name, Params: params}
	}
	c := NewNetClassCascade([]*ir.Constraint{
		def("Default", map[string]string{"is_default": "true", "track_width": "0.2", "via_drill": "0.3"}),
		def("Power", map[string]string{"priority": "0", "track_width": "0.5"}),
		def("HighSpeed", map[string]string{"priority": "1", "track_width": "0.1", "clearance": "0.15"}),
		def("Broken", map[string]string{"priority": "-1", "track_width": "wide"}),
	})
	cases := []struct {
		name    string
		classes []string
		param   string
		want    float64
		from    string
		ok      bool
	}{
		// Membership order is alphabetical in the source; priority decides, so Power wins over HighSpeed.
		{"priority decides", []string{"HighSpeed", "Power"}, "track_width", 0.5, "Power", true},
		// Per field: neither class states a drill, so it falls through to Default.
		{"field falls through", []string{"HighSpeed", "Power"}, "via_drill", 0.3, "Default", true},
		{"default applies to an unclassed net", nil, "track_width", 0.2, "Default", true},
		{"an undefined class states nothing", []string{"NoSuchClass"}, "track_width", 0.2, "Default", true},
		{"an unreadable value is not a limit", []string{"Broken"}, "track_width", 0.2, "Default", true},
		{"stated nowhere", []string{"Power"}, "diff_pair_gap", 0, "", false},
	}
	for _, tc := range cases {
		v, from, ok := c.Declared(tc.classes, tc.param)
		if v != tc.want || from != tc.from || ok != tc.ok {
			t.Errorf("%s: got (%v, %q, %v), want (%v, %q, %v)", tc.name, v, from, ok, tc.want, tc.from, tc.ok)
		}
	}
	if _, _, ok := NewNetClassCascade(nil).Declared([]string{"Power"}, "track_width"); ok {
		t.Error("with no definitions nothing is stated")
	}
}
