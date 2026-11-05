package formats

import (
	"bufio"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
)

// allegroExtract is an Allegro ASCII extract (`extracta` output, a cds2f_<board>.txt) parsed into the
// rows the Allegro reader is held to. An `A!` row names the fields of the `S!` rows that follow it,
// and every row ends with a `!`. Coordinates are in the units the `J!` row states (mils on the
// BeagleBone AI), as decimal text.
type allegroExtract struct {
	comps map[string]extractComp // by ref-des
	pins  []extractPin           // one per component pin, connected or not
	vias  []extractVia
	pinAt map[string][2]float64 // "<ref>.<pin number>" -> board position in mils
	// widths holds every ETCH line and arc width the extract lists, in thousandths of a mil.
	widths map[int64]bool
	// padRows are the padstack rows, one per padstack and layer, in the extract's order.
	padRows []extractPadRow
}

// extractPadRow is one padstack on one layer: its regular pad, thermal relief and antipad, each as
// the extract's shape word and its width and height in mils.
type extractPadRow struct {
	padstack, layer string
	pad, thermal    extractPadShape
	antipad         extractPadShape
}

type extractPadShape struct {
	shape string
	w, h  float64
}

type extractComp struct {
	sym     string
	mirror  bool
	rotDeg  float64
	x, y    float64 // mils
	value   string
	devType string
}

type extractPin struct {
	net, ref, number, name string
}

type extractVia struct {
	x, y float64 // mils
	net  string
}

func readAllegroExtract(t *testing.T, path string) *allegroExtract {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	x := &allegroExtract{comps: map[string]extractComp{}, pinAt: map[string][2]float64{}, widths: map[int64]bool{}}
	var header []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		fields := strings.Split(line, "!")
		if len(fields) < 2 {
			continue
		}
		cells := fields[1 : len(fields)-1]
		switch fields[0] {
		case "A":
			header = cells
			continue
		case "S":
		default:
			continue
		}
		row := map[string]string{}
		for i, h := range header {
			if i < len(cells) {
				row[h] = cells[i]
			}
		}
		switch header[0] {
		case "REFDES":
			x.comps[row["REFDES"]] = extractComp{
				sym:     row["SYM_NAME"],
				mirror:  row["SYM_MIRROR"] == "YES",
				rotDeg:  extractNum(t, row["SYM_ROTATE"]),
				x:       extractNum(t, row["SYM_X"]),
				y:       extractNum(t, row["SYM_Y"]),
				value:   row["COMP_VALUE"],
				devType: row["COMP_DEVICE_LABEL"],
			}
		case "NET_NAME":
			x.pins = append(x.pins, extractPin{net: row["NET_NAME"], ref: row["REFDES"], number: row["PIN_NUMBER"], name: row["PIN_NAME"]})
		case "SYM_NAME":
			if row["PIN_NUMBER"] != "" && row["REFDES"] != "" {
				x.pinAt[row["REFDES"]+"."+row["PIN_NUMBER"]] = [2]float64{extractNum(t, row["PIN_X"]), extractNum(t, row["PIN_Y"])}
			}
		case "CLASS":
			// A line's width is GRAPHIC_DATA_5 and an arc's GRAPHIC_DATA_8.
			if row["CLASS"] == "ETCH" && row["NET_NAME"] != "" {
				w := map[string]string{"LINE": row["GRAPHIC_DATA_5"], "ARC": row["GRAPHIC_DATA_8"]}[row["GRAPHIC_DATA_NAME"]]
				if w != "" {
					x.widths[int64(math.Round(extractNum(t, w)*1000))] = true
				}
			}
		case "PAD_NAME":
			// The technical rows (~DRILL, ~TSM) put words in the size columns, so only the copper
			// rows are read.
			if strings.HasPrefix(row["LAYER"], "~") {
				continue
			}
			shape := func(prefix string) extractPadShape {
				return extractPadShape{shape: row[prefix+"SHAPE1"], w: extractNum(t, row[prefix+"WIDTH"]), h: extractNum(t, row[prefix+"HGHT"])}
			}
			x.padRows = append(x.padRows, extractPadRow{padstack: row["PAD_NAME"], layer: row["LAYER"], pad: shape("PAD"), thermal: shape("TREL"), antipad: shape("APAD")})
		case "VIA_X":
			x.vias = append(x.vias, extractVia{x: extractNum(t, row["VIA_X"]), y: extractNum(t, row["VIA_Y"]), net: row["NET_NAME"]})
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return x
}

func extractNum(t *testing.T, s string) float64 {
	t.Helper()
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatalf("extract number %q: %v", s, err)
	}
	return v
}
