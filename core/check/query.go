package check

import (
	"slices"
	"strings"

	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/refdes"
)

// irModel is the default Model: a fact projection computed once over an ir.Design and shared by
// every rule in a Run, so the common projections are built a single time.
type irModel struct {
	d         *ir.Design
	pinDir    map[string]ir.PinDirection  // "refdes\x00pin" -> direction
	pinName   map[string]string           // "refdes\x00pin" -> declared pin name (for role derivation)
	pins      []PinInst                   // every part-type pin of every component, dedup by designator
	pinConn   map[string]bool             // "refdes\x00pin" present in some net's connections
	pinNet    map[string]string           // "refdes\x00pin" -> first net name it appears on
	pinNetDup []PinNetConflict            // pins claimed by more than one net (malformed input)
	ncChannel bool                        // source carries any no-connect evidence (typed pin or marker net)
	netClass  bool                        // at least one net carries a tool-assigned net class (WS3-105)
	boardNets []BoardNet                  // board tier, populated only by NewModelWithBoard
	hasBoard  bool                        // a non-nil board geometry was attached (tier present, may be empty)
	connected map[string]bool             // ref_des present on >= 1 net
	netByName map[string]*ir.Net          // exact net name -> net (rail/attribute lookups)
	netNames  map[string]bool             // upper-cased net names (for the pair primitive)
	nameCount map[string]int              // exact-name net counts (duplicate-net-name)
	classSet  map[string][]ComponentClass // ref_des -> device_classes set (specific + family tags)
	specs     param.ParamProvider         // params tier seam, populated only by NewModelWithParams
	mpn       map[string]string           // ref_des -> design-side MPN (BomLine, else attribute)
	passNets  map[string][]*ir.Net        // pass-element ref_des -> the distinct nets it touches
	lex       *classify.Lexicon           // naming vocabulary the design was READ with (nil = process defaults)
}

// ModelOption configures a Model at construction. It is variadic on every constructor so an existing
// call site is unchanged.
type ModelOption func(*irModel)

// WithLexicon tells the model which naming vocabulary its design was READ with, so the residual
// name projections (the spec name FFIs, pin-role derivation, and the stamped-role fallback for a
// hand-built IR) answer with the project's conventions rather than a process global (WS3-106).
// Pass the same *classify.Lexicon the loader used; omitting it means the process defaults.
func WithLexicon(lex *classify.Lexicon) ModelOption {
	return func(m *irModel) { m.lex = lex }
}

// NewModel builds the default IR-backed Model for a design.
func NewModel(d *ir.Design, opts ...ModelOption) Model {
	m := &irModel{
		d:         d,
		pinDir:    map[string]ir.PinDirection{},
		pinName:   map[string]string{},
		pinConn:   map[string]bool{},
		pinNet:    map[string]string{},
		connected: map[string]bool{},
		netByName: map[string]*ir.Net{},
		netNames:  map[string]bool{},
		nameCount: map[string]int{},
		classSet:  map[string][]ComponentClass{},
		passNets:  map[string][]*ir.Net{},
	}
	// Apply options FIRST: the device-class fallback below re-derives through the lexicon, so a
	// model built WithLexicon must already carry it by the time that runs.
	for _, opt := range opts {
		opt(m)
	}
	// Index part-type pins by (library, part) so a component's sections resolve to pin
	// directions. The loose "/part" key matches when a section omits the library ref. The
	// same index the ingestion classify pass uses (WS3-071), so part resolution never drifts.
	parts := classify.PartIndex(d)
	for _, c := range d.Components {
		var first *ir.PartType
		for _, s := range c.Sections {
			p := parts[s.LibraryRef+"/"+s.PartRef]
			if p == nil {
				p = parts["/"+s.PartRef]
			}
			if p == nil {
				continue
			}
			if first == nil {
				first = p
			}
			for _, pin := range p.Pins {
				key := c.RefDes + "\x00" + pin.Designator
				if _, seen := m.pinDir[key]; !seen {
					// Dedup by designator: a multi-section part lists shared pins
					// (power) in more than one section's symbol.
					m.pins = append(m.pins, PinInst{Component: c, Designator: pin.Designator})
				}
				m.pinDir[key] = pin.Direction
				m.pinName[key] = pin.Name
				if pin.Direction == ir.PinDirection_PIN_DIRECTION_NO_CONNECT {
					m.ncChannel = true
				}
			}
		}
		m.classSet[c.RefDes] = m.componentClassesOf(c, first)
	}
	// A duplicated ref-des mechanically puts one (ref, pin) key in several nets (each
	// placement gets its own copper), so those pins are the ref-des collision's symptom,
	// not a second malformed-input signal: duplicate-ref-des owns the root cause and
	// pin-net-conflict skips them.
	collided := map[string]bool{}
	for _, rc := range d.GetInputDiagnostics().GetRefDesCollisions() {
		collided[rc.RefDes] = true
	}
	for _, n := range d.Nets {
		m.netByName[n.Name] = n
		m.netNames[strings.ToUpper(n.Name)] = true
		m.nameCount[n.Name]++
		if len(n.NetClasses) > 0 {
			m.netClass = true
		}
		switch name := strings.ToLower(n.Name); {
		case strings.HasPrefix(name, "unconnected"),
			strings.HasPrefix(name, "no_connect"),
			strings.HasPrefix(name, "nc_"):
			m.ncChannel = true // same marker vocabulary as IntentionallyUnconnected
		}
		for _, c := range n.Connections {
			m.connected[c.ComponentRef] = true
			key := c.ComponentRef + "\x00" + c.PinRef
			m.pinConn[key] = true
			if first, seen := m.pinNet[key]; !seen {
				m.pinNet[key] = n.Name
			} else if first != n.Name && !collided[c.ComponentRef] && !refdes.IsPlaceholder(c.ComponentRef) {
				m.recordPinNetConflict(c.ComponentRef, c.PinRef, first, n.Name, n.Prov)
			}
			if passClass(m.ComponentClass(c.ComponentRef)) {
				nets := m.passNets[c.ComponentRef]
				dup := false
				for _, o := range nets {
					if o.Name == n.Name {
						dup = true
						break
					}
				}
				if !dup {
					m.passNets[c.ComponentRef] = append(nets, n)
				}
			}
		}
	}
	return m
}

// lexicon resolves the naming vocabulary this model reads names with, defaulting to the process-level
// one for a model built without WithLexicon (a hand-authored test IR, or a caller that declares no
// project convention). Never nil, so the call sites need no guard.
func (m *irModel) lexicon() *classify.Lexicon {
	if m.lex == nil {
		return classify.ActiveLexicon()
	}
	return m.lex
}

// IsPowerRailName / IsGroundName / IsFeedbackName project this model's naming lexicon over a bare
// name. They are the model-scoped form of the package-level helpers of the same names, which read the
// process globals; prefer these wherever a Model is in hand (WS3-106).
func (m *irModel) IsPowerRailName(name string) bool { return m.lexicon().RoleVocab().IsRail(name) }
func (m *irModel) IsGroundName(name string) bool    { return m.lexicon().RoleVocab().IsGround(name) }
func (m *irModel) IsFeedbackName(name string) bool  { return m.lexicon().RoleVocab().IsFeedback(name) }
func (m *irModel) IsSwitchingName(name string) bool { return m.lexicon().RoleVocab().IsSwitching(name) }
func (m *irModel) IsControlName(name string) bool   { return m.lexicon().RoleVocab().IsControl(name) }
func (m *irModel) IsGateDriveName(name string) bool { return m.lexicon().RoleVocab().IsGateDrive(name) }

// IsGroundNet / IsRailNet answer the role question about a net: the stamped role set when the net
// carries one (authoritative, filled at ingestion), else this model's lexicon over the name. Taking
// the net rather than its name matters because net names are not unique.
func (m *irModel) IsGroundNet(n *ir.Net) bool {
	return NetHasRole(n, ir.Role_ROLE_GROUND, m.IsGroundName)
}

// A REGULATOR INTERNAL IS NOT A RAIL, and this is the one place that decides it (agni 679, 680).
// A buck's feedback tap, switch node, bootstrap node, mode straps, enable and gate-drive supply all
// inherit the name of the rail they serve, so all of them match the rail vocabulary and none carries
// the rail's voltage: the feedback tap sits at the regulator's internal reference, the switch node
// swings to the INPUT rail at the switching frequency, the bootstrap node rides above the output, and
// a mode strap or enable is a logic input at whatever level the sequencer drives.
//
// The general rule the vocabularies encode piecemeal: WHEN A RAIL TOKEN IS A PREFIX AND A KNOWN
// REGULATOR-PIN-FUNCTION SUFFIX FOLLOWS, THE TOKEN NAMES THE CONVERTER RATHER THAN THE VOLTAGE.
// That is why this does not disturb net.signal_level's own case, agni 194's `U3_12_U7_4_3V3`, where
// the token stands free rather than heading a `<rail>_<function>` name. Encoding the grammar itself
// was considered and held: it would decide railhood from a suffix list nobody has enumerated, and
// changing what every board reads as a rail is a larger bet than naming the functions we have met.
//
// Deciding it here rather than per consumer reverses what the role tokens used to say, deliberately.
// Seven rail-quantified consumers read this model and exactly one, the test-point rule, remembered to
// exclude feedback for itself. On one real board that left 48 of 77 "rails" as regulator internals.
// A consumer that genuinely wants every rail-NAMED net still has IsPowerRailName.
func (m *irModel) IsRailNet(n *ir.Net) bool {
	if !NetHasRole(n, ir.Role_ROLE_RAIL, m.IsPowerRailName) {
		return false
	}
	return !m.IsRegulatorInternalNet(n)
}

// IsRegulatorInternalNet reports whether a net belongs to a regulator's own plumbing rather than
// being a supply it produces: a feedback tap, a power-stage node, a configuration or enable input, or
// a gate-drive supply. All four are named after the rail the converter produces, so all four match the
// rail vocabulary, and for all four the voltage token in the name identifies the CONVERTER rather than
// what the net carries.
//
// Exported, and on the interface, because it has two callers that must not drift: IsRailNet
// subtracts it from the rail role, and the two name-derived voltage relations subtract it from BOTH
// sides of their split so a net dropped from one does not reappear in the other. It was briefly a
// private method plus a copy in stdlib/relations, which is the shape that agreed by luck until
// someone added a role to one of them.
func (m *irModel) IsRegulatorInternalNet(n *ir.Net) bool {
	return m.HasAnyRole(n, ir.Role_ROLE_FEEDBACK, ir.Role_ROLE_SWITCHING, ir.Role_ROLE_CONTROL, ir.Role_ROLE_GATE_DRIVE)
}

// HasAnyRole reports whether a net carries any of the named roles, each resolved the way NetHasRole
// resolves one: the stamped set when the net has one, else this model's lexicon over the name.
//
// It exists so a question about SEVERAL roles reads as a list rather than as a boolean expression
// somebody has to extend correctly. IsRegulatorInternalNet was four hand-written disjuncts that grew
// one at a time, and each addition had to remember the matching name fallback.
func (m *irModel) HasAnyRole(n *ir.Net, roles ...ir.Role) bool {
	for _, r := range roles {
		if NetHasRole(n, r, m.nameMatcherFor(r)) {
			return true
		}
	}
	return false
}

// nameMatcherFor returns the lexicon projection that answers for a role when a net carries no stamped
// role set, which is the hand-authored-IR path NetHasRole falls back to. A role with no matcher here
// answers on the stamp alone, which is correct for anything the lexicon does not name.
func (m *irModel) nameMatcherFor(role ir.Role) func(string) bool {
	switch role {
	case ir.Role_ROLE_RAIL:
		return m.IsPowerRailName
	case ir.Role_ROLE_GROUND:
		return m.IsGroundName
	case ir.Role_ROLE_FEEDBACK:
		return m.IsFeedbackName
	case ir.Role_ROLE_SWITCHING:
		return m.IsSwitchingName
	case ir.Role_ROLE_CONTROL:
		return m.IsControlName
	case ir.Role_ROLE_GATE_DRIVE:
		return m.IsGateDriveName
	}
	return func(string) bool { return false }
}

// componentClassesOf resolves a component's device_classes SET: the normalized set stamped at
// ingestion (WS3-071) when present, else a fallback derivation for a design built without the
// ingestion pass (a hand-authored test IR). Because Stamp writes the same derivation, a never-stamped
// component re-derives to the same set. Returns nil for an unclassified component.
func (m *irModel) componentClassesOf(c *ir.Component, pt *ir.PartType) []ComponentClass {
	tags := c.GetDeviceClasses()
	if len(tags) == 0 {
		tags = classify.ClassesOf(m.lexicon().Classify(c, pt))
	}
	out := make([]ComponentClass, len(tags))
	for i, t := range tags {
		out[i] = ComponentClass(t)
	}
	return out
}

// recordPinNetConflict merges a duplicate claim into the pin's conflict entry, creating
// it (seeded with the first two nets) on first detection.
func (m *irModel) recordPinNetConflict(refDes, pin, first, dup string, prov *ir.Provenance) {
	for i := range m.pinNetDup {
		if m.pinNetDup[i].RefDes == refDes && m.pinNetDup[i].Pin == pin {
			if !slices.Contains(m.pinNetDup[i].Nets, dup) {
				m.pinNetDup[i].Nets = append(m.pinNetDup[i].Nets, dup)
			}
			return
		}
	}
	m.pinNetDup = append(m.pinNetDup, PinNetConflict{
		RefDes: refDes, Pin: pin, Nets: []string{first, dup}, Prov: prov,
	})
}

func (m *irModel) Nets() []*ir.Net             { return m.d.Nets }
func (m *irModel) Components() []*ir.Component { return m.d.Components }
func (m *irModel) SourceFormat() string        { return m.d.GetSourceFormat() }
func (m *irModel) HasParams() bool             { return m.specs != nil }
func (m *irModel) HasBoard() bool              { return m.hasBoard }

func (m *irModel) SuppliesDiagnostic(name string) bool {
	return slices.Contains(m.d.GetInputDiagnostics().GetSupplied(), name)
}

func (m *irModel) DanglingEndpoints() []*ir.DanglingEndpoint {
	return m.d.GetInputDiagnostics().GetDanglingEndpoints()
}

func (m *irModel) NoJunctionEndpoints() []*ir.DanglingEndpoint {
	return m.d.GetInputDiagnostics().GetNoJunctionEndpoints()
}

func (m *irModel) JoinedTaps() []*ir.JoinedTap {
	return m.d.GetInputDiagnostics().GetJoinedTaps()
}

func (m *irModel) RefDesCollisions() []*ir.RefDesCollision {
	return m.d.GetInputDiagnostics().GetRefDesCollisions()
}

func (m *irModel) UnresolvedSymbols() []*ir.UnresolvedSymbol {
	return m.d.GetInputDiagnostics().GetUnresolvedSymbols()
}

func (m *irModel) ResolvedSymbols() []*ir.ResolvedSymbol {
	return m.d.GetInputDiagnostics().GetResolvedSymbols()
}

func (m *irModel) UnannotatedComponents() []*ir.UnannotatedComponent {
	return m.d.GetInputDiagnostics().GetUnannotatedComponents()
}

func (m *irModel) UnmodeledBuses() []*ir.BusNotModeled {
	return m.d.GetInputDiagnostics().GetUnmodeledBuses()
}

func (m *irModel) PinDir(refDes, pin string) ir.PinDirection {
	return m.pinDir[refDes+"\x00"+pin]
}

func (m *irModel) PinDeclared(refDes, pin string) bool {
	_, ok := m.pinDir[refDes+"\x00"+pin]
	return ok
}

func (m *irModel) IsConnected(refDes string) bool { return m.connected[refDes] }

func (m *irModel) Pins() []PinInst { return m.pins }

func (m *irModel) PinConnected(refDes, pin string) bool { return m.pinConn[refDes+"\x00"+pin] }

func (m *irModel) PinRole(refDes, pin string) PinRole {
	return classifyPinRole(m, m.pinName[refDes+"\x00"+pin], m.ComponentClass(refDes))
}

// PinName exposes the declared pin name the model already indexes for role derivation. It is
// promoted to the Model interface because the datasheet pin join leads with the NAME: a designator
// is that pin's position in one package, and the same die in another body renumbers it.
func (m *irModel) PinName(refDes, pin string) string { return m.pinName[refDes+"\x00"+pin] }

func (m *irModel) PinNetName(refDes, pin string) string { return m.pinNet[refDes+"\x00"+pin] }

func (m *irModel) PinNetConflicts() []PinNetConflict { return m.pinNetDup }

func (m *irModel) HasNoConnectChannel() bool { return m.ncChannel }

// FormatTypesPowerOut reports whether the design's source format classifies power-output pins (see the
// model.Model contract). Derived from SourceFormat, so it needs no precomputed state.
func (m *irModel) FormatTypesPowerOut() bool { return formatTypesPowerOut(m.d.GetSourceFormat()) }

// HasNetClasses reports whether any net carries a tool-assigned net class (see the model.Model
// contract). Collected in the same nets walk as ncChannel, so the read is O(1).
func (m *irModel) HasNetClasses() bool { return m.netClass }

// NetClassDefs returns the design's net-class definition constraints (see model.Model). Filtered by
// kind rather than assuming ir.Design.constraints holds only these: the node is a general carrier
// and a second kind is expected to land on it.
func (m *irModel) NetClassDefs() []*ir.Constraint {
	var out []*ir.Constraint
	for _, c := range m.d.GetConstraints() {
		if c.GetKind() == kicadNetClassKind {
			out = append(out, c)
		}
	}
	return out
}

// kicadNetClassKind mirrors kicad.ConstraintKindNetClass. Duplicated rather than imported because
// core must not depend on a reader (C1); the two are pinned together by TestNetClassKindAgrees.
const kicadNetClassKind = "netclass"

func (m *irModel) BoardNets() []BoardNet { return m.boardNets }

func (m *irModel) HasNetName(name string) bool { return m.netNames[strings.ToUpper(name)] }

func (m *irModel) NetNameCount(name string) int { return m.nameCount[name] }

// ComponentClass returns the most-specific class of a ref-des's device_classes set; a ref-des the
// design does not carry is ClassUnknown, matching the absent-tolerant contract.
func (m *irModel) ComponentClass(refDes string) ComponentClass {
	tags := make([]string, len(m.classSet[refDes]))
	for i, c := range m.classSet[refDes] {
		tags[i] = string(c)
	}
	return classify.MostSpecific(tags)
}

// HasClass reports whether a ref-des carries class in its device_classes set (WS3-071 family
// membership); false for an unknown ref-des or class, matching the absent-tolerant contract.
func (m *irModel) HasClass(refDes string, class ComponentClass) bool {
	return slices.Contains(m.classSet[refDes], class)
}

// Classes returns a ref-des's full device_classes set (specific class plus family tags), nil for an
// unknown ref-des. The slice is the model's own; callers must not mutate it.
func (m *irModel) Classes(refDes string) []ComponentClass { return m.classSet[refDes] }

// --- generic combinators: select / exists / count over any entity slice ---

// Select returns the elements of xs matching pred (the select primitive).
func Select[T any](xs []T, pred func(T) bool) []T {
	var out []T
	for _, x := range xs {
		if pred(x) {
			out = append(out, x)
		}
	}
	return out
}

// Exists reports whether any element of xs matches pred (the exists primitive).
func Exists[T any](xs []T, pred func(T) bool) bool {
	return slices.ContainsFunc(xs, pred)
}

// Count returns how many elements of xs match pred (the count primitive; the basis for the
// Tier-A aggregate rules).
func Count[T any](xs []T, pred func(T) bool) int {
	n := 0
	for _, x := range xs {
		if pred(x) {
			n++
		}
	}
	return n
}
