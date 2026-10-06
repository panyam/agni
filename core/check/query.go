package check

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/core/param"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/refdes"
)

// irModel is the default Model, a fact projection computed once over an ir.Design and shared by
// every rule in a Run.
type irModel struct {
	d         *ir.Design
	pinDir    map[string]ir.PinDirection  // "refdes\x00pin" -> direction
	pinName   map[string]string           // "refdes\x00pin" -> declared pin name (for role derivation)
	pinless   map[string]bool             // ref-des whose resolved part types declare no pins at all
	unfitted  map[string]bool             // ref-des the design marks do-not-populate
	pins      []PinInst                   // every part-type pin of every component, dedup by designator
	pinConn   map[string]bool             // "refdes\x00pin" present in some net's connections
	pinNet    map[string]string           // "refdes\x00pin" -> first net name it appears on
	pinNetDup []PinNetConflict            // pins claimed by more than one net (malformed input)
	ncChannel bool                        // source carries any no-connect evidence (typed pin or marker net)
	netClass  bool                        // at least one net carries a tool-assigned net class (WS3-105)
	board     *geom.BoardGeometry         // staged by WithBoard, built into boardNets by attachBoard
	boardNets []BoardNet                  // board tier, populated only when WithBoard attached one
	hasBoard  bool                        // a non-nil board geometry was attached (tier present, may be empty)
	connected map[string]bool             // ref_des present on >= 1 net
	netByName map[string]*ir.Net          // exact net name -> net (rail/attribute lookups)
	netNames  map[string]bool             // upper-cased net names (for the pair primitive)
	nameCount map[string]int              // exact-name net counts (duplicate-net-name)
	classSet  map[string][]ComponentClass // ref_des -> device_classes set (specific + family tags)
	specs     param.ParamProvider         // params tier, set only by WithParamProvider
	mpn       map[string]string           // ref_des -> design-side MPN (BomLine, else attribute)
	passNets  map[string][]*ir.Net        // pass-element ref_des -> the distinct nets it touches
	lex       *classify.Lexicon           // naming vocabulary the design was READ with (nil = process defaults)
	internal  map[string]bool             // ref_des the design's intent declares an internal connector
	memo      sync.Map                    // key -> *memoEntry, values derived from this model once (Memo)
}

// ModelOption configures a Model at construction. Options are applied before anything is derived, so an
// option the derivation reads (the lexicon) is in place when it runs.
type ModelOption func(*irModel)

// WithLexicon tells the model which naming vocabulary its design was READ with, so the residual
// name projections (the spec name FFIs, pin-role derivation, and the stamped-role fallback for a
// hand-built IR) answer with the project's conventions rather than a process global (WS3-106).
// Pass the same *classify.Lexicon the loader used; omitting it means the process defaults.
func WithLexicon(lex *classify.Lexicon) ModelOption {
	return func(m *irModel) { m.lex = lex }
}

// WithIntent tells the model what the design's intent declares about its components, so
// ExposedConnector answers for this product rather than for the part (agni issue 831). Omitting it, or
// passing a nil intent, leaves every connector exposed.
func WithIntent(di *configpb.DesignIntent) ModelOption {
	return func(m *irModel) {
		for ref, c := range di.GetComponents() {
			if c.GetExposure() == "internal" {
				if m.internal == nil {
					m.internal = map[string]bool{}
				}
				m.internal[ref] = true
			}
		}
	}
}

// NewModel builds the default IR-backed Model for a design. WithBoard and WithParamProvider attach the
// board and datasheet tiers; without them the model has neither, and the rules that need them are
// not-applicable. The design's MPNs are joined either way.
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
	// Apply options FIRST, because the device-class fallback below re-derives through the lexicon
	// and a model built WithLexicon must already carry it.
	for _, opt := range opts {
		opt(m)
	}
	// Index part-type pins by (library, part) so a component's sections resolve to pin
	// directions. The loose "/part" key matches when a section omits the library ref. This is
	// the index the ingestion classify pass uses (WS3-071), so part resolution never drifts.
	parts := classify.PartIndex(d)
	for _, c := range d.Components {
		if dnpMarked(c) {
			if m.unfitted == nil {
				m.unfitted = map[string]bool{}
			}
			m.unfitted[c.RefDes] = true
		}
		declared := 0
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
			declared += len(p.Pins)
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
		if first != nil && declared == 0 {
			if m.pinless == nil {
				m.pinless = map[string]bool{}
			}
			m.pinless[c.RefDes] = true
		}
	}
	// A duplicated ref-des puts one (ref, pin) key in several nets, since each placement gets
	// its own copper. duplicate-ref-des reports the root cause, so pin-net-conflict skips those pins.
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
	m.attachBoard()
	m.buildMPN()
	m.attachParams()
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

// IsPowerRailName and the five name predicates beside it project this model's naming lexicon over a
// bare name. The package-level helpers of the same names read the process globals, so prefer these
// wherever a Model is in hand (WS3-106).
func (m *irModel) IsPowerRailName(name string) bool { return m.lexicon().RoleVocab().IsRail(name) }
func (m *irModel) IsGroundName(name string) bool    { return m.lexicon().RoleVocab().IsGround(name) }
func (m *irModel) IsFeedbackName(name string) bool  { return m.lexicon().RoleVocab().IsFeedback(name) }
func (m *irModel) IsSwitchingName(name string) bool { return m.lexicon().RoleVocab().IsSwitching(name) }
func (m *irModel) IsControlName(name string) bool   { return m.lexicon().RoleVocab().IsControl(name) }
func (m *irModel) IsGateDriveName(name string) bool { return m.lexicon().RoleVocab().IsGateDrive(name) }

// IsGroundNet reports whether a net carries the ground role, read from the stamped role set when the
// net has one (authoritative, filled at ingestion) and else from this model's lexicon over the name.
// It takes the net rather than its name because net names are not unique. IsRailNet resolves the
// same way.
func (m *irModel) IsGroundNet(n *ir.Net) bool {
	return NetHasRole(n, ir.Role_ROLE_GROUND, m.IsGroundName)
}

// IsRailNet reports whether a net carries the rail role and is not a regulator internal. A REGULATOR
// INTERNAL IS NOT A RAIL, and this is the one place that decides it (agni 679, 680). A buck's
// feedback tap, switch node, bootstrap node, mode straps, enable and gate-drive supply are named after
// the rail they serve, and none of them carries that rail's voltage.
//
// The vocabularies encode one rule piecemeal. WHEN A RAIL TOKEN IS A PREFIX AND A KNOWN
// REGULATOR-PIN-FUNCTION SUFFIX FOLLOWS, THE TOKEN NAMES THE CONVERTER RATHER THAN THE VOLTAGE. So
// net.signal_level's free-standing token (agni 194's `U3_12_U7_4_3V3`) is untouched. Encoding the
// grammar in general was held back because it would decide railhood from a suffix list nobody has
// enumerated. DECISIONS.md carries the reasoning.
//
// Of seven rail-quantified consumers only one excluded feedback for itself, which left 48 of 77
// "rails" on one real board as regulator internals. A consumer that wants every rail-NAMED net uses
// IsPowerRailName.
func (m *irModel) IsRailNet(n *ir.Net) bool {
	if !NetHasRole(n, ir.Role_ROLE_RAIL, m.IsPowerRailName) {
		return false
	}
	return !m.IsRegulatorInternalNet(n)
}

// IsRegulatorInternalNet reports whether a net is a regulator's own plumbing (a feedback tap, a
// power-stage node, a configuration or enable input, or a gate-drive supply) rather than a supply it
// produces. See IsRailNet for why the voltage token in such a name identifies the CONVERTER.
//
// It is exported and on the interface so its two callers cannot drift. IsRailNet subtracts it from
// the rail role, and the two name-derived voltage relations subtract it from BOTH sides of their split
// so a net dropped from one does not reappear in the other.
func (m *irModel) IsRegulatorInternalNet(n *ir.Net) bool {
	return m.HasAnyRole(n, ir.Role_ROLE_FEEDBACK, ir.Role_ROLE_SWITCHING, ir.Role_ROLE_CONTROL, ir.Role_ROLE_GATE_DRIVE)
}

// HasAnyRole reports whether a net carries any of the named roles. Each role resolves as NetHasRole
// resolves it, from the stamped set when the net has one and else through that role's name matcher
// from nameMatcherFor, so a caller listing roles never has to pair each with its fallback.
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

// componentClassesOf resolves a component's device_classes SET. It reads the normalized set stamped
// at ingestion (WS3-071) when present, and otherwise derives one for a design built without the
// ingestion pass, such as a hand-authored test IR. Stamp writes the same derivation, so both paths
// agree. An unclassified component gets an empty, non-nil slice.
func (m *irModel) componentClassesOf(c *ir.Component, pt *ir.PartType) []ComponentClass {
	names := classify.ClassNames(c)
	if len(names) == 0 {
		names = classify.ClassesOf(m.lexicon().Classify(c, pt))
	}
	out := make([]ComponentClass, len(names))
	for i, n := range names {
		out[i] = ComponentClass(n)
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

// PinName exposes the declared pin name the model already indexes for role derivation. It is on the
// Model interface because the datasheet pin join leads with the NAME, since a designator is the pin's
// position in one package and the same die in another body renumbers it.
func (m *irModel) PinName(refDes, pin string) string { return m.pinName[refDes+"\x00"+pin] }

// IsPinlessPart reports whether a component's part type is known and declares no pins (see
// model.Model). A component with no resolved part type is not pinless, since its pins are unknown.
func (m *irModel) IsPinlessPart(refDes string) bool { return m.pinless[refDes] }

// IsFitted reports whether a component is assembled onto the board (see model.Model).
func (m *irModel) IsFitted(refDes string) bool { return !m.unfitted[refDes] }

// dnpMarked reports whether a component carries a do-not-populate flag, KiCad's `dnp` property set to
// yes (the KiCad reader records it as the `dnp` attribute). The key is matched without regard to case
// so another format recording the same flag joins without a change here.
func dnpMarked(c *ir.Component) bool {
	for k, v := range c.GetAttributes() {
		if strings.EqualFold(k, "dnp") {
			switch strings.ToLower(strings.TrimSpace(v)) {
			case "yes", "true", "1":
				return true
			}
		}
	}
	return false
}

func (m *irModel) PinNetName(refDes, pin string) string { return m.pinNet[refDes+"\x00"+pin] }

func (m *irModel) PinNetConflicts() []PinNetConflict { return m.pinNetDup }

func (m *irModel) HasNoConnectChannel() bool { return m.ncChannel }

// FormatTypesPowerOut reports whether the design's source format classifies power-output pins (see the
// model.Model contract). Derived from SourceFormat, so it needs no precomputed state.
func (m *irModel) FormatTypesPowerOut() bool { return formatTypesPowerOut(m.d.GetSourceFormat()) }

// HasNetClasses reports whether any net carries a tool-assigned net class (see the model.Model
// contract). Collected in the same nets walk as ncChannel, so the read is O(1).
func (m *irModel) HasNetClasses() bool { return m.netClass }

// NetClassDefs returns the design's net-class definition constraints (see model.Model). It filters by
// kind because ir.Design.constraints is a general carrier and may hold other kinds.
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
// core must not depend on a reader (C1); TestConstraintKindsAgree in readers/formats pins the two.
const kicadNetClassKind = "netclass"

// NetClassConstraintKind and BoardRulesConstraintKind are the ir.Constraint kinds this model reads,
// exported so a reader's own constants can be held to them by a test outside core.
const (
	NetClassConstraintKind   = kicadNetClassKind
	BoardRulesConstraintKind = "board_rules"
)

// BoardRules returns the copper minimums the design declares (see model.Model), the zero value when
// it declares none. A value that does not parse as a positive number of millimetres is left unset,
// so a rule falls back to its own floor rather than checking against nonsense.
func (m *irModel) BoardRules() BoardRules {
	var r BoardRules
	for _, c := range m.d.GetConstraints() {
		if c.GetKind() != BoardRulesConstraintKind {
			continue
		}
		mm := func(k string) int64 {
			v, err := strconv.ParseFloat(c.GetParams()[k], 64)
			if err != nil || v <= 0 {
				return 0
			}
			return int64(math.Round(v * 1e6))
		}
		r = BoardRules{
			TrackWidthNm: mm("min_track_width"),
			ClearanceNm:  mm("min_clearance"),
			DrillNm:      mm("min_through_hole_diameter"),
			AnnularNm:    mm("min_via_annular_width"),
		}
	}
	return r
}

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

// ExposedConnector is a connector the intent does not declare internal; see Model.ExposedConnector.
func (m *irModel) ExposedConnector(refDes string) bool {
	return m.HasClass(refDes, ClassConnector) && !m.internal[refDes]
}

// Classes returns a ref-des's full device_classes set (specific class plus family tags), nil for an
// unknown ref-des. The slice is the model's own; callers must not mutate it.
func (m *irModel) Classes(refDes string) []ComponentClass { return m.classSet[refDes] }

// --- generic combinators (select, exists, count) over any entity slice ---

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
