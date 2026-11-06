package odb

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// ODB++X (ODX) is ODB++ written as one XML file, rooted at <ODX> (agni issue 1372). Allegro writes it
// as its "ODB++ XML" export. It carries what a job carries, laid out as below.
//
//	CAD_LAYER_LIST/CAD_LAYER            the matrix
//	CAD_SYMBOL_LIST/CAD_SYMBOL          user symbols, each drawn as features
//	CAD_STEP/CAD_PROFILE                the board outline
//	CAD_STEP/CAD_PACKAGE_LIST           packages and their pins, in the package's own frame
//	CAD_STEP/CAD_COMPONENT_LIST         placed parts
//	CAD_STEP/CAD_LNET_LIST              the netlist, as (part, pin) per net
//	CAD_STEP/CAD_LYR_F_LIST/CAD_LYR_F   each layer's features
//
// A layer's features read as a stream. A CAD_SET sets the net, usage, polarity and line symbol that
// every later feature takes, until another CAD_SET changes one of them. A pad drawn for a pin names
// the pin as PIN="REF:pin". parseODX fills the same parsedJob a job directory fills, so the netlist
// and the board are built by the job reader's own code.

// XMLRoot returns the name of the first element in head, the first bytes of an XML file. It skips
// the prolog, comments, processing instructions and a DOCTYPE. It returns "" when head holds no
// element, so a caller choosing a reader by root can refuse a file it cannot place.
func XMLRoot(head []byte) string {
	for {
		i := bytes.IndexByte(head, '<')
		if i < 0 || i+1 >= len(head) {
			return ""
		}
		head = head[i+1:]
		switch head[0] {
		case '?':
			if j := bytes.Index(head, []byte("?>")); j >= 0 {
				head = head[j+2:]
				continue
			}
			return ""
		case '!':
			if bytes.HasPrefix(head, []byte("!--")) {
				if j := bytes.Index(head, []byte("-->")); j >= 0 {
					head = head[j+3:]
					continue
				}
				return ""
			}
			if j := bytes.IndexByte(head, '>'); j >= 0 {
				head = head[j+1:]
				continue
			}
			return ""
		}
		end := bytes.IndexAny(head, " \t\r\n/>")
		if end < 0 {
			return string(head)
		}
		return string(head[:end])
	}
}

// ReadODX reads an ODX file into the neutral IR, as Read reads a job directory. sourceFile is
// provenance only.
func ReadODX(r io.Reader, sourceFile string) (*ir.Design, error) {
	p, err := parseODX(r)
	if err != nil {
		return nil, err
	}
	d := p.toDesign(sourceFile)
	d.Attributes["odb_form"] = "odx"
	return d, nil
}

// ReadODXBoardGeometry reads the board-geometry sidecar out of an ODX file, as ReadBoardGeometry
// reads it out of a job directory, with the same fidelity. sourceFile is provenance only.
func ReadODXBoardGeometry(r io.Reader, sourceFile string) (*geom.BoardGeometry, error) {
	p, err := parseODX(r)
	if err != nil {
		return nil, err
	}
	return p.toBoard(sourceFile)
}

// odxMM is ODX's metric unit. A METRIC file writes coordinates and symbol sizes alike in
// millimetres.
const odxMM = 1e6

// odxFeat is what a layer's feature stream says about one feature beyond its geometry.
type odxFeat struct {
	net   string
	usage string
	pin   string // "REF:pin", for a pad drawn for a pin
}

// odxLayer is one CAD_LYR_F as it reads.
type odxLayer struct {
	ff    *featuresFile
	feats []odxFeat // parallel to ff.features
	syms  map[string]int
}

func (l *odxLayer) symbol(name string) int {
	if name == "" {
		return -1
	}
	if i, ok := l.syms[name]; ok {
		return i
	}
	i := len(l.ff.symbols)
	l.ff.symbols = append(l.ff.symbols, symbolRef{name: name, nm: odxMM})
	l.syms[name] = i
	return i
}

// odxStep is one CAD_STEP as it reads.
type odxStep struct {
	name    string
	profile *featuresFile
	pkgs    []edaPkg
	pkgIdx  map[string]int
	comps   []component
	nets    []string
	pinNet  map[[2]string]int // (ref, pin) -> index into nets
	layers  map[string]*odxLayer
	order   []string // layer names as CAD_LYR_F lists them
}

// odxParser holds the decoder's state between elements.
type odxParser struct {
	units   string
	info    map[string]string
	layers  []matrixLayer
	steps   []*odxStep
	symbols map[string]*featuresFile

	step  *odxStep
	pkg   *edaPkg
	pin   *edaPin
	comp  *component
	net   int
	layer *odxLayer
	state odxFeat // the CAD_SET state of the layer being read
	sym   string  // the CAD_SET line symbol of the layer being read
	neg   bool    // the CAD_SET polarity of the layer being read

	surf    *feature // the CAD_S being read
	poly    *contour // the POLYGON being read
	inHole  bool
	lastPt  pt
	profile []contour
	inProf  bool
}

// parseODX reads an ODX document into a parsedJob.
func parseODX(r io.Reader) (*parsedJob, error) {
	in, err := odxDecoded(bufio.NewReader(r))
	if err != nil {
		return nil, err
	}
	dec := xml.NewDecoder(in)
	// odxDecoded has already turned the text into UTF-8, whatever the declaration says.
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	ps := &odxParser{info: map[string]string{}, symbols: map[string]*featuresFile{}, net: -1}
	root := true
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("odb: odx: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if root {
				if t.Name.Local != "ODX" {
					return nil, fmt.Errorf("odb: not an ODX file (root element %q)", t.Name.Local)
				}
				root = false
			}
			if err := ps.start(t); err != nil {
				return nil, fmt.Errorf("odb: odx: line %d: %w", lineOf(dec), err)
			}
		case xml.EndElement:
			if err := ps.end(t.Name.Local); err != nil {
				return nil, fmt.Errorf("odb: odx: line %d: %w", lineOf(dec), err)
			}
		}
	}
	if root {
		return nil, errors.New("odb: not an ODX file (no root element)")
	}
	return ps.job()
}

func lineOf(dec *xml.Decoder) int {
	n, _ := dec.InputPos()
	return n
}

var xmlEncoding = regexp.MustCompile(`^<\?xml[^>]*?encoding\s*=\s*["']([^"']+)["']`)

// odxDecoded returns br's text as UTF-8. Allegro declares `encoding = "ISO-8859-1"` with spaces
// around the `=`, and encoding/xml reads no declaration spelled that way, so it never asks for a
// charset and then fails on the first byte above 0x7F. odxDecoded reads the declaration itself.
func odxDecoded(br *bufio.Reader) (io.Reader, error) {
	head, _ := br.Peek(256)
	m := xmlEncoding.FindSubmatch(bytes.TrimPrefix(head, []byte("\xef\xbb\xbf")))
	if m == nil {
		return br, nil
	}
	switch strings.ToLower(string(m[1])) {
	case "utf-8", "utf8":
		return br, nil
	case "iso-8859-1", "latin1", "latin-1", "us-ascii", "ascii":
		b, err := limitedRead(br)
		if err != nil {
			return nil, err
		}
		var out strings.Builder
		out.Grow(len(b))
		for _, c := range b {
			out.WriteRune(rune(c))
		}
		return strings.NewReader(out.String()), nil
	}
	return nil, fmt.Errorf("odb: odx: unsupported encoding %q", m[1])
}

func attr(e xml.StartElement, name string) (string, bool) {
	for _, a := range e.Attr {
		if a.Name.Local == name {
			return a.Value, true
		}
	}
	return "", false
}

func attrS(e xml.StartElement, name string) string {
	v, _ := attr(e, name)
	return v
}

// coord reads a coordinate attribute in millimetres as nanometres.
func coord(e xml.StartElement, name string) (int64, error) {
	v, ok := attr(e, name)
	if !ok {
		return 0, fmt.Errorf("<%s> has no %s", e.Name.Local, name)
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return 0, fmt.Errorf("<%s> %s=%q: %w", e.Name.Local, name, v, err)
	}
	return int64(math.Round(f * odxMM)), nil
}

func coordXY(e xml.StartElement, xn, yn string) (pt, error) {
	x, err := coord(e, xn)
	if err != nil {
		return pt{}, err
	}
	y, err := coord(e, yn)
	if err != nil {
		return pt{}, err
	}
	return pt{x, y}, nil
}

func (ps *odxParser) start(e xml.StartElement) error {
	switch e.Name.Local {
	case "ODX_HEADER":
		if v := attrS(e, "SOURCE"); v != "" {
			ps.info["ODB_SOURCE"] = v
		}
	case "CAD_DATA_HEADER":
		ps.units = strings.ToUpper(attrS(e, "UNITS"))
		if ps.units != "METRIC" {
			// No ENGLISH sample has been seen, and its symbol sizes may be in mils where its
			// coordinates are in inches. A guess would read every pad at the wrong size.
			return fmt.Errorf("UNITS=%q: only METRIC ODX is read", attrS(e, "UNITS"))
		}
	case "CAD_LAYER":
		typ := strings.ToUpper(attrS(e, "TYPE"))
		if typ == "COMPONENTS" {
			typ = "COMPONENT"
		}
		pol := "POSITIVE"
		if strings.EqualFold(attrS(e, "POLARITY"), "N") {
			pol = "NEGATIVE"
		}
		ps.layers = append(ps.layers, matrixLayer{
			row: len(ps.layers) + 1, context: strings.ToUpper(attrS(e, "CONTEXT")), typ: typ,
			name: attrS(e, "NAME"), polarity: pol,
			startName: attrS(e, "START_NAME"), endName: attrS(e, "END_NAME"),
		})
	case "CAD_SYMBOL":
		ps.beginLayer()
		ps.symbols[strings.ToLower(attrS(e, "NAME"))] = ps.layer.ff
	case "CAD_STEP":
		ps.step = &odxStep{name: attrS(e, "NAME"), pkgIdx: map[string]int{}, pinNet: map[[2]string]int{}, layers: map[string]*odxLayer{}}
		ps.steps = append(ps.steps, ps.step)
	case "CAD_PROFILE":
		ps.inProf = true
		ps.profile = nil
	case "CAD_PACKAGE":
		if ps.step == nil {
			return errors.New("<CAD_PACKAGE> outside a step")
		}
		name := attrS(e, "NAME")
		ps.step.pkgIdx[name] = len(ps.step.pkgs)
		ps.step.pkgs = append(ps.step.pkgs, edaPkg{name: name})
		ps.pkg = &ps.step.pkgs[len(ps.step.pkgs)-1]
	case "CAD_PIN":
		if ps.pkg == nil {
			return errors.New("<CAD_PIN> outside a package")
		}
		typ := "S"
		switch strings.ToUpper(attrS(e, "TYPE")) {
		case "THRU":
			typ = "T"
		case "BLIND":
			typ = "B"
		}
		// A pin is named by its NUMBER, as CAD_LNP's P and a pad's PIN= name it. NAME can be a
		// placeholder such as "un_1" on a mechanical pin, and two pins of a package can share it.
		name := attrS(e, "NUMBER")
		if name == "" {
			name = attrS(e, "NAME")
		}
		ps.pkg.pins = append(ps.pkg.pins, edaPin{name: name, typ: typ})
		ps.pin = &ps.pkg.pins[len(ps.pkg.pins)-1]
	case "CAD_COMPONENT":
		if ps.step == nil {
			return errors.New("<CAD_COMPONENT> outside a step")
		}
		pkg, ok := ps.step.pkgIdx[attrS(e, "PACKAGE")]
		if !ok {
			pkg = -1
		}
		rot, _ := strconv.ParseFloat(strings.TrimSpace(attrS(e, "ANGLE")), 64)
		ps.step.comps = append(ps.step.comps, component{
			pkg: pkg, rot: rot, ref: attrS(e, "NAME"), part: attrS(e, "PART"),
			mirror: strings.EqualFold(attrS(e, "MIRROR"), "YES"),
			back:   strings.EqualFold(attrS(e, "SIDE"), "BOTTOM"),
		})
		ps.comp = &ps.step.comps[len(ps.step.comps)-1]
	case "CAD_PRP":
		if ps.comp != nil {
			ps.comp.props = append(ps.comp.props, [2]string{attrS(e, "NAME"), attrS(e, "TEXT")})
		}
	case "LOCATION":
		p, err := coordXY(e, "X", "Y")
		if err != nil {
			return err
		}
		switch {
		case ps.pin != nil:
			ps.pin.at = p
		case ps.comp != nil:
			ps.comp.at = p
		}
	case "CAD_LNET":
		if ps.step == nil {
			return errors.New("<CAD_LNET> outside a step")
		}
		ps.net = len(ps.step.nets)
		ps.step.nets = append(ps.step.nets, attrS(e, "NAME"))
	case "CAD_LNP":
		if ps.net >= 0 {
			ps.step.pinNet[[2]string{attrS(e, "C"), attrS(e, "P")}] = ps.net
		}
	case "CAD_LYR_F":
		if ps.step == nil {
			return errors.New("<CAD_LYR_F> outside a step")
		}
		ps.beginLayer()
		name := attrS(e, "LYR_NAME")
		ps.step.layers[strings.ToUpper(name)] = ps.layer
		ps.step.order = append(ps.step.order, name)
	case "CAD_SET":
		if v, ok := attr(e, "NET"); ok {
			ps.state.net = v
		}
		if v, ok := attr(e, "USAGE"); ok {
			ps.state.usage = strings.ToUpper(v)
		}
		if v, ok := attr(e, "POL"); ok {
			ps.neg = strings.EqualFold(v, "N")
		}
		if v, ok := attr(e, "SYM"); ok {
			ps.sym = v
		}
	case "CAD_L", "CAD_A":
		if ps.layer == nil {
			return nil
		}
		f := feature{kind: featLine, positive: !ps.neg}
		var err error
		if f.a, err = coordXY(e, "XS", "YS"); err != nil {
			return err
		}
		if f.b, err = coordXY(e, "XE", "YE"); err != nil {
			return err
		}
		if e.Name.Local == "CAD_A" {
			f.kind = featArc
			if f.center, err = coordXY(e, "XC", "YC"); err != nil {
				return err
			}
			f.cw = strings.EqualFold(attrS(e, "CW"), "YES")
		}
		sym := ps.sym
		if v, ok := attr(e, "SYM"); ok {
			sym = v
		}
		f.sym = ps.layer.symbol(sym)
		ps.addFeature(f, "")
	case "CAD_P":
		if ps.layer == nil {
			return nil
		}
		p, err := coordXY(e, "X", "Y")
		if err != nil {
			return err
		}
		f := feature{kind: featPad, a: p, positive: !ps.neg, sym: ps.layer.symbol(attrS(e, "SYM"))}
		if v, ok := attr(e, "ANGLE"); ok {
			f.rot, _ = strconv.ParseFloat(strings.TrimSpace(v), 64)
		}
		f.mirror = strings.EqualFold(attrS(e, "MIRROR"), "YES")
		ps.addFeature(f, attrS(e, "PIN"))
	case "CAD_S":
		if ps.layer != nil {
			ps.surf = &feature{kind: featSurface, positive: !ps.neg, sym: -1}
		}
	case "ISLAND":
		ps.inHole = false
	case "HOLE":
		ps.inHole = true
	case "OB":
		p, err := coordXY(e, "X", "Y")
		if err != nil {
			return err
		}
		ps.poly = &contour{hole: ps.inHole, pts: []pt{p}}
		ps.lastPt = p
	case "OS":
		if ps.poly == nil {
			return errors.New("<OS> outside a polygon")
		}
		p, err := coordXY(e, "X", "Y")
		if err != nil {
			return err
		}
		ps.poly.pts = append(ps.poly.pts, p)
		ps.lastPt = p
	case "OC":
		if ps.poly == nil {
			return errors.New("<OC> outside a polygon")
		}
		p, err := coordXY(e, "X", "Y")
		if err != nil {
			return err
		}
		c, err := coordXY(e, "XC", "YC")
		if err != nil {
			return err
		}
		arc := flattenArc(ps.lastPt, p, c, strings.EqualFold(attrS(e, "CW"), "YES"))
		ps.poly.pts = append(ps.poly.pts, arc[1:]...)
		ps.lastPt = p
	}
	return nil
}

func (ps *odxParser) end(name string) error {
	switch name {
	case "CAD_PACKAGE":
		ps.pkg = nil
	case "CAD_PIN":
		ps.pin = nil
	case "CAD_COMPONENT":
		ps.comp = nil
	case "CAD_LNET":
		ps.net = -1
	case "CAD_LYR_F", "CAD_SYMBOL":
		ps.layer = nil
	case "CAD_STEP":
		ps.step = nil
	case "POLYGON":
		c := ps.poly
		ps.poly = nil
		switch {
		case c == nil:
		case ps.surf != nil:
			if !c.hole || len(ps.surf.islands) == 0 {
				ps.surf.islands = append(ps.surf.islands, nil)
			}
			isl := &ps.surf.islands[len(ps.surf.islands)-1]
			*isl = append(*isl, *c)
		case ps.inProf:
			ps.profile = append(ps.profile, *c)
		}
	case "CAD_S":
		if ps.surf != nil {
			f := *ps.surf
			ps.surf = nil
			ps.addFeature(f, "")
		}
	case "CAD_PROFILE":
		ps.inProf = false
		if ps.step != nil && len(ps.profile) > 0 {
			isl := make([][]contour, 0, len(ps.profile))
			for _, c := range ps.profile {
				isl = append(isl, []contour{c})
			}
			ps.step.profile = &featuresFile{features: []feature{{kind: featSurface, positive: true, sym: -1, islands: isl}}}
		}
	}
	return nil
}

// beginLayer starts a feature stream with the CAD_SET state reset.
func (ps *odxParser) beginLayer() {
	ps.layer = &odxLayer{ff: &featuresFile{}, syms: map[string]int{}}
	ps.state, ps.sym, ps.neg = odxFeat{}, "", false
}

func (ps *odxParser) addFeature(f feature, pin string) {
	ps.layer.ff.features = append(ps.layer.ff.features, f)
	st := ps.state
	st.pin = pin
	ps.layer.feats = append(ps.layer.feats, st)
}

// job turns the parsed document into the parsedJob the job reader builds from a directory. The step
// is the one named "pcb", as openJob picks, or else the first.
func (ps *odxParser) job() (*parsedJob, error) {
	if ps.units == "" {
		return nil, errors.New("odb: odx: no CAD_DATA_HEADER, so the units are unknown")
	}
	if len(ps.steps) == 0 {
		return nil, errors.New("odb: odx: the file lists no CAD_STEP")
	}
	st := ps.steps[0]
	for _, s := range ps.steps {
		if strings.EqualFold(s.name, "pcb") {
			st = s
			break
		}
	}
	m := &matrix{layers: ps.layers}
	for _, s := range ps.steps {
		m.steps = append(m.steps, s.name)
	}
	p := &parsedJob{job: &job{step: st.name, matrix: m}, info: ps.info, eda: &eda{pkgs: st.pkgs}}
	p.odx = &odxTier{profile: st.profile, layers: map[string]*featuresFile{}, symbols: ps.symbols}

	layerIdx := map[string]int{}
	for i, l := range ps.layers {
		p.eda.layers = append(p.eda.layers, l.name)
		layerIdx[strings.ToUpper(l.name)] = i
	}

	netIdx := map[string]int{}
	for i, n := range st.nets {
		p.eda.nets = append(p.eda.nets, edaNet{name: n})
		if _, ok := netIdx[n]; !ok {
			netIdx[n] = i
		}
	}
	netOf := func(name string) int {
		if name == "" {
			name = noNet
		}
		if i, ok := netIdx[name]; ok {
			return i
		}
		netIdx[name] = len(p.eda.nets)
		p.eda.nets = append(p.eda.nets, edaNet{name: name})
		return netIdx[name]
	}

	// The two sides, in the order componentLayers lists them, so a TOP subnet's component number
	// indexes the side the board builder reads it from.
	// A file that lists no component layer still has a front and a back.
	_, back := m.componentLayers()
	if len(back) == 0 {
		back = []bool{false, true}
	}
	sideOf := map[bool]int{}
	for i, b := range back {
		if _, ok := sideOf[b]; !ok {
			sideOf[b] = i
		}
		p.sides = append(p.sides, nil)
		p.sideTop = append(p.sideTop, !b)
	}
	type pinAt struct{ side, comp, toep int }
	pins := map[[2]string]pinAt{}
	for _, c := range st.comps {
		si, ok := sideOf[c.back]
		if !ok {
			return nil, fmt.Errorf("odb: odx: %s is on a side the layer list has no component layer for", c.ref)
		}
		if c.pkg >= 0 && c.pkg < len(st.pkgs) {
			for k, pp := range st.pkgs[c.pkg].pins {
				net := -1
				if i, ok := st.pinNet[[2]string{c.ref, pp.name}]; ok {
					net = i
				}
				c.pins = append(c.pins, toeprint{num: k, name: pp.name, at: odxPlace(c, pp.at), rot: c.rot, net: net})
				pins[[2]string{c.ref, pp.name}] = pinAt{si, len(p.sides[si]), k}
			}
		}
		p.sides[si] = append(p.sides[si], c)
	}

	// Each feature becomes an FID under the subnet eda/data would put it in. A pin's pad is a TOP
	// subnet of that pin. A via's lands and drill hit, which share a net and a position, are one VIA
	// subnet. Anything else on a net is a TRC subnet of the net.
	type viaKey struct {
		net int
		at  [2]int64
	}
	vias := map[viaKey]int{}
	tops := map[pinAt][2]int{} // pin -> (net, subnet)
	trc := map[[2]int]int{}
	for _, name := range st.order {
		upper := strings.ToUpper(name)
		l := st.layers[upper]
		p.odx.layers[upper] = l.ff
		li, ok := layerIdx[upper]
		if !ok {
			continue
		}
		typ := byte('C')
		if ps.layers[li].typ == "DRILL" {
			typ = 'H'
		}
		for fi, f := range l.feats {
			id := fid{typ: typ, layer: li, feat: fi}
			if f.pin != "" {
				ref, pin, ok := strings.Cut(f.pin, ":")
				at, known := pins[[2]string{ref, pin}]
				if ok && known {
					// A pin's pads on every layer are one subnet, under the net its first pad names.
					at2, found := tops[at]
					if !found {
						side := "T"
						if back[at.side] {
							side = "B"
						}
						ni := netOf(f.net)
						net := &p.eda.nets[ni]
						net.subnets = append(net.subnets, subnet{typ: "TOP", side: side, comp: at.comp, toep: at.toep})
						at2 = [2]int{ni, len(net.subnets) - 1}
						tops[at] = at2
					}
					sn := &p.eda.nets[at2[0]].subnets[at2[1]]
					sn.fids = append(sn.fids, id)
					continue
				}
			}
			// A via's lands say USAGE="VIA". Its drill hit says only its net, so any drill hit that
			// is not a pin's is taken for the via at its position. A CAD_SET keeps its usage until
			// the next one changes it, and Allegro writes a via's traces after its pads without
			// resetting it, so the usage marks pads alone (agni 1383).
			if (f.usage == "VIA" && l.ff.features[fi].kind == featPad) || typ == 'H' {
				ni := netOf(f.net)
				k := viaKey{ni, micron(l.ff.features[fi].a)}
				si, found := vias[k]
				if !found {
					net := &p.eda.nets[ni]
					net.subnets = append(net.subnets, subnet{typ: "VIA"})
					si = len(net.subnets) - 1
					vias[k] = si
				}
				p.eda.nets[ni].subnets[si].fids = append(p.eda.nets[ni].subnets[si].fids, id)
				continue
			}
			// A pour on no net is kept on $NONE$, as eda/data keeps it, so a board's unnamed copper is
			// still drawn. A line on no net is artwork on a copper layer, which the board's own read
			// leaves out too.
			if f.net == "" && l.ff.features[fi].kind != featSurface {
				continue
			}
			ni := netOf(f.net)
			k := [2]int{ni, li}
			si, found := trc[k]
			if !found {
				net := &p.eda.nets[ni]
				net.subnets = append(net.subnets, subnet{typ: "TRC"})
				si = len(net.subnets) - 1
				trc[k] = si
			}
			p.eda.nets[ni].subnets[si].fids = append(p.eda.nets[ni].subnets[si].fids, id)
		}
	}
	return p, nil
}

// odxPlace puts a package pin's position, in the package's own frame, onto the board. It turns the
// pin clockwise by the part's angle and then mirrors a bottom-side part across its Y axis. That is
// the order localOffset inverts, so a pad composes back onto the point ODX draws it at.
func odxPlace(c component, local pt) pt {
	rad := -c.rot * math.Pi / 180
	cos, sin := math.Cos(rad), math.Sin(rad)
	x, y := float64(local.x), float64(local.y)
	x, y = cos*x-sin*y, sin*x+cos*y
	if c.back != c.mirror {
		x = -x
	}
	return pt{c.at.x + int64(math.Round(x)), c.at.y + int64(math.Round(y))}
}
