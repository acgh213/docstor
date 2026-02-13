package docs_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/acgh213/docstor/internal/clients"
	"github.com/acgh213/docstor/internal/docs"
	"github.com/acgh213/docstor/internal/testutil"
)

// ---------------------------------------------------------------------------
// Tenant isolation
// ---------------------------------------------------------------------------

func TestTenantIsolation_Documents(t *testing.T) {
	pool := testutil.SetupDB(t)
	ctx := context.Background()
	repo := docs.NewRepository(pool)

	// Two tenants, each with a user.
	fixA := testutil.QuickFixture(t, pool, "Tenant-A", "a@example.com", "admin")
	fixB := testutil.QuickFixture(t, pool, "Tenant-B", "b@example.com", "admin")

	// Tenant A creates a document.
	docA, err := repo.Create(ctx, docs.CreateInput{
		TenantID:  fixA.TenantID,
		Path:      "secret/playbook",
		Title:     "Secret Playbook",
		CreatedBy: fixA.UserID,
		Body:      "Top-secret body from Tenant A.",
		Message:   "init",
	})
	if err != nil {
		t.Fatalf("create doc: %v", err)
	}

	// Tenant B must NOT see it in List.
	listB, err := repo.List(ctx, fixB.TenantID, nil, nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, d := range listB {
		if d.ID == docA.ID {
			t.Fatal("Tenant B can see Tenant A's document in List")
		}
	}

	// Tenant B must NOT get it by ID.
	_, err = repo.GetByID(ctx, fixB.TenantID, docA.ID)
	if err != docs.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// Tenant B must NOT get it by path.
	_, err = repo.GetByPath(ctx, fixB.TenantID, "secret/playbook")
	if err != docs.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestTenantIsolation_Revisions(t *testing.T) {
	pool := testutil.SetupDB(t)
	ctx := context.Background()
	repo := docs.NewRepository(pool)

	fixA := testutil.QuickFixture(t, pool, "Tenant-A", "a@rev.com", "admin")
	fixB := testutil.QuickFixture(t, pool, "Tenant-B", "b@rev.com", "admin")

	docA, err := repo.Create(ctx, docs.CreateInput{
		TenantID:  fixA.TenantID,
		Path:      "rev-test",
		Title:     "Rev Test",
		CreatedBy: fixA.UserID,
		Body:      "body v1",
		Message:   "init",
	})
	if err != nil {
		t.Fatalf("create doc: %v", err)
	}

	// Tenant B cannot read Tenant A's revision.
	_, err = repo.GetRevision(ctx, fixB.TenantID, *docA.CurrentRevisionID)
	if err != docs.ErrNotFound {
		t.Fatalf("expected ErrNotFound for cross-tenant revision, got %v", err)
	}

	// ListRevisions returns nothing for Tenant B.
	revs, err := repo.ListRevisions(ctx, fixB.TenantID, docA.ID)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(revs) != 0 {
		t.Fatalf("expected 0 revisions for tenant B, got %d", len(revs))
	}
}

func TestTenantIsolation_Clients(t *testing.T) {
	pool := testutil.SetupDB(t)
	ctx := context.Background()
	clientRepo := clients.NewRepository(pool)

	fixA := testutil.QuickFixture(t, pool, "Tenant-A", "a@cli.com", "admin")
	fixB := testutil.QuickFixture(t, pool, "Tenant-B", "b@cli.com", "admin")

	// Tenant A creates a client.
	cA, err := clientRepo.Create(ctx, clients.CreateInput{
		TenantID: fixA.TenantID,
		Name:     "Client Alpha",
		Code:     "ALPHA",
	})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	// Tenant B cannot list it.
	listB, err := clientRepo.List(ctx, fixB.TenantID)
	if err != nil {
		t.Fatalf("list clients: %v", err)
	}
	for _, c := range listB {
		if c.ID == cA.ID {
			t.Fatal("Tenant B can see Tenant A's client")
		}
	}

	// Tenant B cannot Get it.
	_, err = clientRepo.Get(ctx, fixB.TenantID, cA.ID)
	if err != clients.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestTenantIsolation_Search(t *testing.T) {
	pool := testutil.SetupDB(t)
	ctx := context.Background()
	repo := docs.NewRepository(pool)

	fixA := testutil.QuickFixture(t, pool, "Tenant-A", "a@search.com", "admin")
	fixB := testutil.QuickFixture(t, pool, "Tenant-B", "b@search.com", "admin")

	_, err := repo.Create(ctx, docs.CreateInput{
		TenantID:  fixA.TenantID,
		Path:      "search-isolation",
		Title:     "Unique Elephantine Document",
		CreatedBy: fixA.UserID,
		Body:      "The elephantine content is here.",
		Message:   "init",
	})
	if err != nil {
		t.Fatalf("create doc: %v", err)
	}

	// Tenant B searches for same keyword — must get 0 results.
	results, err := repo.Search(ctx, fixB.TenantID, "elephantine", docs.SearchFilters{}, 50)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 search results for Tenant B, got %d", len(results))
	}

	// Tenant A should find it.
	results, err = repo.Search(ctx, fixA.TenantID, "elephantine", docs.SearchFilters{}, 50)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results for Tenant A, got 0")
	}
}

// ---------------------------------------------------------------------------
// Revision conflict detection
// ---------------------------------------------------------------------------

func TestRevisionConflictDetection(t *testing.T) {
	pool := testutil.SetupDB(t)
	ctx := context.Background()
	repo := docs.NewRepository(pool)

	fix := testutil.QuickFixture(t, pool, "Conflict-Tenant", "conflict@test.com", "admin")

	doc, err := repo.Create(ctx, docs.CreateInput{
		TenantID:  fix.TenantID,
		Path:      "conflict-test",
		Title:     "Conflict Test",
		CreatedBy: fix.UserID,
		Body:      "original body",
		Message:   "v1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	rev1 := *doc.CurrentRevisionID

	// Update with correct base → succeeds.
	doc2, err := repo.Update(ctx, fix.TenantID, doc.ID, docs.UpdateInput{
		Body:           "updated body",
		Message:        "v2",
		BaseRevisionID: rev1,
		UpdatedBy:      fix.UserID,
	})
	if err != nil {
		t.Fatalf("update with correct base: %v", err)
	}

	// current_revision_id must have changed.
	if *doc2.CurrentRevisionID == rev1 {
		t.Fatal("current_revision_id should have changed after update")
	}

	// Update with STALE base → ErrConflict.
	_, err = repo.Update(ctx, fix.TenantID, doc.ID, docs.UpdateInput{
		Body:           "this should fail",
		Message:        "stale",
		BaseRevisionID: rev1, // stale!
		UpdatedBy:      fix.UserID,
	})
	if err != docs.ErrConflict {
		t.Fatalf("expected ErrConflict for stale base, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Revert
// ---------------------------------------------------------------------------

func TestRevert(t *testing.T) {
	pool := testutil.SetupDB(t)
	ctx := context.Background()
	repo := docs.NewRepository(pool)

	fix := testutil.QuickFixture(t, pool, "Revert-Tenant", "revert@test.com", "admin")

	// Create doc (revision 1).
	doc, err := repo.Create(ctx, docs.CreateInput{
		TenantID:  fix.TenantID,
		Path:      "revert-test",
		Title:     "Revert Test",
		CreatedBy: fix.UserID,
		Body:      "body version 1",
		Message:   "v1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	rev1ID := *doc.CurrentRevisionID

	// Update → revision 2.
	doc, err = repo.Update(ctx, fix.TenantID, doc.ID, docs.UpdateInput{
		Body:           "body version 2",
		Message:        "v2",
		BaseRevisionID: rev1ID,
		UpdatedBy:      fix.UserID,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	rev2ID := *doc.CurrentRevisionID

	// Revert to revision 1 → creates revision 3.
	doc, err = repo.Revert(ctx, fix.TenantID, doc.ID, rev1ID, fix.UserID)
	if err != nil {
		t.Fatalf("revert: %v", err)
	}
	rev3ID := *doc.CurrentRevisionID

	// rev3 must be new.
	if rev3ID == rev1ID || rev3ID == rev2ID {
		t.Fatalf("revert should create a new revision, got %s", rev3ID)
	}

	// rev3 body matches rev1.
	rev3, err := repo.GetRevision(ctx, fix.TenantID, rev3ID)
	if err != nil {
		t.Fatalf("get rev3: %v", err)
	}
	if rev3.BodyMarkdown != "body version 1" {
		t.Fatalf("rev3 body = %q, want %q", rev3.BodyMarkdown, "body version 1")
	}

	// rev2 still exists (never deleted).
	rev2, err := repo.GetRevision(ctx, fix.TenantID, rev2ID)
	if err != nil {
		t.Fatalf("rev2 should still exist: %v", err)
	}
	if rev2.BodyMarkdown != "body version 2" {
		t.Fatalf("rev2 body = %q, want %q", rev2.BodyMarkdown, "body version 2")
	}

	// document.current_revision_id == rev3.
	fresh, err := repo.GetByID(ctx, fix.TenantID, doc.ID)
	if err != nil {
		t.Fatalf("get doc: %v", err)
	}
	if *fresh.CurrentRevisionID != rev3ID {
		t.Fatalf("document current_revision_id = %s, want %s", *fresh.CurrentRevisionID, rev3ID)
	}

	// Total revisions = 3.
	all, err := repo.ListRevisions(ctx, fix.TenantID, doc.ID)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 revisions, got %d", len(all))
	}
}

// ---------------------------------------------------------------------------
// Sensitivity access helper (unit-level, no DB)
// ---------------------------------------------------------------------------

func TestCanAccess(t *testing.T) {
	tests := []struct {
		role        string
		sensitivity docs.Sensitivity
		want        bool
	}{
		{"reader", docs.SensitivityPublic, true},
		{"editor", docs.SensitivityPublic, true},
		{"admin", docs.SensitivityPublic, true},
		{"reader", docs.SensitivityRestricted, false},
		{"editor", docs.SensitivityRestricted, true},
		{"admin", docs.SensitivityRestricted, true},
		{"reader", docs.SensitivityConfidential, false},
		{"editor", docs.SensitivityConfidential, true},
		{"admin", docs.SensitivityConfidential, true},
	}
	for _, tt := range tests {
		name := fmt.Sprintf("%s/%s", tt.role, tt.sensitivity)
		t.Run(name, func(t *testing.T) {
			got := docs.CanAccess(tt.role, tt.sensitivity)
			if got != tt.want {
				t.Errorf("CanAccess(%q, %q) = %v, want %v", tt.role, tt.sensitivity, got, tt.want)
			}
		})
	}
}


