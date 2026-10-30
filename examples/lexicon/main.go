// Command lexicon is the naming-lexicon rung of the Agni examples ladder. It shows the vocabulary a
// design's names are read with and where each pattern came from, explains one net's roles, and says
// what taking the design's convention away would change. It is the walkthrough form of
// `agni lexicon`. The narration lives in the sidecar walkthrough.md (demokit FromMarkdown), and this
// file only binds the steps that run engine code.
//
// Run modes (see the Makefile): `make run` (plain text), `make demo` (TUI boxes),
// `make runquiet` (non-interactive defaults, CI-safe), `make doc` (render to markdown).
package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/demokit"

	agni "github.com/panyam/agni"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"

	"github.com/panyam/agni/examples/common"
	"github.com/panyam/agni/fshost"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/service"

	_ "github.com/panyam/agni/stdlib/relations"     // the fact base every datalog rule reads
	_ "github.com/panyam/agni/stdlib/rules/builtin" // register the built-in rule catalog the try step runs
)

//go:embed walkthrough.md
var walkthroughMD []byte

// The bundled fixture and the net the walkthrough explains. They are package-level because
// walkthrough.md states what they do and lexicon_test.go holds those claims against the fixture.
const (
	defaultDesign = "../common/designs/lexicon-demo"
	defaultNet    = "SENSOR_3V3"
)

func main() {
	design := common.AskPath("design", defaultDesign)
	net := defaultNet

	demo := demokit.New("lexicon").
		Dir("lexicon").
		FromMarkdownBytes(walkthroughMD)

	demo.Bind("design").Input(design.Def()).Run(func(ctx demokit.StepContext) *demokit.StepResult {
		design.Capture(ctx)
		fmt.Printf("Design: %s\n", design.Path())
		return nil
	})

	demo.Bind("lexicon").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		s, err := open(design.Path())
		if err != nil {
			return demokit.Errf("%v", err)
		}
		r, err := s.lex.GetLexicon(context.Background(), &webapi.GetLexiconRequest{Snapshot: s.snap})
		if err != nil {
			return demokit.Errf("%v", err)
		}
		fmt.Print(lexiconLines(r))
		return nil
	})

	demo.Bind("net").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		if v, ok := ctx.Inputs["net"].(string); ok && v != "" {
			net = v
		}
		s, err := open(design.Path())
		if err != nil {
			return demokit.Errf("%v", err)
		}
		r, err := s.lex.ExplainNet(context.Background(), &webapi.ExplainNetRequest{Snapshot: s.snap, Net: net})
		if err != nil {
			return demokit.Errf("%v", err)
		}
		fmt.Print(explainLines(r))
		return nil
	})

	demo.Bind("try").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		s, err := open(design.Path())
		if err != nil {
			return demokit.Errf("%v", err)
		}
		r, err := s.lex.CompareLexicons(context.Background(), &webapi.CompareLexiconsRequest{Snapshot: s.snap, Conventions: withoutLexicon})
		if err != nil {
			return demokit.Errf("%v", err)
		}
		fmt.Print(changeLines(r))
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}

// withoutLexicon is the candidate the try step compares against: a convention stating no lexicon,
// which reads the design's names with the built-in vocabularies alone.
var withoutLexicon = &configpb.NamingConvention{Name: "built-ins-only"}

// session is one design ingested into the services `agni serve` runs, over the folder holding it.
type session struct {
	lex  *service.LexiconService
	snap string
}

// open serves the folder holding path as the mount "ex" and ingests the design, the way a server
// does for a design under one of its mounts. The services read through fshost, the browser engine's
// file host, so nothing here opens a file by path.
func open(path string) (*session, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	m := fshost.Mount{Name: "ex", FS: os.DirFS(filepath.Dir(abs))}
	eng, err := agni.New()
	if err != nil {
		return nil, err
	}
	host := fshost.New(m)
	resolver := &service.ProjectResolver{
		Store:  projects.NewFSStore(projects.Tree{Mount: m.Name, FS: m.FS}),
		Config: &fshost.ConfigResolver{Mounts: []fshost.Mount{m}},
	}
	checks := service.NewCheckService(host, eng.Catalog(), nil, "", host, resolver)
	snap, err := checks.Snapshots().Ingest(context.Background(), &webapi.IngestRequest{
		Snapshot: &webapi.Snapshot{Uri: "mount://ex/" + filepath.Base(abs)},
	})
	if err != nil {
		return nil, err
	}
	return &session{lex: service.NewLexiconService(checks), snap: snap.GetName()}, nil
}

func sourceOf(e *webapi.LexiconEntry) string {
	s := strings.ToLower(strings.TrimPrefix(e.GetSource().String(), "LEXICON_SOURCE_"))
	if e.GetConvention() != "" {
		s += " (" + e.GetConvention() + ")"
	}
	return s
}

// lexiconLines counts the entries by where they came from, then lists the rail vocabulary, which is
// the one this design's convention adds to.
func lexiconLines(r *webapi.GetLexiconResponse) string {
	var b strings.Builder
	counts := map[string]int{}
	var order []string
	for _, e := range r.GetEntries() {
		s := sourceOf(e)
		if counts[s] == 0 {
			order = append(order, s)
		}
		counts[s]++
	}
	for _, s := range order {
		fmt.Fprintf(&b, "%3d from %s\n", counts[s], s)
	}
	b.WriteString("net.rail:\n")
	for _, e := range r.GetEntries() {
		if e.GetVocabulary() == "net.rail" {
			fmt.Fprintf(&b, "  %-44s %s\n", e.GetValue(), sourceOf(e))
		}
	}
	return b.String()
}

func explainLines(r *webapi.ExplainNetResponse) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", r.GetNet())
	if len(r.GetRoles()) == 0 {
		b.WriteString("  no roles\n")
	}
	for _, role := range r.GetRoles() {
		fmt.Fprintf(&b, "  %s: %s\n", strings.ToLower(strings.TrimPrefix(role.GetRoleKind().String(), "ROLE_")), role.GetReason())
	}
	for _, e := range r.GetMatches() {
		fmt.Fprintf(&b, "  matches %s %s %s, from %s\n", e.GetVocabulary(), e.GetKind(), e.GetValue(), sourceOf(e))
	}
	return b.String()
}

func changeLines(r *webapi.CompareLexiconsResponse) string {
	var b strings.Builder
	for _, t := range r.GetTables() {
		fmt.Fprintf(&b, "%s: %d\n", t.GetName(), len(t.GetRows()))
		for _, row := range t.GetRows() {
			fmt.Fprintf(&b, "  %s\n", strings.Join(row.GetCells(), " | "))
		}
	}
	return b.String()
}
