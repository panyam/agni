package query

import "testing"

// A length reads in millimetres and anything else as the engine holds it (agni issue 1035).
func TestDisplayShowsALengthInMillimetres(t *testing.T) {
	num := func(f float64, unit string) Value { return Value{S: "engine", Num: &f, BaseUnit: unit} }
	for _, c := range []struct {
		v    Value
		want string
	}{
		{num(0.0001524, "m"), "0.1524"},
		{num(8e-05, "m"), "0.08"},
		{num(3.3, "V"), "engine"},
		{num(12, ""), "engine"},
		{Value{S: "GND"}, "GND"},
		{Value{Absent: true, BaseUnit: "m"}, ""},
	} {
		if got := Display(c.v); got != c.want {
			t.Errorf("Display(%+v) = %q, want %q", c.v, got, c.want)
		}
	}
}
