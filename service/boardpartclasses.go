package service

import (
	"context"

	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/core/render"
)

// boardPartClasses is the render option that gives each placed part its design classes, most
// specific first, so the stack view colours a part by what it is (agni 1653). A board holds no
// classes, so the join is by ref-des. A part the design does not hold, or a design that does not
// read, gets no class, and its body keeps the plain colour.
func (s *DesignService) boardPartClasses(ctx context.Context, snap Snap) render.Option {
	d, err := LoaderOr(snap, s.loader).Design(ctx, snap.Netlist, snap.Overlay.ReadOptions()...)
	if err != nil {
		return render.WithPartClasses(nil)
	}
	byRef := map[string][]string{}
	for _, c := range d.GetComponents() {
		if names := classify.ClassNames(c); len(names) > 0 {
			byRef[c.GetRefDes()] = classify.BySpecificity(names)
		}
	}
	return render.WithPartClasses(func(ref string) []string { return byRef[ref] })
}
