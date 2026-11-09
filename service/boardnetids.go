package service

import (
	"context"

	"github.com/panyam/agni/core/render"
)

// boardNetIDs is the render option that keys a board's nets with the design's net ids, so a
// highlight or a pick naming a net by id joins the board as it joins a schematic (agni issue 1440).
// A board names its nets and holds no ids, so the join is by name, through netNames.
func (s *DesignService) boardNetIDs(ctx context.Context, snap Snap) render.Option {
	names := s.readNetNames(ctx, snap)
	if len(names.idByName) == 0 {
		return render.WithNetIDs(nil)
	}
	return render.WithNetIDs(func(n render.BoardNet) string { return names.id(n.Name) })
}
