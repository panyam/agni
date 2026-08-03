package geda

import (
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// gedaPointsToMil converts a gEDA text size (in points) to world units (mils). gEDA world
// coordinates are mils (1/1000 inch) and a point is 1/72 inch, so one point is 1000/72 mils.
const gedaPointsToMil = 1000.0 / 72.0

// attrBlockFields reads a gEDA "{ ... }" attribute block, returning both the key/value map (as
// readAttrBlock does) and, for the geometry reader, one geom.Field per attribute placed at the
// attribute text's own coordinates. Unlike readAttrBlock this keeps each T object's position,
// size, angle, alignment, and visibility, so ref-des/value text is drawn where gschem drew it
// rather than stamped at the symbol origin. Returns the index just past the block.
//
// A gEDA attribute is a text object:
//
//	T x y color size visibility show_name_value angle alignment num_lines
//	key=value
//
// show_name_value selects the drawn text: 0 = "key=value", 1 = value only, 2 = name only.
func attrBlockFields(lines []string, start int) (map[string]string, []*geom.Field, int) {
	attrs := map[string]string{}
	var fields []*geom.Field
	if start >= len(lines) || strings.TrimSpace(lines[start]) != "{" {
		return attrs, fields, start
	}
	i := start + 1
	for i < len(lines) && strings.TrimSpace(lines[i]) != "}" {
		f := strings.Fields(lines[i])
		if len(f) == 0 || f[0] != "T" {
			i++
			continue
		}
		text, next := readText(lines, i)
		i = next
		key, val, ok := splitAttr(text)
		if !ok {
			continue
		}
		attrs[key] = val
		fields = append(fields, attrField(f, key, val))
	}
	if i < len(lines) {
		i++ // consume "}"
	}
	return attrs, fields, i
}

// attrField builds a geom.Field from a gEDA attribute T header f and its key/value.
func attrField(f []string, key, val string) *geom.Field {
	x, _ := atof(field(f, 1))
	y, _ := atof(field(f, 2))
	size, _ := atof(field(f, 4))
	visible := field(f, 5) == "1"
	show := atoiInt(field(f, 6))
	angle := atoiInt(field(f, 7))
	align := atoiInt(field(f, 8))

	return &geom.Field{
		Name:        fieldName(key),
		Value:       shownText(show, key, val),
		Origin:      gedaPt(x, y),
		Height:      int64(size * gedaPointsToMil),
		Visible:     visible,
		RotationDeg: int32(angle % 360),
		Justify:     gedaAlign(align),
	}
}

// fieldName maps a gEDA attribute key to the neutral field name.
func fieldName(key string) string {
	switch key {
	case "refdes":
		return "Reference"
	case "value":
		return "Value"
	default:
		return key
	}
}

// shownText renders what gschem draws for a show_name_value setting: value only (1), name only
// (2), or "name=value" (0).
func shownText(show int, key, val string) string {
	switch show {
	case 2:
		return key
	case 0:
		return key + "=" + val
	default: // 1
		return val
	}
}

// gedaAlign maps a gEDA text alignment code (0-8, a 3x3 grid with a lower-left origin) to the
// canonical "<h> <v>" justify string.
func gedaAlign(a int) string {
	h := []string{"left", "center", "right"}[min(max(a/3, 0), 2)]
	v := []string{"bottom", "middle", "top"}[min(max(a%3, 0), 2)]
	return h + " " + v
}
