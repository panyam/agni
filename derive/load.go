package derive

import (
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"strings"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	derivepb "github.com/panyam/agni/gen/go/agni/v1/derive"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// LoadRecipes walks fsys for *.textproto Recipes and validates each: a name, a
// compilable doc_title_pattern, and table rules whose patterns compile and whose
// limit_kind names resolve to agni.v1.param.LimitKind values. All-or-nothing, like
// param.LoadSet: a recipe corpus with a broken recipe must not silently shrink.
func LoadRecipes(fsys fs.FS) ([]*derivepb.Recipe, error) {
	var out []*derivepb.Recipe
	err := walkTextprotos(fsys, func(path string, data []byte) error {
		r := &derivepb.Recipe{}
		if err := prototext.Unmarshal(data, r); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if r.Name == "" {
			return fmt.Errorf("%s: recipe has no name", path)
		}
		if _, err := regexp.Compile(r.DocTitlePattern); err != nil {
			return fmt.Errorf("%s: doc_title_pattern: %w", path, err)
		}
		for _, tr := range r.Tables {
			if _, err := regexp.Compile(tr.TitlePattern); err != nil {
				return fmt.Errorf("%s: title_pattern %q: %w", path, tr.TitlePattern, err)
			}
			if kind, ok := parampb.LimitKind_value[tr.LimitKind]; !ok || kind == 0 {
				return fmt.Errorf("%s: unknown limit_kind %q", path, tr.LimitKind)
			}
		}
		out = append(out, r)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// LoadPatches walks fsys for *.textproto Patches. A patch must carry its keys (doc
// and table content hashes), the corrected text's position, and a non-empty note:
// an unexplained correction cannot be reviewed, and patches are the layer human
// trust flows through.
func LoadPatches(fsys fs.FS) ([]*derivepb.Patch, error) {
	var out []*derivepb.Patch
	err := walkTextprotos(fsys, func(path string, data []byte) error {
		p := &derivepb.Patch{}
		if err := prototext.Unmarshal(data, p); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		switch {
		case p.Name == "":
			return fmt.Errorf("%s: patch has no name", path)
		case p.DocContentHash == "" || p.TableContentHash == "":
			return fmt.Errorf("%s: patch %s must pin doc and table content hashes", path, p.Name)
		case p.Note == "":
			return fmt.Errorf("%s: patch %s has no note (an unexplained correction cannot be reviewed)", path, p.Name)
		}
		out = append(out, p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// walkTextprotos visits every *.textproto under fsys in walk order.
func walkTextprotos(fsys fs.FS, visit func(path string, data []byte) error) error {
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".textproto") {
			return nil
		}
		f, err := fsys.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			return err
		}
		return visit(path, data)
	})
}

// MarshalManifest renders a RunManifest as textproto for storage beside the derived
// PartSpec (the lockfile of the run).
func MarshalManifest(m *derivepb.RunManifest) ([]byte, error) {
	return prototext.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(m)
}

// MarshalSpec renders a derived PartSpec as textproto, the same hand-diffable form
// the fixture corpus uses.
func MarshalSpec(s *parampb.PartSpec) ([]byte, error) {
	return prototext.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(proto.Message(s))
}
