package projects

import (
	"context"
	"errors"
	"fmt"
	"sync"
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

// Two servers add designs to one project at once through Update, and every design stays (agni issue
// 1823).
func TestTwoServersUpdatingOneProjectLoseNoDesign(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	a, b := NewProjects(recordstest.At(t, dir)), NewProjects(recordstest.At(t, dir))
	if err := a.Create(ctx, "lab", &webapi.Project{Name: "projects/lab"}); err != nil {
		t.Fatal(err)
	}
	if err := b.Create(ctx, "lab", &webapi.Project{Name: "projects/lab"}); !errors.Is(err, service.ErrAlreadyExists) {
		t.Fatalf("a second Create of lab = %v, want ErrAlreadyExists", err)
	}
	const n = 10
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := a
			if i%2 == 1 {
				r = b
			}
			_, errs[i] = r.Update(ctx, "lab", func(p *webapi.Project) error {
				p.Designs = append(p.Designs, fmt.Sprintf("designs/d%d", i))
				return nil
			})
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err := a.Get(ctx, "lab")
	if err != nil || len(got.GetDesigns()) != n {
		t.Fatalf("lab holds %d designs after %d racing Updates, %v", len(got.GetDesigns()), n, err)
	}
	if _, err := a.Update(ctx, "none", func(*webapi.Project) error { return nil }); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("Update of a project no one made = %v, want ErrNotFound", err)
	}
}
