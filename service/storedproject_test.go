package service

import (
	"context"
	"errors"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func TestAStoredProjectIsCreatedUpdatedListedAndDeleted(t *testing.T) {
	ctx := context.Background()
	svc := NewProjectService(fixtureStore()).WithRecords(NewMemProjectRecords())
	p, err := svc.CreateProject(ctx, &webapi.CreateProjectRequest{ProjectId: "b200", Project: &webapi.Project{Designs: []string{"designs/mem", "designs/mul"}}})
	if err != nil {
		t.Fatal(err)
	}
	if p.GetName() != "projects/b200" || p.GetTitle() != "b200" || len(p.GetDesigns()) != 2 {
		t.Errorf("created = %+v, want projects/b200 titled b200 with two designs", p)
	}
	up, err := svc.UpdateProject(ctx, &webapi.UpdateProjectRequest{
		Project:    &webapi.Project{Name: "projects/b200", Title: "B200", Designs: []string{"designs/other"}},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"title"}},
	})
	if err != nil || up.GetTitle() != "B200" || len(up.GetDesigns()) != 2 {
		t.Errorf("UpdateProject(title) = %+v, %v; want the new title and both designs kept", up, err)
	}
	list, err := svc.ListProjects(ctx, &webapi.ListProjectsRequest{})
	if err != nil || len(list.GetProjects()) != 3 || list.GetProjects()[0].GetName() != "projects/b200" {
		t.Errorf("ListProjects = %v, %v; want the stored b200 among the two mounted projects", list.GetProjects(), err)
	}
	if got, err := svc.GetProject(ctx, &webapi.GetProjectRequest{Name: "projects/b200"}); err != nil || got.GetTitle() != "B200" {
		t.Errorf("GetProject = %+v, %v; want the stored project", got, err)
	}
	if err := svc.DeleteProject(ctx, &webapi.DeleteProjectRequest{Name: "projects/b200"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetProject(ctx, &webapi.GetProjectRequest{Name: "projects/b200"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetProject after delete = %v, want ErrNotFound", err)
	}
}

func TestAProjectServiceRefusesWhatItCannotWrite(t *testing.T) {
	ctx := context.Background()
	svc := NewProjectService(fixtureStore()).WithRecords(NewMemProjectRecords())
	for _, tc := range []struct {
		req  *webapi.CreateProjectRequest
		want error
	}{
		{&webapi.CreateProjectRequest{ProjectId: "Bad"}, ErrInvalidArgument},
		{&webapi.CreateProjectRequest{ProjectId: "b200", Project: &webapi.Project{Designs: []string{"projects/x/designs/y"}}}, ErrInvalidArgument},
		{&webapi.CreateProjectRequest{ProjectId: "b200", Project: &webapi.Project{Designs: []string{"designs/a", "designs/a"}}}, ErrInvalidArgument},
		{&webapi.CreateProjectRequest{ProjectId: "gateway"}, ErrAlreadyExists},
	} {
		if _, err := svc.CreateProject(ctx, tc.req); !errors.Is(err, tc.want) {
			t.Errorf("CreateProject(%v) = %v, want %v", tc.req, err, tc.want)
		}
	}
	if _, err := svc.CreateProject(ctx, &webapi.CreateProjectRequest{ProjectId: "b200"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateProject(ctx, &webapi.CreateProjectRequest{ProjectId: "b200"}); !errors.Is(err, ErrAlreadyExists) {
		t.Errorf("second CreateProject = %v, want ErrAlreadyExists", err)
	}
	if _, err := svc.UpdateProject(ctx, &webapi.UpdateProjectRequest{Project: &webapi.Project{Name: "projects/gateway", Title: "x"}}); !errors.Is(err, ErrFailedPrecondition) {
		t.Errorf("UpdateProject of a mounted project = %v, want ErrFailedPrecondition", err)
	}
	if err := svc.DeleteProject(ctx, &webapi.DeleteProjectRequest{Name: "projects/gateway"}); !errors.Is(err, ErrFailedPrecondition) {
		t.Errorf("DeleteProject of a mounted project = %v, want ErrFailedPrecondition", err)
	}
	if _, err := svc.UpdateProject(ctx, &webapi.UpdateProjectRequest{Project: &webapi.Project{Name: "projects/b200"}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"uri"}}}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("UpdateProject of uri = %v, want ErrInvalidArgument", err)
	}
	if err := svc.DeleteProject(ctx, &webapi.DeleteProjectRequest{Name: "projects/none"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteProject of a missing project = %v, want ErrNotFound", err)
	}
	unstored := NewProjectService(fixtureStore())
	if _, err := unstored.CreateProject(ctx, &webapi.CreateProjectRequest{ProjectId: "b200"}); !errors.Is(err, ErrFailedPrecondition) {
		t.Errorf("CreateProject with no records = %v, want ErrFailedPrecondition", err)
	}
}
