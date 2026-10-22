package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/panyam/mcpkit/client"
	"github.com/panyam/mcpkit/core"
	"google.golang.org/protobuf/encoding/protojson"
	"gopkg.in/yaml.v3"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// The exercise asks mission #909's five questions of one design through an MCP client, the way a model
// would, and checks every reply (agni issue 1121). A spec under exercise/ names the folder, the design
// and the calls each question takes. Every reply must state the question it answers before its rows
// and end in a viewer link (or, over stdio, the reason it has none); its rows must equal what the CLI
// answers to the same request, encoded through the same csv encoder; and the strings a spec pins must
// be in it, so a wrong question fails even when the two transports agree.

// spec is one exercise file.
type spec struct {
	Folder    string     `yaml:"folder"`
	Design    string     `yaml:"design"`
	Revision  string     `yaml:"revision"`
	Questions []question `yaml:"questions"`
}

type question struct {
	Ask   string `yaml:"ask"`
	Calls []call `yaml:"calls"`
}

// A call is one tool call and how to check its reply. Args and CLI may say {design} and {revision},
// which become the design's and its revision's mount URIs.
type call struct {
	Tool string         `yaml:"tool"`
	Args map[string]any `yaml:"args"`
	// CLI is the `agni` command asking the same thing, run with the exercise's mount.
	CLI []string `yaml:"cli"`
	// Table names the reply's table the CLI's csv must equal; empty compares the trace instead.
	Table string `yaml:"table"`
	// Has is text the reply must contain.
	Has []string `yaml:"has"`
	// Fails says the reply is a tool error and the CLI exits non-zero, a question agni can't answer.
	Fails bool `yaml:"fails"`
}

// outcome is one call's line of the report.
type outcome struct {
	question, tool, rows, link string
	took                       time.Duration
	problems                   []string
}

// exercise runs the spec at specPath, writing a markdown report to out, and fails when any reply did.
func exercise(args []string) error {
	fs := flag.NewFlagSet("exercise", flag.ContinueOnError)
	transport := fs.String("transport", "http", "http starts `agni serve --mcp` and checks each link opens; stdio starts `agni mcp`")
	out := fs.String("out", "", "also write the report as markdown to this file")
	webDir := fs.String("web-dir", filepath.Join("..", "..", "web"), "the built viewer `agni serve` links to over http (make ui)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: ask-over-mcp exercise [--transport http|stdio] [--out report.md] <spec.yaml>")
	}
	report, err := runExercise(context.Background(), agniBinary(), fs.Arg(0), *transport, *webDir)
	if *out != "" && report != "" {
		if werr := os.WriteFile(*out, []byte(report), 0o600); werr != nil && err == nil {
			err = werr
		}
	}
	fmt.Print(report)
	return err
}

// runExercise asks every question of specPath and returns the report, with an error naming how many
// calls failed when any did.
func runExercise(ctx context.Context, agni, specPath, transport, webDir string) (string, error) {
	b, err := os.ReadFile(specPath) // #nosec G304 -- the spec is the caller's argument
	if err != nil {
		return "", err
	}
	var sp spec
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&sp); err != nil {
		return "", fmt.Errorf("%s: %w", specPath, err)
	}
	if len(sp.Questions) == 0 {
		return "", fmt.Errorf("%s asks nothing", specPath)
	}
	root, err := filepath.Abs(filepath.Join(filepath.Dir(specPath), sp.Folder))
	if err != nil {
		return "", err
	}
	vars := map[string]string{
		"{design}":   "mount://" + mountName + "/" + strings.Trim(sp.Design, "/"),
		"{revision}": "mount://" + mountName + "/" + strings.Trim(sp.Revision, "/"),
	}
	c, stop, err := startServer(ctx, agni, root, transport, webDir)
	if err != nil {
		return "", err
	}
	defer stop()

	var outs []outcome
	for _, q := range sp.Questions {
		for _, cl := range q.Calls {
			outs = append(outs, ask(ctx, c, agni, root, transport, vars, q.Ask, cl))
		}
	}
	return render(sp, transport, outs)
}

// startServer starts agni's MCP server over root and connects a client to it.
func startServer(ctx context.Context, agni, root, transport, webDir string) (*client.Client, func(), error) {
	impl := core.Implementation{Name: "ask-over-mcp-exercise", Version: "0"}
	mount := mountName + "=" + root
	if transport == "stdio" {
		c := client.NewClient("", impl, client.WithCommandTransport(agni, []string{"mcp", "--mount", mount}))
		if err := c.Connect(ctx); err != nil {
			return nil, nil, err
		}
		return c, func() { _ = c.Close() }, nil
	}
	web, err := filepath.Abs(webDir)
	if err != nil {
		return nil, nil, err
	}
	cmd := exec.CommandContext(ctx, agni, "serve", "--mcp", "--addr", "127.0.0.1:0", "--mount", mount, "--web-dir", web) // #nosec G204 -- agni is this repo's binary
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	stop := func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }
	url, err := mcpURL(stderr)
	if err != nil {
		stop()
		return nil, nil, err
	}
	c := client.NewClient(url, impl)
	if err := c.Connect(ctx); err != nil {
		stop()
		return nil, nil, err
	}
	return c, func() { _ = c.Close(); stop() }, nil
}

var mcpLine = regexp.MustCompile(`MCP: (http://\S+/mcp) `)

// mcpURL reads the server's banner until it names its MCP endpoint, and drains the rest so the server
// never blocks on a full pipe.
func mcpURL(r io.Reader) (string, error) {
	sc := bufio.NewScanner(r)
	var seen []string
	for sc.Scan() {
		if m := mcpLine.FindStringSubmatch(sc.Text()); m != nil {
			go func() { _, _ = io.Copy(io.Discard, r) }()
			return m[1], nil
		}
		seen = append(seen, sc.Text())
	}
	return "", fmt.Errorf("agni serve named no MCP endpoint; it said:\n%s", strings.Join(seen, "\n"))
}

// ask makes one call and checks its reply.
func ask(ctx context.Context, c *client.Client, agni, root, transport string, vars map[string]string, q string, cl call) outcome {
	o := outcome{question: q, tool: cl.Tool}
	args := fill(cl.Args, vars).(map[string]any)
	start := time.Now()
	res, err := c.ToolCallFull(ctx, cl.Tool, args)
	o.took = time.Since(start)
	if err != nil {
		o.problems = append(o.problems, "call failed: "+err.Error())
		return o
	}
	var text strings.Builder
	for _, x := range res.Content {
		text.WriteString(x.Text)
	}
	reply := text.String()
	if res.IsError != cl.Fails {
		o.problems = append(o.problems, fmt.Sprintf("tool error %v, want %v: %s", res.IsError, cl.Fails, firstLine(reply)))
	}
	for _, h := range cl.Has {
		if !strings.Contains(reply, h) {
			o.problems = append(o.problems, fmt.Sprintf("the reply lacks %q", h))
		}
	}
	if !cl.Fails && !strings.HasPrefix(reply, "design: ") && !strings.HasPrefix(reply, "diff: ") {
		o.problems = append(o.problems, "the reply doesn't open with the design and the question")
	}
	o.link = checkLink(reply, transport, &o.problems)

	data := map[string]any{}
	if b, err := json.Marshal(res.StructuredContent); err == nil {
		_ = json.Unmarshal(b, &data)
	}
	cliOut, cliErr := runCLI(agni, root, fill(toAny(cl.CLI), vars).([]any))
	switch {
	case cl.Fails:
		if cliErr == nil {
			o.problems = append(o.problems, "the CLI answered a question the tool refused")
		}
		o.rows = "refused"
	case cliErr != nil:
		o.problems = append(o.problems, "the CLI failed: "+cliErr.Error())
	case cl.Table != "":
		o.rows = compareTable(data, cl.Table, cliOut, &o.problems)
	default:
		o.rows = compareTrace(data, cliOut, &o.problems)
	}
	return o
}

var viewerLink = regexp.MustCompile(`in the viewer: (\S+)`)

// checkLink finds the reply's viewer links and, over HTTP, checks each opens. Over stdio there is no
// viewer, so the reply must say why it has no link instead.
func checkLink(reply, transport string, problems *[]string) string {
	links := viewerLink.FindAllStringSubmatch(reply, -1)
	switch {
	case len(links) == 0 && strings.Contains(reply, "no viewer link: "):
		if transport == "http" {
			*problems = append(*problems, "a served reply has no link: "+firstLine(reply[strings.Index(reply, "no viewer link: "):]))
		}
		return "none, says why"
	case len(links) == 0:
		*problems = append(*problems, "the reply has no link and says nothing about one")
		return "missing"
	case transport != "http":
		return fmt.Sprintf("%d", len(links))
	}
	for _, m := range links {
		resp, err := http.Get(m[1]) // #nosec G107 -- a link the local server just minted
		if err != nil {
			*problems = append(*problems, "a link doesn't open: "+err.Error())
			return "broken"
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			*problems = append(*problems, fmt.Sprintf("%s answers %s", m[1], resp.Status))
			return "broken"
		}
	}
	if len(links) == 1 {
		return "opens"
	}
	return fmt.Sprintf("%d, all open", len(links))
}

// compareTable encodes the reply's table named name as the CLI encodes csv and compares the two.
func compareTable(data map[string]any, name, cliOut string, problems *[]string) string {
	tables, _ := data["tables"].([]any)
	for _, t := range tables {
		b, err := json.Marshal(t)
		if err != nil {
			continue
		}
		var tab webapi.Table
		if err := protojson.Unmarshal(b, &tab); err != nil || tab.GetName() != name {
			continue
		}
		var csv bytes.Buffer
		if err := service.WriteTableCSV(&csv, &tab); err != nil {
			*problems = append(*problems, err.Error())
			return "?"
		}
		if csv.String() != cliOut {
			*problems = append(*problems, fmt.Sprintf("the %s rows differ from the CLI's:\nMCP:\n%s\nCLI:\n%s", name, csv.String(), cliOut))
		}
		return fmt.Sprintf("%d, as the CLI", len(tab.GetRows()))
	}
	*problems = append(*problems, fmt.Sprintf("the reply has no %s table", name))
	return "?"
}

// compareTrace compares the reply's trace with `agni trace --format json`, as parsed JSON, since
// protojson's whitespace is not stable.
func compareTrace(data map[string]any, cliOut string, problems *[]string) string {
	answer, _ := data["answer"].(map[string]any)
	var cli any
	if err := json.Unmarshal([]byte(cliOut), &cli); err != nil {
		*problems = append(*problems, "the CLI's trace is not JSON: "+err.Error())
		return "?"
	}
	if !reflect.DeepEqual(answer["trace"], cli) {
		*problems = append(*problems, "the trace differs from the CLI's")
		return "?"
	}
	nets, _ := answer["trace"].(map[string]any)["nets"].([]any)
	return fmt.Sprintf("%d net(s), as the CLI", len(nets))
}

func runCLI(agni, root string, args []any) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("the spec names no CLI command to compare with")
	}
	argv := []string{fmt.Sprint(args[0]), "--mount", mountName + "=" + root}
	for _, a := range args[1:] {
		argv = append(argv, fmt.Sprint(a))
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(agni, argv...) // #nosec G204 -- agni is this repo's binary, the args the spec's
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("agni %s: %v: %s", strings.Join(argv, " "), err, firstLine(stderr.String()))
	}
	return stdout.String(), nil
}

// fill replaces {design} and {revision} in every string of v.
func fill(v any, vars map[string]string) any {
	switch t := v.(type) {
	case string:
		for k, val := range vars {
			t = strings.ReplaceAll(t, k, val)
		}
		return t
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = fill(e, vars)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, e := range t {
			out[k] = fill(e, vars)
		}
		return out
	case nil:
		return map[string]any{}
	}
	return v
}

func toAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return s
}

// render is the report, one row per call in the mission log's table form, and an error when any call
// failed.
func render(sp spec, transport string, outs []outcome) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "## Asking %s over MCP (%s)\n\n", sp.Design, transport)
	b.WriteString("| question | call | rows | link | time | result |\n| --- | --- | --- | --- | --- | --- |\n")
	failed := 0
	for _, o := range outs {
		result := "ok"
		if len(o.problems) > 0 {
			failed++
			result = "FAILED"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", o.question, o.tool, o.rows, o.link, o.took.Round(time.Millisecond), result)
	}
	for _, o := range outs {
		for _, p := range o.problems {
			fmt.Fprintf(&b, "\n- %s (%s): %s", o.question, o.tool, p)
		}
	}
	b.WriteString("\n")
	if failed > 0 {
		return b.String(), fmt.Errorf("%d of %d calls failed", failed, len(outs))
	}
	return b.String(), nil
}
