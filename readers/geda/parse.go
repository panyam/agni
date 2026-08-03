package geda

import (
	"strconv"
	"strings"
)

// gEDA gschem is line-oriented. Most objects are one line; a text (T) or path (H) object is a
// header line whose last field is a line count followed by that many content lines; and any
// object may be followed by a "{ ... }" attribute block whose contents are T objects. These
// helpers read those two shapes.

// readAttrBlock reads a "{ ... }" attribute block if one begins at lines[start], returning the
// key/value attributes from the T objects inside, EVERY net= value in source order (a component
// carries one net= per hidden pin tap, e.g. net=jtag_power:14 and net=GND:7, which the single-valued
// map would collapse), and the index just past the block. If lines[start] is not "{", it returns
// empties and start unchanged.
func readAttrBlock(lines []string, start int) (attrs map[string]string, nets []string, next int) {
	attrs = map[string]string{}
	if start >= len(lines) || strings.TrimSpace(lines[start]) != "{" {
		return attrs, nets, start
	}
	i := start + 1
	for i < len(lines) && strings.TrimSpace(lines[i]) != "}" {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "T ") {
			text, nx := readText(lines, i)
			if k, v, ok := splitAttr(text); ok {
				attrs[k] = v
				if k == "net" {
					nets = append(nets, v)
				}
			}
			i = nx
			continue
		}
		i++
	}
	if i < len(lines) {
		i++ // consume "}"
	}
	return attrs, nets, i
}

// readText consumes a gEDA text (T) or path (H) object: a header line whose last field is the
// content-line count, followed by that many content lines. It returns the joined content and
// the index just past the object.
func readText(lines []string, start int) (string, int) {
	f := strings.Fields(lines[start])
	n := 0
	if len(f) > 0 {
		if v, ok := lastInt(f); ok {
			n = v
		}
	}
	i := start + 1
	var body []string
	for ; i < len(lines) && len(body) < n; i++ {
		body = append(body, lines[i])
	}
	return strings.Join(body, "\n"), i
}

// splitAttr parses the first line of a text object as a "key=value" attribute.
func splitAttr(text string) (key, val string, ok bool) {
	if nl := strings.IndexByte(text, '\n'); nl >= 0 {
		text = text[:nl]
	}
	eq := strings.IndexByte(text, '=')
	if eq <= 0 {
		return "", "", false
	}
	return strings.TrimSpace(text[:eq]), strings.TrimSpace(text[eq+1:]), true
}

func lastInt(f []string) (int, bool) {
	if len(f) == 0 {
		return 0, false
	}
	return parseInt(f[len(f)-1])
}

func parseInt(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	return n, err == nil
}
