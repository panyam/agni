// Package gltf reads a glTF 2.0 model's triangles and colours (agni issues 1687 and 1652).
//
// Say Jetson's board names a part's model as `${ANT3DMDL}/R_0402_1005Metric.step`. Antmicro publishes
// that library as glTF rather than STEP, so the KiCad reader reads the glTF beside the named file. A
// glTF file is JSON naming binary buffers, and a .glb packs the JSON and one buffer into one file.
// Read returns each triangle primitive with its positions in the model's root frame, after every
// node's transform, and its material's base colour.
//
// Fidelity: lossy-bounded. Read keeps triangle positions, indices and each material's base colour
// factor. It drops normals, texture coordinates, textures, animation, skins, cameras and every
// material property but the base colour. It refuses a file that requires an extension changing
// geometry, such as Draco or meshopt compression, and a primitive that is not a triangle list. A
// texture or material extension it does not know is ignored, since geometry does not depend on it.
// Positions are in the file's own units and axes. glTF says metres with Y up, and the caller decides
// what a given library means.
package gltf

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

// Model is the triangles of every mesh a glTF scene places.
type Model struct {
	Primitives []Primitive
}

// Primitive is one triangle list. Positions are in the model's root frame. Indices name three
// positions per triangle. Color is the material's base colour as RGBA in 0 to 1, or opaque grey when
// the primitive has no material.
type Primitive struct {
	Positions [][3]float64
	Indices   []uint32
	Color     [4]float64
}

// Points returns every position of every primitive.
func (m *Model) Points() [][3]float64 {
	var out [][3]float64
	for _, p := range m.Primitives {
		out = append(out, p.Positions...)
	}
	return out
}

// ErrUnsupported reports a file Read refuses: a required extension that changes geometry, or a
// primitive that is not a triangle list.
var ErrUnsupported = errors.New("gltf: unsupported")

// Opener returns the bytes of a file a .gltf names by a relative URI, such as its .bin buffer. Read
// opens no file itself.
type Opener func(uri string) ([]byte, error)

// Read parses a .gltf (JSON) or .glb (binary) model. open supplies each external buffer, and may be
// nil for a .glb or for a file whose buffers are data URIs.
func Read(data []byte, open Opener) (*Model, error) {
	var doc document
	var glbBin []byte
	if bytes.HasPrefix(data, []byte("glTF")) {
		j, bin, err := splitGLB(data)
		if err != nil {
			return nil, err
		}
		data, glbBin = j, bin
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("gltf: %w", err)
	}
	if !strings.HasPrefix(doc.Asset.Version, "2.") {
		return nil, fmt.Errorf("%w: version %q, want 2.x", ErrUnsupported, doc.Asset.Version)
	}
	for _, e := range doc.ExtensionsRequired {
		if geometryExtensions[e] {
			return nil, fmt.Errorf("%w: requires %s", ErrUnsupported, e)
		}
	}
	buffers := make([][]byte, len(doc.Buffers))
	for i, b := range doc.Buffers {
		switch {
		case b.URI == "" && glbBin != nil && i == 0:
			buffers[i] = glbBin
		case strings.HasPrefix(b.URI, "data:"):
			comma := strings.IndexByte(b.URI, ',')
			if comma < 0 || !strings.Contains(b.URI[:comma], ";base64") {
				return nil, fmt.Errorf("%w: buffer %d's data URI is not base64", ErrUnsupported, i)
			}
			d, err := base64.StdEncoding.DecodeString(b.URI[comma+1:])
			if err != nil {
				return nil, fmt.Errorf("gltf: buffer %d: %w", i, err)
			}
			buffers[i] = d
		case b.URI != "":
			if open == nil {
				return nil, fmt.Errorf("gltf: buffer %d names %q and no opener was given", i, b.URI)
			}
			d, err := open(b.URI)
			if err != nil {
				return nil, fmt.Errorf("gltf: buffer %q: %w", b.URI, err)
			}
			buffers[i] = d
		default:
			return nil, fmt.Errorf("gltf: buffer %d has no data", i)
		}
		if len(buffers[i]) < b.ByteLength {
			return nil, fmt.Errorf("gltf: buffer %d holds %d bytes, want %d", i, len(buffers[i]), b.ByteLength)
		}
	}
	r := reader{doc: &doc, buffers: buffers}
	scene := 0
	if doc.Scene != nil {
		scene = *doc.Scene
	}
	var roots []int
	if scene < len(doc.Scenes) {
		roots = doc.Scenes[scene].Nodes
	} else {
		roots = r.rootNodes()
	}
	m := &Model{}
	for _, n := range roots {
		if err := r.walk(n, identity, m, 0); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// geometryExtensions are required extensions that change how positions or indices are stored.
var geometryExtensions = map[string]bool{
	"KHR_draco_mesh_compression": true,
	"EXT_meshopt_compression":    true,
	"KHR_meshopt_compression":    true,
	"KHR_mesh_quantization":      true,
}

type document struct {
	Asset struct {
		Version string `json:"version"`
	} `json:"asset"`
	ExtensionsRequired []string `json:"extensionsRequired"`
	Scene              *int     `json:"scene"`
	Scenes             []struct {
		Nodes []int `json:"nodes"`
	} `json:"scenes"`
	Nodes []struct {
		Children    []int     `json:"children"`
		Mesh        *int      `json:"mesh"`
		Matrix      []float64 `json:"matrix"`
		Translation []float64 `json:"translation"`
		Rotation    []float64 `json:"rotation"`
		Scale       []float64 `json:"scale"`
	} `json:"nodes"`
	Meshes []struct {
		Primitives []struct {
			Attributes map[string]int `json:"attributes"`
			Indices    *int           `json:"indices"`
			Material   *int           `json:"material"`
			Mode       *int           `json:"mode"`
		} `json:"primitives"`
	} `json:"meshes"`
	Accessors []struct {
		BufferView    *int   `json:"bufferView"`
		ByteOffset    int    `json:"byteOffset"`
		ComponentType int    `json:"componentType"`
		Normalized    bool   `json:"normalized"`
		Count         int    `json:"count"`
		Type          string `json:"type"`
		Sparse        any    `json:"sparse"`
	} `json:"accessors"`
	BufferViews []struct {
		Buffer     int `json:"buffer"`
		ByteOffset int `json:"byteOffset"`
		ByteLength int `json:"byteLength"`
		ByteStride int `json:"byteStride"`
	} `json:"bufferViews"`
	Buffers []struct {
		URI        string `json:"uri"`
		ByteLength int    `json:"byteLength"`
	} `json:"buffers"`
	Materials []struct {
		PBR *struct {
			BaseColorFactor []float64 `json:"baseColorFactor"`
		} `json:"pbrMetallicRoughness"`
	} `json:"materials"`
}

// splitGLB returns a .glb's JSON chunk and its binary chunk, if any.
func splitGLB(data []byte) (jsonChunk, bin []byte, err error) {
	if len(data) < 20 {
		return nil, nil, errors.New("gltf: .glb too short")
	}
	if v := binary.LittleEndian.Uint32(data[4:8]); v != 2 {
		return nil, nil, fmt.Errorf("%w: .glb version %d", ErrUnsupported, v)
	}
	total := int(binary.LittleEndian.Uint32(data[8:12]))
	if total > len(data) {
		return nil, nil, errors.New("gltf: .glb is truncated")
	}
	for off := 12; off+8 <= total; {
		n := int(binary.LittleEndian.Uint32(data[off : off+4]))
		kind := binary.LittleEndian.Uint32(data[off+4 : off+8])
		if off+8+n > total {
			return nil, nil, errors.New("gltf: .glb chunk runs past the file")
		}
		chunk := data[off+8 : off+8+n]
		switch kind {
		case 0x4E4F534A: // "JSON"
			jsonChunk = chunk
		case 0x004E4942: // "BIN\0"
			if bin == nil {
				bin = chunk
			}
		}
		off += 8 + n
	}
	if jsonChunk == nil {
		return nil, nil, errors.New("gltf: .glb has no JSON chunk")
	}
	return jsonChunk, bin, nil
}

type mat4 [16]float64 // column-major, as glTF writes a matrix

var identity = mat4{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}

func (a mat4) mul(b mat4) mat4 {
	var c mat4
	for col := 0; col < 4; col++ {
		for row := 0; row < 4; row++ {
			s := 0.0
			for k := 0; k < 4; k++ {
				s += a[k*4+row] * b[col*4+k]
			}
			c[col*4+row] = s
		}
	}
	return c
}

func (a mat4) apply(p [3]float64) [3]float64 {
	return [3]float64{
		a[0]*p[0] + a[4]*p[1] + a[8]*p[2] + a[12],
		a[1]*p[0] + a[5]*p[1] + a[9]*p[2] + a[13],
		a[2]*p[0] + a[6]*p[1] + a[10]*p[2] + a[14],
	}
}

// trs builds a node's matrix from its translation, rotation (a unit quaternion x, y, z, w) and scale.
func trs(t, r, s []float64) mat4 {
	tx, ty, tz := 0.0, 0.0, 0.0
	if len(t) == 3 {
		tx, ty, tz = t[0], t[1], t[2]
	}
	qx, qy, qz, qw := 0.0, 0.0, 0.0, 1.0
	if len(r) == 4 {
		qx, qy, qz, qw = r[0], r[1], r[2], r[3]
	}
	sx, sy, sz := 1.0, 1.0, 1.0
	if len(s) == 3 {
		sx, sy, sz = s[0], s[1], s[2]
	}
	return mat4{
		(1 - 2*(qy*qy+qz*qz)) * sx, 2 * (qx*qy + qz*qw) * sx, 2 * (qx*qz - qy*qw) * sx, 0,
		2 * (qx*qy - qz*qw) * sy, (1 - 2*(qx*qx+qz*qz)) * sy, 2 * (qy*qz + qx*qw) * sy, 0,
		2 * (qx*qz + qy*qw) * sz, 2 * (qy*qz - qx*qw) * sz, (1 - 2*(qx*qx+qy*qy)) * sz, 0,
		tx, ty, tz, 1,
	}
}

type reader struct {
	doc     *document
	buffers [][]byte
}

// rootNodes returns the nodes no other node names as a child, for a file with no scene.
func (r reader) rootNodes() []int {
	child := map[int]bool{}
	for _, n := range r.doc.Nodes {
		for _, c := range n.Children {
			child[c] = true
		}
	}
	var out []int
	for i := range r.doc.Nodes {
		if !child[i] {
			out = append(out, i)
		}
	}
	return out
}

func (r reader) walk(i int, parent mat4, m *Model, depth int) error {
	if i < 0 || i >= len(r.doc.Nodes) {
		return fmt.Errorf("gltf: node %d does not exist", i)
	}
	if depth > 64 {
		return errors.New("gltf: nodes nest more than 64 deep")
	}
	n := r.doc.Nodes[i]
	local := identity
	if len(n.Matrix) == 16 {
		copy(local[:], n.Matrix)
	} else {
		local = trs(n.Translation, n.Rotation, n.Scale)
	}
	world := parent.mul(local)
	if n.Mesh != nil {
		if *n.Mesh < 0 || *n.Mesh >= len(r.doc.Meshes) {
			return fmt.Errorf("gltf: node %d names mesh %d, which does not exist", i, *n.Mesh)
		}
		for _, p := range r.doc.Meshes[*n.Mesh].Primitives {
			if p.Mode != nil && *p.Mode != 4 {
				return fmt.Errorf("%w: primitive mode %d, want 4 (triangles)", ErrUnsupported, *p.Mode)
			}
			pos, ok := p.Attributes["POSITION"]
			if !ok {
				continue
			}
			v, err := r.vec3(pos)
			if err != nil {
				return err
			}
			prim := Primitive{Color: [4]float64{0.6, 0.6, 0.6, 1}}
			for _, q := range v {
				prim.Positions = append(prim.Positions, world.apply(q))
			}
			if p.Indices != nil {
				if prim.Indices, err = r.indices(*p.Indices, len(v)); err != nil {
					return err
				}
			} else {
				for k := 0; k+2 < len(v); k += 3 {
					prim.Indices = append(prim.Indices, uint32(k), uint32(k+1), uint32(k+2))
				}
			}
			if p.Material != nil && *p.Material >= 0 && *p.Material < len(r.doc.Materials) {
				if pbr := r.doc.Materials[*p.Material].PBR; pbr != nil && len(pbr.BaseColorFactor) == 4 {
					copy(prim.Color[:], pbr.BaseColorFactor)
				}
			}
			m.Primitives = append(m.Primitives, prim)
		}
	}
	for _, c := range n.Children {
		if err := r.walk(c, world, m, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// view returns an accessor's bytes, its stride and its element count.
func (r reader) view(a int, elemSize int) ([]byte, int, int, error) {
	if a < 0 || a >= len(r.doc.Accessors) {
		return nil, 0, 0, fmt.Errorf("gltf: accessor %d does not exist", a)
	}
	acc := r.doc.Accessors[a]
	if acc.Sparse != nil {
		return nil, 0, 0, fmt.Errorf("%w: sparse accessor %d", ErrUnsupported, a)
	}
	if acc.BufferView == nil {
		return nil, 0, 0, fmt.Errorf("%w: accessor %d has no buffer view", ErrUnsupported, a)
	}
	bv := *acc.BufferView
	if bv < 0 || bv >= len(r.doc.BufferViews) {
		return nil, 0, 0, fmt.Errorf("gltf: buffer view %d does not exist", bv)
	}
	v := r.doc.BufferViews[bv]
	if v.Buffer < 0 || v.Buffer >= len(r.buffers) {
		return nil, 0, 0, fmt.Errorf("gltf: buffer %d does not exist", v.Buffer)
	}
	stride := v.ByteStride
	if stride == 0 {
		stride = elemSize
	}
	start := v.ByteOffset + acc.ByteOffset
	end := start + (acc.Count-1)*stride + elemSize
	if acc.Count == 0 {
		return nil, stride, 0, nil
	}
	if start < 0 || end > v.ByteOffset+v.ByteLength || end > len(r.buffers[v.Buffer]) {
		return nil, 0, 0, fmt.Errorf("gltf: accessor %d runs past its buffer", a)
	}
	return r.buffers[v.Buffer][start:end], stride, acc.Count, nil
}

func (r reader) vec3(a int) ([][3]float64, error) {
	if a < 0 || a >= len(r.doc.Accessors) {
		return nil, fmt.Errorf("gltf: accessor %d does not exist", a)
	}
	acc := r.doc.Accessors[a]
	if acc.Type != "VEC3" || acc.ComponentType != 5126 {
		return nil, fmt.Errorf("%w: POSITION accessor %d is %s of type %d, want VEC3 of float", ErrUnsupported, a, acc.Type, acc.ComponentType)
	}
	b, stride, n, err := r.view(a, 12)
	if err != nil {
		return nil, err
	}
	out := make([][3]float64, n)
	for i := range out {
		o := i * stride
		for k := 0; k < 3; k++ {
			f := math.Float32frombits(binary.LittleEndian.Uint32(b[o+4*k:]))
			out[i][k] = float64(f)
		}
	}
	return out, nil
}

func (r reader) indices(a, nVerts int) ([]uint32, error) {
	acc := r.doc.Accessors[a]
	if acc.Type != "SCALAR" {
		return nil, fmt.Errorf("%w: index accessor %d is %s, want SCALAR", ErrUnsupported, a, acc.Type)
	}
	size := map[int]int{5121: 1, 5123: 2, 5125: 4}[acc.ComponentType]
	if size == 0 {
		return nil, fmt.Errorf("%w: index accessor %d has component type %d", ErrUnsupported, a, acc.ComponentType)
	}
	b, stride, n, err := r.view(a, size)
	if err != nil {
		return nil, err
	}
	out := make([]uint32, n)
	for i := range out {
		o := i * stride
		switch size {
		case 1:
			out[i] = uint32(b[o])
		case 2:
			out[i] = uint32(binary.LittleEndian.Uint16(b[o:]))
		case 4:
			out[i] = binary.LittleEndian.Uint32(b[o:])
		}
		if int(out[i]) >= nVerts {
			return nil, fmt.Errorf("gltf: index %d names vertex %d of %d", i, out[i], nVerts)
		}
	}
	return out, nil
}
