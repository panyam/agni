package gltf

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
)

// triangleBuffer is one triangle's three float positions and three uint16 indices, padded to four
// bytes.
func triangleBuffer(pts [3][3]float32) []byte {
	var b bytes.Buffer
	for _, p := range pts {
		for _, v := range p {
			_ = binary.Write(&b, binary.LittleEndian, v)
		}
	}
	for _, i := range []uint16{0, 1, 2, 0} {
		_ = binary.Write(&b, binary.LittleEndian, i)
	}
	return b.Bytes()
}

// doc is a one-triangle glTF whose buffer is uri. nodes and scenes are spliced in as JSON, so a
// test can place the mesh under a transform.
func doc(uri string, nodes, extra string) string {
	return `{"asset":{"version":"2.0"},` + extra + `"scene":0,"scenes":[{"nodes":[0]}],"nodes":` + nodes + `,
"meshes":[{"primitives":[{"attributes":{"POSITION":0},"indices":1,"material":0}]}],
"materials":[{"pbrMetallicRoughness":{"baseColorFactor":[0.1,0.2,0.3,1]}}],
"accessors":[{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"},
             {"bufferView":1,"componentType":5123,"count":3,"type":"SCALAR"}],
"bufferViews":[{"buffer":0,"byteOffset":0,"byteLength":36},{"buffer":0,"byteOffset":36,"byteLength":6}],
"buffers":[{"uri":"` + uri + `","byteLength":44}]}`
}

var tri = [3][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 2, 0}}

func dataURI(b []byte) string {
	return "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(b)
}

func near(a, b [3]float64) bool {
	for k := range a {
		if math.Abs(a[k]-b[k]) > 1e-6 {
			return false
		}
	}
	return true
}

func TestReadsATriangleWithItsColour(t *testing.T) {
	m, err := Read([]byte(doc(dataURI(triangleBuffer(tri)), `[{"mesh":0}]`, "")), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Primitives) != 1 {
		t.Fatalf("%d primitives, want 1", len(m.Primitives))
	}
	p := m.Primitives[0]
	if len(p.Positions) != 3 || !near(p.Positions[2], [3]float64{0, 2, 0}) {
		t.Errorf("positions %v", p.Positions)
	}
	if len(p.Indices) != 3 || p.Indices[1] != 1 {
		t.Errorf("indices %v", p.Indices)
	}
	if p.Color != [4]float64{0.1, 0.2, 0.3, 1} {
		t.Errorf("colour %v", p.Color)
	}
}

// A parent turns a quarter about Z and its child moves 5 along X, so the child's (1, 0, 0) lands at
// (0, 6, 0): the child's own move first, then the parent's turn.
func TestAppliesNodeTransformsParentFirst(t *testing.T) {
	q := math.Sqrt(0.5)
	nodes := `[{"rotation":[0,0,` + jsonNum(q) + `,` + jsonNum(q) + `],"children":[1]},{"translation":[5,0,0],"mesh":0}]`
	m, err := Read([]byte(doc(dataURI(triangleBuffer(tri)), nodes, "")), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Primitives[0].Positions[1]; !near(got, [3]float64{0, 6, 0}) {
		t.Errorf("(1,0,0) placed at %v, want (0,6,0)", got)
	}
	m, err = Read([]byte(doc(dataURI(triangleBuffer(tri)), `[{"scale":[2,3,4],"mesh":0}]`, "")), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Primitives[0].Positions[2]; !near(got, [3]float64{0, 6, 0}) {
		t.Errorf("(0,2,0) scaled to %v, want (0,6,0)", got)
	}
}

func jsonNum(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}

func TestReadsAnExternalBufferThroughTheOpener(t *testing.T) {
	asked := ""
	m, err := Read([]byte(doc("part.bin", `[{"mesh":0}]`, "")), func(uri string) ([]byte, error) {
		asked = uri
		return triangleBuffer(tri), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if asked != "part.bin" || len(m.Points()) != 3 {
		t.Errorf("asked for %q, read %d points", asked, len(m.Points()))
	}
	if _, err := Read([]byte(doc("part.bin", `[{"mesh":0}]`, "")), nil); err == nil {
		t.Error("an external buffer with no opener read with no error")
	}
}

func TestReadsAGLB(t *testing.T) {
	js := doc("", `[{"mesh":0}]`, "")
	js = strings.Replace(js, `"uri":"",`, "", 1)
	for len(js)%4 != 0 {
		js += " "
	}
	bin := triangleBuffer(tri)
	var b bytes.Buffer
	b.WriteString("glTF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(2))
	_ = binary.Write(&b, binary.LittleEndian, uint32(12+8+len(js)+8+len(bin)))
	_ = binary.Write(&b, binary.LittleEndian, uint32(len(js)))
	b.WriteString("JSON")
	b.WriteString(js)
	_ = binary.Write(&b, binary.LittleEndian, uint32(len(bin)))
	b.WriteString("BIN\x00")
	b.Write(bin)
	m, err := Read(b.Bytes(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Points()) != 3 {
		t.Errorf("read %d points, want 3", len(m.Points()))
	}
}

func TestRefusesWhatItCannotRead(t *testing.T) {
	buf := dataURI(triangleBuffer(tri))
	for name, src := range map[string]string{
		"Draco":       doc(buf, `[{"mesh":0}]`, `"extensionsRequired":["KHR_draco_mesh_compression"],`),
		"lines":       strings.Replace(doc(buf, `[{"mesh":0}]`, ""), `"material":0}`, `"material":0,"mode":1}`, 1),
		"glTF 1":      strings.Replace(doc(buf, `[{"mesh":0}]`, ""), `"version":"2.0"`, `"version":"1.0"`, 1),
		"bad index":   strings.Replace(doc(buf, `[{"mesh":0}]`, ""), `"count":3,"type":"VEC3"`, `"count":2,"type":"VEC3"`, 1),
		"short":       strings.Replace(doc(buf, `[{"mesh":0}]`, ""), `"byteLength":44`, `"byteLength":440`, 1),
		"not a model": `{"asset":`,
	} {
		if _, err := Read([]byte(src), nil); err == nil {
			t.Errorf("%s: read with no error", name)
		}
	}
	src := doc(buf, `[{"mesh":0}]`, `"extensionsRequired":["KHR_draco_mesh_compression"],`)
	if _, err := Read([]byte(src), nil); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Draco: err = %v, want ErrUnsupported", err)
	}
}

// Antmicro's library requires KHR_texture_basisu. A texture extension does not change geometry, so
// the file reads.
func TestIgnoresATextureExtension(t *testing.T) {
	src := doc(dataURI(triangleBuffer(tri)), `[{"mesh":0}]`, `"extensionsRequired":["KHR_texture_basisu"],`)
	if _, err := Read([]byte(src), nil); err != nil {
		t.Errorf("err = %v", err)
	}
}
