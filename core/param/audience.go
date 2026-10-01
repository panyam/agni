package param

import (
	"strings"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// AudienceKey is the PartSpec attribute that records WHO is entitled to see a part's datasheet data,
// as a comma-separated list of team or license identifiers. Datasheet data is vendor-licensed (C16), so
// a shared spec library may hold parts not every team may see.
//
// It is RECORD-ONLY: nothing enforces it yet. Enforcement, a ParamProvider returning nil for an
// un-entitled MPN, is WS10-011. It is a free-form attribute rather than a proto field because it is a
// per-deployment annotation, not part of the extracted datasheet contract.
const AudienceKey = "audience"

// Audience returns the team/license identifiers entitled to a part's datasheet data, parsed from the
// AudienceKey attribute (comma-separated, trimmed). It is nil when unset, which means "not annotated"
// and NOT "no one". Until WS10-011 enforces anything, an unset audience is visible to all.
func Audience(spec *parampb.PartSpec) []string {
	if spec == nil {
		return nil
	}
	raw := spec.GetAttributes()[AudienceKey]
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if t := strings.TrimSpace(part); t != "" {
			out = append(out, t)
		}
	}
	return out
}
