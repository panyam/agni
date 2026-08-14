package check

import (
	"slices"
	"strings"

	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/datasheet/param"
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
			} else if first != n.Name && !collided[c.ComponentRef] && !placeholderRefDes(c.ComponentRef) {
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

// IsGroundNet / IsRailNet answer the role question about a net: the stamped role set when the net
// carries one (authoritative, filled at ingestion), else this model's lexicon over the name. Taking
// the net rather than its name matters because net names are not unique.
func (m *irModel) IsGroundNet(n *ir.Net) bool {
	return NetHasRole(n, NetRoleGround, m.IsGroundName)
}

func (m *irModel) IsRailNet(n *ir.Net) bool {
	return NetHasRole(n, NetRoleRail, m.IsPowerRailName)
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

// placeholderRefDes reports whether a reference designator is an unannotated placeholder rather
// than an identity: "R?", "C?", "REF**", and the partially-annotated "C?1845" a tool leaves when
// only some digits are assigned.
//
// It is the second suppression input to the pin-uniqueness index, alongside a known ref-des
// collision, and for the same reason. "A pin belongs to exactly one net" is a claim about a PIN,
// and (R?, 1) does not name one: on one export 176 distinct un-annotated resistors shared that
// key, so the index saw a single pin sitting on 129 nets. Reporting that as malformed input says
// something false about the netlist — the design is fine, the key is not a key. Suppressing it is
// not hiding a defect, it is declining to assert uniqueness over something that has no identity.
// What the design DOES have is un-annotated parts, which is a separate and truthful finding.
//
// Matching on "?" ANYWHERE rather than as a suffix is deliberate: a partially-annotated designator
// puts it mid-string. "?" is not a legal character in a reference designator in any tool that
// writes one, so the wider match cannot catch a real ref-des.
//
// readers/kicad has its own placeholderRef for the same concept, applied at read time to decide
// whether a footprint is a real part. The two are deliberately separate because readers do not
// import core; they must agree on what a placeholder looks like, so change them together.
func placeholderRefDes(ref string) bool {
	return strings.Contains(ref, "?") || strings.HasSuffix(ref, "**")
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

func (m *irModel) DanglingEndpoints() []*ir.DanglingEndpoint {
	return m.d.GetInputDiagnostics().GetDanglingEndpoints()
}

func (m *irModel) NoJunctionEndpoints() []*ir.DanglingEndpoint {
	return m.d.GetInputDiagnostics().GetNoJunctionEndpoints()
}

func (m *irModel) RefDesCollisions() []*ir.RefDesCollision {
	return m.d.GetInputDiagnostics().GetRefDesCollisions()
}

func (m *irModel) UnresolvedSymbols() []*ir.UnresolvedSymbol {
	return m.d.GetInputDiagnostics().GetUnresolvedSymbols()
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
