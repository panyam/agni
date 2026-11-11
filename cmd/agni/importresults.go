package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/panyam/agni/core/results"
	"github.com/panyam/agni/core/results/foreign"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/spf13/cobra"
)

// importResultsCmd reads another tool's check report into a results document (WS3-104).
//
// It is a command rather than a `formats` reader because a results file describes a design it does
// not contain, so it cannot answer "give me the netlist" as every registry capability must. The output
// is an ordinary check-result document, and `agni results` renders it with no special case.
func importResultsCmd() *cobra.Command {
	var design, out string
	cmd := &cobra.Command{
		Use:   "import-results <report.json>",
		Short: "Import another tool's check report as a check-result document",
		Long: "Read a kicad-cli DRC or ERC JSON report (`kicad-cli pcb drc --format json`, " +
			"`kicad-cli sch erc --format json`) into a check-result document, so a foreign tool's " +
			"findings become findings in the model rather than something a human reads side by side.\n\n" +
			"--design attaches the findings to entities by the names the report prints (ref-des, pin, " +
			"net); without it the findings are imported unattached. What could not be attached is " +
			"reported in the document's import summary, never dropped.\n\n" +
			"The result is deliberately a WEAKER artifact than an `agni check` run: a vendor report is a " +
			"flat violation list with no coverage axis, so the document records that rather than " +
			"pretending otherwise.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := os.Open(args[0])
			if err != nil {
				return err
			}
			defer f.Close()
			doc, err := foreign.ReadKiCad(f, time.Now())
			if err != nil {
				return fmt.Errorf("%s: %w", args[0], err)
			}
			if design != "" {
				m, err := readModel(design)
				if err != nil {
					return err
				}
				foreign.Join(m, doc)
				doc.Design.Source = design
				doc.Design.ContentHash = hashSource(design)
			}
			if out != "" {
				if err := writeResults(out, doc); err != nil {
					return err
				}
			}
			return writeImportSummary(cmd.OutOrStdout(), doc)
		},
	}
	cmd.Flags().StringVar(&design, "design", "", "the design the report is about; attaches findings to its components, pins and nets")
	cmd.Flags().StringVarP(&out, "out", "o", "", "write the check-result document here (render it later with `agni results`)")
	return cmd
}

// writeImportSummary prints what was imported and what could not be attached. A reader needs the
// unattached residue to judge the import. So the summary goes to stdout on every run rather than
// behind a verbose flag.
func writeImportSummary(w io.Writer, doc *checkspb.CheckResults) error {
	s := doc.GetImportSummary()
	fmt.Fprintf(w, "%s %s — %d finding(s) from %s\n",
		doc.GetMeta().GetProducer(), doc.GetMeta().GetProducerVersion(), len(doc.GetFindings()), doc.GetDesign().GetSource())
	if s == nil {
		fmt.Fprintln(w, "not attached to a design (pass --design to join findings to components and nets)")
		return nil
	}
	fmt.Fprintf(w, "attached to an entity: %d of %d\n", s.GetJoined(), s.GetFindings())
	for _, u := range s.GetUnjoined() {
		fmt.Fprintf(w, "  %4d not attached — %s\n", u.GetCount(), u.GetReason())
		for _, ex := range u.GetExamples() {
			fmt.Fprintf(w, "         e.g. %s\n", ex)
		}
	}
	fmt.Fprintln(w, "\nThis document carries no coverage axis: a vendor report lists violations and says")
	fmt.Fprintln(w, "nothing about what it did not check, so its silence must not be read as a pass.")
	return nil
}

// readExternalReports reads the documents `agni review --external` names (agni issue 152). Each must
// be a check-result document, and a raw vendor JSON report is refused with the command that converts
// it, since the review cannot join a vendor's prose to entities by itself.
func readExternalReports(paths []string) ([]*checkspb.CheckResults, error) {
	var out []*checkspb.CheckResults
	for _, p := range paths {
		b, err := os.ReadFile(p) // #nosec G304 -- reads a file the user named
		if err != nil {
			return nil, err
		}
		doc, err := results.Parse(b)
		if err != nil {
			return nil, fmt.Errorf("--external %s: %w (convert a vendor report first with `agni import-results --design <design> -o <doc.json> %s`)", p, err, p)
		}
		out = append(out, doc)
	}
	return out, nil
}
