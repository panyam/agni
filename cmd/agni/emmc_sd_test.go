package main

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/stdlib/profiles"
)

// The shipped eMMC profile, read from an EDIF netlist through the CLI's reader, leaves a 4-bit SD card
// bus with a card detect line alone and judges the same bus without one (agni issue 1246). The bus is
// named as the MNT Reform motherboard names its SD2 slot, which the profile used to report nine
// missing signals, a missing pull-up and a dangling clock on.
func TestTheEMMCProfileLeavesAnSDCardSlotAlone(t *testing.T) {
	var emmc []*check.Rule
	for _, r := range check.DefaultCatalog().Rules() {
		if r.Tags[profiles.TagProfile] == profiles.EMMC.Name {
			emmc = append(emmc, r)
		}
	}
	if len(emmc) != 4 {
		t.Fatalf("want the four shipped eMMC rules in the catalog, got %d", len(emmc))
	}
	for _, tc := range []struct {
		file string
		want []string
	}{
		{"with-card-detect.edn", nil},
		{"no-card-detect.edn", []string{
			"profile/emmc-missing-pullup SD2_CMD",
			"profile/emmc-signal-missing SD2_CMD DAT4",
			"profile/emmc-signal-missing SD2_CMD DAT5",
			"profile/emmc-signal-missing SD2_CMD DAT6",
			"profile/emmc-signal-missing SD2_CMD DAT7",
			"profile/emmc-signal-missing SD2_CMD RST",
		}},
	} {
		m, err := readModel(filepath.Join("testdata", "emmc-sd", tc.file))
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, f := range check.RunBackground(m, emmc) {
			line := f.Rule + " " + check.EntityRef(f.Subject)
			if i := strings.LastIndex(f.Message, "missing required signal "); i >= 0 {
				line += " " + strings.TrimPrefix(f.Message[i:], "missing required signal ")
			}
			got = append(got, line)
		}
		sort.Strings(got)
		if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
			t.Errorf("%s: eMMC findings\n got %q\nwant %q", tc.file, got, tc.want)
		}
	}
}
