package odb

import (
	"fmt"
	"strings"
)

// component is one CMP record of a components file and the records under it.
type component struct {
	pkg    int // index into eda.pkgs
	at     pt
	rot    float64 // clockwise degrees
	mirror bool
	ref    string
	part   string
	props  [][2]string // PRP records in file order
	pins   []toeprint
	back   bool // read from the bottom components layer
}

// toeprint is one TOP record: where a pin lands on the board, and the net number it is on.
type toeprint struct {
	num  int
	at   pt
	rot  float64
	net  int
	name string
}

// parseComponents reads one side's components file.
func parseComponents(b []byte, back bool) ([]component, error) {
	u := unitsOf(b)
	var out []component
	var cur *component
	err := lines(b, func(n int, line string) error {
		w, _ := fields(line)
		if len(w) == 0 {
			return nil
		}
		switch w[0] {
		case "CMP":
			if len(w) < 7 {
				return fmt.Errorf("line %d: CMP needs package, x, y, rotation, mirror and name", n)
			}
			c := component{back: back}
			var err error
			if c.pkg, err = atoi(w[1]); err != nil {
				return fmt.Errorf("line %d: %w", n, err)
			}
			if c.at, err = coordPair(w[2], w[3], u); err != nil {
				return fmt.Errorf("line %d: %w", n, err)
			}
			if c.rot, err = atof(w[4]); err != nil {
				return fmt.Errorf("line %d: %w", n, err)
			}
			c.mirror = strings.EqualFold(w[5], "M")
			c.ref = w[6]
			if len(w) > 7 {
				c.part = w[7]
			}
			out = append(out, c)
			cur = &out[len(out)-1]
		case "PRP":
			if cur != nil && len(w) >= 2 {
				v := ""
				if len(w) >= 3 {
					v = w[2]
				}
				cur.props = append(cur.props, [2]string{w[1], v})
			}
		case "TOP":
			if cur == nil || len(w) < 9 {
				return fmt.Errorf("line %d: TOP needs pin, x, y, rotation, mirror, net, subnet and name", n)
			}
			t := toeprint{}
			var err error
			if t.num, err = atoi(w[1]); err != nil {
				return fmt.Errorf("line %d: %w", n, err)
			}
			if t.at, err = coordPair(w[2], w[3], u); err != nil {
				return fmt.Errorf("line %d: %w", n, err)
			}
			if t.rot, err = atof(w[4]); err != nil {
				return fmt.Errorf("line %d: %w", n, err)
			}
			if t.net, err = atoi(w[6]); err != nil {
				return fmt.Errorf("line %d: %w", n, err)
			}
			t.name = w[8]
			cur.pins = append(cur.pins, t)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("odb: components: %w", err)
	}
	return out, nil
}

// componentLayers lists the matrix's component layers, front first, and says which side each is.
// A COMPONENT layer above the first copper layer is the top, below the last the bottom; a job
// whose matrix leaves that unclear falls back to the name (comp_+_top / comp_+_bot).
func (m *matrix) componentLayers() (names []string, back []bool) {
	first, last := -1, -1
	for _, l := range m.layers {
		if l.isCopper() {
			if first < 0 {
				first = l.row
			}
			last = l.row
		}
	}
	for _, l := range m.layers {
		if l.typ != "COMPONENT" {
			continue
		}
		isBack := strings.Contains(strings.ToLower(l.name), "bot")
		if first >= 0 && !strings.Contains(strings.ToLower(l.name), "top") && !isBack {
			isBack = l.row > last
		}
		names = append(names, l.name)
		back = append(back, isBack)
	}
	return names, back
}
