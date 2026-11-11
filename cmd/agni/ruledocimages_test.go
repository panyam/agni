package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestFirstImageHandlerComposesSources checks the /rule-docs/ composition (WS3-093, agni issue 75).
// A card from each of the four families resolves 200 through the one handler, and a card in none
// 404s. The web takes this one route for every rule's card, so a family missing from the composition
// would 404 its cards and the panel would show "no card yet".
func TestFirstImageHandlerComposesSources(t *testing.T) {
	h := ruleDocImages()
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}
	// A built-in card resolves (the first source).
	if rec := get("/images/decoupling-present.svg"); rec.Code != http.StatusOK {
		t.Errorf("built-in card status = %d, want 200", rec.Code)
	}
	// An intent card resolves through the fall-through to the second source.
	rec := get("/images/protection-ovp.svg")
	if rec.Code != http.StatusOK {
		t.Fatalf("intent card status = %d, want 200 (composition did not reach the intent source)", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/svg+xml" {
		t.Errorf("intent card content-type = %q, want image/svg+xml", ct)
	}
	// A profile card and a datalog card resolve through the same route (agni issue 75).
	for _, p := range []string{"/images/profile-termination.svg", "/images/power-pin-mistyped.svg"} {
		if rec := get(p); rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/svg+xml" {
			t.Errorf("%s: status %d, content-type %q, want 200 image/svg+xml", p, rec.Code, rec.Header().Get("Content-Type"))
		}
	}
	// A card in no source 404s.
	if rec := get("/images/does-not-exist.svg"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown card status = %d, want 404", rec.Code)
	}
}
