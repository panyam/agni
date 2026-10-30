package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/check/naming"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
)

// lexiconAPI is what `agni lexicon` asks, in process or of a --remote server.
type lexiconAPI interface {
	GetLexicon(context.Context, *webapi.GetLexiconRequest) (*webapi.GetLexiconResponse, error)
	ExplainNet(context.Context, *webapi.ExplainNetRequest) (*webapi.ExplainNetResponse, error)
	CompareLexicons(context.Context, *webapi.CompareLexiconsRequest) (*webapi.CompareLexiconsResponse, error)
}

func lexiconCmd() *cobra.Command {
	var netName, tryPath, format, remoteURL, outPath string
	cmd := &cobra.Command{
		Use:   "lexicon <design>",
		Short: "Show the naming vocabulary a design is read with, why a net has its roles, and what a convention would change",
		Long: "lexicon prints the effective naming lexicon of a design's read: every pattern of every vocabulary, " +
			"and where it came from (built-in, the server's --conventions, the project, the design, or the request). " +
			"--net explains one net's roles, each with its reason, and lists every pattern its name matches, " +
			"exclusions included. --try reads the design again with a candidate conventions file in place of the " +
			"one it uses, runs the checks under both, and lists the nets whose roles change and the findings " +
			"that appear or disappear (agni issue 1266).",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if netName != "" && tryPath != "" {
				return fmt.Errorf("--net and --try ask different questions; pass one")
			}
			switch {
			case format == "yaml" && (netName != "" || tryPath != ""):
				return fmt.Errorf("--format yaml writes the lexicon itself, so it takes neither --net nor --try")
			case format != "text" && format != "json" && format != "yaml":
				return fmt.Errorf("--format %s: want text, json or yaml", format)
			}
			var candidate *configpb.NamingConvention
			if tryPath != "" {
				c, err := naming.Load(tryPath)
				if err != nil {
					return fmt.Errorf("--try %s: %w", tryPath, err)
				}
				candidate = c
			}
			closeOut, err := redirectOut(cmd, outPath)
			if err != nil {
				return err
			}
			defer closeOut()

			ll := &localLoader{loader: newLoader()}
			checks := service.NewCheckService(ll, check.DefaultCatalog(), nil, "", nil, cliProjects())
			var svc lexiconAPI = service.NewLexiconService(checks)
			var snaps snapshotAPI = checks.Snapshots()
			addr := cliArgURI
			if remoteURL != "" {
				r, err := dialRemote(cmd.Context(), remoteURL)
				if err != nil {
					return err
				}
				svc, snaps, addr = r.lexicon(), r.snapshots(), r.uri
			}
			uri, err := addr(args[0])
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			snap, drop, err := ingest(ctx, snaps, &webapi.Snapshot{Uri: uri})
			if err != nil {
				return rpcFailed(err)
			}
			defer drop()
			out := cmd.OutOrStdout()

			var resp proto.Message
			switch {
			case netName != "":
				r, err := svc.ExplainNet(ctx, &webapi.ExplainNetRequest{Snapshot: snap, Net: netName})
				if err != nil {
					return rpcFailed(err)
				}
				resp = r
				if format == "text" {
					writeExplainNet(out, r)
				}
			case candidate != nil:
				r, err := svc.CompareLexicons(ctx, &webapi.CompareLexiconsRequest{Snapshot: snap, Conventions: candidate})
				if err != nil {
					return rpcFailed(err)
				}
				resp = r
				if format == "text" {
					writeLexiconChanges(out, r)
				}
			default:
				r, err := svc.GetLexicon(ctx, &webapi.GetLexiconRequest{Snapshot: snap})
				if err != nil {
					return rpcFailed(err)
				}
				resp = r
				switch format {
				case "text":
					writeLexicon(out, r)
				case "yaml":
					name := strings.TrimSuffix(filepath.Base(args[0]), filepath.Ext(args[0])) + "-lexicon"
					b, err := lexiconYAML(name, r.GetEffective())
					if err != nil {
						return err
					}
					_, err = out.Write(b)
					return err
				}
			}
			if format == "json" {
				// protojson of the wire message (C31).
				b, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(resp)
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(b))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&netName, "net", "", "explain this net's roles, by its name or a hierarchy leaf only one net has")
	cmd.Flags().StringVar(&tryPath, "try", "", "a conventions YAML to compare against the one the design is read with")
	cmd.Flags().StringVar(&format, "format", "text", "text | json (the wire message) | yaml (the effective lexicon as a conventions file a project can carry)")
	remoteFlag(cmd, &remoteURL)
	outFileFlag(cmd, &outPath)
	return cmd
}

// sourceLabel names where an entry came from, with the convention's name when it has one.
func sourceLabel(e *webapi.LexiconEntry) string {
	s := strings.ToLower(strings.TrimPrefix(e.GetSource().String(), "LEXICON_SOURCE_"))
	if s == "builtin" {
		s = "built-in"
	}
	if e.GetConvention() != "" {
		s += " (" + e.GetConvention() + ")"
	}
	if e.GetReplaced() {
		s += ", replaced"
	}
	return s
}

func writeLexicon(w io.Writer, r *webapi.GetLexiconResponse) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	last := ""
	for _, e := range r.GetEntries() {
		if e.GetVocabulary() != last {
			if last != "" {
				fmt.Fprintln(tw)
			}
			fmt.Fprintln(tw, e.GetVocabulary())
			last = e.GetVocabulary()
		}
		fmt.Fprintf(tw, "  %s\t%s\t%s\n", e.GetKind(), e.GetValue(), sourceLabel(e))
	}
	tw.Flush()
}

func roleSourceLabel(r *ir.NetRole) string {
	return strings.ToLower(strings.TrimPrefix(r.GetSource().String(), "ROLE_SOURCE_"))
}

func writeExplainNet(w io.Writer, r *webapi.ExplainNetResponse) {
	fmt.Fprintln(w, r.GetNet())
	if len(r.GetRoles()) == 0 {
		fmt.Fprintln(w, "  no roles")
	}
	for _, role := range r.GetRoles() {
		line := fmt.Sprintf("  %s (%s)", strings.ToLower(strings.TrimPrefix(role.GetRoleKind().String(), "ROLE_")), roleSourceLabel(role))
		if role.GetReason() != "" {
			line += ": " + role.GetReason()
		}
		fmt.Fprintln(w, line)
	}
	fmt.Fprintln(w)
	if len(r.GetMatches()) == 0 {
		fmt.Fprintln(w, "its name matches no net-name pattern")
		return
	}
	fmt.Fprintln(w, "its name matches")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, e := range r.GetMatches() {
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", e.GetVocabulary(), e.GetKind(), e.GetValue(), sourceLabel(e))
	}
	tw.Flush()
}

func writeLexiconChanges(w io.Writer, r *webapi.CompareLexiconsResponse) {
	for i, t := range r.GetTables() {
		if i > 0 {
			fmt.Fprintln(w)
		}
		switch t.GetName() {
		case "role_changes":
			fmt.Fprintf(w, "%d net(s) change roles\n", len(t.GetRows()))
		case "finding_changes":
			fmt.Fprintf(w, "%d finding(s) change\n", len(t.GetRows()))
		}
		if len(t.GetRows()) == 0 {
			continue
		}
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		cols := make([]string, len(t.GetColumns()))
		for j, c := range t.GetColumns() {
			cols[j] = c.GetName()
		}
		fmt.Fprintln(tw, "  "+strings.Join(cols, "\t"))
		for _, row := range t.GetRows() {
			cells := make([]string, len(row.GetCells()))
			for j, c := range row.GetCells() {
				if c == "" {
					c = "-"
				}
				cells[j] = c
			}
			fmt.Fprintln(tw, "  "+strings.Join(cells, "\t"))
		}
		tw.Flush()
	}
}

// lexiconYAML writes lx as a conventions file, field names as the YAML binding reads them, in the
// order the schema declares them, so naming.Parse reads it back to the same lexicon.
func lexiconYAML(name string, lx *configpb.NamingLexicon) ([]byte, error) {
	b, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(&configpb.NamingConvention{Name: name, Lexicon: lx})
	if err != nil {
		return nil, err
	}
	// JSON is YAML, and a yaml.Node keeps the keys in the order protojson wrote them.
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	blockStyle(&doc)
	header := "# The effective naming lexicon, every list stated whole with replace: true, so a design read\n" +
		"# with this file as its conventions is read exactly as this one was. Written by agni lexicon.\n"
	body, err := yaml.Marshal(&doc)
	if err != nil {
		return nil, err
	}
	return append([]byte(header), body...), nil
}

func blockStyle(n *yaml.Node) {
	n.Style &^= yaml.FlowStyle
	for _, c := range n.Content {
		blockStyle(c)
	}
}
