package main

import (
	"fmt"
	"io"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/panyam/agni/core/diff"
	"github.com/panyam/agni/internal/service"
)

// writeDiffJSON emits the diff as a DiffDesignsResponse in protojson form, the same message
// the web API's DiffService serves (WS9-004), so the CLI and the viewer parse one shape.
// EmitUnpopulated keeps empty lists/maps present, so a no-change diff is still a well-formed
// object rather than fields that appear and vanish per run.
func writeDiffJSON(w io.Writer, rep *diff.Report) error {
	b, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(service.DiffResponseProto(rep))
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}
