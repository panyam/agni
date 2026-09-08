package main

import (
	"bytes"
	"net"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestParseServerSpecShapes(t *testing.T) {
	for _, tc := range []struct {
		name, in  string
		self      bool
		url       string
		wantError bool
	}{
		{name: "empty mints nothing", in: ""},
		{name: "self", in: "self", self: true},
		{name: "url", in: "http://localhost:8080", url: "http://localhost:8080"},
		{name: "url trailing slash trimmed", in: "http://localhost:8080/", url: "http://localhost:8080"},
		{name: "bare host is not a url", in: "localhost:8080", wantError: true},
		{name: "self with a non-numeric port", in: "self:web", wantError: true},
		{name: "self with an out-of-range port", in: "self:99999", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseServerSpec(tc.in)
			if tc.wantError {
				if err == nil {
					t.Fatalf("parseServerSpec(%q) = %+v, want an error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseServerSpec(%q): %v", tc.in, err)
			}
			defer closeSpec(got)
			if got.self != tc.self {
				t.Errorf("self = %v, want %v", got.self, tc.self)
			}
			if got.url != tc.url {
				t.Errorf("url = %q, want %q", got.url, tc.url)
			}
			if got.self && got.ln == nil {
				t.Error("self parsed without binding a listener, so nothing holds the port")
			}
		})
	}
}

// TestSelfHoldsItsPort is what makes "fails on a taken port" exact rather than likely: the port is
// bound while the flag is parsed, so nothing can take it between the check and the serve. A probe
// that bound and closed would pass this test and still lose the race in the field.
func TestSelfHoldsItsPort(t *testing.T) {
	spec, err := parseServerSpec("self")
	if err != nil {
		t.Fatal(err)
	}
	defer closeSpec(spec)

	addr := spec.ln.Addr().String()
	second, err := net.Listen("tcp", addr)
	if err == nil {
		second.Close()
		t.Fatalf("%s was still bindable, so the spec is not holding it", addr)
	}
	if !strings.HasPrefix(spec.base(), "http://") {
		t.Errorf("base() = %q, want an http address", spec.base())
	}
}

// TestSelfPortInUseFailsBeforeAnyWork covers the choice to fail rather than fall back to a free port.
// An explicit port is an assertion about where the links will point, so binding elsewhere would mint
// links resolving on whatever else is listening.
func TestSelfPortInUseFailsBeforeAnyWork(t *testing.T) {
	held, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	_, port, _ := net.SplitHostPort(held.Addr().String())

	_, err = parseServerSpec("self:" + port)
	if err == nil {
		t.Fatal("a port already in use parsed cleanly")
	}
	if !strings.Contains(err.Error(), "in use") {
		t.Errorf("error does not say the port is in use: %v", err)
	}
}

// TestSelfLinksAMintedMount is the ticket's own complaint, as a test.
//
// A mount minted for one run means nothing on a server that was not started with it, so linkTarget
// refuses to build a link from one. Under `--server self` the premise is gone, because this process
// serves the table it just minted. Red-checks by restoring the unconditional Declared check.
func TestSelfLinksAMintedMount(t *testing.T) {
	ws, err := newCLIWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	const uri = "mount://minted/board.edn"

	if got, why := linkTarget(ws, uri, false); got != "" || why == "" {
		t.Errorf("without a server: got %q (%q), want no link and a reason", got, why)
	}
	got, why := linkTarget(ws, uri, true)
	if got == "" {
		t.Fatalf("under --server self a minted mount still refused to link: %s", why)
	}
	if want := "minted/board.edn"; got != want {
		t.Errorf("link target = %q, want %q", got, want)
	}
}

// TestURLBaseStillWorksAndSaysItIsDeprecated: the alias keeps every existing invocation running,
// including the `check ... --url-base ...` line `agni open` prints, and says so once on stderr.
func TestURLBaseStillWorksAndSaysItIsDeprecated(t *testing.T) {
	cmd := &cobra.Command{}
	var errOut bytes.Buffer
	cmd.SetErr(&errOut)

	spec, err := resolveServer(cmd, "", "http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	if spec.url != "http://localhost:8080" {
		t.Errorf("url = %q, want the --url-base value", spec.url)
	}
	if !strings.Contains(errOut.String(), "deprecated") {
		t.Errorf("no deprecation note on stderr: %q", errOut.String())
	}
}

// TestServerWinsOverURLBase pins the precedence, since both may be passed while the alias lives.
func TestServerWinsOverURLBase(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetErr(&bytes.Buffer{})
	spec, err := resolveServer(cmd, "http://a.example", "http://b.example")
	if err != nil {
		t.Fatal(err)
	}
	if spec.url != "http://a.example" {
		t.Errorf("url = %q, want the --server value to win", spec.url)
	}
}

func closeSpec(s serverSpec) {
	if s.ln != nil {
		s.ln.Close()
	}
}
