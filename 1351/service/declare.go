package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"gopkg.in/yaml.v3"
)

// DeclareCompanion answers the design.yaml that declares companion_uri as a companion of the design
// whose entry is design_uri (agni issue 1367). It writes nothing; the page saves the text where it
// can and shows it where it cannot.
//
// The companion must be in the design's mount and inside its folder, since a descriptor's companions
// stay inside it, and either is an invalid argument otherwise. An existing descriptor is edited as a
// YAML node tree, so its comments and other keys are kept, and a new one names the entry and extends
// the nearest project.yaml above the folder, since a design under a project that names none is a
// load error. internal/projects reads descriptors and imports this package, so the tests hold the
// answer to its parser (TestDeclaredDescriptorsLoad).
func (s *WorkspaceService) DeclareCompanion(ctx context.Context, req *webapi.DeclareCompanionRequest) (*webapi.DeclareCompanionResponse, error) {
	if s.files == nil {
		return nil, fmt.Errorf("%w: this host does not read design files", ErrInvalidArgument)
	}
	design, err := ParseArtifactURI(req.GetDesignUri())
	if err != nil {
		return nil, err
	}
	companion, err := ParseArtifactURI(req.GetCompanionUri())
	if err != nil {
		return nil, err
	}
	if companion.Mount != design.Mount {
		return nil, fmt.Errorf("%w: %s is in another mount from the design, so its design.yaml cannot name it", ErrInvalidArgument, companion)
	}
	folder := parentPath(design.Path)
	rel, ok := relInside(folder, companion.Path)
	if !ok {
		return nil, fmt.Errorf("%w: %s is outside the design's folder %q, and a design.yaml declares only what sits inside it; move it in, or keep it attached to the page", ErrInvalidArgument, companion.Path, folder)
	}
	descriptor, err := artifact.New(design.Mount, joinPath(folder, DesignDescriptorName))
	if err != nil {
		return nil, err
	}
	old, readErr := s.files.ReadFile(ctx, descriptor)
	existed := readErr == nil
	var text []byte
	if existed {
		if text, err = addCompanion(old, rel); err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrInvalidArgument, descriptor, err)
		}
	} else {
		text = s.newDescriptor(ctx, design, folder, rel)
	}
	return &webapi.DeclareCompanionResponse{
		DescriptorUri: descriptor.String(),
		DesignYaml:    string(text),
		Existed:       existed,
		Changed:       !existed || !bytes.Equal(old, text),
	}, nil
}

// newDescriptor is a design.yaml naming the design as ProposeDesigns would, its entry and the companion, and extending the nearest
// project.yaml above the folder when there is one.
func (s *WorkspaceService) newDescriptor(ctx context.Context, design artifact.URI, folder, rel string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "name: %s\n", yamlScalar(designID(stemOf(design.Path))))
	if p := s.projectAbove(ctx, design.Mount, folder); p != "" {
		fmt.Fprintf(&b, "extends: %s\n", yamlScalar(p))
	}
	fmt.Fprintf(&b, "entry: %s\ncompanions:\n  - %s\n", yamlScalar(path.Base(design.Path)), yamlScalar(rel))
	return []byte(b.String())
}

// projectAbove is the path from folder to the nearest project.yaml at or above it, "" when none is
// there. It reads as ProposeDesigns' extendsFor does.
func (s *WorkspaceService) projectAbove(ctx context.Context, mount, folder string) string {
	up := ""
	for dir := folder; ; dir = parentPath(dir) {
		u, err := artifact.New(mount, joinPath(dir, ProjectDescriptorName))
		if err != nil {
			return ""
		}
		if _, err := s.files.ReadFile(ctx, u); err == nil {
			return up + ProjectDescriptorName
		}
		if dir == "" {
			return ""
		}
		up += "../"
	}
}

// addCompanion adds rel to a descriptor's companions, keeping the rest of the document, and returns
// the document unchanged when it lists rel already.
func addCompanion(doc []byte, rel string) ([]byte, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(doc, &root); err != nil {
		return nil, err
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("the descriptor is not a mapping")
	}
	m := root.Content[0]
	var seq *yaml.Node
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == "companions" {
			seq = m.Content[i+1]
		}
	}
	if seq == nil {
		seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "companions"}, seq)
	}
	if seq.Kind != yaml.SequenceNode {
		return nil, errors.New("companions is not a list")
	}
	for _, c := range seq.Content {
		if path.Clean(strings.TrimSpace(c.Value)) == rel {
			return doc, nil
		}
	}
	seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: rel})
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// relInside is target's path relative to folder when target sits inside it.
func relInside(folder, target string) (string, bool) {
	if folder == "" {
		return target, target != ""
	}
	if !strings.HasPrefix(target, folder+"/") {
		return "", false
	}
	return strings.TrimPrefix(target, folder+"/"), true
}
