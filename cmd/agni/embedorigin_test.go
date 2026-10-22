package main

import "testing"

func TestCheckEmbedOriginsTakesOnlyAnOrigin(t *testing.T) {
	if err := checkEmbedOrigins([]string{"https://portal.example", "http://127.0.0.1:9000"}); err != nil {
		t.Errorf("an origin was refused: %v", err)
	}
	for _, bad := range []string{"*", "portal.example", "https://portal.example/", "https://portal.example/app", "ftp://portal.example", "https://user@portal.example"} {
		if checkEmbedOrigins([]string{bad}) == nil {
			t.Errorf("%q was accepted as an origin", bad)
		}
	}
}
