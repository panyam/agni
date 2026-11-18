package main

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
)

// projectsCmd groups the commands that keep projects on a server (agni issue 1696). A project there
// is a list of design names, and a design can be in several. Deleting a project deletes no design.
func projectsCmd() *cobra.Command {
	var remoteURL string
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "Create projects on an agni server, and list designs in them",
	}
	cmd.PersistentFlags().StringVar(&remoteURL, "remote", "", "the agni serve that keeps the projects, such as http://localhost:8080 (required)")
	client := func(cmd *cobra.Command) (webapiconnect.ProjectServiceClient, error) {
		if remoteURL == "" {
			return nil, fmt.Errorf("agni projects needs --remote, the URL of an agni serve started with --store")
		}
		r, err := dialRemote(cmd.Context(), remoteURL)
		if err != nil {
			return nil, err
		}
		return webapiconnect.NewProjectServiceClient(r.http, r.url), nil
	}

	var title string
	create := &cobra.Command{
		Use:   "create <id>",
		Short: "Make a project listing no designs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			p, err := unary(cmd.Context(), c.CreateProject, &webapi.CreateProjectRequest{ProjectId: args[0], Project: &webapi.Project{Title: title}})
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), p.GetName())
			return nil
		},
	}
	create.Flags().StringVar(&title, "title", "", "the project's title; the id when empty")

	add := &cobra.Command{
		Use:   "add <project> <design>...",
		Short: "List designs in a project, each as <id> or designs/<id>",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client(cmd)
			if err != nil {
				return err
			}
			name := projectName(args[0])
			p, err := unary(cmd.Context(), c.GetProject, &webapi.GetProjectRequest{Name: name})
			if err != nil {
				return err
			}
			designs := p.GetDesigns()
			for _, d := range args[1:] {
				if n := designName(d); !slices.Contains(designs, n) {
					designs = append(designs, n)
				}
			}
			p, err = unary(cmd.Context(), c.UpdateProject, &webapi.UpdateProjectRequest{
				Project:    &webapi.Project{Name: name, Designs: designs},
				UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"designs"}},
			})
			if err != nil {
				return err
			}
			for _, d := range p.GetDesigns() {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", p.GetName(), d)
			}
			return nil
		},
	}
	cmd.AddCommand(create, add)
	return cmd
}
