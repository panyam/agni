// Command viewer-agent answers a reader's questions in their agni viewer tab (agni issue 1907). It
// stands in for an LLM agent with its thinking scripted, so the same run shows the same thing every
// time. Start `agni serve --mcp --viewer-rooms`, open a design, turn on "Let agents drive this tab",
// and run
//
//	go run . --server http://127.0.0.1:8080 --design kicad/hier_numeric_root.kicad_sch
//
// Then ask anything in the tab's Ask box. The agent shows the design's parts under the box and
// proposes highlighting the first. Accept it and the agent highlights the part. README.md shows the
// same loop driven by Claude Code.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/panyam/mcpkit/client"
	"github.com/panyam/mcpkit/core"
)

func main() {
	server := flag.String("server", "http://127.0.0.1:8080", "the agni server, as `agni serve --mcp --viewer-rooms` prints it")
	design := flag.String("design", "", "the design the tab shows, as mount/path (required)")
	token := flag.String("token", os.Getenv("AGNI_TOKEN"), "a bearer token for a server run with --auth, as `agni login` mints it (default $AGNI_TOKEN)")
	once := flag.Bool("once", false, "exit once the first question's proposal is answered")
	flag.Parse()
	if *design == "" {
		fmt.Fprintln(os.Stderr, "viewer-agent: --design is required, as mount/path")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, *server, *design, *token, *once); err != nil && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, "viewer-agent:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, server, design, token string, once bool) error {
	var opts []client.ClientOption
	if token != "" {
		opts = append(opts, client.WithClientBearerToken(token))
	}
	c := client.NewClient(strings.TrimSuffix(server, "/")+"/mcp", core.Implementation{Name: "viewer-agent", Version: "0"}, opts...)
	if err := c.Connect(ctx); err != nil {
		return fmt.Errorf("connecting to %s/mcp: %w", server, err)
	}
	defer c.Close()
	a := NewAgent(c, design, os.Stdout)
	if err := a.Listen(ctx); err != nil {
		return err
	}
	fmt.Printf("listening for questions on %s\n", design)
	return a.Run(ctx, once)
}
