// Command ask-over-mcp asks a design questions the way an LLM client does, through agni's MCP
// server (agni issue 910). It starts `agni mcp` over a folder, connects mcpkit's client to it, and
// makes the calls a model would make, printing what the model reads back: the question, the rows
// with what they rest on, and why there is no viewer link. The narration lives in the sidecar
// walkthrough.md (demokit FromMarkdown), and this file only binds the steps that run code.
//
// Run modes (see the Makefile): `make run` (plain text), `make demo` (TUI boxes),
// `make runquiet` (non-interactive defaults, CI-safe), `make doc` (render to markdown).
// `ask-over-mcp exercise <spec.yaml>` instead runs mission #909's exercise (exercise.go).
package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/panyam/demokit"

	"github.com/panyam/agni/examples/common"
)

//go:embed walkthrough.md
var walkthroughMD []byte

// The folder served and the design asked about by default, package-level because walkthrough.md
// states what they hold and ask_test.go holds those claims against them.
const (
	defaultFolder = "../tutorial-project"
	defaultDesign = "designs/gateway"
)

// questions are the calls the walkthrough makes after connecting, in order, by step id.
var questions = []struct {
	step, tool string
	args       map[string]any
}{
	{"designs", "designs", map[string]any{}},
	{"look", "look", map[string]any{"name": "net.has_test_point"}},
	{"query", "query", map[string]any{"program": `net.rail(?net), not net.has_test_point(?net) => ?net`}},
	{"trace", "trace", map[string]any{"from": "J1.1", "to": "U1.1"}},
	{"diff", "diff", map[string]any{"design": mountName + "/" + defaultDesign + "/gateway.edn", "against": mountName + "/" + defaultDesign + "/gateway-rev-b.edn"}},
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "exercise" {
		if err := exercise(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "exercise:", err)
			os.Exit(1)
		}
		return
	}
	folder := common.AskPath("folder", defaultFolder)
	ctx := context.Background()
	var s *session

	demo := demokit.New("ask-over-mcp").
		Dir("ask-over-mcp").
		FromMarkdownBytes(walkthroughMD)

	demo.Bind("connect").Input(folder.Def()).Run(func(sc demokit.StepContext) *demokit.StepResult {
		folder.Capture(sc)
		root, design, err := mountFor(folder.Path(), defaultDesign)
		if err != nil {
			return demokit.Errf("%v", err)
		}
		agni := agniBinary()
		if s, err = connect(ctx, agni, root, design); err != nil {
			return demokit.Errf("%v", err)
		}
		tools, err := s.tools(ctx)
		if err != nil {
			return demokit.Errf("%v", err)
		}
		fmt.Printf("Started %s mcp over %s, asking about %s\n", agni, root, design)
		fmt.Printf("Tools: %s\n", strings.Join(tools, ", "))
		return nil
	})
	for _, q := range questions {
		demo.Bind(q.step).Run(func(demokit.StepContext) *demokit.StepResult {
			if s == nil {
				return demokit.Errf("not connected")
			}
			text, err := s.ask(ctx, q.tool, q.args)
			if err != nil {
				return demokit.Errf("%v", err)
			}
			fmt.Print(text)
			return nil
		})
	}

	common.SetupRenderer(demo)
	demo.Execute()
	if s != nil {
		s.close()
	}
}
