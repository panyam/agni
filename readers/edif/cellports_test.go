package edif

import (
	"strings"
	"testing"
)

const cellPortsEDIF = `(edif CELLPORTS
  (edifVersion 2 0 0)
  (design CELLPORTS (cellRef TOP (libraryRef LIB)))
  (library LIB
    (cell PHY (view V (viewType NETLIST)
      (interface (port MDIO (designator "1")) (port RST (designator "2")) (port NC))))
    (cell TOP (view V (viewType NETLIST) (interface)
      (contents
        (instance U2 (viewRef V (cellRef PHY)) (designator "U2"))
        (instance U3 (viewRef V (cellRef PHY (libraryRef LIB))) (designator "U3")
          (portInstance RST (designator "7")))
        (net MDIO_NET (joined (portRef MDIO (instanceRef U2)) (portRef MDIO (instanceRef U3))))
        (net RST_N (joined (portRef RST (instanceRef U2)) (portRef RST (instanceRef U3))))
        (net FLOAT (joined (portRef NC (instanceRef U2)))))))))
`

// A port's own designator maps the connections that name it (agni issue 1193): MDIO_NET lands on
// pin 1, the pin the part type declares, rather than on a pin MDIO it does not have. An instance's
// portInstance table still wins over the cell's, and a port with no designator keeps its name.
func TestACellPortsDesignatorMapsItsConnections(t *testing.T) {
	d, err := Read(strings.NewReader(cellPortsEDIF), "cellports.edn")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ net, ref, pin string }{
		{"MDIO_NET", "U2", "1"},
		{"MDIO_NET", "U3", "1"},
		{"RST_N", "U2", "2"},
		{"RST_N", "U3", "7"},
		{"FLOAT", "U2", "NC"},
	} {
		n := netByName(d, tc.net)
		if n == nil || !hasConn(n, tc.ref, tc.pin) {
			t.Errorf("%s should connect %s.%s, got %v", tc.net, tc.ref, tc.pin, n.GetConnections())
		}
	}
	if n := netByName(d, "MDIO_NET"); hasConn(n, "U2", "MDIO") {
		t.Error("MDIO_NET still connects the port name U2.MDIO")
	}
}
