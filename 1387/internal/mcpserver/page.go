package mcpserver

import (
	"encoding/json"

	viewerpb "github.com/panyam/agni/gen/go/agni/v1/viewer"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// pageAction is a ViewerPageService call. A host makes it on a viewer it already has open, beside
// the link that opens one (agni issue 1155). method is the rpc's bare name, and body is its request
// in protojson. The viewer's postMessage envelope carries the same pair, so a host embedding the
// viewer sends it unchanged rather than opening the link in a new tab.
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

// linkAction applies a link to an open viewer. Any link an answer carries can be applied this way.
// It returns nil for no link.
func linkAction(url string) *pageAction {
	if url == "" {
		return nil
	}
	return newPageAction("ApplyLocation", &viewerpb.ApplyLocationRequest{Url: url})
}

// verdictAction focuses one verdict on an open viewer. It sends the verdict's rule and subjects, as
// the verdict's link does. A viewer that has not answered the verdict then asks that one rule.
func verdictAction(id, rule string, subjects []string) *pageAction {
	return newPageAction("LocateVerdict", &viewerpb.LocateVerdictRequest{Id: id, Rule: rule, Subjects: subjects})
}
