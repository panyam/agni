// Package yamlpb binds a YAML document to a protobuf message, so a config the operator writes as YAML
// keeps the message as its one schema (CONSTRAINTS C26).
//
// The plain route converts YAML to a generic tree, the tree to JSON, and binds the JSON with protojson.
// It loses two things this package keeps. A YAML scalar's type is decided by its spelling rather than
// by the field it fills, so `pin: 9` becomes a JSON number and protojson refuses it for a string field,
// where an author plainly meant the pin named "9". And protojson's errors carry no line, so a typo in a
// two-hundred-row map is reported without saying where.
//
// Decode walks the YAML node tree beside the message descriptor instead. Each scalar is written as the
// JSON type its FIELD wants, every key is checked against the field names with the line it sits on,
// and protojson does the binding, so presence, ranges and nested messages follow protojson's rules.
// Only a field's proto name is accepted as a key, never its lowerCamel JSON name, so one file has one
// spelling. A number or a bool is judged by the tag YAML resolved for the scalar, because yaml.v3
// will otherwise truncate 1.5 into an integer field and read "yes" as true.
package yamlpb

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"gopkg.in/yaml.v3"
)

// Decode binds node, a YAML mapping, into msg. An unknown key, a list where a scalar belongs, or a
// scalar that cannot be the field's type is an error naming the key and its line. A null or empty
// node leaves msg unset.
func Decode(node *yaml.Node, msg proto.Message) error {
	v, err := toJSON(node, msg.ProtoReflect().Descriptor(), "")
	if err != nil {
		return err
	}
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return protojson.Unmarshal(b, msg)
}

func resolve(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	if n != nil && n.Kind == yaml.DocumentNode && len(n.Content) == 1 {
		return resolve(n.Content[0])
	}
	return n
}

func isNull(n *yaml.Node) bool {
	return n == nil || n.Kind == 0 || (n.Kind == yaml.ScalarNode && n.Tag == "!!null")
}

// toJSON converts a mapping node for message md. path is the dotted key path, for errors.
func toJSON(node *yaml.Node, md protoreflect.MessageDescriptor, path string) (map[string]any, error) {
	n := resolve(node)
	if isNull(n) {
		return nil, nil
	}
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("line %d: %s must be a mapping of fields", n.Line, describe(path))
	}
	out := map[string]any{}
	fields := md.Fields()
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		fd := fields.ByName(protoreflect.Name(k.Value))
		if fd == nil {
			return nil, fmt.Errorf("line %d: unknown key %q in %s (known: %s)", k.Line, k.Value, describe(path), known(md))
		}
		if _, dup := out[k.Value]; dup {
			return nil, fmt.Errorf("line %d: key %q appears twice in %s", k.Line, k.Value, describe(path))
		}
		val, err := fieldJSON(v, fd, join(path, k.Value))
		if err != nil {
			return nil, err
		}
		if val != nil {
			out[k.Value] = val
		}
	}
	return out, nil
}

func fieldJSON(node *yaml.Node, fd protoreflect.FieldDescriptor, path string) (any, error) {
	n := resolve(node)
	if isNull(n) {
		return nil, nil
	}
	switch {
	case fd.IsMap():
		if n.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("line %d: %s must be a mapping", n.Line, describe(path))
		}
		out := map[string]any{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := resolve(n.Content[i]), n.Content[i+1]
			if k.Kind != yaml.ScalarNode || k.Tag == "!!null" {
				return nil, fmt.Errorf("line %d: %s takes plain, non-empty keys", k.Line, describe(path))
			}
			if _, dup := out[k.Value]; dup {
				return nil, fmt.Errorf("line %d: %q appears twice in %s", k.Line, k.Value, describe(path))
			}
			val, err := valueJSON(v, fd.MapValue(), join(path, k.Value))
			if err != nil {
				return nil, err
			}
			if val == nil {
				// An entry whose value is empty still exists, which matters to a map of messages.
				val = map[string]any{}
			}
			out[k.Value] = val
		}
		return out, nil
	case fd.IsList():
		if n.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf("line %d: %s must be a list", n.Line, describe(path))
		}
		out := make([]any, 0, len(n.Content))
		for i, e := range n.Content {
			val, err := valueJSON(e, fd, fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return nil, err
			}
			if val == nil {
				return nil, fmt.Errorf("line %d: %s has an empty entry", resolve(e).Line, describe(path))
			}
			out = append(out, val)
		}
		return out, nil
	}
	return valueJSON(n, fd, path)
}

// valueJSON converts one value of fd's kind, ignoring cardinality.
func valueJSON(node *yaml.Node, fd protoreflect.FieldDescriptor, path string) (any, error) {
	n := resolve(node)
	if isNull(n) {
		return nil, nil
	}
	if fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind {
		return toJSON(n, fd.Message(), path)
	}
	if n.Kind != yaml.ScalarNode {
		return nil, fmt.Errorf("line %d: %s must be a single value", n.Line, describe(path))
	}
	switch fd.Kind() {
	case protoreflect.StringKind:
		return n.Value, nil
	case protoreflect.BoolKind:
		var b bool
		if err := n.Decode(&b); err != nil || n.Tag != "!!bool" {
			return nil, fmt.Errorf("line %d: %s must be true or false, not %q", n.Line, describe(path), n.Value)
		}
		return b, nil
	case protoreflect.EnumKind:
		return n.Value, nil
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		var f float64
		if err := n.Decode(&f); err != nil || (n.Tag != "!!int" && n.Tag != "!!float") || math.IsInf(f, 0) || math.IsNaN(f) {
			return nil, fmt.Errorf("line %d: %s must be a number, not %q", n.Line, describe(path), n.Value)
		}
		return json.Number(strconv.FormatFloat(f, 'g', -1, 64)), nil
	case protoreflect.BytesKind:
		return nil, fmt.Errorf("line %d: %s holds bytes, which YAML config does not write", n.Line, describe(path))
	default: // the integer kinds
		var i int64
		if err := n.Decode(&i); err != nil || n.Tag != "!!int" {
			return nil, fmt.Errorf("line %d: %s must be a whole number, not %q", n.Line, describe(path), n.Value)
		}
		return json.Number(strconv.FormatInt(i, 10)), nil
	}
}

func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

func describe(path string) string {
	if path == "" {
		return "the document"
	}
	return fmt.Sprintf("%q", path)
}

func known(md protoreflect.MessageDescriptor) string {
	fs := md.Fields()
	names := make([]string, 0, fs.Len())
	for i := 0; i < fs.Len(); i++ {
		names = append(names, string(fs.Get(i).Name()))
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
