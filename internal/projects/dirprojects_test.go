package projects

import (
	"context"
	"errors"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/records/recordstest"
	"github.com/panyam/agni/service"
)

func TestDirProjectsKeepARecordAcrossAReopen(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	r := NewProjects(recordstest.At(t, dir))
	p := &webapi.Project{Name: "projects/b200", Title: "B200", Designs: []string{"designs/mem", "designs/mul"}}
	if err := r.Put(ctx, "b200", p); err != nil {
		t.Fatal(err)
	}
	again := NewProjects(recordstest.At(t, dir))
	got, err := again.Get(ctx, "b200")
	if err != nil || got.GetTitle() != "B200" || len(got.GetDesigns()) != 2 {
		t.Errorf("Get after reopening = %+v, %v; want B200 with two designs", got, err)
	}
	list, err := again.List(ctx)
	if err != nil || len(list) != 1 {
		t.Errorf("List = %v, %v; want one record and no temporary file", list, err)
	}
	if err := again.Delete(ctx, "b200"); err != nil {
		t.Fatal(err)
	}
	if _, err := again.Get(ctx, "b200"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("Get after delete = %v, want ErrNotFound", err)
	}
	if err := again.Delete(ctx, "b200"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("second Delete = %v, want ErrNotFound", err)
	}
	if _, err := again.Get(ctx, "../x"); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("Get of an id that leaves the folder = %v, want ErrInvalidArgument", err)
	}
}
