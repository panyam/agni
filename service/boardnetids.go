package service

import (
	"context"

	"github.com/panyam/agni/core/render"
)

// boardNetIDs is the render option that keys a board's nets with the design's net ids, so a
// highlight or a pick naming a net by id joins the board as it joins a schematic (agni issue 1440).
// A board names its nets and holds no ids, so the join is by name. A board net takes the id of the
// design's one net with its name. A name two nets share answers "", because the board draws one
// net under that name and cannot say which schematic net it is. A design that fails to read keys
// the board by name alone, since the board still draws and only the id join is lost.
//
// It reads the netlist as nameJoinSpecs does, through the snapshot's loader with the snapshot's
// options, so the ids match the ones the design's findings carry.
func (s *DesignService) boardNetIDs(ctx context.Context, snap Snap) render.Option {
	d, err := LoaderOr(snap, s.loader).Design(ctx, snap.Netlist, snap.Overlay.ReadOptions()...)
	if err != nil {
		return render.WithNetIDs(nil)
	}
	byName := map[string]string{}
	shared := map[string]bool{}
	for _, n := range d.GetNets() {
		name, id := n.GetName(), n.GetId()
		if name == "" || id == "" {
			continue
		}
		if prev, ok := byName[name]; ok && prev != id {
			shared[name] = true
		}
		byName[name] = id
	}
	return render.WithNetIDs(func(n render.BoardNet) string {
		if shared[n.Name] {
			return ""
		}
		return byName[n.Name]
	})
}
