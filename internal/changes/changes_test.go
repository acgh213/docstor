package changes_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/acgh213/docstor/internal/changes"
	"github.com/acgh213/docstor/internal/testutil"
)

func TestChanges_CRUD(t *testing.T) {
	pool := testutil.SetupDB(t)
	tid := testutil.CreateTenant(t, pool, "Changes-CRUD")
	uid := testutil.CreateUser(t, pool, "changes-crud@test.com", "Changes User", "password123")
	testutil.CreateMembership(t, pool, tid, uid, "editor")

	repo := changes.NewRepository(pool)
	ctx := context.Background()

	now := time.Now()
	ch, err := repo.Create(ctx, changes.CreateInput{
		TenantID:            tid,
		Title:               "Upgrade Firewall",
		DescriptionMarkdown: "Upgrade to v3.0",
		RiskLevel:           "high",
		WindowStart:         &now,
		CreatedBy:           uid,
	})
	if err != nil {
		t.Fatal(err)
	}
	if ch.Title != "Upgrade Firewall" {
		t.Errorf("title = %q, want Upgrade Firewall", ch.Title)
	}
	if ch.Status != "draft" {
		t.Errorf("status = %q, want draft", ch.Status)
	}
	if ch.RiskLevel != "high" {
		t.Errorf("risk_level = %q, want high", ch.RiskLevel)
	}

	// Get
	got, err := repo.Get(ctx, tid, ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Upgrade Firewall" {
		t.Errorf("get title = %q", got.Title)
	}

	// Update
	updated, err := repo.Update(ctx, tid, ch.ID, changes.UpdateInput{
		Title:               "Upgrade Firewall v2",
		DescriptionMarkdown: "Updated desc",
		RiskLevel:           "medium",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Upgrade Firewall v2" {
		t.Errorf("updated title = %q", updated.Title)
	}
	if updated.RiskLevel != "medium" {
		t.Errorf("updated risk = %q", updated.RiskLevel)
	}

	// List
	list, err := repo.List(ctx, tid, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Errorf("list length = %d, want 1", len(list))
	}

	// Delete (only draft)
	err = repo.Delete(ctx, tid, ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.Get(ctx, tid, ch.ID)
	if err != changes.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestChanges_StatusTransitions(t *testing.T) {
	pool := testutil.SetupDB(t)
	tid := testutil.CreateTenant(t, pool, "Changes-Trans")
	uid := testutil.CreateUser(t, pool, "changes-trans@test.com", "Trans User", "password123")
	testutil.CreateMembership(t, pool, tid, uid, "editor")

	repo := changes.NewRepository(pool)
	ctx := context.Background()

	ch, _ := repo.Create(ctx, changes.CreateInput{
		TenantID:  tid,
		Title:     "Test Transition",
		RiskLevel: "low",
		CreatedBy: uid,
	})

	// draft -> approved
	ch, err := repo.Transition(ctx, tid, ch.ID, uid, "approved")
	if err != nil {
		t.Fatal(err)
	}
	if ch.Status != "approved" {
		t.Errorf("status = %q, want approved", ch.Status)
	}
	if ch.ApprovedBy == nil {
		t.Error("approved_by should be set")
	}

	// approved -> in_progress
	ch, err = repo.Transition(ctx, tid, ch.ID, uid, "in_progress")
	if err != nil {
		t.Fatal(err)
	}
	if ch.Status != "in_progress" {
		t.Errorf("status = %q, want in_progress", ch.Status)
	}

	// in_progress -> completed
	ch, err = repo.Transition(ctx, tid, ch.ID, uid, "completed")
	if err != nil {
		t.Fatal(err)
	}
	if ch.Status != "completed" {
		t.Errorf("status = %q, want completed", ch.Status)
	}
	if ch.CompletedAt == nil {
		t.Error("completed_at should be set")
	}

	// Invalid: completed -> draft
	_, err = repo.Transition(ctx, tid, ch.ID, uid, "draft")
	if err == nil {
		t.Error("expected error for invalid transition")
	}
}

func TestChanges_RollbackTransition(t *testing.T) {
	pool := testutil.SetupDB(t)
	tid := testutil.CreateTenant(t, pool, "Changes-Rollback")
	uid := testutil.CreateUser(t, pool, "changes-rollback@test.com", "Roll User", "password123")
	testutil.CreateMembership(t, pool, tid, uid, "editor")

	repo := changes.NewRepository(pool)
	ctx := context.Background()

	ch, _ := repo.Create(ctx, changes.CreateInput{
		TenantID: tid, Title: "Rollback Test", RiskLevel: "low", CreatedBy: uid,
	})
	repo.Transition(ctx, tid, ch.ID, uid, "approved")
	repo.Transition(ctx, tid, ch.ID, uid, "in_progress")
	ch, err := repo.Transition(ctx, tid, ch.ID, uid, "rolled_back")
	if err != nil {
		t.Fatal(err)
	}
	if ch.Status != "rolled_back" {
		t.Errorf("status = %q, want rolled_back", ch.Status)
	}
}

func TestChanges_DeleteOnlyDraftOrCancelled(t *testing.T) {
	pool := testutil.SetupDB(t)
	tid := testutil.CreateTenant(t, pool, "Changes-DelRestrict")
	uid := testutil.CreateUser(t, pool, "changes-delrestrict@test.com", "Del User", "password123")
	testutil.CreateMembership(t, pool, tid, uid, "editor")

	repo := changes.NewRepository(pool)
	ctx := context.Background()

	ch, _ := repo.Create(ctx, changes.CreateInput{
		TenantID: tid, Title: "No Delete", RiskLevel: "low", CreatedBy: uid,
	})
	repo.Transition(ctx, tid, ch.ID, uid, "approved")

	// Cannot delete approved
	err := repo.Delete(ctx, tid, ch.ID)
	if err == nil {
		t.Error("should not be able to delete approved change")
	}
}

func TestChanges_Links(t *testing.T) {
	pool := testutil.SetupDB(t)
	tid := testutil.CreateTenant(t, pool, "Changes-Links")
	uid := testutil.CreateUser(t, pool, "changes-links@test.com", "Link User", "password123")
	testutil.CreateMembership(t, pool, tid, uid, "editor")

	// Create a doc to link to
	var docID uuid.UUID
	pool.QueryRow(context.Background(),
		`INSERT INTO documents (tenant_id, path, title, doc_type, sensitivity, created_by) VALUES ($1, 'test/change-link', 'Test Doc', 'doc', 'public-internal', $2) RETURNING id`,
		tid, uid).Scan(&docID)

	repo := changes.NewRepository(pool)
	ctx := context.Background()

	ch, _ := repo.Create(ctx, changes.CreateInput{
		TenantID: tid, Title: "With Links", RiskLevel: "low", CreatedBy: uid,
	})

	// Add link
	err := repo.AddLink(ctx, tid, ch.ID, "document", docID)
	if err != nil {
		t.Fatal(err)
	}

	// List links
	links, err := repo.ListLinks(ctx, tid, ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].LinkedType != "document" {
		t.Errorf("linked_type = %q, want document", links[0].LinkedType)
	}
	if links[0].LinkedName != "Test Doc" {
		t.Errorf("linked_name = %q, want Test Doc", links[0].LinkedName)
	}

	// Remove link
	err = repo.RemoveLink(ctx, tid, links[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	links, _ = repo.ListLinks(ctx, tid, ch.ID)
	if len(links) != 0 {
		t.Errorf("expected 0 links after removal, got %d", len(links))
	}
}

func TestChanges_TenantIsolation(t *testing.T) {
	pool := testutil.SetupDB(t)
	tid1 := testutil.CreateTenant(t, pool, "Changes-T1")
	tid2 := testutil.CreateTenant(t, pool, "Changes-T2")
	uid1 := testutil.CreateUser(t, pool, "changes-t1@test.com", "T1 User", "password123")
	uid2 := testutil.CreateUser(t, pool, "changes-t2@test.com", "T2 User", "password123")
	testutil.CreateMembership(t, pool, tid1, uid1, "editor")
	testutil.CreateMembership(t, pool, tid2, uid2, "editor")

	repo := changes.NewRepository(pool)
	ctx := context.Background()

	repo.Create(ctx, changes.CreateInput{TenantID: tid1, Title: "T1 Change", RiskLevel: "low", CreatedBy: uid1})
	repo.Create(ctx, changes.CreateInput{TenantID: tid2, Title: "T2 Change", RiskLevel: "low", CreatedBy: uid2})

	list1, _ := repo.List(ctx, tid1, "", nil)
	list2, _ := repo.List(ctx, tid2, "", nil)

	if len(list1) != 1 || list1[0].Title != "T1 Change" {
		t.Errorf("T1 should only see its own change")
	}
	if len(list2) != 1 || list2[0].Title != "T2 Change" {
		t.Errorf("T2 should only see its own change")
	}
}
