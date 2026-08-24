package main

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/internal/mounts"
)

// The banner's whole job is to be paste-able. A mount minted by the serving process means nothing to
// a second agni, so a line without --mount is illustrative rather than usable.
func TestOpenBannerPrintsAPasteableCheckCommand(t *testing.T) {
	var b bytes.Buffer
	m := mounts.Mount{Name: "gateway", Root: "/boards/gw"}
	openBanner(&b, "http://127.0.0.1:5000/", m, "mount://gateway/designs/gw.edn", "/designs/gateway/designs/gw.edn/view")
	out := b.String()

	if !strings.Contains(out, "--mount gateway=/boards/gw") {
		t.Errorf("the check line must declare the mount, or it fails when pasted:\n%s", out)
	}
	if !strings.Contains(out, "mount://gateway/designs/gw.edn") {
		t.Errorf("the check line must name the design it opened:\n%s", out)
	}
	if !strings.Contains(out, "--url-base http://127.0.0.1:5000\n") {
		t.Errorf("--url-base must be a bare origin, since VerdictURL concatenates rather than joins:\n%s", out)
	}
}

// serveURLs ends its addresses with a slash. Doubling it is cosmetic in a browser and is not cosmetic
// in --url-base.
func TestOpenBannerTrimsTheTrailingSlash(t *testing.T) {
	var b bytes.Buffer
	openBanner(&b, "http://127.0.0.1:5000/", mounts.Mount{Name: "m", Root: "/r"}, "mount://m/x.edn", "/designs/m/x.edn/view")
	if strings.Contains(b.String(), "5000//designs") {
		t.Errorf("doubled slash in the design URL:\n%s", b.String())
	}
	if strings.Contains(b.String(), "url-base http://127.0.0.1:5000/") {
		t.Errorf("--url-base kept its trailing slash:\n%s", b.String())
	}
}

func TestRefuseTooBroadRoot(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory on this host")
	}
	if err := refuseTooBroadRoot(home); err == nil {
		t.Error("serving the home directory must be refused")
	} else if !strings.Contains(err.Error(), "home directory") {
		t.Errorf("the refusal should say why, got %v", err)
	}
	// A trailing slash and a redundant path element name the same directory and must be refused too,
	// or the guard is a string comparison wearing a filesystem's clothes.
	if err := refuseTooBroadRoot(home + "/./"); err == nil {
		t.Error("an unnormalized spelling of the home directory must also be refused")
	}
	if err := refuseTooBroadRoot("/"); err == nil {
		t.Error("serving a filesystem root must be refused")
	}
	if err := refuseTooBroadRoot(t.TempDir()); err != nil {
		t.Errorf("an ordinary directory is fine, got %v", err)
	}
	// A directory INSIDE the home directory is the ordinary case and must not be caught by a prefix
	// match, which is the obvious wrong implementation of this guard.
	if err := refuseTooBroadRoot(filepath.Join(home, "boards", "gw")); err != nil {
		t.Errorf("a directory under home is not the home directory, got %v", err)
	}
}

// freePort has to return something actually bindable, not merely a number.
func TestFreePortIsBindable(t *testing.T) {
	p, err := freePort()
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", p))
	if err != nil {
		t.Fatalf("the port freePort returned is not bindable: %v", err)
	}
	l.Close()
	if p == "0" {
		t.Error("freePort must resolve the kernel's assignment, not hand back :0")
	}
}
