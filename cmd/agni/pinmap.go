package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/panyam/agni/core/check"
)

// pinmapCmd drafts a design intent's io_map for one device from the netlist (agni issue 1238).
//
// The draft agrees with the netlist by construction, so checking the design against it passes. Its
// use is the inventory a reviewer edits into the map they sign off, and the map a LATER revision is
// checked against. The header says so, because a draft checked unedited against its own board reads
// as a sign-off that never happened.
func pinmapCmd() *cobra.Command {
	var device, out string
	var pins []string
	var hops int
	cmd := &cobra.Command{
		Use:   "pinmap <file>",
		Short: "Draft a pin map for one device from the netlist, with each pin's far end",
		Long: "pinmap lists every pin of --device with the net it sits on, and walks from each signal pin\n" +
			"through series parts as `agni trace` does to find the far end, a pin on another IC or\n" +
			"connector. It writes the rows as a design intent's io_map, which `agni check --intent-path`\n" +
			"reads, and `agni check --format html` then lays out one row per pin with the route to each\n" +
			"far end.\n\n" +
			"A pin reaching exactly one device pin gets it as `to`. A pin reaching several lists them in a\n" +
			"comment, since the netlist cannot say which one the signal is for, and a pin on ground or a\n" +
			"supply rail is not walked. The draft agrees with the netlist it came from, so review each row\n" +
			"against the datasheet or the interface before relying on it.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if device == "" {
				return fmt.Errorf("--device is required: name the part whose pins to map, e.g. --device J1")
			}
			d, err := readDesign(args[0])
			if err != nil {
				return err
			}
			m := check.NewModel(d)
			if !m.HasComponent(device) {
				return fmt.Errorf("no component %s in this design", device)
			}
			rows := check.PartnersOf(m, device, hops)
			if len(pins) > 0 {
				if rows, err = pickPins(m, device, rows, pins); err != nil {
					return err
				}
			}
			closeOut, err := redirectOut(cmd, out)
			if err != nil {
				return err
			}
			defer closeOut()
			name := d.GetName()
			if name == "" {
				name = strings.TrimSuffix(filepath.Base(args[0]), filepath.Ext(args[0]))
			}
			return writePinMap(cmd.OutOrStdout(), args[0], name, device, rows)
		},
	}
	cmd.Flags().StringVar(&device, "device", "", "the part whose pins to map, by ref-des (required)")
	cmd.Flags().StringSliceVar(&pins, "pin", nil, "map only these pins, by designator or functional name (repeatable)")
	cmd.Flags().IntVar(&hops, "hops", check.DefaultTraceHops, "how many series parts a far-end walk may cross")
	outFileFlag(cmd, &out)
	return cmd
}

// pickPins keeps the rows a --pin names, in the order named. A pin is resolved by the lookup trace
// and the IO-map rules use, so a functional name works as well as a designator.
func pickPins(m check.Model, device string, rows []check.PinPartners, names []string) ([]check.PinPartners, error) {
	byPin := map[string]check.PinPartners{}
	for _, r := range rows {
		byPin[r.Pin] = r
	}
	var out []check.PinPartners
	for _, name := range names {
		r, ok := byPin[name]
		if !ok {
			l := check.LookupPin(m, device, name)
			switch l.Kind {
			case check.PinFound:
				r = byPin[l.Designator]
			case check.PinAmbiguous:
				return nil, fmt.Errorf("--pin %s names several pins of %s: %s", name, device, strings.Join(l.Candidates, ", "))
			default:
				return nil, fmt.Errorf("--pin %s: %s has no such pin", name, device)
			}
		}
		out = append(out, r)
	}
	return out, nil
}

// writePinMap writes the draft as an intent file: a header saying where it came from and what it
// does not prove, then one io_map row per pin on a net. A pin on no net gets a comment rather than a
// row, since a row with no net declares nothing a rule can check.
func writePinMap(w io.Writer, source, name, device string, rows []check.PinPartners) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# A pin map for %s, drafted by `agni pinmap` from the netlist of %s.\n", device, source)
	b.WriteString("# Every row agrees with that netlist, because it was read from it. Check each row against the\n")
	b.WriteString("# datasheet or the interface it implements, then check this design and its later revisions\n")
	b.WriteString("# against the result with `agni check <design> --intent-path <this file>`, which replaces the\n")
	b.WriteString("# design's own intent for that run.\n")
	fmt.Fprintf(&b, "name: %s\n", yamlScalar(name+"-"+device+"-pin-map"))
	b.WriteString("intent:\n  io_map:\n")
	for _, r := range rows {
		if r.Reason == check.PartnerReasonNoNet {
			fmt.Fprintf(&b, "    # %s.%s is on no net.\n", device, r.Pin)
			continue
		}
		row := fmt.Sprintf("    - {device: %s, pin: %s, net: %s", yamlScalar(device), yamlScalar(r.Pin), yamlScalar(r.Net))
		var note string
		switch {
		case r.Reason != "":
			note = fmt.Sprintf("%s, so no far end was looked for", r.Reason)
		case len(r.Partners) == 1:
			row += fmt.Sprintf(", to: {device: %s, pin: %s}", yamlScalar(r.Partners[0].RefDes), yamlScalar(r.Partners[0].Pin))
		case len(r.Partners) == 0:
			note = "reaches no other device pin"
		default:
			note = "reaches " + joinEndpoints(r.Partners) + "; pick the far end"
		}
		row += "}"
		if r.Name != "" && r.Name != r.Pin {
			note = strings.TrimPrefix(r.Name+"; "+note, "; ")
			note = strings.TrimSuffix(note, "; ")
		}
		if note != "" {
			row += "  # " + note
		}
		b.WriteString(row + "\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func joinEndpoints(es []check.Endpoint) string {
	s := make([]string, len(es))
	for i, e := range es {
		s[i] = e.String()
	}
	return strings.Join(s, ", ")
}

// yamlScalar spells a value as YAML would, quoting it when it would otherwise read as something else.
// Net names carry slashes, spaces and leading symbols (`/Expansion connector/CAN0.DOUT`, `+3V3`),
// and a pin designator like `01` or `on` would be read as a number or a boolean.
func yamlScalar(s string) string {
	b, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Sprintf("%q", s)
	}
	out := strings.TrimSuffix(string(b), "\n")
	if !strings.HasPrefix(out, `"`) && !strings.HasPrefix(out, "'") && strings.ContainsAny(out, ",{}[]#:") {
		return fmt.Sprintf("%q", s)
	}
	return out
}
