package intent

import (
	"fmt"
	"sort"
	"strings"

	"github.com/panyam/agni/core/check"
)

// Strap GROUPS (WS3-120): several strap nets read as one number, and the address collisions that
// become visible once you can read it.
//
// The per-net rule (property-strap) asks "does this pin latch the intended level". This asks the two
// questions a per-net form structurally cannot: does the GROUP encode the intended number, and do two
// devices on one bus encode the SAME number. The second is the one worth automating — an address
// clash is invisible in a schematic review and surfaces on the bench as an intermittent bus fault
// that looks like anything but a strap.

// RuleStrapAddressCollision is the cross-group collision rule's fixed name. Unlike the per-group
// rules it is not slugified from a declaration, because it is a property of the SET.
const RuleStrapAddressCollision = "strap-address-collision"

// strapBit is one net's contribution to a group's value.
type strapBit struct {
	net   string
	level int  // 0 or 1
	known bool // false when nothing evidences the level
	from  string
}

// groupValue decodes a group's OBSERVED number, MSB-first.
//
// ok is false when any bit is unevidenced, and that is the whole subtlety of this rule. An unbiased
// strap pin is NORMAL (fit a resistor only for the non-default state), so the common partial group is
// not an error — it is a group whose missing bits sit at the part's internal default. Where the
// declaration states that default, the bits resolve and the group decodes. Where it does not, the
// value is genuinely unknown, and decoding it anyway would invent an address that could then collide
// with a real one.
func groupValue(m check.Model, g StrapGroup) (value int, bits []strapBit, ok bool) {
	ok = true
	for _, netName := range g.Nets {
		b := strapBit{net: netName}
		if n := netNamed(m, netName); n != nil {
			up, down := check.NetBias(m, n)
			switch {
			case up:
				b.level, b.known, b.from = 1, true, "pull-up"
			case down:
				b.level, b.known, b.from = 0, true, "pull-down"
			}
		}
		if !b.known {
			// No resistor, or a divider holding neither rail. The declared default is the only thing
			// that can resolve it, and only the declaration knows the part's internal pull.
			switch g.Default {
			case "high":
				b.level, b.known, b.from = 1, true, "declared default"
			case "low":
				b.level, b.known, b.from = 0, true, "declared default"
			}
		}
		if !b.known {
			ok = false
		}
		bits = append(bits, b)
		value = value<<1 | b.level
	}
	return value, bits, ok
}

// netContext turns a list of net names into context entities sharing one role, for a message that
// names several at once. Order is preserved: it is the order the message prints them in.
func netContext(names []string, role string) []check.ContextSubject {
	out := make([]check.ContextSubject, 0, len(names))
	for _, n := range names {
		out = append(out, check.ContextSubject{Kind: check.KindNet, Subject: n, Role: role})
	}
	return out
}

// undecidedNets lists the bits nothing evidenced, for a message that tells a reviewer which pins to
// look at rather than only that the group could not be read.
func undecidedNets(bits []strapBit) []string {
	var out []string
	for _, b := range bits {
		if !b.known {
			out = append(out, b.net)
		}
	}
	return out
}

// strapGroupRule checks ONE declared group's encoded value. One rule per group (the Subsystems shape)
// so distinct review items bind and report independently.
func strapGroupRule(g StrapGroup) *check.Rule {
	return &check.Rule{
		Name:     "strap-group-" + slug(g.Name),
		Severity: "warning",
		Summary:  fmt.Sprintf("the %s strap group does not encode the value the design intent declares", g.Name),
		Detail:   intentDoc(docKeyStrapGroup),
		Impact:   "a multi-pin strap encodes a number the part reads at reset — its address on a shared bus, its boot source, its bus width. Encoding the wrong number does not look like a wiring fault: the board powers up and the part runs, configured as something else, and on a shared bus it may answer to an address another device already owns.",
		Remedy:   intentRemedy(docKeyStrapGroup),
		Reads:    []string{"component-on-net", "component.class", "net.ground", "rail"},
		Tags:     intentTags(),
		Eval: check.FailuresOnly(func(m check.Model) []check.Finding {
			// A declared net absent from the design is the presence forms' business, not this rule's.
			for _, netName := range g.Nets {
				if netNamed(m, netName) == nil {
					return nil
				}
			}
			got, bits, ok := groupValue(m, g)
			if !ok {
				return []check.Finding{{
					Kind: check.KindNet, Subject: g.Nets[0], Inconclusive: true,
					Message: fmt.Sprintf("strap group %q on %s cannot be read: %s carry no bias and the group declares no default level, so the encoded value is unknown (declaring the part's internal pull as `default` would resolve it)",
						g.Name, g.Device, strings.Join(undecidedNets(bits), ", ")),
					// The part, then the nets that could not be read, in the order the message names
					// them. The subject is only the FIRST strap net, so the others were named in prose
					// and reachable nowhere (agni issue 349). The group NAME is a declaration from the
					// intent file rather than a design entity, so it is not context.
					Context: append(
						[]check.ContextSubject{{Kind: check.KindComponent, Subject: g.Device, Role: "device"}},
						netContext(undecidedNets(bits), "undecided")...),
				}}
			}
			if got == g.Value {
				return nil
			}
			return []check.Finding{{
				Kind: check.KindNet, Subject: g.Nets[0],
				Message: fmt.Sprintf("strap group %q on %s encodes %d, but the design intent declares %d (%s)",
					g.Name, g.Device, got, g.Value, describeBits(bits)),
				// The part the group straps. The subject is one of the group's nets, so the device the
				// whole finding is about was named in prose only.
				Context: []check.ContextSubject{{Kind: check.KindComponent, Subject: g.Device, Role: "device"}},
			}}
		}),
	}
}

// describeBits renders the observed bits MSB-first so a finding says WHICH pin is wrong, not only
// that the number is.
func describeBits(bits []strapBit) string {
	parts := make([]string, 0, len(bits))
	for _, b := range bits {
		parts = append(parts, fmt.Sprintf("%s=%d via %s", b.net, b.level, b.from))
	}
	return strings.Join(parts, ", ")
}

// strapCollisionRule reports two groups on the SAME declared bus encoding the same number. It is
// necessarily cross-group, so unlike the per-group rules there is one of it for the whole
// declaration.
//
// A group whose value could not be decoded is EXCLUDED rather than defaulted. This is the load-bearing
// guard: a fabricated address could fabricate a collision, and a confident accusation that two
// innocent parts clash is worse than saying nothing. Those groups are already reported inconclusive by
// their own rule, so the gap is visible rather than silent.
func strapCollisionRule(groups []StrapGroup) *check.Rule {
	return &check.Rule{
		Name:     RuleStrapAddressCollision,
		Severity: "error",
		Summary:  "two devices on one bus strap to the same address",
		Detail:   intentDoc(RuleStrapAddressCollision),
		Impact:   "two parts answering to one address on a shared bus both drive it when either is addressed. The bus goes unreliable in a way that reads as noise or marginal timing rather than as a wiring fault, and it is invisible in a schematic review because each strap is individually correct.",
		Remedy:   intentRemedy(RuleStrapAddressCollision),
		Reads:    []string{"component-on-net", "component.class", "net.ground", "rail"},
		Tags:     intentTags(),
		Eval: check.FailuresOnly(func(m check.Model) []check.Finding {
			type decoded struct {
				g     StrapGroup
				value int
			}
			byBus := map[string][]decoded{}
			for _, g := range groups {
				if g.Bus == "" {
					continue // opted out: not an address on any shared bus
				}
				missing := false
				for _, netName := range g.Nets {
					if netNamed(m, netName) == nil {
						missing = true
					}
				}
				if missing {
					continue
				}
				v, _, ok := groupValue(m, g)
				if !ok {
					continue // undecidable: never invent an address, and never a collision from one
				}
				byBus[g.Bus] = append(byBus[g.Bus], decoded{g: g, value: v})
			}

			var out []check.Finding
			for _, bus := range sortedKeys(byBus) {
				seen := map[int][]decoded{}
				for _, d := range byBus[bus] {
					seen[d.value] = append(seen[d.value], d)
				}
				for _, v := range sortedIntKeys(seen) {
					clash := seen[v]
					if len(clash) < 2 {
						continue
					}
					names := make([]string, 0, len(clash))
					// Built in the same pass as the message, so the chips and the sentence cannot list
					// the devices in different orders. `decoded` is local to this function, which is why
					// this is inline rather than a helper.
					devices := make([]check.ContextSubject, 0, len(clash))
					for _, d := range clash {
						names = append(names, fmt.Sprintf("%s (%s)", d.g.Device, d.g.Name))
						devices = append(devices, check.ContextSubject{Kind: check.KindComponent, Subject: d.g.Device, Role: "device"})
					}
					out = append(out, check.Finding{
						Kind:    check.KindNet,
						Subject: clash[0].g.Nets[0],
						Message: fmt.Sprintf("%s both strap to address %d on bus %s; two devices answering one address make that bus unreliable",
							strings.Join(names, " and "), v, bus),
						// Both colliding devices, in message order. This is the case that made context a
						// LIST with non-unique roles rather than one entity per part: two entities play
						// exactly the same role here (agni issue 349). The bus is a declared label from
						// the intent file, not a design entity, so it is not context.
						Context: devices,
					})
				}
			}
			return out
		}),
	}
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedIntKeys[V any](m map[int]V) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

// collidableGroups reports whether any bus carries two or more declared groups, the only situation in
// which a collision is expressible. Compiling the rule below that threshold would put a check in the
// catalog that can never fail on this declaration, and a review item bound to it would read a pass it
// did not earn — the compiles-to-nothing false pass.
func collidableGroups(groups []StrapGroup) bool {
	perBus := map[string]int{}
	for _, g := range groups {
		if g.Bus != "" {
			perBus[g.Bus]++
			if perBus[g.Bus] > 1 {
				return true
			}
		}
	}
	return false
}
