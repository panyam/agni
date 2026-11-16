package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// boardCmd writes a design's board geometry, the same agni.v1.geom.BoardGeometry that
// DesignService.GetBoardGeometry serves (agni issue 1634). The text form counts what the board
// holds. --format json and --format binary write the whole message, for a front end that draws it
// or a test that compares against it. docsite/content/build/board-geometry.md says what it means.
//
// It reads the board through the loader, without a server, so `make exercise-clients` takes its
// counts as the answer every client's GetBoardGeometry must match. The served board names a net the
// board and schematic name differently by the schematic's name, which changes no count.
func boardCmd() *cobra.Command {
	var format, out string
	c := &cobra.Command{
		Use:   "board <design>",
		Short: "Write a board's geometry: counts, or the whole message in JSON or binary protobuf",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "text" && format != "json" && format != "binary" {
				return fmt.Errorf("--format %s: want text, json or binary", format)
			}
			if format == "binary" && out == "" {
				return fmt.Errorf("--format binary writes protobuf bytes, so name a file with -o")
			}
			ws, err := workspace()
			if err != nil {
				return err
			}
			src, err := newDesignResolver(ws).Resolve(context.Background(), args[0])
			if err != nil {
				return err
			}
			noteSource(os.Stderr, src)
			b, err := newLoader().BoardGeometry(localOf(src.BoardURI))
			if err != nil {
				return err
			}
			if b == nil {
				return fmt.Errorf("%s has no board", args[0])
			}
			var body []byte
			switch format {
			case "json":
				if body, err = (protojson.MarshalOptions{Multiline: true, Indent: "  "}).Marshal(b); err != nil {
					return err
				}
				body = append(body, '\n')
			case "binary":
				if body, err = proto.Marshal(b); err != nil {
					return err
				}
			}
			if out == "" {
				w := cmd.OutOrStdout()
				if format == "text" {
					return writeBoardCounts(w, b)
				}
				_, err = w.Write(body)
				return err
			}
			if format == "text" {
				f, err := os.Create(out) // #nosec G304 -- the user names the output file
				if err != nil {
					return err
				}
				defer f.Close()
				return writeBoardCounts(f, b)
			}
			return os.WriteFile(out, body, 0o600)
		},
	}
	c.Flags().StringVar(&format, "format", "text", "text (counts), json (the BoardGeometry message) or binary (the message as protobuf bytes, with -o)")
	c.Flags().StringVarP(&out, "out", "o", "", "write to this file instead of stdout")
	return c
}

// BoardCounts are the numbers `agni board` prints and the clients exercise compares.
type BoardCounts struct {
	Layers, Parts, Nets, Tracks, Vias, Zones int
}

// CountBoard counts a board the way every language's exercise example does. Tracks are the routed
// segments of every net, arcs included.
func CountBoard(b *geom.BoardGeometry) BoardCounts {
	c := BoardCounts{Layers: len(b.GetLayers()), Parts: len(b.GetPlacements()), Nets: len(b.GetNets()), Zones: len(b.GetZones())}
	for _, n := range b.GetNets() {
		c.Tracks += len(n.GetSegments())
		c.Vias += len(n.GetVias())
	}
	return c
}

func writeBoardCounts(w io.Writer, b *geom.BoardGeometry) error {
	c := CountBoard(b)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "layers\t%d\nparts\t%d\nnets with copper\t%d\ntracks\t%d\nvias\t%d\nzones\t%d\n", c.Layers, c.Parts, c.Nets, c.Tracks, c.Vias, c.Zones)
	fmt.Fprintf(tw, "units\t%d nm per unit\n", b.GetUnitNm())
	return tw.Flush()
}
