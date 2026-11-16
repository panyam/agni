package allegro

// A part's height in Allegro is a field on its place-bound outline (agni 1643). Each placed part
// (0x2D) owns a package geometry shape (0x28) or rectangle (0x0E), which names the part at +12 and
// heads a chain of field blocks (0x03), each linked to the next at +8. The height is the field of
// sub-type 0x66 whose two bytes at +2 read 0x10 0x01, an integer in the board's units. The package
// definition (0x2B) owns an outline of its own with the same field, which no part reads.
//
// parasdr's 16.6 board gives 284 parts a height this way, and the ODX file Allegro exported from it
// states the same 284. The BeagleBone AI's 17.2 board stores the same fields at the 17.2 offsets.
// Its extract states no heights, so those values have no oracle.

const (
	boundOwner   = 12
	fieldNext    = 8
	heightSub    = 0x66
	heightField0 = 0x10
	heightField1 = 0x01
)

// partHeights maps each placed part's key to its height in nanometres. A part with more than one
// outline stating a height, as a top and a bottom place bound, takes the taller.
func (p *board) partHeights() map[uint32]int64 {
	out := map[uint32]int64{}
	for _, off := range p.keys {
		var head uint32
		switch p.b[off] {
		case 0x28:
			head = p.u32(off, p.l.boundFields28)
		case 0x0E:
			head = p.u32(off, p.l.boundFields0E)
		default:
			continue
		}
		owner := p.u32(off, boundOwner)
		if inst, ok := p.at(owner, tagFootInst); !ok || inst < 0 {
			continue
		}
		for k, steps := head, 0; k != 0 && steps <= 64; steps++ {
			fo, ok := p.at(k, 0x03)
			if !ok {
				break
			}
			if p.b[fo+p.l.fieldSub] == heightSub && p.b[fo+2] == heightField0 && p.b[fo+3] == heightField1 {
				if h := p.nm(float64(int32(le32(p.b, fo+p.l.fieldBody)))); h > out[owner] {
					out[owner] = h
				}
			}
			k = le32(p.b, fo+fieldNext)
		}
	}
	return out
}
