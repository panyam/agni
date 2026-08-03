package profiles

import "github.com/panyam/agni/core/check"

// Profiles is the built-in interface-profile set. Adding an interface is one entry here plus its
// Profile value; the compiler and registration below need no change.
var Profiles = []Profile{SPINOR, EMMC, CAN, LIN, A2B, PCIE, SGMII}

// ByName returns the built-in profile with the given Name (SPI_NOR, eMMC, CAN, LIN); ok is false if
// none matches. It is the lookup a naming map (WS3-054) uses to find the core profile it re-binds.
func ByName(name string) (Profile, bool) {
	for _, p := range Profiles {
		if p.Name == name {
			return p, true
		}
	}
	return Profile{}, false
}

func init() {
	var rules []*check.Rule
	for _, p := range Profiles {
		rules = append(rules, Compile(p)...)
	}
	check.RegisterSource(check.NewSource("profile", rules))
}
