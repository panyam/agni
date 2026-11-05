// Package boardlayer names a board's copper layers in the contract's KiCad-style vocabulary, so every
// board reader renames its source's layers the same way and the renderer and board rules read every
// format alike.
package boardlayer

import "strconv"

// CopperName names copper layer i of n, top to bottom: the first is F.Cu, the last B.Cu, and the
// inner ones In1.Cu upward. A one-layer board's only layer is F.Cu.
func CopperName(i, n int) string {
	switch {
	case i == 0:
		return "F.Cu"
	case i == n-1:
		return "B.Cu"
	}
	return "In" + strconv.Itoa(i) + ".Cu"
}
