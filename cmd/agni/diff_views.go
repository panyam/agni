package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/core/num"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// runViews answers `agni diff <design>`, which compares the design's schematic with its board
// (agni issue 1590). Both formats read the DiffDesigns response for views_snapshot, so the text and
// the json come from the one comparison, as the viewer's does.
func runViews(cmd *cobra.Command, design, format string) error {
	resp, err := viewsResponse(cmd, design)
	if err != nil {
		return err
	}
	if format == "json" {
		out, err := protoJSON(resp)
		if err != nil {
			return err
		}
		_, err = cmd.OutOrStdout().Write([]byte(out))
		return err
	}
	writeViews(cmd.OutOrStdout(), design, resp, diffListLimit)
	return nil
}

// viewsResponse asks the DiffDesigns rpc, over the CLI's local loader, how well design's two views
// agree. The design resolves as every command resolves it (C32), so naming the folder, its entry or
// its board compares the same two files.
func viewsResponse(cmd *cobra.Command, design string) (*webapi.DiffDesignsResponse, error) {
	u, err := cliArgURI(design)
	if err != nil {
		return nil, err
	}
	ll := &localLoader{loader: newLoader()}
	snaps := service.NewSnapshotService(ll, cliProjects(), nil, "")
	snap, drop, err := ingest(cmd.Context(), snaps, &webapi.Snapshot{Uri: u, AsNamed: readAsNamed})
	if err != nil {
		return nil, rpcFailed(err)
	}
	defer drop()
	resp, err := service.NewDiffService(ll, cliProjects()).DiffDesigns(cmd.Context(), &webapi.DiffDesignsRequest{ViewsSnapshot: snap})
	return resp, rpcFailed(err)
}

// writeViews prints the counts, then each list of what did not match, at most limit entries a list.
func writeViews(w io.Writer, design string, resp *webapi.DiffDesignsResponse, limit int) {
	v := resp.GetViews()
	fmt.Fprintf(w, "views of %s: the schematic against the board\n\n", design)
	if !v.GetCompared() {
		fmt.Fprintf(w, "Not compared: %s.\n", v.GetNotCompared())
		return
	}
	rep := resp.GetReport()
	var schOnly, boardOnly, differ, renamed []string
	for _, nc := range rep.GetNets() {
		switch nc.GetKind() {
		case "deleted":
			schOnly = append(schOnly, nc.GetName())
		case "new":
			boardOnly = append(boardOnly, nc.GetName())
		case "hard":
			differ = append(differ, pinsDiffer(nc))
		case "renamed":
			renamed = append(renamed, nc.GetOldName()+" is "+nc.GetName()+" on the board")
		}
	}
	fmt.Fprintf(w, "Parts: %s\n", tallyLine(v.GetParts(), "schematic parts are on the board", len(rep.GetComponentsAdded())))
	fmt.Fprintf(w, "Pins:  %s\n", tallyLine(v.GetPins(), "pins on a schematic net are on a net on the board", len(v.GetBoardOnlyPins())))
	fmt.Fprintf(w, "Nets:  %s\n", tallyLine(v.GetNets(), "schematic nets match a board net", len(boardOnly)))
	list(w, "Parts only on the schematic", rep.GetComponentsRemoved(), limit)
	list(w, "Parts only on the board", rep.GetComponentsAdded(), limit)
	list(w, "Nets only on the schematic", schOnly, limit)
	list(w, "Nets only on the board", boardOnly, limit)
	list(w, "Nets whose pins differ", differ, limit)
	list(w, "Nets named differently, matched by their pins", renamed, limit)
	list(w, "Pins on a net only on the schematic", v.GetSchematicOnlyPins(), limit)
	list(w, "Pins on a net only on the board", v.GetBoardOnlyPins(), limit)
}

// tallyLine reads a tally as "1,121 of 1,123 schematic parts are on the board, and 3 more are on
// the board alone".
func tallyLine(t *webapi.ViewAgreement_Tally, matched string, boardOnly int) string {
	s := fmt.Sprintf("%s of %s %s", thousands(t.GetMatched()), thousands(t.GetSchematic()), matched)
	if boardOnly > 0 {
		s += fmt.Sprintf(", and %s more are on the board alone", thousands(num.Int32(boardOnly)))
	}
	return s
}

// pinsDiffer reads a net whose pins differ as "OUT: the board adds R4.1 and lacks C1.1".
func pinsDiffer(nc *webapi.DiffReport_NetChange) string {
	var parts []string
	if a := nc.GetAdded(); len(a) > 0 {
		parts = append(parts, "the board adds "+strings.Join(a, " "))
	}
	if r := nc.GetRemoved(); len(r) > 0 {
		parts = append(parts, "the board lacks "+strings.Join(r, " "))
	}
	return nc.GetName() + ": " + strings.Join(parts, " and ")
}

func list(w io.Writer, title string, items []string, limit int) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%s (%d):\n", title, len(items))
	for i, it := range items {
		if limit > 0 && i == limit {
			fmt.Fprintf(w, "  ... and %d more (--format json lists them all)\n", len(items)-limit)
			break
		}
		fmt.Fprintf(w, "  %s\n", it)
	}
}

// thousands writes n with a comma every three digits, as "3,310".
func thousands(n int32) string {
	s := fmt.Sprint(n)
	if n < 0 {
		return "-" + thousands(-n)
	}
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
