package intent

import (
	"fmt"
	"os"

	"github.com/panyam/agni/core/check"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
)

// LoadFile reads a single intent declaration from a YAML file (the --intent-path flag). The os
// dependency lives here, isolated from the WASM-clean Parse/Load path. Unlike profiles' LoadDir, this
// reads one file rather than a directory to merge, because a design has ONE intended architecture.
func LoadFile(path string) (Declaration, error) {
	f, err := os.Open(path)
	if err != nil {
		return Declaration{}, fmt.Errorf("intent: reading --intent-path %q: %w", path, err)
	}
	defer f.Close()
	d, err := Load(f)
	if err != nil {
		return Declaration{}, fmt.Errorf("intent: %s: %w", path, err)
	}
	return d, nil
}

// LoadFileProto reads a single intent declaration from a YAML file as its proto, validated (see
// ParseProto). The CLI's --intent-path sends it on the request.
func LoadFileProto(path string) (*configpb.DesignIntent, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("intent: reading --intent-path %q: %w", path, err)
	}
	in, err := ParseProto(b)
	if err != nil {
		return nil, fmt.Errorf("intent: %s: %w", path, err)
	}
	return in, nil
}

// Source compiles a declaration into a named check.RuleSource so the CLI can splice the intent rules
// into the catalog via check.CatalogWith, the same way --profile-path splices interface profiles. The
// name is the namespace prefix and must differ from every other source name (CatalogWith rejects a
// duplicate); the CLI uses "intent".
func Source(name string, d Declaration) check.RuleSource {
	return check.NewSource(name, Compile(d))
}
