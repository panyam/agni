package profiles

import (
	"sort"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// sdGood is sdSlot with CMD and the four data lines each pulled up to +3V3, as an SD host wires them.
func sdGood() *ir.Design {
	d := sdSlot(true)
	loads := []string{"U1.20"}
	for i, n := range d.Nets {
		if n.Name == "SD2_CMD" || strings.HasPrefix(n.Name, "SD2_DATA") {
			r := "R" + itoa(i+1)
			d.Components = append(d.Components, comps(r)...)
			n.Connections = append(n.Connections, &ir.Connection{ComponentRef: r, PinRef: "1"})
			loads = append(loads, r+".2")
		}
	}
	d.Nets = append(d.Nets, net("+3V3", loads...))
	return d
}

func sdFindings(d *ir.Design, p Profile) []string {
	var got []string
	for _, f := range check.RunBackground(check.NewModel(d), Compile(p)) {
		line := f.Rule + " " + check.EntityRef(f.Subject)
		if i := strings.LastIndex(f.Message, "missing required signal "); i >= 0 {
			line += " " + strings.TrimPrefix(f.Message[i:], "missing required signal ")
		}
		got = append(got, line)
	}
	sort.Strings(got)
	return got
}

func TestAnSDSlotWiredCompletelyIsSilent(t *testing.T) {
	if got := sdFindings(sdGood(), SD); len(got) != 0 {
		t.Fatalf("a complete, pulled-up SD slot: want no finding, got %q", got)
	}
}

// The SD profile's pull-up and completeness checks fire on the bus the eMMC profile now leaves alone
// (agni issue 1294). sdSlot has no pull-ups, as the MNT Reform's SD2 slot has none outside its level
// shifter.
func TestAnSDSlotWithoutPullUpsOrCardDetectFires(t *testing.T) {
	want := []string{
		"sd-missing-pullup SD2_CMD",
		"sd-missing-pullup SD2_DATA0",
		"sd-missing-pullup SD2_DATA1",
		"sd-missing-pullup SD2_DATA2",
		"sd-missing-pullup SD2_DATA3",
		"sd-signal-missing SD2_CMD CD",
	}
	if got := sdFindings(sdSlot(false), SD); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("SD slot, no pull-ups, no card detect:\n got %q\nwant %q", got, want)
	}
}

// An 8-bit eMMC carries SD's CMD, CLK and DAT names, and its DAT4 and RST say it is not an SD card,
// so the SD profile leaves it to the eMMC profile.
func TestTheSDProfileLeavesAnEMMCAlone(t *testing.T) {
	m := check.NewModel(emmcGood())
	if got := sdFindings(emmcGood(), SD); len(got) != 0 || InUse(m, SD) {
		t.Errorf("an eMMC bus: want the SD profile not in use and silent, got InUse %v, %q", InUse(m, SD), got)
	}
}

// On a board carrying both, each profile judges its own bus and only that one.
func TestSDAndEMMCSplitABoardBetweenThem(t *testing.T) {
	d := emmcGood()
	sd := sdGood()
	for _, n := range sd.Nets {
		for _, c := range n.Connections {
			if c.ComponentRef == "U1" {
				c.ComponentRef = "U9"
			}
		}
	}
	d.Components = append(d.Components, comps("U9", "J1")...)
	for _, c := range sd.Components {
		if strings.HasPrefix(c.RefDes, "R") {
			c.RefDes = "R9" + c.RefDes[1:]
			d.Components = append(d.Components, c)
		}
	}
	for _, n := range sd.Nets {
		for _, c := range n.Connections {
			if strings.HasPrefix(c.ComponentRef, "R") {
				c.ComponentRef = "R9" + c.ComponentRef[1:]
			}
		}
		if n.Name == "+3V3" {
			for _, existing := range d.Nets {
				if existing.Name == "+3V3" {
					existing.Connections = append(existing.Connections, n.Connections...)
				}
			}
			continue
		}
		d.Nets = append(d.Nets, n)
	}
	m := check.NewModel(d)
	if got := sdFindings(d, SD); len(got) != 0 {
		t.Errorf("SD profile on a board with a good SD slot and a good eMMC: want silent, got %q", got)
	}
	if got := sdFindings(d, EMMC); len(got) != 0 {
		t.Errorf("eMMC profile on the same board: want silent, got %q", got)
	}
	sdNets, emmcNets := Nets(m, SD), Nets(m, EMMC)
	if !sdNets["SD2_CMD"] || sdNets["MMC_CMD"] || !emmcNets["MMC_CMD"] || emmcNets["SD2_CMD"] {
		t.Errorf("review scope: SD %v, eMMC %v, want each its own bus", sdNets, emmcNets)
	}
}
