package viewerrooms

import (
	"strings"
	"unicode"

	viewerpb "github.com/panyam/agni/gen/go/agni/v1/viewer"
	"github.com/panyam/mcpkit/experimental/ext/rooms"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// toolPrefix starts every page tool's name, so an agent tells the viewer's tools from agni's
// analysis tools on the same server.
const toolPrefix = "viewer_"

// descriptions are what tools/list says of each ViewerPageService method. Generated descriptors carry
// no comments, so they are written here, and TestEveryPageMethodIsATool fails on a method without one.
var descriptions = map[string]string{
	"GetLocation": "Where an open viewer tab is now: its design, sheet, view and focused verdict, query or trace. Changes nothing.",
	"ApplyLocation": "Move an open viewer tab to a location, as following a link would. Pass url (a viewer link, such as the one an answer from check, trace or query carries) or location (only the fields to change). " +
		"Returns where the tab landed and whether a verdict it named was found.",
	"LocateVerdict": "Focus one verdict in an open viewer tab and draw its proof. id is the verdict id from check; rule and subjects let a tab that has not run that rule ask it.",
	"SelectFinding": "Focus one finding of the tab's last check run, as clicking its row does. subject is a net name or reference designator.",
	"LocateEntity":  "Highlight a net, component or pin in an open viewer tab, as clicking a query result does. kind is net, component or pin; pin is set only for a pin.",
	"ShowSheet":     "Open one sheet of the design in an open viewer tab. sheet is a sheet id, as GetLocation reports.",
}

// toolNote ends every page tool's description, saying how a call picks its tab.
const toolNote = " design names the design the tab shows, as mount/path. A call goes to the tab named by page (list_pages lists them), " +
	"else the tab of that design the reader looked at last, among the tabs whose reader turned on \"let my agents drive this tab\"."

// ToolDecls are the page tools the bridge declares: one per ViewerPageService method, named
// viewer_<method in snake_case>, taking the request message's fields and the design, and read-only
// when the method is marked NO_SIDE_EFFECTS.
func ToolDecls() []rooms.PageToolDecl {
	sd := viewerpb.File_agni_v1_viewer_page_proto.Services().ByName("ViewerPageService")
	ms := sd.Methods()
	out := make([]rooms.PageToolDecl, 0, ms.Len())
	for i := range ms.Len() {
		m := ms.Get(i)
		name := string(m.Name())
		out = append(out, rooms.PageToolDecl{
			Name:        ToolName(name),
			Description: descriptions[name] + toolNote,
			InputSchema: ToolSchema(m.Input()),
			ReadOnly:    readOnly(m),
		})
	}
	return out
}

// ToolName is a method's tool: GetLocation is viewer_get_location.
func ToolName(method string) string {
	var b strings.Builder
	b.WriteString(toolPrefix)
	for i, r := range method {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}

func readOnly(m protoreflect.MethodDescriptor) bool {
	o, _ := m.Options().(*descriptorpb.MethodOptions)
	return o.GetIdempotencyLevel() == descriptorpb.MethodOptions_NO_SIDE_EFFECTS
}

// ToolSchema is a page tool's input schema: the request message's, with a required design beside
// its fields, which the bridge maps to the room and the tab drops before it decodes the request.
func ToolSchema(md protoreflect.MessageDescriptor) map[string]any {
	s := MessageSchema(md)
	s["properties"].(map[string]any)["design"] = map[string]any{"type": "string"}
	s["required"] = []any{"design"}
	return s
}

// MessageSchema is the JSON Schema of a message as protojson writes it, the mapping mcpkit#1609 sets
// out for protogen: properties under their JSON names, 64-bit integers as strings, bytes as base64,
// enums by name, the well-known types in their own JSON forms (wellKnown), and nothing required,
// since proto3 has no required fields and these protos carry no field_behavior. web/src/toolschema.ts is the same walk
// over protobuf-es descriptors, and both are held to testdata/viewer_tools.json.
func MessageSchema(md protoreflect.MessageDescriptor) map[string]any {
	return messageSchema(md, map[protoreflect.FullName]bool{})
}

func messageSchema(md protoreflect.MessageDescriptor, visiting map[protoreflect.FullName]bool) map[string]any {
	if s, ok := wellKnown[md.FullName()]; ok {
		return s()
	}
	if visiting[md.FullName()] {
		return map[string]any{"type": "object"}
	}
	visiting[md.FullName()] = true
	defer delete(visiting, md.FullName())
	props := map[string]any{}
	fs := md.Fields()
	for i := range fs.Len() {
		f := fs.Get(i)
		props[f.JSONName()] = fieldSchema(f, visiting)
	}
	return map[string]any{"type": "object", "properties": props}
}

func fieldSchema(f protoreflect.FieldDescriptor, visiting map[protoreflect.FullName]bool) map[string]any {
	switch {
	case f.IsMap():
		return map[string]any{"type": "object", "additionalProperties": valueSchema(f.MapValue(), visiting)}
	case f.IsList():
		return map[string]any{"type": "array", "items": valueSchema(f, visiting)}
	}
	return valueSchema(f, visiting)
}

// valueSchema is one value of f, so a list's element or a map's value.
func valueSchema(f protoreflect.FieldDescriptor, visiting map[protoreflect.FullName]bool) map[string]any {
	switch f.Kind() {
	case protoreflect.BoolKind:
		return map[string]any{"type": "boolean"}
	case protoreflect.StringKind:
		return map[string]any{"type": "string"}
	case protoreflect.BytesKind:
		return map[string]any{"type": "string", "contentEncoding": "base64"}
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind, protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return map[string]any{"type": "integer"}
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind, protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return map[string]any{"type": "string"}
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return map[string]any{"type": "number"}
	case protoreflect.EnumKind:
		vs := f.Enum().Values()
		names := make([]any, vs.Len())
		for i := range vs.Len() {
			names[i] = string(vs.Get(i).Name())
		}
		return map[string]any{"type": "string", "enum": names}
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return messageSchema(f.Message(), visiting)
	}
	return map[string]any{}
}

// wellKnown are the types protojson writes in a JSON form of their own rather than as their fields:
// a wrapper as its bare value, Struct as any object, a Timestamp as an RFC 3339 string.
var wellKnown = map[protoreflect.FullName]func() map[string]any{
	"google.protobuf.Timestamp":   func() map[string]any { return map[string]any{"type": "string", "format": "date-time"} },
	"google.protobuf.Duration":    func() map[string]any { return map[string]any{"type": "string"} },
	"google.protobuf.FieldMask":   func() map[string]any { return map[string]any{"type": "string"} },
	"google.protobuf.Struct":      func() map[string]any { return map[string]any{"type": "object"} },
	"google.protobuf.Value":       func() map[string]any { return map[string]any{} },
	"google.protobuf.ListValue":   func() map[string]any { return map[string]any{"type": "array"} },
	"google.protobuf.Any":         func() map[string]any { return map[string]any{"type": "object"} },
	"google.protobuf.BoolValue":   func() map[string]any { return map[string]any{"type": "boolean"} },
	"google.protobuf.StringValue": func() map[string]any { return map[string]any{"type": "string"} },
	"google.protobuf.BytesValue":  func() map[string]any { return map[string]any{"type": "string", "contentEncoding": "base64"} },
	"google.protobuf.Int32Value":  func() map[string]any { return map[string]any{"type": "integer"} },
	"google.protobuf.UInt32Value": func() map[string]any { return map[string]any{"type": "integer"} },
	"google.protobuf.Int64Value":  func() map[string]any { return map[string]any{"type": "string"} },
	"google.protobuf.UInt64Value": func() map[string]any { return map[string]any{"type": "string"} },
	"google.protobuf.FloatValue":  func() map[string]any { return map[string]any{"type": "number"} },
	"google.protobuf.DoubleValue": func() map[string]any { return map[string]any{"type": "number"} },
}
