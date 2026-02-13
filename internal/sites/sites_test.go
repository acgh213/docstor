package sites_test

import (
	"context"
	"testing"

	"github.com/exedev/docstor/internal/sites"
	"github.com/exedev/docstor/internal/testutil"
)

func TestSites_CRUD(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := sites.NewRepository(pool)
	ctx := context.Background()
	f := testutil.QuickFixture(t, pool, "SiteTest", "site@site.com", "editor")
	clientID := testutil.CreateClient(t, pool, f.TenantID, "Site Client", "SC")

	// Create
	site, err := repo.Create(ctx, sites.CreateInput{
		TenantID: f.TenantID,
		ClientID: clientID,
		Name:     "HQ",
		Address:  "123 Main St",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if site.Name != "HQ" {
		t.Errorf("Name = %q, want HQ", site.Name)
	}

	// Get
	got, err := repo.Get(ctx, f.TenantID, site.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Address != "123 Main St" {
		t.Errorf("Address = %q, want 123 Main St", got.Address)
	}

	// Update
	updated, err := repo.Update(ctx, f.TenantID, site.ID, sites.UpdateInput{
		ClientID: clientID,
		Name:     "HQ Updated",
		Address:  "456 Oak Ave",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "HQ Updated" {
		t.Errorf("Name = %q, want HQ Updated", updated.Name)
	}

	// ListByClient
	list, err := repo.ListByClient(ctx, f.TenantID, clientID)
	if err != nil {
		t.Fatalf("ListByClient: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("ListByClient count = %d, want 1", len(list))
	}

	// Delete
	if err := repo.Delete(ctx, f.TenantID, site.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = repo.Get(ctx, f.TenantID, site.ID)
	if err != sites.ErrNotFound {
		t.Errorf("after delete: got %v, want ErrNotFound", err)
	}
}

func TestSites_TenantIsolation(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := sites.NewRepository(pool)
	ctx := context.Background()

	fA := testutil.QuickFixture(t, pool, "SiteIsoA", "a@site.com", "editor")
	fB := testutil.QuickFixture(t, pool, "SiteIsoB", "b@site.com", "editor")
	clientA := testutil.CreateClient(t, pool, fA.TenantID, "Client A", "CA")

	site, err := repo.Create(ctx, sites.CreateInput{
		TenantID: fA.TenantID,
		ClientID: clientA,
		Name:     "Secret Site",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = repo.Get(ctx, fB.TenantID, site.ID)
	if err != sites.ErrNotFound {
		t.Errorf("tenant B Get: got %v, want ErrNotFound", err)
	}

	listB, err := repo.List(ctx, fB.TenantID, nil)
	if err != nil {
		t.Fatalf("List B: %v", err)
	}
	for _, s := range listB {
		if s.ID == site.ID {
			t.Error("tenant B can see tenant A's site")
		}
	}
}
