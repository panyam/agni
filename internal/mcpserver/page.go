package mcpserver

import (
	"encoding/json"

	viewerpb "github.com/panyam/agni/gen/go/agni/v1/viewer"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// pageAction is a ViewerPageService call a host can make on a viewer it already has open, beside the
// link that opens one (agni issue 1155). method is the rpc's bare name and body its request in
// protojson, the pair the viewer's postMessage envelope carries, so a host embedding the viewer sends
// it as it stands rather than opening the link in a new tab.
type pageAction struct {
	Method string          `json:"method"`
	Body   json.RawMessage `json:"body"`
}

// newPageAction spells req as the call to method. Only the fields req sets are written, since an
// empty field in a page request means "leave it as it is".
func newPageAction(method string, req proto.Message) *pageAction {
	b, err := protojson.Marshal(req)
	if err != nil {
		return nil
	}
	return &pageAction{Method: method, Body: b}
}

// linkAction applies a link to an open viewer, which every link an answer carries can be. Nil for no
// link.
func linkAction(url string) *pageAction {
	if url == "" {
		return nil
	}
	return newPageAction("ApplyLocation", &viewerpb.ApplyLocationRequest{Url: url})
}

// verdictAction focuses one verdict on an open viewer, with the rule and subjects that let a viewer
// that has not answered it ask that one rule, as the verdict's link does.
func verdictAction(id, rule string, subjects []string) *pageAction {
	return newPageAction("LocateVerdict", &viewerpb.LocateVerdictRequest{Id: id, Rule: rule, Subjects: subjects})
}
