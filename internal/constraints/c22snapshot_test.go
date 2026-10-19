package constraints

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// C22, as amended by agni issue 1061: a question about a design names a snapshot of it, never the
// design's files or the config it is read under, since the snapshot resolved both once when it was
// ingested. A request carrying its own uri or overlay beside a snapshot would let one question read
// the design a second way.
//
// The analysis services are swept by descriptor. The exempt methods ask about something other than a
// loaded design: a stored convention or checklist, a set's text, a stored review, or a review the
// caller already holds.
func TestC22AnalysisRequestsNameASnapshot(t *testing.T) {
	services := []protoreflect.ServiceDescriptor{
		webapi.File_agni_v1_webapi_checks_proto.Services().ByName("CheckService"),
		webapi.File_agni_v1_webapi_design_proto.Services().ByName("DesignService"),
		webapi.File_agni_v1_webapi_query_proto.Services().ByName("QueryService"),
		webapi.File_agni_v1_webapi_review_proto.Services().ByName("ReviewService"),
		webapi.File_agni_v1_webapi_diff_proto.Services().ByName("DiffService"),
	}
	exempt := map[string]bool{
		"GetNamingConvention": true, "GetReviewManifest": true, "ParseQuerySet": true,
		"GetReview": true, "ListReviews": true, "DeleteReview": true, "RenderReviewReport": true,
	}
	// ListRelations keeps an overlay for the vocabulary asked about before any design is read, and
	// refuses one beside a snapshot.
	overlayAllowed := map[string]bool{"ListRelations": true}
	checked := 0
	for _, svc := range services {
		if svc == nil {
			t.Fatal("an analysis service is missing from the descriptors, so this check proves nothing")
		}
		for i := 0; i < svc.Methods().Len(); i++ {
			m := svc.Methods().Get(i)
			name := string(m.Name())
			if exempt[name] {
				continue
			}
			checked++
			fields := m.Input().Fields()
			snapshots := 0
			for j := 0; j < fields.Len(); j++ {
				f := string(fields.Get(j).Name())
				switch {
				case f == "uri" || strings.HasSuffix(f, "_uri") || f == "as_named":
					t.Errorf("%s/%s takes %q (C22): a question names a snapshot, and the snapshot names the design", svc.Name(), name, f)
				case f == "overlay" && !overlayAllowed[name]:
					t.Errorf("%s/%s takes an overlay (C22): the snapshot carries the config its questions are answered under", svc.Name(), name)
				case f == "snapshot" || strings.HasSuffix(f, "_snapshot"):
					snapshots++
				}
			}
			if snapshots == 0 {
				t.Errorf("%s/%s names no snapshot (C22): a question about a design names the snapshot it is about", svc.Name(), name)
			}
		}
	}
	if checked < 15 {
		t.Fatalf("only %d analysis methods were checked; the services or the exemptions changed under this test", checked)
	}
}
