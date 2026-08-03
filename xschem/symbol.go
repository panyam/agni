package xschem

// Symbol (.sym) geometry: the pins of a symbol, needed to place a component's terminals on the
// schematic grid for netlisting. In xschem a pin is a box object on the pin layer (5) carrying
// a {name=.. pinnumber=..} attribute block; the pin's connection point is the box centre.

// symbolPin is one pin of a symbol in symbol-local coordinates.
type symbolPin struct {
	x, y   float64 // connection point (box centre), symbol-local
	number string  // pinnumber attribute -> pin designator
	name   string  // name attribute -> pin name
	dir    string  // dir attribute (in/out/inout); "" when absent
}

// symbolPins extracts the pins from a parsed .sym object stream. A pin is a B (box) object
// whose attribute block names it (name= and/or pinnumber=); its connection point is the box
// centre. Boxes without a name/pinnumber (graphic rectangles) are ignored.
func symbolPins(objs []object) []symbolPin {
	var pins []symbolPin
	for _, o := range objs {
		if o.typ != 'B' {
			continue
		}
		// B layer x1 y1 x2 y2 {props}
		p := props(lastBrace(o))
		if _, hasName := p["name"]; !hasName {
			if _, hasNum := p["pinnumber"]; !hasNum {
				continue // a graphic box, not a pin
			}
		}
		x1, ok1 := atoi(o.word(1))
		y1, ok2 := atoi(o.word(2))
		x2, ok3 := atoi(o.word(3))
		y2, ok4 := atoi(o.word(4))
		if !(ok1 && ok2 && ok3 && ok4) {
			continue
		}
		num := p["pinnumber"]
		if num == "" {
			num = p["name"]
		}
		pins = append(pins, symbolPin{
			x:      (x1 + x2) / 2,
			y:      (y1 + y2) / 2,
			number: num,
			name:   p["name"],
			dir:    p["dir"],
		})
	}
	return pins
}
