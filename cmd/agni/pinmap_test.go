package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// agni issue 1238. The draft for U2 finds MDIO's far end through R1 by itself, and leaves the reset
// with none, since nothing else is on its net. Checked as an intent file, the draft lays out the
// same pin map a hand-written one does, and every row passes, because it was read off this netlist.
func TestPinmapDraftsTheFarEndsAndChecksAsTheMap(t *testing.T) {
	freshWorkspace(t)
	draft := runCLI(t, rootCmd(), "pinmap", phySignalFixture, "--device", "U2")
	for _, want := range []string{
		`- {device: U2, pin: "1", net: MDIO_NET, to: {device: U3, pin: "1"}}  # MDIO`,
		`- {device: U2, pin: "2", net: RST_N}  # RST; reaches no other device pin`,
		"agrees with that netlist",
	} {
		if !strings.Contains(draft, want) {
			t.Errorf("the draft lacks %q:\n%s", want, draft)
		}
	}

	path := filepath.Join(t.TempDir(), "u2.yaml")
	if err := os.WriteFile(path, []byte(draft), 0o600); err != nil {
		t.Fatal(err)
	}
	got := pinMapRowsWith(t, phySignalFixture, []string{"--intent-path", path}, ioMapRules[:3]...)
	want := [][]string{
		{"U2", "1", "", "MDIO_NET", "MDIO_NET", "U3.1", "MDIO_NET -> [R1] -> MDIO_OUT", "pass", "pass", "pass", "", "pass"},
		{"U2", "2", "", "RST_N", "RST_N", "", "", "pass", "pass", "not-considered", "", "pass"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("pin_map from the draft =\n%v\nwant\n%v", got, want)
	}
}

// --pin narrows the draft and takes the functional name, through the lookup trace uses.
func TestPinmapTakesOnePinByName(t *testing.T) {
	freshWorkspace(t)
	draft := runCLI(t, rootCmd(), "pinmap", phySignalFixture, "--device", "U2", "--pin", "MDIO")
	if !strings.Contains(draft, `pin: "1"`) || strings.Contains(draft, `pin: "2"`) {
		t.Errorf("--pin MDIO should draft pin 1 alone:\n%s", draft)
	}
}
