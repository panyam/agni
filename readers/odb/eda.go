package odb

import (
	"fmt"
	"strings"
)

// eda is the parsed eda/data file: the nets, which copper features each net owns, and the packages
// components are placed from.
type eda struct {
	layers []string // the LYR list, which an FID's layer number indexes
	nets   []edaNet // in file order; a component pin's net number indexes this
	pkgs   []edaPkg // in file order; a CMP record's package number indexes this
}

type edaNet struct {
	name    string
	subnets []subnet
}

// subnet is one SNT record and the features under it. A TOP subnet is a component pin's pad (side,
// component number, toeprint number); TRC, VIA and PLN are a net's tracks, vias and planes.
type subnet struct {
	typ        string
	side       string
	comp, toep int
	fids       []fid
}

// fid points at one feature: its kind (C copper, L laminate, H hole), the layer number in the LYR
// list, and the feature's index in that layer's features file.
type fid struct {
	typ   byte
	layer int
	feat  int
}

type edaPkg struct {
	name string
	pins []edaPin
}

// edaPin is one pin of a package: its name, its type (T through-hole, S surface, B blind), and its
// centre in the package's own frame.
type edaPin struct {
	name string
	typ  string
	at   pt
}

// noNet is the name ODB++ gives the net of unconnected features.
const noNet = "$NONE$"

func parseEDA(b []byte) (*eda, error) {
	u := unitsOf(b)
	e := &eda{}
	var net *edaNet
	var sn *subnet
	var pkg *edaPkg
	err := lines(b, func(n int, line string) error {
		w, _ := fields(line)
		if len(w) == 0 {
			return nil
		}
		switch w[0] {
		case "LYR":
			e.layers = append(e.layers, w[1:]...)
		case "NET":
			name := ""
			if len(w) > 1 {
				name = w[1]
			}
			e.nets = append(e.nets, edaNet{name: name})
			net, sn, pkg = &e.nets[len(e.nets)-1], nil, nil
		case "SNT":
			if net == nil || len(w) < 2 {
				return fmt.Errorf("line %d: SNT outside a net", n)
			}
			s := subnet{typ: strings.ToUpper(w[1]), comp: -1, toep: -1}
			if s.typ == "TOP" {
				if len(w) < 5 {
					return fmt.Errorf("line %d: SNT TOP needs side, component and toeprint", n)
				}
				s.side = strings.ToUpper(w[2])
				var err error
				if s.comp, err = atoi(w[3]); err != nil {
					return fmt.Errorf("line %d: %w", n, err)
				}
				if s.toep, err = atoi(w[4]); err != nil {
					return fmt.Errorf("line %d: %w", n, err)
				}
			}
			net.subnets = append(net.subnets, s)
			sn = &net.subnets[len(net.subnets)-1]
		case "FID":
			if sn == nil || len(w) < 4 {
				return fmt.Errorf("line %d: FID outside a subnet", n)
			}
			layer, err := atoi(w[2])
			if err != nil {
				return fmt.Errorf("line %d: %w", n, err)
			}
			feat, err := atoi(w[3])
			if err != nil {
				return fmt.Errorf("line %d: %w", n, err)
			}
			sn.fids = append(sn.fids, fid{typ: strings.ToUpper(w[1])[0], layer: layer, feat: feat})
		case "PKG":
			if len(w) < 2 {
				return fmt.Errorf("line %d: PKG with no name", n)
			}
			e.pkgs = append(e.pkgs, edaPkg{name: w[1]})
			pkg, net, sn = &e.pkgs[len(e.pkgs)-1], nil, nil
		case "PIN":
			if pkg == nil || len(w) < 5 {
				return fmt.Errorf("line %d: PIN outside a package", n)
			}
			at, err := coordPair(w[3], w[4], u)
			if err != nil {
				return fmt.Errorf("line %d: %w", n, err)
			}
			pkg.pins = append(pkg.pins, edaPin{name: w[1], typ: strings.ToUpper(w[2]), at: at})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("odb: eda/data: %w", err)
	}
	return e, nil
}

// netName is a net number's name, "" for the unconnected net or a number the file never defined.
func (e *eda) netName(i int) string {
	if e == nil || i < 0 || i >= len(e.nets) {
		return ""
	}
	if name := e.nets[i].name; name != noNet {
		return name
	}
	return ""
}
