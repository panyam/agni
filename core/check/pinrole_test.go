package check

import "testing"

func TestClassifyPinRole(t *testing.T) {
	cases := []struct {
		name  string
		class ComponentClass
		want  PinRole
	}{
		{"A", ClassLED, RoleAnode},
		{"a", ClassDiode, RoleAnode},
		{"+", ClassLED, RoleAnode},
		{"K", ClassLED, RoleCathode},
		{"CATHODE", ClassTVS, RoleCathode},
		{"K", ClassZener, RoleCathode}, // a Zener is a diode: it has anode/cathode (WS3-078)
		{"-", ClassDiode, RoleCathode},
		{"A", ClassIC, RoleUnknown}, // polarity is class-gated: an IC's "A" is a signal
		{"K", ClassResistor, RoleUnknown},
		{"CLKA", ClassLED, RoleUnknown}, // exact-token match, never substring
		{"VCC", ClassIC, RolePower},
		{"VDD", ClassCapacitor, RolePower},
		{"GND", ClassIC, RoleGround},
		{"VSS", ClassIC, RoleGround},
		{"~", ClassResistor, RoleUnknown},
		{"", ClassLED, RoleUnknown},
	}
	for _, tc := range cases {
		if got := classifyPinRole(tc.name, tc.class); got != tc.want {
			t.Errorf("classifyPinRole(%q, %s) = %s, want %s", tc.name, tc.class, got, tc.want)
		}
	}
}
