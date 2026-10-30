package classify

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// nameTokens splits a hierarchy leaf into its alphanumeric tokens, so LPC_VCC reads as LPC and VCC.
var nameTokens = regexp.MustCompile(`[A-Za-z0-9]+`)

// inferRails adds the rail role to each net that the rail vocabulary missed but whose name carries a
// supply word as a LATER token (LPC_VCC, FAN_VDD) while the net feeds a fitted part's power_in pin.
// Both halves are required. The name alone would make EN_VCC a rail, and a supply pin alone would make
// a regulator's sense or bias net one. A net with a single physical pin carries nothing, so an unwired
// supply pin's stub (KiCad's unconnected-(U30-VPP_FAST)) is not one either. A connector's own power_in pin is not counted, because on a
// receptacle it marks where a cable delivers or takes power rather than a part drawing it (agni issues
// 1243, 1254). The role records the pin that justified it.
//
// It runs after rolesFor and after StampPowerInPins, since it reads the pin types that pass fills.
func inferRails(d *ir.Design, v *RoleVocab) {
	if !v.InfersRails() {
		return
	}
	index := PartIndex(d)
	comps := map[string]*ir.Component{}
	for _, c := range d.GetComponents() {
		comps[c.GetRefDes()] = c
	}
	for _, n := range d.GetNets() {
		if hasRole(n, ir.Role_ROLE_RAIL) || hasRole(n, ir.Role_ROLE_GROUND) || physicalPins(n) < 2 {
			continue
		}
		word := laterSupplyToken(v, n.GetName())
		if word == "" {
			continue
		}
		pin := firstSupplyLoad(index, comps, n)
		if pin == "" {
			continue
		}
		addNetRoleWhy(n, ir.Role_ROLE_RAIL, ir.RoleSource_ROLE_SOURCE_CONVENTION,
			fmt.Sprintf("the name's token %s is a supply word and the net feeds %s, a supply input", word, pin))
	}
}

// laterSupplyToken returns the first token after the leaf's first that the supply-pin vocabulary
// matches, or "". The first token is left to the rail vocabulary, which already reads a VCC prefix.
func laterSupplyToken(v *RoleVocab, name string) string {
	toks := nameTokens.FindAllString(roleLeaf(name), -1)
	for _, t := range toks[min(1, len(toks)):] {
		if v.IsSupplyPin(t) {
			return t
		}
	}
	return ""
}

// firstSupplyLoad names the first power_in pin on n, in ref-des then pin order, that belongs to a
// fitted part other than a connector, as "U18 pin 1 (VDD)". "" when there is none.
func firstSupplyLoad(index map[string]*ir.PartType, comps map[string]*ir.Component, n *ir.Net) string {
	var loads []string
	for _, conn := range n.GetConnections() {
		c := comps[conn.GetComponentRef()]
		if c == nil || unfitted(c) || isConnector(c) {
			continue
		}
		p := pinOf(index, c, conn.GetPinRef())
		if p == nil || p.GetDirection() != ir.PinDirection_PIN_DIRECTION_POWER_IN {
			continue
		}
		loads = append(loads, fmt.Sprintf("%s pin %s (%s)", c.GetRefDes(), conn.GetPinRef(), p.GetName()))
	}
	if len(loads) == 0 {
		return ""
	}
	sort.Strings(loads)
	return loads[0]
}

func pinOf(index map[string]*ir.PartType, c *ir.Component, designator string) *ir.Pin {
	for _, s := range c.GetSections() {
		pt := index[s.GetLibraryRef()+"/"+s.GetPartRef()]
		if pt == nil {
			pt = index["/"+s.GetPartRef()]
		}
		for _, p := range pt.GetPins() {
			if p.GetDesignator() == designator {
				return p
			}
		}
	}
	return nil
}

func hasRole(n *ir.Net, role ir.Role) bool {
	for _, r := range n.GetRoles() {
		if r.GetRoleKind() == role {
			return true
		}
	}
	return false
}

func isConnector(c *ir.Component) bool {
	for _, t := range c.GetDeviceClasses() {
		if t.GetClass() == string(ClassConnector) {
			return true
		}
	}
	return false
}

// unfitted reports a do-not-populate part the way check.Model reads one, the `dnp` attribute.
func unfitted(c *ir.Component) bool {
	for k, val := range c.GetAttributes() {
		if strings.EqualFold(k, "dnp") {
			switch strings.ToLower(strings.TrimSpace(val)) {
			case "yes", "true", "1":
				return true
			}
		}
	}
	return false
}

// physicalPins counts n's connections to real parts, leaving out a power symbol's virtual pin.
func physicalPins(n *ir.Net) int {
	k := 0
	for _, c := range n.GetConnections() {
		if !strings.HasPrefix(c.GetComponentRef(), "#") {
			k++
		}
	}
	return k
}
