package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/formats"
	"github.com/panyam/agni/validate"
)

// validateCmd is the reader-health smoke over real design files (WS6-007): for every file,
// run the netlist and/or faithful-geometry reader the registry claims for its extension and
// assert the structural invariants in the validate package. This is how "a reader is ready
// when verified against real files" runs over a private corpus the committed tests can
// never see.
func validateCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "validate <file|dir>...",
		Short: "Reader-health smoke: read every design file and check structural invariants",
		Long: "validate reads each file with the reader(s) its extension claims (netlist and/or\n" +
			"faithful geometry) and checks reader-health invariants: the parse succeeds, the\n" +
			"result is structurally non-empty, and placements resolve to symbols. Directories\n" +
			"are walked recursively; files with no reader are skipped in a walk but fail when\n" +
			"named explicitly. Exits non-zero if any file fails. --format json emits the\n" +
			"canonical webapi.ValidateReport wire form.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "text" && format != "json" {
				return fmt.Errorf("unknown --format %q (want: text, json)", format)
			}
			rep, err := buildValidateReport(newLoader(), args)
			if err != nil {
				return err
			}
			if format == "json" {
				b, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(rep)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
			} else {
				writeValidateText(cmd.OutOrStdout(), rep)
			}
			if rep.Failed > 0 {
				cmd.SilenceUsage = true
				return fmt.Errorf("%d of %d file(s) failed validation", rep.Failed, rep.Failed+rep.Passed)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "text", "output format: text | json (protojson of webapi.ValidateReport)")
	return cmd
}

// buildValidateReport expands args (files as named, directories walked recursively in
// sorted order) and validates each. A walk skips extensions no reader claims — the same
// semantics as the file tree greying them out — while an explicitly named file with no
// reader is a failure, since the user asserted it should validate.
func buildValidateReport(l *formats.Loader, args []string) (*webapi.ValidateReport, error) {
	rep := &webapi.ValidateReport{}
	for _, arg := range args {
		fi, err := os.Stat(arg)
		if err != nil {
			return nil, err
		}
		if !fi.IsDir() {
			addValidation(rep, validateFile(l, arg, true))
			continue
		}
		var files []string
		err = filepath.WalkDir(arg, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." && d.Name() != ".." && path != arg {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.IsDir() {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		sort.Strings(files)
		for _, f := range files {
			if formats.ByExt(f) == nil {
				rep.Skipped++
				continue
			}
			addValidation(rep, validateFile(l, f, false))
		}
	}
	return rep, nil
}

func addValidation(rep *webapi.ValidateReport, fv *webapi.FileValidation) {
	rep.Files = append(rep.Files, fv)
	if fv.Ok {
		rep.Passed++
	} else {
		rep.Failed++
	}
}

// validateFile runs the invariants for the tiers the file's format carries. explicit marks
// a file the user named directly (an unclaimed extension then fails instead of skipping).
func validateFile(l *formats.Loader, path string, explicit bool) *webapi.FileValidation {
	fv := &webapi.FileValidation{Path: path, Format: formats.NameForExt(path)}
	f := formats.ByExt(path)
	if f == nil {
		fv.Problems = append(fv.Problems, "no reader for this extension")
		return fv
	}
	if formats.HasNetlist(path) {
		d, err := l.ReadDesign(path)
		if err != nil {
			fv.Problems = append(fv.Problems, "netlist: "+err.Error())
		} else {
			fv.Problems = append(fv.Problems, prefixed("netlist", validate.Design(d))...)
			fv.Netlist = &webapi.NetlistCounts{
				Components: int32(len(d.GetComponents())),
				Nets:       int32(len(d.GetNets())),
			}
		}
	}
	if formats.HasFaithful(path) {
		g, err := l.FaithfulGeometry(path)
		if err != nil {
			fv.Problems = append(fv.Problems, "geometry: "+err.Error())
		} else {
			fv.Problems = append(fv.Problems, prefixed("geometry", validate.Geometry(g))...)
			placements, wires := 0, 0
			for _, s := range g.GetSheets() {
				placements += len(s.GetPlacements())
				wires += len(s.GetWires())
			}
			fv.Geometry = &webapi.GeometryCounts{
				Sheets:     int32(len(g.GetSheets())),
				Symbols:    int32(len(g.GetSymbols())),
				Placements: int32(placements),
				Wires:      int32(wires),
				Resolved:   int32(validate.Resolved(g)),
			}
		}
	}
	fv.Ok = len(fv.Problems) == 0
	return fv
}

func prefixed(tier string, problems []string) []string {
	out := make([]string, 0, len(problems))
	for _, p := range problems {
		out = append(out, tier+": "+p)
	}
	return out
}

// writeValidateText renders the report as a table plus a summary line.
func writeValidateText(w io.Writer, rep *webapi.ValidateReport) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "FILE\tFORMAT\tSTATUS\tDETAIL")
	for _, fv := range rep.Files {
		status, detail := "ok", countsLine(fv)
		if !fv.Ok {
			status, detail = "FAIL", strings.Join(fv.Problems, "; ")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", fv.Path, fv.Format, status, detail)
	}
	tw.Flush()
	fmt.Fprintf(w, "\n%d passed, %d failed, %d skipped (no reader)\n", rep.Passed, rep.Failed, rep.Skipped)
}

// countsLine is the passing row's detail: whatever tiers the format produced.
func countsLine(fv *webapi.FileValidation) string {
	var parts []string
	if n := fv.GetNetlist(); n != nil {
		parts = append(parts, fmt.Sprintf("%d comps, %d nets", n.GetComponents(), n.GetNets()))
	}
	if g := fv.GetGeometry(); g != nil {
		parts = append(parts, fmt.Sprintf("%d sheets, %d placements (%d resolved), %d wires",
			g.GetSheets(), g.GetPlacements(), g.GetResolved(), g.GetWires()))
	}
	return strings.Join(parts, " · ")
}
