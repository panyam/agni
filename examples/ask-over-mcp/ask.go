package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/mcpkit/client"
	"github.com/panyam/mcpkit/core"
)

// mountName is the mount the example serves its folder under, so the design names it as ex/<path>.
const mountName = "ex"

// A session is an MCP client connected to `agni mcp`, which it started over the folder holding the
// design.
type session struct {
	c      *client.Client
	design string
}

// agniBinary is the agni the example runs: AGNI_BIN when set, then this repo's bin/agni when it was
// built, then whatever agni is on PATH. The repo's own binary comes before PATH because an older
// installed agni would answer with yesterday's tools.
func agniBinary() string {
	if b := strings.TrimSpace(os.Getenv("AGNI_BIN")); b != "" {
		return b
	}
	if b, err := filepath.Abs(filepath.Join("..", "..", "bin", "agni")); err == nil {
		if _, err := os.Stat(b); err == nil {
			return b
		}
	}
	return "agni"
}

// mountFor is the folder to serve and the design within it, as the tools name it. A folder is served
// whole with the design at design within it, and a file is served from its own folder.
func mountFor(path, design string) (root, name string, err error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", "", err
	}
	if !info.IsDir() {
		return filepath.Dir(abs), mountName + "/" + filepath.Base(abs), nil
	}
	return abs, mountName + "/" + strings.Trim(design, "/"), nil
}

// connect starts `agni mcp` over root and connects to it.
func connect(ctx context.Context, agni, root, design string) (*session, error) {
	c := client.NewClient("", core.Implementation{Name: "ask-over-mcp", Version: "0"},
		client.WithCommandTransport(agni, []string{"mcp", "--mount", mountName + "=" + root}))
	if err := c.Connect(ctx); err != nil {
		return nil, fmt.Errorf("starting %s mcp: %w", agni, err)
	}
	return &session{c: c, design: design}, nil
}

func (s *session) close() { _ = s.c.Close() }

// tools is the names of the tools the server offers.
func (s *session) tools(ctx context.Context) ([]string, error) {
	defs, err := s.c.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, d := range defs {
		names = append(names, d.Name)
	}
	return names, nil
}

// ask calls tool with args, adding the design when the tool takes one and the caller didn't name
// it, and answers the text an agent reads. A tool error is an error carrying that text.
func (s *session) ask(ctx context.Context, tool string, args map[string]any) (string, error) {
	if _, ok := args["design"]; !ok && tool != "designs" {
		args["design"] = s.design
	}
	res, err := s.c.ToolCallFull(ctx, tool, args)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, c := range res.Content {
		b.WriteString(c.Text)
	}
	if res.IsError {
		return "", fmt.Errorf("%s: %s", tool, b.String())
	}
	return b.String(), nil
}
