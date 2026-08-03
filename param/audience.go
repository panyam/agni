package param

import (
	"strings"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// AudienceKey is the PartSpec attribute that records WHO is entitled to see a part's datasheet data —
// a comma-separated list of team / license identifiers. Datasheet data is vendor-licensed (C16), so a
// shared spec library may hold parts not every team may see; this captures that on the data itself.
//
// It is RECORD-ONLY today: nothing enforces it (a single team, so a gate would be vestigial). The
// enforcement — a ParamProvider that returns nil for an un-entitled MPN — is WS10-011. Stored as a
// free-form attribute rather than a proto field on purpose: it is a per-deployment annotation, not
// part of the extracted datasheet contract, and no proto churn until enforcement gives it teeth.
const AudienceKey = "audience"

// Audience returns the team/license identifiers entitled to a part's datasheet data, parsed from the
// AudienceKey attribute (comma-separated, trimmed). It is nil when unset — an unset audience means
// "not annotated", NOT "no one": until WS10-011 enforces anything, an unset audience is visible to all.
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
