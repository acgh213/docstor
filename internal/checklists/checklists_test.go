package checklists_test

import (
	"context"
	"testing"

	"github.com/exedev/docstor/internal/checklists"
	"github.com/exedev/docstor/internal/testutil"
)

func TestChecklists_CRUD(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := checklists.NewRepository(pool)
	ctx := context.Background()
	f := testutil.QuickFixture(t, pool, "CLTest", "cl@cl.com", "editor")

	// Create
	cl, err := repo.Create(ctx, checklists.CreateInput{
		TenantID:    f.TenantID,
		CreatedBy:   f.UserID,
		Name:        "Deploy Checklist",
		Description: "Pre-deploy steps",
		Items:       []string{"Backup DB", "Notify team", "Run tests"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if cl.Name != "Deploy Checklist" {
		t.Errorf("Name = %q, want Deploy Checklist", cl.Name)
	}

	// Get
	got, err := repo.Get(ctx, f.TenantID, cl.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Items) != 3 {
		t.Errorf("Items = %d, want 3", len(got.Items))
	}

	// List
	list, err := repo.List(ctx, f.TenantID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	found := false
	for _, item := range list {
		if item.ID == cl.ID {
			found = true
		}
	}
	if !found {
		t.Error("checklist not found in list")
	}

	// Delete
	if err := repo.Delete(ctx, f.TenantID, cl.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = repo.Get(ctx, f.TenantID, cl.ID)
	if err != checklists.ErrNotFound {
		t.Errorf("after delete: got %v, want ErrNotFound", err)
	}
}

func TestChecklists_InstanceToggleAndAutoComplete(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := checklists.NewRepository(pool)
	ctx := context.Background()
	f := testutil.QuickFixture(t, pool, "CLInst", "inst@cl.com", "editor")

	// Create checklist with 2 items
	cl, err := repo.Create(ctx, checklists.CreateInput{
		TenantID:  f.TenantID,
		CreatedBy: f.UserID,
		Name:      "Short List",
		Items:     []string{"Step 1", "Step 2"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Start instance
	inst, err := repo.StartInstance(ctx, checklists.StartInput{
		TenantID:    f.TenantID,
		ChecklistID: cl.ID,
		CreatedBy:   f.UserID,
	})
	if err != nil {
		t.Fatalf("StartInstance: %v", err)
	}
	if inst.Status != "in_progress" {
		t.Errorf("Status = %q, want in_progress", inst.Status)
	}

	// Get instance items
	items, err := repo.ListInstanceItems(ctx, f.TenantID, inst.ID)
	if err != nil {
		t.Fatalf("ListInstanceItems: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}

	// Toggle first item
	toggled, err := repo.ToggleItem(ctx, f.TenantID, inst.ID, items[0].ItemID, f.UserID)
	if err != nil {
		t.Fatalf("ToggleItem 1: %v", err)
	}
	if !toggled.Done {
		t.Error("item 1 should be done after toggle")
	}

	// Instance still in_progress
	instAfter1, err := repo.GetInstance(ctx, f.TenantID, inst.ID)
	if err != nil {
		t.Fatalf("GetInstance: %v", err)
	}
	if instAfter1.Status != "in_progress" {
		t.Errorf("Status after 1 toggle = %q, want in_progress", instAfter1.Status)
	}

	// Toggle second item — should auto-complete
	_, err = repo.ToggleItem(ctx, f.TenantID, inst.ID, items[1].ItemID, f.UserID)
	if err != nil {
		t.Fatalf("ToggleItem 2: %v", err)
	}

	instAfter2, err := repo.GetInstance(ctx, f.TenantID, inst.ID)
	if err != nil {
		t.Fatalf("GetInstance: %v", err)
	}
	if instAfter2.Status != "completed" {
		t.Errorf("Status after all toggles = %q, want completed", instAfter2.Status)
	}

	// Toggle first item back off
	untoggled, err := repo.ToggleItem(ctx, f.TenantID, inst.ID, items[0].ItemID, f.UserID)
	if err != nil {
		t.Fatalf("ToggleItem untoggle: %v", err)
	}
	if untoggled.Done {
		t.Error("item 1 should be undone after re-toggle")
	}

	instAfter3, err := repo.GetInstance(ctx, f.TenantID, inst.ID)
	if err != nil {
		t.Fatalf("GetInstance: %v", err)
	}
	if instAfter3.Status != "in_progress" {
		t.Errorf("Status after untoggle = %q, want in_progress", instAfter3.Status)
	}
}

func TestChecklists_TenantIsolation(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := checklists.NewRepository(pool)
	ctx := context.Background()

	fA := testutil.QuickFixture(t, pool, "CLIsoA", "a@cl.com", "editor")
	fB := testutil.QuickFixture(t, pool, "CLIsoB", "b@cl.com", "editor")

	cl, err := repo.Create(ctx, checklists.CreateInput{
		TenantID:  fA.TenantID,
		CreatedBy: fA.UserID,
		Name:      "Secret Checklist",
		Items:     []string{"Item 1"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = repo.Get(ctx, fB.TenantID, cl.ID)
	if err != checklists.ErrNotFound {
		t.Errorf("tenant B Get: got %v, want ErrNotFound", err)
	}

	listB, err := repo.List(ctx, fB.TenantID)
	if err != nil {
		t.Fatalf("List B: %v", err)
	}
	for _, item := range listB {
		if item.ID == cl.ID {
			t.Error("tenant B can see tenant A's checklist")
		}
	}
}
