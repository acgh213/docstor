package templates_test

import (
	"context"
	"testing"

	"github.com/exedev/docstor/internal/templates"
	"github.com/exedev/docstor/internal/testutil"
)

func TestTemplates_CRUD(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := templates.NewRepository(pool)
	ctx := context.Background()
	f := testutil.QuickFixture(t, pool, "TmplTest", "tmpl@tmpl.com", "editor")

	// Create
	tmpl, err := repo.Create(ctx, templates.CreateInput{
		TenantID:     f.TenantID,
		CreatedBy:    f.UserID,
		Name:         "Runbook Template",
		TemplateType: "runbook",
		BodyMarkdown: "## Steps\n1. Do thing",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tmpl.Name != "Runbook Template" {
		t.Errorf("Name = %q, want Runbook Template", tmpl.Name)
	}
	if tmpl.TemplateType != "runbook" {
		t.Errorf("TemplateType = %q, want runbook", tmpl.TemplateType)
	}

	// Get
	got, err := repo.Get(ctx, f.TenantID, tmpl.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.BodyMarkdown != "## Steps\n1. Do thing" {
		t.Errorf("BodyMarkdown mismatch")
	}

	// Update
	updated, err := repo.Update(ctx, f.TenantID, tmpl.ID, templates.UpdateInput{
		Name:         "Updated Template",
		TemplateType: "doc",
		BodyMarkdown: "## Updated",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "Updated Template" {
		t.Errorf("Name = %q, want Updated Template", updated.Name)
	}

	// List
	list, err := repo.List(ctx, f.TenantID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	found := false
	for _, item := range list {
		if item.ID == tmpl.ID {
			found = true
		}
	}
	if !found {
		t.Error("template not found in list")
	}

	// Delete
	if err := repo.Delete(ctx, f.TenantID, tmpl.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = repo.Get(ctx, f.TenantID, tmpl.ID)
	if err != templates.ErrNotFound {
		t.Errorf("after delete: got %v, want ErrNotFound", err)
	}
}

func TestTemplates_TenantIsolation(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := templates.NewRepository(pool)
	ctx := context.Background()

	fA := testutil.QuickFixture(t, pool, "TmplIsoA", "a@tmpl.com", "editor")
	fB := testutil.QuickFixture(t, pool, "TmplIsoB", "b@tmpl.com", "editor")

	tmpl, err := repo.Create(ctx, templates.CreateInput{
		TenantID:     fA.TenantID,
		CreatedBy:    fA.UserID,
		Name:         "Secret Template",
		TemplateType: "doc",
		BodyMarkdown: "secret",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = repo.Get(ctx, fB.TenantID, tmpl.ID)
	if err != templates.ErrNotFound {
		t.Errorf("tenant B Get: got %v, want ErrNotFound", err)
	}

	listB, err := repo.List(ctx, fB.TenantID)
	if err != nil {
		t.Fatalf("List B: %v", err)
	}
	for _, item := range listB {
		if item.ID == tmpl.ID {
			t.Error("tenant B can see tenant A's template")
		}
	}
}

func TestTemplates_DefaultType(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := templates.NewRepository(pool)
	ctx := context.Background()
	f := testutil.QuickFixture(t, pool, "TmplDefault", "def@tmpl.com", "editor")

	tmpl, err := repo.Create(ctx, templates.CreateInput{
		TenantID:     f.TenantID,
		CreatedBy:    f.UserID,
		Name:         "No Type",
		TemplateType: "", // empty should default to "doc"
		BodyMarkdown: "body",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tmpl.TemplateType != "doc" {
		t.Errorf("TemplateType = %q, want doc (default)", tmpl.TemplateType)
	}
}
