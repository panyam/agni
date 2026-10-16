package query

import "strconv"

// Display is a value as a person reads it in an answer: a cell, a csv field, a rule's message.
//
// A length is published in metres (see stdlib/relations), so a 0.08 mm track is the value 8e-05
// with base unit m, and a threshold written `0.1mm`, which the engine folds to metres, compares
// against it correctly. A board is drawn in millimetres, so Display shows a length in millimetres
// again, and the cell's number is then the one the board's own tools print. Every other value is shown as the engine holds it (agni issue 1035).
//
// Twelve significant digits drop the noise of scaling by 1000, so 0.1524 mm reads 0.1524 and not
// 0.15240000000000004.
func Display(v Value) string {
	if v.Num != nil && !v.Absent && v.BaseUnit == "m" {
		return strconv.FormatFloat(*v.Num*1000, 'g', 12, 64)
	}
	return v.S
}
