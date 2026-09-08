package main

import (
	"strings"
	"testing"
)

// The most likely failure of `agni open` is someone outside a repo checkout, where the default
// --web-dir is a relative path that does not exist. Rung 3 handed a reader exactly that command and
// promised `agni open` "works that out for itself" (agni issue 625).
//
// #640 rewrote the message while this was in flight, and named all three routes, which is what the
// page now documents. Nothing pinned it, so this does: the message is the one thing standing between
// a first-time reader and a dead end, and it is a string, which is the kind of thing that gets
// shortened by someone tidying.
func TestWebDirErrorNamesEveryWayToSetIt(t *testing.T) {
	_, _, err := resolveWebAssets(t.TempDir()+"/absent", func(string) string { return "" })
	if err == nil {
		t.Fatal("a missing --web-dir must fail")
	}
	for _, want := range []string{"--web-dir", "AGNI_WEB_DIR", "web_dir", "agni.yaml"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not mention %q, so a reader cannot act on it:\n%v", want, err)
		}
	}
}
