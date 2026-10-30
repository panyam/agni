package main

import (
	"fmt"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// backAnnotatedShown is how many relabelled placements the note names before counting the rest.
const backAnnotatedShown = 5

// backAnnotatedNote is the stderr line for a schematic whose back-annotation renamed parts the page
// draws under another designator, empty when it renamed none (agni issue 1230). Every read names the
// part by the back-annotated designator, so a reader comparing the drawing with an answer needs to
// know which labels were replaced.
func backAnnotatedNote(file string, g *geom.SchematicGeometry) string {
	ba := g.GetBackAnnotated()
	if len(ba) == 0 {
		return ""
	}
	var named []string
	for _, p := range ba[:min(len(ba), backAnnotatedShown)] {
		if p.GetDrawnRefDes() == "" {
			named = append(named, p.GetRefDes()+" has no designator drawn")
		} else {
			named = append(named, p.GetDrawnRefDes()+" is "+p.GetRefDes())
		}
	}
	if more := len(ba) - len(named); more > 0 {
		named = append(named, fmt.Sprintf("%d more", more))
	}
	return fmt.Sprintf("note: %s back-annotates %d placement(s) under a designator its page does not draw, and they are read by the back-annotated one (%s).\n",
		file, len(ba), strings.Join(named, ", "))
}
