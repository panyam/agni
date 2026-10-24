package viewerrooms

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	viewerpb "github.com/panyam/agni/gen/go/agni/v1/viewer"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/typepb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// twinPath is the fixture this walk and web/src/toolschema.ts are both held to. The bridge refuses a
// tab's tool whose schema differs from the declaration by a byte, so a drift between the two walks
// would leave every tab's tools refused with nothing failing but the agent's call.
var twinPath = filepath.Join("testdata", "viewer_tools.json")

type twinTool struct {
	Name        string         `json:"name"`
	ReadOnly    bool           `json:"readOnly"`
	InputSchema map[string]any `json:"inputSchema"`
}

type twin struct {
	Tools []twinTool `json:"tools"`
	// Messages are types both runtimes ship, covering what ViewerPageService does not use: the
	// well-known types' own JSON forms, enums, 64-bit integers, a double and bytes. None carries
	// descriptor.proto's options, whose FeatureSet differs between protobuf releases.
	Messages map[string]map[string]any `json:"messages"`
}

// twinMessages are the fixture's messages, keyed there by full name.
var twinMessages = []protoreflect.MessageDescriptor{
	(&structpb.Struct{}).ProtoReflect().Descriptor(),
	(&structpb.Value{}).ProtoReflect().Descriptor(),
	(&wrapperspb.Int64Value{}).ProtoReflect().Descriptor(),
	(&wrapperspb.BytesValue{}).ProtoReflect().Descriptor(),
	(&descriptorpb.UninterpretedOption{}).ProtoReflect().Descriptor(),
	(&descriptorpb.SourceCodeInfo{}).ProtoReflect().Descriptor(),
	(&typepb.Field{}).ProtoReflect().Descriptor(),
}

func built() twin {
	var t twin
	for _, d := range ToolDecls() {
		t.Tools = append(t.Tools, twinTool{Name: d.Name, ReadOnly: d.ReadOnly, InputSchema: d.InputSchema})
	}
	t.Messages = map[string]map[string]any{}
	for _, md := range twinMessages {
		t.Messages[string(md.FullName())] = MessageSchema(md)
	}
	return t
}

// asJSON is v as it reads back from JSON, so a Go map and the fixture compare as the bridge compares
// a declaration with a tab's schema.
func asJSON(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestToolsMatchTheTwinFixture holds the Go walk to testdata/viewer_tools.json. UPDATE_GOLDEN=1
// rewrites it; read the diff, since the TS walk has to change the same way.
func TestToolsMatchTheTwinFixture(t *testing.T) {
	got := built()
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		b, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(twinPath, append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(twinPath)
	if err != nil {
		t.Fatalf("%v; run UPDATE_GOLDEN=1 go test ./internal/viewerrooms/ to write it", err)
	}
	var want any
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if g := asJSON(t, got); !reflect.DeepEqual(g, want) {
		b, _ := json.MarshalIndent(g, "", "  ")
		t.Fatalf("the declared tools differ from %s; got\n%s", twinPath, b)
	}
}

// TestEveryPageMethodIsATool is the census: each ViewerPageService method is one tool with a
// description, and no description names a method that is gone.
func TestEveryPageMethodIsATool(t *testing.T) {
	ms := viewerpb.File_agni_v1_viewer_page_proto.Services().ByName("ViewerPageService").Methods()
	decls := ToolDecls()
	if len(decls) != ms.Len() || ms.Len() == 0 {
		t.Fatalf("%d tools for %d methods", len(decls), ms.Len())
	}
	seen := map[string]bool{}
	for i := range ms.Len() {
		name := string(ms.Get(i).Name())
		if descriptions[name] == "" {
			t.Errorf("%s has no description in tools.go", name)
		}
		seen[name] = true
	}
	for name := range descriptions {
		if !seen[name] {
			t.Errorf("tools.go describes %s, which ViewerPageService no longer has", name)
		}
	}
}

func TestToolNamesAndReadOnly(t *testing.T) {
	want := map[string]bool{
		"viewer_get_location":   true,
		"viewer_apply_location": false,
		"viewer_locate_verdict": false,
		"viewer_select_finding": false,
		"viewer_locate_entity":  false,
		"viewer_show_sheet":     false,
	}
	got := map[string]bool{}
	for _, d := range ToolDecls() {
		got[d.Name] = d.ReadOnly
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tools %v, want %v", got, want)
	}
}

// TestPropertiesUseTheJSONName is the mapping's first rule: the schema describes the JSON a tab
// decodes with protojson, so trace_hops is traceHops.
func TestPropertiesUseTheJSONName(t *testing.T) {
	s := MessageSchema((&viewerpb.ViewerLocation{}).ProtoReflect().Descriptor())
	props := s["properties"].(map[string]any)
	if _, ok := props["traceHops"]; !ok {
		t.Errorf("no traceHops in %v", props)
	}
	if _, ok := props["trace_hops"]; ok {
		t.Errorf("trace_hops, the proto name, in %v", props)
	}
}

// TestARecursiveMessageEndsAtItsSecondVisit: DescriptorProto nests itself, and the walk writes the
// inner one as a bare object rather than recursing forever.
func TestARecursiveMessageEndsAtItsSecondVisit(t *testing.T) {
	s := MessageSchema((&descriptorpb.DescriptorProto{}).ProtoReflect().Descriptor())
	nested := s["properties"].(map[string]any)["nestedType"].(map[string]any)["items"]
	if !reflect.DeepEqual(nested, map[string]any{"type": "object"}) {
		t.Fatalf("nestedType items %v", nested)
	}
}
