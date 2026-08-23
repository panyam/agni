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
		out = append(out, check.ContextSubject{Entity: check.Entity{Kind: check.KindNet, Ref: n}, Role: role})
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
		// The device and every net the group straps. A strap group IS an N-tuple: the value it encodes
		// is a property of all the bits together, and no single net carries it. This rule used to name
		// g.Nets[0] as a stand-in with the rest in prose, which its own comment recorded as "the others
		// were named in prose and reachable nowhere".
		SubjectShape:        strapShape(len(g.Nets)),
		Eval:                func(m check.Model) []check.Verdict { return strapGroupVerdicts(m, g) },
		StatesConsideredSet: true,
	}
}

// strapShape is the declared tuple for a group of n nets: the device, then one element per bit.
func strapShape(n int) []string {
	out := make([]string, 0, n+1)
	out = append(out, check.KindComponent)
	for i := 0; i < n; i++ {
		out = append(out, check.KindNet)
	}
	return out
}

// strapGroupVerdicts decides ONE subject, the group itself, because a strap group has exactly one
// question and one answer: does the wiring encode the number the intent declares.
//
// THE ARITY IS THE POINT HERE and it is not two. A 4-bit address strap is a device and four nets, and
// the answer belongs to all five together. This is the case that shows why a pair-shaped fix would
// have been the wrong shape: nothing about the identity problem is specific to two.
//
// THE PASS IS NEW AND IS THE WHOLE VALUE. A strap group that encodes the declared address reported
// nothing, exactly like one whose nets the design does not carry. For an intent rule, "the board
// straps this part to 0x48 as declared" is the sentence the reviewer opened the report for.
func strapGroupVerdicts(m check.Model, g StrapGroup) []check.Verdict {
	subjects := make([]check.Entity, 0, len(g.Nets)+1)
	subjects = append(subjects, check.ComponentEntity(g.Device))
	for _, netName := range g.Nets {
		subjects = append(subjects, check.NetNameEntity(netName))
	}
	v := check.Verdict{Subjects: subjects}

	// A declared net absent from the design is the presence forms' business, not this rule's. It is
	// still this rule's SUBJECT, so it says so rather than vanishing: an intent file naming a net the
	// board does not have used to leave no trace here at all.
	for _, netName := range g.Nets {
		if netNamed(m, netName) == nil {
			v.Outcome = check.NotConsidered
			v.Reason = fmt.Sprintf("the design carries no net named %q, so the group's bits cannot be read (the presence forms report the missing net)", netName)
			return []check.Verdict{v}
		}
	}

	got, bits, ok := groupValue(m, g)
	if !ok {
		msg := fmt.Sprintf("strap group %q on %s cannot be read: %s carry no bias and the group declares no default level, so the encoded value is unknown (declaring the part's internal pull as `default` would resolve it)",
			g.Name, g.Device, strings.Join(undecidedNets(bits), ", "))
		v.Outcome = check.Inconclusive
		v.Witness = &check.Witness{Statement: msg}
		v.Finding = &check.Finding{
			Subject: check.NetNameEntity(g.Nets[0]), Inconclusive: true, Message: msg,
			// The part, then the nets that could not be read, in the order the message names
			// them. The FINDING's subject is one net, because a reader is told one place to start;
			// the verdict above names the whole group (agni issue 349). The group NAME is a
			// declaration from the intent file rather than a design entity, so it is not context.
			Context: append(
				[]check.ContextSubject{check.Ctx(check.ComponentEntity(g.Device), "device")},
				netContext(undecidedNets(bits), "undecided")...),
		}
		return []check.Verdict{v}
	}
	if got == g.Value {
		v.Outcome = check.Pass
		v.Witness = &check.Witness{
			Statement: fmt.Sprintf("strap group %q on %s encodes %d, which is what the design intent declares (%s)",
				g.Name, g.Device, got, describeBits(bits)),
			Terms: []check.WitnessTerm{
				{Label: "encoded", Value: fmt.Sprint(got)},
				{Label: "declared", Value: fmt.Sprint(g.Value)},
			},
		}
		return []check.Verdict{v}
	}
	msg := fmt.Sprintf("strap group %q on %s encodes %d, but the design intent declares %d (%s)",
		g.Name, g.Device, got, g.Value, describeBits(bits))
	v.Outcome = check.Fail
	v.Witness = &check.Witness{
		Statement: msg,
		Terms: []check.WitnessTerm{
			{Label: "encoded", Value: fmt.Sprint(got)},
			{Label: "declared", Value: fmt.Sprint(g.Value)},
		},
	}
	v.Finding = &check.Finding{
		Subject: check.NetNameEntity(g.Nets[0]),
		Message: msg,
		// The part the group straps. The finding's subject is one of the group's nets, so the device
		// the whole finding is about was named in prose only.
		Context: []check.ContextSubject{check.Ctx(check.ComponentEntity(g.Device), "device")},
	}
	return []check.Verdict{v}
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
		// The subject is a PAIR of devices, which is what let this rule state a considered set after
		// being the one intent rule that could not (agni issue 391).
		//
		// The obstacle was never evidence, it was arity, and the arity was being read off the wrong
		// thing. A collision REPORT is about however many devices share an address, which is 2 on one
		// bus and 4 on the next inside one rule, and Rule.SubjectShape is fixed per rule. But the
		// QUESTION the rule answers is binary: do these two devices strap to the same number. Three
		// devices sharing an address is three yes answers to that question, not one answer about three
		// devices. Pairs give a fixed shape without inventing a subject kind.
		SubjectShape:        []string{check.KindComponent, check.KindComponent},
		Eval:                func(m check.Model) []check.Verdict { return strapCollisionVerdicts(m, groups) },
		StatesConsideredSet: true,
	}
}

// strapReading is one group's decoded address, or the reason it has none.
type strapReading struct {
	value int
	ok    bool
	why   string // why the address could not be read, when ok is false
}

// readGroup decodes one declared group once, so a bus with n groups decodes n times rather than once
// per pair.
//
// Both failure modes are kept as REASONS rather than collapsed into a bool. Under the old
// failures-only body each was a bare `continue` and reached the report as the same silence a
// correctly-addressed pair produced; a pair this rule declines to judge now says which half it could
// not read and why.
func readGroup(m check.Model, g StrapGroup) strapReading {
	for _, netName := range g.Nets {
		if netNamed(m, netName) == nil {
			return strapReading{why: fmt.Sprintf("the design carries no net named %q", netName)}
		}
	}
	v, bits, ok := groupValue(m, g)
	if !ok {
		return strapReading{why: fmt.Sprintf("%s carry no bias and the group declares no default level, so its address is unknown",
			strings.Join(undecidedNets(bits), ", "))}
	}
	return strapReading{value: v, ok: true}
}

// strapCollisionVerdicts decides every PAIR of declared groups sharing a bus.
//
// A pass is the sentence this rule existed to be unable to say. Two parts on one bus straping to
// different addresses is the ordinary, correct state of every board, and it used to report exactly
// what a bus nobody declared reported, and what a pair whose addresses could not be read reported,
// which is nothing.
//
// AN UNREADABLE GROUP IS NOT-CONSIDERED, NEVER A PASS, and that is the guard the old body already
// had in a weaker form. Decoding a group with unevidenced bits would invent an address, and an
// invented address can invent a COLLISION: a confident accusation that two innocent parts clash is
// worse than saying nothing. It can equally invent the absence of one, which is what a pass over an
// unreadable group would assert. The old body dropped such a group before any comparison, so the
// pairs it belonged to left no trace at all.
func strapCollisionVerdicts(m check.Model, groups []StrapGroup) []check.Verdict {
	read := make([]strapReading, len(groups))
	byBus := map[string][]int{}
	for i, g := range groups {
		if g.Bus == "" {
			continue // opted out: not an address on any shared bus
		}
		read[i] = readGroup(m, g)
		byBus[g.Bus] = append(byBus[g.Bus], i)
	}

	var out []check.Verdict
	for _, bus := range sortedKeys(byBus) {
		idx := byBus[bus]
		for i := 0; i < len(idx); i++ {
			for j := i + 1; j < len(idx); j++ {
				ga, gb := groups[idx[i]], groups[idx[j]]
				if ga.Device == gb.Device {
					// NOT a subject. This rule is about two PARTS answering one address, and one part
					// declaring two groups on a bus is a declaration to read, not a bus fault. A verdict
					// here would also name the same device twice and key on itself.
					continue
				}
				out = append(out, strapPairVerdict(bus, ga, gb, read[idx[i]], read[idx[j]]))
			}
		}
	}
	return out
}

// strapPairVerdict answers the one binary question for one pair.
func strapPairVerdict(bus string, ga, gb StrapGroup, ra, rb strapReading) check.Verdict {
	v := check.Verdict{Subjects: []check.Entity{check.ComponentEntity(ga.Device), check.ComponentEntity(gb.Device)}}
	switch {
	case !ra.ok && !rb.ok:
		v.Outcome = check.NotConsidered
		v.Reason = fmt.Sprintf("neither address on bus %s could be read: %s for %s, and %s for %s",
			bus, ra.why, ga.Device, rb.why, gb.Device)
		return v
	case !ra.ok, !rb.ok:
		unread, other := ga, gb
		why := ra.why
		if ra.ok {
			unread, other, why = gb, ga, rb.why
		}
		v.Outcome = check.NotConsidered
		v.Reason = fmt.Sprintf("%s's address on bus %s could not be read (%s), so it cannot be compared with %s",
			unread.Device, bus, why, other.Device)
		return v
	case ra.value != rb.value:
		v.Outcome = check.Pass
		v.Witness = &check.Witness{
			Statement: fmt.Sprintf("%s straps to %d and %s to %d on bus %s, so the two answer to different addresses",
				ga.Device, ra.value, gb.Device, rb.value, bus),
			Terms: []check.WitnessTerm{
				{Label: ga.Device, Value: fmt.Sprint(ra.value)},
				{Label: gb.Device, Value: fmt.Sprint(rb.value)},
				{Label: "bus", Value: bus},
			},
		}
		return v
	}

	names := []string{fmt.Sprintf("%s (%s)", ga.Device, ga.Name), fmt.Sprintf("%s (%s)", gb.Device, gb.Name)}
	msg := fmt.Sprintf("%s both strap to address %d on bus %s; two devices answering one address make that bus unreliable",
		strings.Join(names, " and "), ra.value, bus)
	v.Outcome = check.Fail
	v.Witness = &check.Witness{
		Statement: msg,
		Terms: []check.WitnessTerm{
			{Label: ga.Device, Value: fmt.Sprint(ra.value)},
			{Label: gb.Device, Value: fmt.Sprint(rb.value)},
			{Label: "bus", Value: bus},
		},
	}
	v.Finding = &check.Finding{
		Subject: check.Entity{Kind: check.KindNet, Ref: ga.Nets[0]},
		Message: msg,
		// Both colliding devices, in message order. This is the case that made context a LIST with
		// non-unique roles rather than one entity per part: two entities play exactly the same role
		// here (agni issue 349). The bus is a declared label from the intent file, not a design
		// entity, so it is not context.
		Context: []check.ContextSubject{
			{Entity: check.Entity{Kind: check.KindComponent, Ref: ga.Device}, Role: "device"},
			{Entity: check.Entity{Kind: check.KindComponent, Ref: gb.Device}, Role: "device"},
		},
	}
	return v
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
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
