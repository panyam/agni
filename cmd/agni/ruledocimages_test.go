package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/panyam/agni/stdlib/rules/builtin"
	"github.com/panyam/agni/stdlib/rules/intent"
)

// TestFirstImageHandlerComposesSources checks the /rule-docs/ composition (WS3-093): a request for a
// built-in card and a request for an intent card both resolve 200 through the one handler, and a card
// in neither source 404s. This is the runtime path the web uses to render an intent rule's schematic
// card, so it must serve both embed FSes from the single namespace — a built-in-only handler (the
// pre-WS3-093 wiring) would 404 the intent card and the panel would show "no card yet".
func TestFirstImageHandlerComposesSources(t *testing.T) {
	h := firstImageHandler(builtin.RuleDocImageHandler(), intent.RuleDocImageHandler())
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
	// A card in neither source 404s.
	if rec := get("/images/does-not-exist.svg"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown card status = %d, want 404", rec.Code)
	}
}
