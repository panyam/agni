package artifact

import (
	"strings"
	"testing"
)

func TestParseRoundTrip(t *testing.T) {
	cases := []struct{ in, mount, path string }{
		{"mount://boards/designs/gateway/gateway.edn", "boards", "designs/gateway/gateway.edn"},
		{"mount://boards/a.edn", "boards", "a.edn"},
		{"mount://boards", "boards", ""},  // the mount root, which a directory listing asks for
		{"mount://boards/", "boards", ""}, // same thing, trailing slash
		{"mount://boards/./a/../b.edn", "boards", "b.edn"},
	}
	for _, c := range cases {
		u, err := Parse(c.in)
		if err != nil {
			t.Fatalf("Parse(%q) = %v", c.in, err)
		}
		if u.Mount != c.mount || u.Path != c.path {
			t.Errorf("Parse(%q) = {%q %q}, want {%q %q}", c.in, u.Mount, u.Path, c.mount, c.path)
		}
		// String round-trips to something Parse reads back identically, which is what lets a URI be
		// stored, logged, and sent without a normalization step at each hop.
		again, err := Parse(u.String())
		if err != nil || again != u {
			t.Errorf("round trip of %q via %q = {%+v} %v", c.in, u.String(), again, err)
		}
	}
}

// TestParseRejects is the containment boundary. A permissive parse here is a path traversal
// everywhere above it, so each of these has to be an error rather than a best-effort reading.
func TestParseRejects(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"bare path", "designs/gateway/gateway.edn", "missing"},
		{"host path", "/Users/me/board.edn", "missing"},
		{"other scheme", "s3://bucket/board.edn", "not supported"},
		{"file scheme", "file:///etc/passwd", "not supported"},
		{"no mount", "mount:///a.edn", "mount is required"},
		{"empty", "", "missing"},
		{"escapes", "mount://boards/../secrets.edn", "escapes"},
		{"escapes deeper", "mount://boards/a/../../secrets.edn", "escapes"},
		{"escapes via dot segments", "mount://boards/a/b/../../../x", "escapes"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(c.in)
			if err == nil {
				t.Fatalf("Parse(%q) succeeded, want an error mentioning %q", c.in, c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("Parse(%q) = %q, want it to mention %q", c.in, err, c.want)
			}
		})
	}
}

// TestNewAndParseAgree: code holding the two halves must not be able to assemble a string that
// Parse would reject, or containment would depend on which constructor a caller happened to use.
func TestNewAndParseAgree(t *testing.T) {
	if _, err := New("boards", "../escape"); err == nil {
		t.Error("New should apply the same containment rules as Parse")
	}
	if _, err := New("", "a.edn"); err == nil {
		t.Error("New should require a mount")
	}
	if _, err := New("bo/ards", "a.edn"); err == nil {
		t.Error("a mount carrying a separator would split one URI into two readings")
	}
	u, err := New("boards", "a/b.edn")
	if err != nil {
		t.Fatal(err)
	}
	if u.String() != "mount://boards/a/b.edn" {
		t.Errorf("String = %q", u.String())
	}
}

// TestResolve is the operation a reader performs when a file names another file relative to itself:
// a schematic's sub-sheets, a symbol library. It replaces a hand-rolled sibling join whose own
// comment warned that mixing path and filepath separators is invisible on unix and breaks on
// Windows. A URI path is always slash-separated, so that split has nowhere to hide.
func TestResolve(t *testing.T) {
	base, err := Parse("mount://boards/designs/gateway/gateway.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ rel, want string }{
		{"sub/sheet1.kicad_sch", "mount://boards/designs/gateway/sub/sheet1.kicad_sch"},
		{"gateway.kicad_pcb", "mount://boards/designs/gateway/gateway.kicad_pcb"},
		{"./symbols/gateway.kicad_sym", "mount://boards/designs/gateway/symbols/gateway.kicad_sym"},
		{"../shared/lib.kicad_sym", "mount://boards/designs/shared/lib.kicad_sym"},
		// A full URI reference replaces everything, mount included.
		{"mount://other/x.edn", "mount://other/x.edn"},
		// A leading slash is AUTHORITY-relative, per RFC 3986, so it names the mount root rather
		// than the current directory. Still contained: the authority is the mount.
		{"/shared/lib.kicad_sym", "mount://boards/shared/lib.kicad_sym"},
	}
	for _, c := range cases {
		got, err := base.Resolve(c.rel)
		if err != nil {
			t.Fatalf("Resolve(%q) = %v", c.rel, err)
		}
		if got.String() != c.want {
			t.Errorf("Resolve(%q) = %q, want %q", c.rel, got, c.want)
		}
	}
}

// TestResolveCannotEscape: being inside a design is not authority to leave the mount. A file that
// names its way out is a file reading past the boundary its own mount established.
func TestResolveCannotEscape(t *testing.T) {
	base, _ := Parse("mount://boards/designs/gateway/gateway.kicad_sch")
	for _, rel := range []string{"../../../etc/passwd", "../../..", "/../etc/passwd", "mount://boards/../x"} {
		if got, err := base.Resolve(rel); err == nil {
			t.Errorf("Resolve(%q) = %q, want an error", rel, got)
		}
	}
}

func TestDirBaseJoin(t *testing.T) {
	u, _ := Parse("mount://boards/designs/gateway/gateway.edn")
	if got := u.Dir().String(); got != "mount://boards/designs/gateway" {
		t.Errorf("Dir = %q", got)
	}
	if got := u.Base(); got != "gateway.edn" {
		t.Errorf("Base = %q", got)
	}
	child, err := u.Dir().Join("symbols", "lib.kicad_sym")
	if err != nil {
		t.Fatal(err)
	}
	if child.String() != "mount://boards/designs/gateway/symbols/lib.kicad_sym" {
		t.Errorf("Join = %q", child)
	}

	// A mount root is its own parent and has no base, so walking up terminates instead of looping.
	root, _ := Parse("mount://boards")
	// A mount root is its own parent, so walking up terminates instead of looping.
	if root.Dir() != root || root.Base() != "" {
		t.Errorf("mount root: Dir = %+v, Base = %q", root.Dir(), root.Base())
	}
}

func TestIsZero(t *testing.T) {
	if !(URI{}).IsZero() {
		t.Error("the zero URI names nothing")
	}
	u, _ := Parse("mount://boards")
	if u.IsZero() {
		t.Error("a mount root names something")
	}
}
