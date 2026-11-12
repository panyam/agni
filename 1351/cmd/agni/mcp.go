package main

import (
	"strings"

	"github.com/panyam/agni/internal/server"
	"github.com/spf13/cobra"
)

// mcpCmd serves the same MCP tools as `serve --mcp`, over stdio, for a client that starts its server
// itself (agni issue 910). It composes the services exactly as serve does, through runViewer, and
// serves them on stdin and stdout in place of a listener.
func mcpCmd() *cobra.Command {
	var serverURL, paramsDir, paramsURL, profilePath, conventions string
	var queryBudget int64
	c := &cobra.Command{
		Use:   "mcp",
		Short: "Serve agni's analysis to an LLM client over the Model Context Protocol, on stdio",
		Long: "mcp serves queries, checks, traces, diffs and reviews as MCP tools on stdin and stdout, for a\n" +
			"client such as Claude Code that starts the server itself. It reads the designs under --mount\n" +
			"(or the mounts in agni.yaml), as serve does. `agni serve --mcp` serves the same tools over HTTP\n" +
			"beside the viewer. With --server naming a running agni serve that mounts the same names, each\n" +
			"answer links to it in the viewer.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runViewer(cmd, viewerOpts{
				mcp: mcpStdio, mcpLinks: strings.TrimSuffix(serverURL, "/"),
				paramsDir: paramsDir, paramsURL: paramsURL, profilePath: profilePath, conventions: conventions,
				budget:      server.Budget{Enforce: queryBudget},
				designCache: defaultDesignCache,
			})
		},
	}
	c.Flags().StringVar(&serverURL, "server", "", "a running `agni serve` (http://localhost:8080) that mounts the same names, for each answer to link to in the viewer; empty withholds links and each answer says so")
	c.Flags().StringVar(&paramsDir, "params", "", "directory of seeded PartSpec textprotos, as serve takes it")
	c.Flags().StringVar(&paramsURL, "params-url", "", "a PartSpecService to read published PartSpecs from, as serve takes it")
	c.Flags().StringVar(&profilePath, "profile-path", "", "directory of YAML interface-profile declarations, as serve takes it")
	c.Flags().StringVar(&conventions, "conventions", "", "an operator naming-convention config (YAML), as serve takes it")
	c.Flags().Int64Var(&queryBudget, "query-budget", 0, "cap each query's work, as serve's --query-budget does; 0 enforces nothing")
	return c
}
