package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/agni/fshost"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
	"github.com/spf13/cobra"
)

// proposeCmd is the CLI face of WorkspaceService.ProposeDesigns (agni issue 854): the designs a
// folder or a zip of files makes, each with the design.yaml that would declare it, and every file no
// design reads with the reason. The viewer shows the same proposal for a drop, so the two agree.
func proposeCmd() *cobra.Command {
	var format string
	c := &cobra.Command{
		Use:   "propose <folder|file.zip>",
		Short: "Propose the designs a folder or zip of files makes, as design.yaml text",
		Long: "Group the files under a folder (or inside a .zip) into the designs they make and print the " +
			"design.yaml that would declare each, plus every file no design reads and why. A folder " +
			"already holding a design.yaml is reported as declared. --format json is the " +
			"ProposeDesignsResponse the ProposeDesigns rpc returns.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "text" && format != "json" {
				return fmt.Errorf("unknown --format %q (want: text, json)", format)
			}
			abs, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			st, err := os.Stat(abs)
			if err != nil {
				return err
			}
			root, sub := abs, ""
			if !st.IsDir() {
				if !strings.EqualFold(filepath.Ext(abs), ".zip") {
					return fmt.Errorf("%s is a file; propose takes a folder or a .zip", args[0])
				}
				root, sub = filepath.Dir(abs), filepath.Base(abs)
			}
			// Zips expand here as they do in the browser, so `agni propose drop.zip` answers what the
			// page would.
			host := fshost.New(fshost.Mount{Name: "drop", FS: fshost.ExpandZips(os.DirFS(root))})
			svc := service.NewWorkspaceService(host.Workspace()).WithDesignFiles(nil, host.Workspace())
			resp, err := svc.ProposeDesigns(cmd.Context(), &webapi.ProposeDesignsRequest{Uri: "mount://drop/" + filepath.ToSlash(sub)})
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			if format == "json" {
				out, err := protoJSON(resp)
				if err != nil {
					return err
				}
				_, err = fmt.Fprint(w, out)
				return err
			}
			if len(resp.GetDesigns()) == 0 {
				fmt.Fprintf(w, "no designs under %s\n", args[0])
			}
			for _, d := range resp.GetDesigns() {
				folder := d.GetFolder()
				if folder == "" {
					folder = "."
				}
				head := "# " + filepath.Join(folder, service.DesignDescriptorName)
				if d.GetDeclared() {
					head += "  (declared)"
				}
				fmt.Fprintf(w, "%s  %d file(s)\n", head, len(d.GetFiles()))
				if d.GetNote() != "" {
					fmt.Fprintf(w, "# note: %s\n", d.GetNote())
				}
				fmt.Fprintln(w, strings.TrimRight(d.GetDesignYaml(), "\n"))
				fmt.Fprintln(w)
			}
			if len(resp.GetUnread()) > 0 {
				fmt.Fprintln(w, "# not read")
				for _, u := range resp.GetUnread() {
					fmt.Fprintf(w, "#   %s: %s\n", u.GetPath(), u.GetReason())
				}
			}
			return nil
		},
	}
	c.Flags().StringVar(&format, "format", "text", "text (each design.yaml, then what is not read) or json (the ProposeDesignsResponse)")
	return c
}
