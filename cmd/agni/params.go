package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/panyam/agni/core/check"
	rpt "github.com/panyam/agni/core/report"
	"github.com/panyam/agni/datasheet/param"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// paramsCmd prints one part's whole datasheet record.
//
// The query relations carry the part of a datasheet a query can BIND: join keys, comparable scalars,
// and closed vocabularies. Everything else in the record is read rather than queried, and until this
// command there was no way to read it outside the browser, which made "the flat relations do not
// carry that, read the record instead" an answer nobody could act on (agni issue 547; DECISIONS.md,
// "The datasheet tier is normalized into narrow relations, never flattened into a wider tuple").
//
// It takes NO DESIGN. A spec library is not a design, and the datasheet relations already answer
// against a seeded corpus with none loaded (query.NewSpecLibBase). --design exists only so a design's
// PROJECT can supply the corpus, which is the tier precedence Overlay.SpecsOr states.
func paramsCmd() *cobra.Command {
	var paramsDir, designPath, format string
	c := &cobra.Command{
		Use:   "params <mpn>",
		Short: "Print one part's datasheet record: every parameter with its conditions, pins, citations, and verification state",
		Long: `Print the seeded PartSpec for one manufacturer part number: its source documents, every
parameter with its limit kind, bounds, test conditions and citation, the pins the datasheet declares,
and any constraints between them.

This is the record behind a query answer. The datalog relations carry what a query can bind (param,
param.range, param.typ, param.pin, ...); the conditions a value is valid under, the pin bindings, the
full provenance and the verification state live here.

It needs no design. Name a corpus with --params, or name a design with --design so the project that
design belongs to supplies its own params/ (which WINS over --params, since a project owns its
parameters the way it owns its profiles).

  agni params LM1117 --params seed/
  agni params LM1117 --design designs/gateway/gateway.kicad_sch
  agni params LM1117 --params seed/ --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch format {
			case "text", "json":
			default:
				return fmt.Errorf("unknown --format %q: want text or json", format)
			}
			specs, corpus, err := paramsCorpus(paramsDir, designPath)
			if err != nil {
				return err
			}
			if specs == nil {
				return fmt.Errorf("no datasheet corpus: name one with --params <dir>, or --design <path> for a design whose project declares one")
			}
			spec := specs.Lookup(args[0])
			if spec == nil {
				// Never an empty record. A part nobody has transcribed and a part with no parameters
				// are different answers, and printing an empty spec for the first says the second.
				// The match is case-insensitive but never fuzzy (param.ParamSet.Lookup): a near-miss
				// MPN is a different part until a human says otherwise.
				return fmt.Errorf("no seeded spec for mpn %q in %s", args[0], corpus)
			}
			if format == "json" {
				return writeSpecJSON(cmd.OutOrStdout(), spec)
			}
			return writeSpecText(cmd.OutOrStdout(), spec)
		},
	}
	c.Flags().StringVar(&paramsDir, "params", "", "directory of seeded PartSpec textprotos (the datasheet corpus). A project's own params/ wins over this when --design names a design in one")
	c.Flags().StringVar(&designPath, "design", "", "a design whose PROJECT supplies the corpus, for a part seeded in a project rather than a loose directory")
	c.Flags().StringVar(&format, "format", "text", "output format: text or json (json emits the PartSpec itself, the contract type)")
	return c
}

// paramsCorpus resolves the corpus to read and a name for it, from the two routes in.
//
// The project WINS over the flag (Overlay.SpecsOr), which is the opposite of the mount rule and
// deliberate: a project owns its parameters the way it owns its profiles. A command reading its tier
// from the flag alone is the bug shape that made intake's datasheet-gap section absent rather than
// empty inside a project (agni issue 474).
func paramsCorpus(paramsDir, designPath string) (param.ParamProvider, string, error) {
	var flagSpecs param.ParamProvider
	if paramsDir != "" {
		set, err := param.LoadSet(os.DirFS(paramsDir))
		if err != nil {
			return nil, "", fmt.Errorf("--params %s: %w", paramsDir, err)
		}
		flagSpecs = set
	}
	if designPath == "" {
		return flagSpecs, paramsDir, nil
	}
	_, ov, err := readDesignWithConfig(designPath)
	if err != nil {
		return nil, "", err
	}
	if ov.Specs != nil {
		return ov.Specs, "the project's params/", nil
	}
	return flagSpecs, paramsDir, nil
}

// writeSpecJSON emits the PartSpec itself in protojson form, matching how every other --format json
// in this CLI emits its wire message.
//
// The BARE PartSpec rather than a wrapper: no RPC serves this, and inventing a response message for
// one consumer would put a wire shape in the schema that nothing speaks. The spec IS the contract
// type (C-"a type crossing a runtime boundary is a proto"), so a script binding to this binds to the
// same message the workbench and the params panel carry.
func writeSpecJSON(w io.Writer, spec *parampb.PartSpec) error {
	b, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(spec)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

// writeSpecText renders the record as a sequence of report.Table sections.
//
// Through report.Table rather than a private printer, because the check and diff renderers were
// written in cmd/, the second was copied from the first, and the copies drifted (agni issue 380). A
// section is omitted when the spec carries nothing for it, so a minimal seeded part prints two short
// tables instead of five empty ones.
func writeSpecText(w io.Writer, spec *parampb.PartSpec) error {
	fmt.Fprintf(w, "%s\n", specHeading(spec))
	for _, s := range specSections(spec) {
		fmt.Fprintf(w, "\n%s\n", s.heading)
		if err := rpt.TableText(w, s.table); err != nil {
			return err
		}
	}
	return nil
}

// specHeading is the part's identity line: what it is, before any of what it says.
func specHeading(spec *parampb.PartSpec) string {
	var qual []string
	if m := spec.GetManufacturer(); m != "" {
		qual = append(qual, m)
	}
	if c := spec.GetDeviceClass(); c != "" {
		qual = append(qual, c)
	}
	if len(qual) == 0 {
		return spec.GetMpn()
	}
	return fmt.Sprintf("%s  (%s)", spec.GetMpn(), strings.Join(qual, ", "))
}

// specSection is one titled table of the record.
type specSection struct {
	heading string
	table   rpt.Table
}

// specSections builds the tables in reading order: what the values were read FROM, then the values,
// then the terminals they bind to, then the constraints between those.
func specSections(spec *parampb.PartSpec) []specSection {
	var out []specSection
	if t, ok := docsTable(spec); ok {
		out = append(out, specSection{"Documents", t})
	}
	if t, ok := parametersTable(spec); ok {
		out = append(out, specSection{"Parameters", t})
	}
	if t, ok := packagesTable(spec); ok {
		out = append(out, specSection{"Packages", t})
	}
	if t, ok := pinsTable(spec); ok {
		out = append(out, specSection{"Pins", t})
	}
	if t, ok := relationsTable(spec); ok {
		out = append(out, specSection{"Pin relations", t})
	}
	return out
}

// docsTable lists the source documents, each with the content hash staleness is decided on. The
// locator rides in the provenance column: it is where this corpus keeps the file, which is the one
// thing a reader chasing a citation actually needs.
func docsTable(spec *parampb.PartSpec) (rpt.Table, bool) {
	t := rpt.Table{Columns: []string{"id", "title", "vendor", "content hash"}}
	for _, d := range spec.GetDocs() {
		t.Rows = append(t.Rows, rpt.TableRow{
			Cells: []string{d.GetId(), d.GetTitle(), d.GetVendor(), d.GetContentHash()},
			Cites: nonEmpty(d.GetLocator()),
		})
	}
	return t, len(t.Rows) > 0
}

// parametersTable is the record's centre: every value with the kind of claim it makes, the
// conditions it holds under, the terminals it binds to, and whether anyone has stood behind it.
func parametersTable(spec *parampb.PartSpec) (rpt.Table, bool) {
	t := rpt.Table{Columns: []string{"symbol", "name", "kind", "min", "typ", "max", "unit", "conditions", "variant", "pins", "verification"}}
	for _, p := range spec.GetParameters() {
		v := p.GetValue()
		t.Rows = append(t.Rows, rpt.TableRow{
			Cells: []string{
				p.GetSymbol(),
				p.GetName(),
				param.LimitKindToken(p.GetLimitKind()),
				optNum(v.GetMin, v != nil && v.Min != nil),
				optNum(v.GetTyp, v != nil && v.Typ != nil),
				optNum(v.GetMax, v != nil && v.Max != nil),
				p.GetUnit(),
				conditionsCell(p.GetConditions()),
				p.GetAppliesTo(),
				strings.Join(p.GetPinRefs(), " "),
				verificationCell(spec, p),
			},
			Cites: nonEmpty(check.Citation(spec, p)),
		})
	}
	return t, len(t.Rows) > 0
}

// packagesTable lists the bodies this document covers, which is what makes a pin NUMBER meaningful.
func packagesTable(spec *parampb.PartSpec) (rpt.Table, bool) {
	t := rpt.Table{Columns: []string{"id", "name", "mpn suffix"}}
	for _, p := range spec.GetPackages() {
		t.Rows = append(t.Rows, rpt.TableRow{Cells: []string{p.GetId(), p.GetName(), p.GetMpnSuffix()}})
	}
	return t, len(t.Rows) > 0
}

// pinsTable lists the declared terminals. The numbers column is package-qualified, because a pin
// number is a fact about a BODY rather than about the die: the same silicon in a different package
// numbers differently.
func pinsTable(spec *parampb.PartSpec) (rpt.Table, bool) {
	t := rpt.Table{Columns: []string{"id", "name", "function", "numbers", "description"}}
	for _, p := range spec.GetPins() {
		nums := make([]string, 0, len(p.GetNumbers()))
		for _, n := range p.GetNumbers() {
			nums = append(nums, n.GetPackageRef()+":"+n.GetNumber())
		}
		t.Rows = append(t.Rows, rpt.TableRow{
			Cells: []string{p.GetId(), p.GetName(), param.PinFunctionToken(p.GetFunction()), strings.Join(nums, " "), p.GetDescription()},
			Cites: nonEmpty(check.PinCitation(spec, p)),
		})
	}
	return t, len(t.Rows) > 0
}

// relationsTable lists the constraints between two terminals. The bound is on subject MINUS
// reference, so the column order is the requirement and swapping it inverts the meaning.
func relationsTable(spec *parampb.PartSpec) (rpt.Table, bool) {
	t := rpt.Table{Columns: []string{"subject", "reference", "modality", "min", "max", "unit", "conditions", "as printed"}}
	for _, r := range spec.GetRelations() {
		d := r.GetDifference()
		t.Rows = append(t.Rows, rpt.TableRow{
			Cells: []string{
				r.GetSubjectPinRef(),
				r.GetReferencePinRef(),
				param.ModalityToken(r.GetModality()),
				optNum(d.GetMin, d != nil && d.Min != nil),
				optNum(d.GetMax, d != nil && d.Max != nil),
				r.GetUnit(),
				conditionsCell(r.GetConditions()),
				r.GetRaw(),
			},
			Cites: nonEmpty(check.RelationCitation(spec, r)),
		})
	}
	return t, len(t.Rows) > 0
}

// verificationCell reports whether anyone has stood behind a value, and what to do when the document
// has moved on.
//
// STALE NAMES BOTH REVISIONS, which is the whole reason Verification snapshots the printed identity
// beside the hash. "Verified against Rev K, corpus now holds Rev L" is a task someone can pick up;
// two content hashes is not (DECISIONS.md, "A document revision is recorded for the reader, and never
// compared"). The state itself comes from param.VerificationOfIn, so staleness is decided on the
// hash here exactly as it is everywhere else, and never on these strings.
func verificationCell(spec *parampb.PartSpec, p *parampb.Parameter) string {
	state := param.VerificationOfIn(spec, p)
	v := p.GetVerification()
	switch state {
	case param.Stale:
		now := check.DocTitle(spec, p.GetProv().GetDocRef())
		return fmt.Sprintf("stale (%s checked %q, corpus holds %q)", by(v), v.GetDocRevision(), now)
	case param.Verified:
		return fmt.Sprintf("verified (%s, %s)", by(v), v.GetAt())
	case param.Unknown:
		return fmt.Sprintf("unknown (%s checked %q; this corpus records no revision to compare)", by(v), v.GetDocRevision())
	default:
		return string(param.Unverified)
	}
}

// by names the verifier, or says so when the record does not.
func by(v *parampb.Verification) string {
	if s := v.GetBy(); s != "" {
		return s
	}
	return "someone"
}

// conditionsCell renders the conditions a value holds under, preferring the vendor's own words. The
// raw text is always populated for a captured condition, and it is what a reviewer matches against
// the page.
func conditionsCell(cs []*parampb.Condition) string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		if r := c.GetRaw(); r != "" {
			out = append(out, r)
			continue
		}
		out = append(out, c.GetSymbol())
	}
	return strings.Join(out, "; ")
}

// optNum renders an optional bound, empty when absent. Absence is a real state (a max-only row), so
// it must not arrive as a zero.
func optNum(get func() float64, present bool) string {
	if !present {
		return ""
	}
	return strconv.FormatFloat(get(), 'g', -1, 64)
}

// nonEmpty wraps a citation for the provenance column, dropping an empty one so a row with no
// recorded source shows a blank cell rather than a stray separator.
func nonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}
