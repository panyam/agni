package formats

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// altiumJobs are the ODB++ jobs Altium Designer wrote for foxBMS's boards, fetched as the odb-altium
// samples artifact (CC-BY-4.0, agni-samples PROVENANCE.md). They are the reader's only jobs from a
// tool other than KiCad.
const altiumJobs = "../../tools/samples/odb/altium/foxbms"

// TestODBAltiumJobsHoldTogether reads each Altium job and holds its two tiers to each other, since
// there is no netlist of these boards to hold them to (agni issue 1268). Every pin the netlist
// connects has a sized land on the board on the same net, its name is the pin's own rather than
// Altium's designator-prefixed toeprint name, every through-hole land is drilled, and the design
// is named.
func TestODBAltiumJobsHoldTogether(t *testing.T) {
	jobs, _ := filepath.Glob(filepath.Join(altiumJobs, "*", "*.odb.zip"))
	if len(jobs) == 0 {
		// Fatal rather than skipped: an absent corpus must not read as a pass.
		t.Fatalf("no Altium jobs under %s; run `make samples-oracle`", altiumJobs)
	}
	for _, job := range jobs {
		t.Run(filepath.Base(filepath.Dir(job)), func(t *testing.T) {
			if _, err := os.Stat(job); err != nil {
				t.Fatal(err)
			}
			l := &Loader{}
			d, err := l.ReadDesign(job)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			g, err := l.BoardGeometry(job)
			if err != nil {
				t.Fatalf("board: %v", err)
			}
			if d.GetName() == "" || g.GetDesignRef() != d.GetName() {
				t.Errorf("design %q, board %q, want both named for the job", d.GetName(), g.GetDesignRef())
			}

			type land struct {
				net     string
				sized   bool
				through bool
				drilled bool
			}
			lands := map[string]land{}
			for _, pl := range g.GetPlacements() {
				for _, p := range pl.GetPads() {
					through := len(p.GetLayers()) > 0 && p.GetLayers()[0] == "*.Cu"
					lands[pl.GetRefDes()+"."+p.GetNumber()] = land{
						net:     p.GetNet(),
						sized:   p.GetSize().GetX() > 0 && p.GetSize().GetY() > 0,
						through: through,
						drilled: p.GetDrill() > 0,
					}
				}
			}

			pins, prefixed, missing, wrongNet, unsized := 0, 0, 0, 0, 0
			for _, n := range d.GetNets() {
				for _, c := range n.GetConnections() {
					pins++
					if strings.HasPrefix(c.GetPinRef(), c.GetComponentRef()+"-") {
						prefixed++
					}
					l, ok := lands[c.GetComponentRef()+"."+c.GetPinRef()]
					switch {
					case !ok:
						missing++
					case l.net != n.GetName():
						wrongNet++
					case !l.sized:
						unsized++
					}
				}
			}
			if pins == 0 {
				t.Fatal("the netlist connects no pins")
			}
			if prefixed+missing+wrongNet+unsized > 0 {
				t.Errorf("of %d connected pins: %d carry the designator in the pin name, %d have no land, %d sit on a land of another net, %d have a land of no size",
					pins, prefixed, missing, wrongNet, unsized)
			}
			// A land on no net is mechanical, and its hole may be one this job leaves out: hw-master's
			// Micro-Fit pegs are pins named empty-1 with no drill on the job's only drill layer.
			undrilled := 0
			for _, l := range lands {
				if l.through && !l.drilled && l.net != "" {
					undrilled++
				}
			}
			if undrilled > 0 {
				t.Errorf("%d through-hole lands have no drill", undrilled)
			}
		})
	}
}
