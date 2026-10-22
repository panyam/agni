package main

import (
	"net"
	"testing"
)

// --addr host:0 binds a free port before serving, so what the banner prints is a port a client can
// reach (agni issue 1121), and any other address is left for the serve to bind.
func TestServeOnPortZeroNamesThePortItGot(t *testing.T) {
	ln, err := bindFreePort("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if ln == nil {
		t.Fatal("port 0 bound nothing")
	}
	defer ln.Close()
	if _, port, _ := net.SplitHostPort(ln.Addr().String()); port == "0" || port == "" {
		t.Errorf("bound %s, want the port the kernel chose", ln.Addr())
	}
	for _, addr := range []string{":8080", "127.0.0.1:8091", "nonsense"} {
		if ln, err := bindFreePort(addr); ln != nil || err != nil {
			t.Errorf("%s bound %v (%v), want it left for the serve", addr, ln, err)
		}
	}
}
