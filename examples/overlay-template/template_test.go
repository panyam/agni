package main

import (
	"testing"

	"github.com/panyam/agni/check"
	"github.com/panyam/agni/formats"
)

// TestTemplateComposes is the template's smoke test: it proves the scaffold builds into a
// working overlay — the custom reader and rule are registered with the engine. Once you fill
// in myfmt/ and myrules/, extend this to assert your reader loads a fixture and your rule fires.
func TestTemplateComposes(t *testing.T) {
	if formats.ByExt("x.myfmt") == nil {
		t.Error(".myfmt reader not registered (myfmt.init did not run — is it blank-imported?)")
	}
	if check.DefaultCatalog().Lookup("myco/example-rule") == nil {
		t.Error("myco/example-rule not in the catalog (myrules.init did not run)")
	}
}
