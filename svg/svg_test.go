package svg

import "testing"

func TestCanvas_ElementsAndDoc(t *testing.T) {
	c := Open(160, 120, A("font-family", "monospace"))
	c.El("rect", I("x", 0), I("y", 0), F("width", 160), F("height", 120), A("fill", "#fff"))
	c.El("circle", F("cx", 12.34), F("cy", 5), F("r", 2.5), A("fill", "#d11"))
	got := c.String()

	want := `<svg xmlns="http://www.w3.org/2000/svg" width="160.0" height="120.0" viewBox="0 0 160.0 120.0" font-family="monospace">` +
		"\n" + `<rect x="0" y="0" width="160.0" height="120.0" fill="#fff"/>` +
		"\n" + `<circle cx="12.3" cy="5.0" r="2.5" fill="#d11"/>` +
		"\n</svg>\n"
	if got != want {
		t.Errorf("canvas output mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestText_EscapesContent(t *testing.T) {
	c := Open(10, 10)
	c.Text("A<B>&\"'", F("x", 1), A("text-anchor", "middle"))
	got := c.String()
	// The content must be XML-escaped; attributes are written verbatim.
	wantFrag := `<text x="1.0" text-anchor="middle">A&lt;B&gt;&amp;&#34;&#39;</text>`
	if !contains(got, wantFrag) {
		t.Errorf("escaped text fragment not found.\n got: %q\nwant fragment: %q", got, wantFrag)
	}
}

func TestF_OneDecimal(t *testing.T) {
	if a := F("x", 3.0); a.val != "3.0" {
		t.Errorf("F(3.0) = %q, want 3.0", a.val)
	}
	if a := F("x", 3.14159); a.val != "3.1" {
		t.Errorf("F(3.14159) = %q, want 3.1", a.val)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
