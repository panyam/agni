package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/mesh"
	"github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

// The triangle budgets GetBoardMeshes uses (agni issue 1652). DefaultMeshTriangles is the board's
// budget when a request names none, and MaxMeshTriangles the most a request may ask for.
// MeshTrianglesEach caps every mesh, whatever the budget, since a part drawn a few millimetres wide
// shows no more detail than that.
const (
	DefaultMeshTriangles = 300_000
	MaxMeshTriangles     = 2_000_000
	MeshTrianglesEach    = 20_000
)

// BoardMeshLoader reads model meshes by the keys in a board's model_instances (agni issue 1652). It
// is a port of its own, beside Loader, so a loader that reads no meshes needs no change. keys are
// ModelInstance.mesh values. A key the loader cannot read goes in unread with its reason. A host
// resolves each key as its board read resolved the model, so the service opens no file (C1, C13).
type BoardMeshLoader interface {
	BoardMeshes(ctx context.Context, uri artifact.URI, keys []string) (meshes map[string]*mesh.Mesh, unread map[string]string, err error)
}

// GetBoardMeshes answers a page of the snapshot's board meshes, fitted to the request's
// triangle budget by mesh.Fit. Every page of one request comes from one fitted set, which the
// service keeps for the few budgets asked for most recently, so a page does not read the models
// again.
func (s *DesignService) GetBoardMeshes(ctx context.Context, req *webapi.GetBoardMeshesRequest) (*webapi.GetBoardMeshesResponse, error) {
	tris := req.GetMaxTriangles()
	switch {
	case tris < 0:
		return nil, fmt.Errorf("%w: max_triangles is %d, and must be 0 or more", ErrInvalidArgument, tris)
	case tris == 0:
		tris = DefaultMeshTriangles
	case tris > MaxMeshTriangles:
		tris = MaxMeshTriangles
	}
	bytes := req.GetMaxBytes()
	switch {
	case bytes < 0:
		return nil, fmt.Errorf("%w: max_bytes is %d, and must be 0 or more", ErrInvalidArgument, bytes)
	case bytes == 0:
		bytes = DefaultBoardPageBytes
	case bytes > MaxBoardPageBytes:
		bytes = MaxBoardPageBytes
	}
	ctx, snap, err := s.projects.Snapshot(ctx, req.GetSnapshot())
	if err != nil {
		return nil, err
	}
	at := 0
	if t := req.GetPageToken(); t != "" {
		if at, err = parseMeshToken(t, snap.Revision, tris); err != nil {
			return nil, err
		}
	}
	fit, err := s.fittedMeshes(ctx, snap, int(tris))
	if err != nil {
		return nil, err
	}
	resp := &webapi.GetBoardMeshesResponse{}
	if at == 0 {
		resp.Simplified, resp.Blocks, resp.Unread, resp.Triangles = fit.simplified, fit.blocks, fit.unread, fit.triangles
	}
	size := proto.Size(resp)
	for i := at; i < len(fit.meshes); i++ {
		cost := protowire.SizeTag(1) + protowire.SizeBytes(proto.Size(fit.meshes[i]))
		if size+cost > int(bytes) && i > at {
			resp.NextPageToken = meshToken(snap.Revision, tris, i)
			break
		}
		resp.Meshes = append(resp.Meshes, fit.meshes[i])
		size += cost
	}
	return resp, nil
}

// fitted is one board's meshes fitted to one budget, in key order, with what the fit did.
type fitted struct {
	meshes     []*geom.ModelMesh
	simplified []string
	blocks     []string
	unread     []*geom.UnreadModel
	triangles  int64
}

// meshCache keeps the fitted sets for the last few (snapshot, revision, budget) asked for.
type meshCache struct {
	mu    sync.Mutex
	order []string
	sets  map[string]*fitted
}

const meshCacheSize = 4

func (c *meshCache) get(key string) *fitted {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sets[key]
}

func (c *meshCache) put(key string, f *fitted) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sets == nil {
		c.sets = map[string]*fitted{}
	}
	if _, ok := c.sets[key]; !ok {
		c.order = append(c.order, key)
	}
	c.sets[key] = f
	for len(c.order) > meshCacheSize {
		delete(c.sets, c.order[0])
		c.order = c.order[1:]
	}
}

func (s *DesignService) fittedMeshes(ctx context.Context, snap Snap, budget int) (*fitted, error) {
	key := fmt.Sprintf("%s|%s|%d", snap.Name, snap.Revision, budget)
	if f := s.meshes.get(key); f != nil {
		return f, nil
	}
	b, err := s.boardFor(ctx, snap)
	if err != nil {
		return nil, err
	}
	// Each key's footprint is the largest area any of its parts covers, so a mesh one large part
	// shares with small ones stays a mesh as long as the large part would.
	parts := map[string]int32{}
	foot := map[string]float64{}
	upm := 1e6 / float64(max(b.GetUnitNm(), 1))
	for _, p := range b.GetPlacements() {
		a := placementAreaMm2(p, upm)
		for _, in := range p.GetModelInstances() {
			parts[in.GetMesh()]++
			foot[in.GetMesh()] = math.Max(foot[in.GetMesh()], a)
		}
	}
	keys := make([]string, 0, len(parts))
	for k := range parts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	f := &fitted{}
	var read map[string]*mesh.Mesh
	unread := map[string]string{}
	if ml, ok := LoaderOr[Loader](snap, s.loader).(BoardMeshLoader); ok && len(keys) > 0 {
		if read, unread, err = ml.BoardMeshes(ctx, snap.BoardRead(), keys); err != nil {
			return nil, ClassifyLoadErr(err)
		}
	} else {
		for _, k := range keys {
			unread[k] = "this server reads no model meshes"
		}
	}
	var items []mesh.Item
	for _, k := range keys {
		if m := read[k]; m != nil {
			items = append(items, mesh.Item{Key: k, Mesh: m, Footprint: foot[k]})
		}
	}
	plan := mesh.Fit(items, MeshTrianglesEach, budget)
	for _, it := range items {
		m := plan.Meshes[it.Key]
		if m == nil {
			continue
		}
		f.meshes = append(f.meshes, modelMesh(it.Key, m, it.Mesh.Triangles()))
	}
	f.simplified, f.blocks, f.triangles = plan.Simplified, plan.Blocks, int64(plan.Triangles)
	for _, k := range keys {
		if why, ok := unread[k]; ok {
			f.unread = append(f.unread, &geom.UnreadModel{Path: k, Reason: why, Parts: parts[k]})
		}
	}
	s.meshes.put(key, f)
	return f, nil
}

// placementAreaMm2 is the area of the box round a placement's pads, in square millimetres.
func placementAreaMm2(p *geom.ComponentPlacement, upm float64) float64 {
	var lo, hi [2]float64
	for i, pad := range p.GetPads() {
		hx, hy := float64(pad.GetSize().GetX())/2, float64(pad.GetSize().GetY())/2
		x, y := float64(pad.GetAt().GetX()), float64(pad.GetAt().GetY())
		if i == 0 {
			lo, hi = [2]float64{x - hx, y - hy}, [2]float64{x + hx, y + hy}
			continue
		}
		lo = [2]float64{math.Min(lo[0], x-hx), math.Min(lo[1], y-hy)}
		hi = [2]float64{math.Max(hi[0], x+hx), math.Max(hi[1], y+hy)}
	}
	return (hi[0] - lo[0]) * (hi[1] - lo[1]) / (upm * upm)
}

// modelMesh packs a mesh for the wire, with float positions and each colour as one RGBA word.
func modelMesh(key string, m *mesh.Mesh, source int) *geom.ModelMesh {
	out := &geom.ModelMesh{Key: key, SourceTriangles: int32(min(source, math.MaxInt32))} // #nosec G115 -- clamped
	for _, p := range m.Parts {
		mp := &geom.MeshPart{Indices: p.Indices, Positions: make([]float32, 0, 3*len(p.Positions))}
		for _, q := range p.Positions {
			mp.Positions = append(mp.Positions, float32(q[0]), float32(q[1]), float32(q[2]))
		}
		var rgba uint32
		for _, c := range p.Color {
			rgba = rgba<<8 | uint32(math.Round(math.Max(0, math.Min(1, c))*255))
		}
		mp.Rgba = rgba
		out.Parts = append(out.Parts, mp)
	}
	return out
}

// meshToken encodes where the next page starts, with the revision and budget it belongs to.
func meshToken(revision string, budget int64, index int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("m1|%s|%d|%d", revision, budget, index)))
}

func parseMeshToken(t, revision string, budget int64) (int, error) {
	bad := func(why string) (int, error) {
		return 0, fmt.Errorf("%w: page_token %q %s; start again with an empty page_token", ErrInvalidArgument, t, why)
	}
	raw, err := base64.RawURLEncoding.DecodeString(t)
	if err != nil {
		return bad("is not a token this server wrote")
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 4 || parts[0] != "m1" {
		return bad("is not a token this server wrote")
	}
	if parts[1] != revision {
		return bad("belongs to another revision of the design")
	}
	if parts[2] != strconv.FormatInt(budget, 10) {
		return bad("belongs to another max_triangles")
	}
	index, err := strconv.Atoi(parts[3])
	if err != nil || index < 1 {
		return bad("is not a token this server wrote")
	}
	return index, nil
}
