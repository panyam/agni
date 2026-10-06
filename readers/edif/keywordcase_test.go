package edif

import (
	"bytes"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode"

	"google.golang.org/protobuf/proto"
)

// valueKeywords are the EDIF keywords the reader reads as a value rather than a list head: a port
// direction, a unit, an orientation and a justification. Each is respelled only as the first
// argument of the list that holds it (valueHeads), since the same word elsewhere is a name, such as a
// port called OUTPUT.
var valueKeywords = map[string]bool{
	"INPUT": true, "OUTPUT": true, "INOUT": true, "DISTANCE": true,
	"R0": true, "R90": true, "R180": true, "R270": true, "MX": true, "MY": true, "MXR90": true, "MYR90": true,
	"UPPERLEFT": true, "UPPERCENTER": true, "UPPERRIGHT": true, "CENTERLEFT": true, "CENTERCENTER": true,
	"CENTERRIGHT": true, "LOWERLEFT": true, "LOWERCENTER": true, "LOWERRIGHT": true,
}

var valueHeads = map[string]bool{"direction": true, "unit": true, "orientation": true, "justify": true}

// respell rewrites every keyword in an EDIF document with spell, meaning the word opening each list
// and each value keyword, and leaves names, numbers and strings as written. EDIF strings hold no
// escaped quote, so a quote always toggles.
func respell(src []byte, spell func(string) string) []byte {
	var out bytes.Buffer
	inString := false
	afterParen := false
	valueNext := false
	for i := 0; i < len(src); {
		c := src[i]
		if c == '"' {
			inString = !inString
		}
		if inString || c == '"' || strings.ContainsRune(" \t\r\n", rune(c)) {
			out.WriteByte(c)
			i++
			continue
		}
		if c == '(' || c == ')' {
			out.WriteByte(c)
			afterParen = c == '('
			valueNext = false
			i++
			continue
		}
		k := i
		for k < len(src) && !strings.ContainsRune(" \t\r\n()\"", rune(src[k])) {
			k++
		}
		word := string(src[i:k])
		head := afterParen
		if head || (valueNext && valueKeywords[strings.ToUpper(word)]) {
			word = spell(word)
		}
		out.WriteString(word)
		valueNext = head && valueHeads[strings.ToLower(word)]
		afterParen = false
		i = k
	}
	return out.Bytes()
}

// TestKeywordCaseNeverChangesARead holds the reader to EDIF's rule that keywords are
// case-insensitive (agni issue 941). Every committed fixture is read as written and again with its
// keywords in upper case, lower case and a mixed case, and each read must equal the original, the
// netlist for every fixture and the drawing for every schematic export.
func TestKeywordCaseNeverChangesARead(t *testing.T) {
	files, err := filepath.Glob("testdata/*.ed[ns]")
	if err != nil || len(files) < 20 {
		t.Fatalf("expected the EDIF fixtures, found %d: %v", len(files), err)
	}
	rng := rand.New(rand.NewSource(941))
	spellings := map[string]func(string) string{
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"mixed": func(s string) string {
			r := []rune(s)
			for i := range r {
				if rng.Intn(2) == 0 {
					r[i] = unicode.ToUpper(r[i])
				} else {
					r[i] = unicode.ToLower(r[i])
				}
			}
			return string(r)
		},
	}
	schematics := 0
	for _, f := range files {
		name := filepath.Base(f)
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		want, err := Read(bytes.NewReader(src), name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		isSchematic := strings.HasSuffix(name, ".eds")
		var wantGeo proto.Message
		if isSchematic {
			schematics++
			if wantGeo, err = ReadSchematic(bytes.NewReader(src), name); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
		for how, spell := range spellings {
			alt := respell(src, spell)
			if bytes.Equal(alt, src) {
				continue
			}
			got, err := Read(bytes.NewReader(alt), name)
			if err != nil {
				t.Errorf("%s in %s case: %v", name, how, err)
				continue
			}
			if !proto.Equal(got, want) {
				t.Errorf("%s in %s case reads %d components and %d nets, against %d and %d as written",
					name, how, len(got.GetComponents()), len(got.GetNets()), len(want.GetComponents()), len(want.GetNets()))
			}
			if isSchematic {
				geo, err := ReadSchematic(bytes.NewReader(alt), name)
				if err != nil {
					t.Errorf("%s schematic in %s case: %v", name, how, err)
				} else if !proto.Equal(geo, wantGeo) {
					t.Errorf("%s in %s case draws differently from the file as written", name, how)
				}
			}
		}
	}
	if schematics == 0 {
		t.Fatal("no schematic export among the fixtures, so the drawing half went unchecked")
	}
}

// TestAltiumCasedNetlistReads reads a netlist in the casing Altium's EDIF For PCB export writes,
// which capitalizes Instance, InstanceRef, LibraryRef, Net, Joined, PortRef, Property and String.
func TestAltiumCasedNetlistReads(t *testing.T) {
	d := readEDN(t, "altium-case.edn")
	refs := map[string]bool{}
	for _, c := range d.GetComponents() {
		refs[c.GetRefDes()] = true
	}
	for _, want := range []string{"R1", "R2", "C1"} {
		if !refs[want] {
			t.Errorf("component %s missing, read %v", want, refs)
		}
	}
	if len(refs) != 3 {
		t.Errorf("read %d components, want 3: %v", len(refs), refs)
	}
	pins := map[string][]string{}
	for _, n := range d.GetNets() {
		for _, c := range n.GetConnections() {
			pins[n.GetName()] = append(pins[n.GetName()], c.GetComponentRef()+"."+c.GetPinRef())
		}
	}
	for net, want := range map[string]string{"VIN": "C1.1 R1.1", "VOUT": "R1.2 R2.1", "GND": "C1.2 R2.2"} {
		got := pins[net]
		slices.Sort(got)
		if strings.Join(got, " ") != want {
			t.Errorf("net %s joins %v, want %s", net, got, want)
		}
	}
}

// TestKeywordNamedIdentifiersKeepTheirCase covers a net and an instance named like keywords, `Net`
// and `Instance`, in a file that also writes those keywords in Altium's case. The parser shares one
// node among every atom spelled the same (agni issue 945), so respelling a head in place would
// respell the names with it, and the net would read as `net` and the part as `instance`.
func TestKeywordNamedIdentifiersKeepTheirCase(t *testing.T) {
	src := string(readFixture(t, "altium-case.edn"))
	for old, repl := range map[string]string{
		"(Net VOUT":        "(Net Net",
		"(Instance R2\n":   "(Instance Instance\n",
		"(InstanceRef R2)": "(InstanceRef Instance)",
	} {
		if !strings.Contains(src, old) {
			t.Fatalf("altium-case.edn no longer holds %q", old)
		}
		src = strings.ReplaceAll(src, old, repl)
	}
	d, err := Read(strings.NewReader(src), "keyword-names.edn")
	if err != nil {
		t.Fatal(err)
	}
	refs := map[string]bool{}
	for _, c := range d.GetComponents() {
		refs[c.GetRefDes()] = true
	}
	if !refs["Instance"] || len(refs) != 3 {
		t.Errorf("components %v, want R1, C1 and Instance", refs)
	}
	pins := map[string][]string{}
	for _, n := range d.GetNets() {
		for _, c := range n.GetConnections() {
			pins[n.GetName()] = append(pins[n.GetName()], c.GetComponentRef()+"."+c.GetPinRef())
		}
	}
	got := pins["Net"]
	slices.Sort(got)
	if strings.Join(got, " ") != "Instance.1 R1.2" {
		t.Errorf("net Net joins %v, want Instance.1 R1.2; nets read %v", got, pins)
	}
}
