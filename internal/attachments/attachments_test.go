package attachments_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/acgh213/docstor/internal/attachments"
	"github.com/acgh213/docstor/internal/docs"
	"github.com/acgh213/docstor/internal/testutil"
)

func TestAttachments_CreateAndGet(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := attachments.NewRepo(pool)
	ctx := context.Background()
	f := testutil.QuickFixture(t, pool, "AttTest", "att@att.com", "editor")

	att := &attachments.Attachment{
		ID:          uuid.New(),
		TenantID:    f.TenantID,
		Filename:    "report.pdf",
		ContentType: "application/pdf",
		SizeBytes:   12345,
		SHA256:      "abc123def456",
		StorageKey:  "tenants/" + f.TenantID.String() + "/ab/abc123def456",
		CreatedBy:   f.UserID,
	}
	if err := repo.CreateAttachment(ctx, att); err != nil {
		t.Fatalf("CreateAttachment: %v", err)
	}

	got, err := repo.GetAttachment(ctx, f.TenantID, att.ID)
	if err != nil {
		t.Fatalf("GetAttachment: %v", err)
	}
	if got.Filename != "report.pdf" {
		t.Errorf("Filename = %q, want report.pdf", got.Filename)
	}
	if got.SizeBytes != 12345 {
		t.Errorf("SizeBytes = %d, want 12345", got.SizeBytes)
	}
}

func TestAttachments_SHA256Dedup(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := attachments.NewRepo(pool)
	ctx := context.Background()
	f := testutil.QuickFixture(t, pool, "AttDedup", "dedup@att.com", "editor")

	sha := "deadbeef" + uuid.New().String()[:8]
	att := &attachments.Attachment{
		ID:          uuid.New(),
		TenantID:    f.TenantID,
		Filename:    "file1.txt",
		ContentType: "text/plain",
		SizeBytes:   100,
		SHA256:      sha,
		StorageKey:  "key1",
		CreatedBy:   f.UserID,
	}
	if err := repo.CreateAttachment(ctx, att); err != nil {
		t.Fatalf("CreateAttachment: %v", err)
	}

	found, err := repo.FindBySHA256(ctx, f.TenantID, sha)
	if err != nil {
		t.Fatalf("FindBySHA256: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find attachment by SHA256, got nil")
	}
	if found.ID != att.ID {
		t.Errorf("found ID = %s, want %s", found.ID, att.ID)
	}

	// Different SHA not found
	notFound, err := repo.FindBySHA256(ctx, f.TenantID, "nonexistent")
	if err != nil {
		t.Fatalf("FindBySHA256 miss: %v", err)
	}
	if notFound != nil {
		t.Error("expected nil for nonexistent SHA256")
	}
}

func TestAttachments_LinkAndList(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := attachments.NewRepo(pool)
	ctx := context.Background()
	f := testutil.QuickFixture(t, pool, "AttLink", "link@att.com", "editor")

	// Create a doc to link to
	docRepo := docs.NewRepository(pool)
	doc, err := docRepo.Create(ctx, docs.CreateInput{
		TenantID:  f.TenantID,
		Path:      "att-link-test-" + uuid.New().String()[:8],
		Title:     "Attachment Link Test",
		CreatedBy: f.UserID,
		Body:      "body",
		Message:   "init",
	})
	if err != nil {
		t.Fatalf("create doc: %v", err)
	}

	att := &attachments.Attachment{
		ID:          uuid.New(),
		TenantID:    f.TenantID,
		Filename:    "linked.pdf",
		ContentType: "application/pdf",
		SizeBytes:   500,
		SHA256:      "sha-" + uuid.New().String()[:8],
		StorageKey:  "key-linked",
		CreatedBy:   f.UserID,
	}
	if err := repo.CreateAttachment(ctx, att); err != nil {
		t.Fatalf("CreateAttachment: %v", err)
	}

	// Create link
	link := &attachments.AttachmentLink{
		ID:           uuid.New(),
		TenantID:     f.TenantID,
		AttachmentID: att.ID,
		LinkedType:   "document",
		LinkedID:     doc.ID,
	}
	if err := repo.CreateLink(ctx, link); err != nil {
		t.Fatalf("CreateLink: %v", err)
	}

	// ListByDocument
	list, err := repo.ListByDocument(ctx, f.TenantID, doc.ID)
	if err != nil {
		t.Fatalf("ListByDocument: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListByDocument count = %d, want 1", len(list))
	}
	if list[0].ID != att.ID {
		t.Errorf("listed attachment ID = %s, want %s", list[0].ID, att.ID)
	}
}

func TestAttachments_TenantIsolation(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := attachments.NewRepo(pool)
	ctx := context.Background()

	fA := testutil.QuickFixture(t, pool, "AttIsoA", "a@att.com", "editor")
	fB := testutil.QuickFixture(t, pool, "AttIsoB", "b@att.com", "editor")

	att := &attachments.Attachment{
		ID:          uuid.New(),
		TenantID:    fA.TenantID,
		Filename:    "secret.pdf",
		ContentType: "application/pdf",
		SizeBytes:   100,
		SHA256:      "iso-" + uuid.New().String()[:8],
		StorageKey:  "key-iso",
		CreatedBy:   fA.UserID,
	}
	if err := repo.CreateAttachment(ctx, att); err != nil {
		t.Fatalf("CreateAttachment: %v", err)
	}

	// Tenant B cannot get it
	got, err := repo.GetAttachment(ctx, fB.TenantID, att.ID)
	if err != nil {
		t.Fatalf("GetAttachment B: unexpected error %v", err)
	}
	if got != nil {
		t.Error("tenant B should not see tenant A's attachment")
	}

	// Tenant B list should not include it
	listB, err := repo.ListAll(ctx, fB.TenantID)
	if err != nil {
		t.Fatalf("ListAll B: %v", err)
	}
	for _, a := range listB {
		if a.ID == att.ID {
			t.Error("tenant B can see tenant A's attachment in list")
		}
	}
}

func TestEvidenceBundles_CRUD(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := attachments.NewRepo(pool)
	ctx := context.Background()
	f := testutil.QuickFixture(t, pool, "BundleTest", "bundle@att.com", "editor")

	// Create bundle
	bundle := &attachments.EvidenceBundle{
		ID:          uuid.New(),
		TenantID:    f.TenantID,
		Name:        "Q4 Audit",
		Description: "Quarterly evidence",
		CreatedBy:   f.UserID,
	}
	if err := repo.CreateBundle(ctx, bundle); err != nil {
		t.Fatalf("CreateBundle: %v", err)
	}

	// Get bundle
	got, err := repo.GetBundle(ctx, f.TenantID, bundle.ID)
	if err != nil {
		t.Fatalf("GetBundle: %v", err)
	}
	if got.Name != "Q4 Audit" {
		t.Errorf("Name = %q, want Q4 Audit", got.Name)
	}

	// Create an attachment to add
	att := &attachments.Attachment{
		ID:          uuid.New(),
		TenantID:    f.TenantID,
		Filename:    "evidence.pdf",
		ContentType: "application/pdf",
		SizeBytes:   999,
		SHA256:      "bnd-" + uuid.New().String()[:8],
		StorageKey:  "key-bundle",
		CreatedBy:   f.UserID,
	}
	if err := repo.CreateAttachment(ctx, att); err != nil {
		t.Fatalf("CreateAttachment: %v", err)
	}

	// Add item to bundle
	item := &attachments.EvidenceBundleItem{
		ID:           uuid.New(),
		TenantID:     f.TenantID,
		BundleID:     bundle.ID,
		AttachmentID: att.ID,
		Note:         "Q4 compliance report",
	}
	if err := repo.AddBundleItem(ctx, item); err != nil {
		t.Fatalf("AddBundleItem: %v", err)
	}

	// List items
	items, err := repo.ListBundleItems(ctx, f.TenantID, bundle.ID)
	if err != nil {
		t.Fatalf("ListBundleItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}

	// Remove item
	if err := repo.RemoveBundleItem(ctx, f.TenantID, bundle.ID, att.ID); err != nil {
		t.Fatalf("RemoveBundleItem: %v", err)
	}

	items2, _ := repo.ListBundleItems(ctx, f.TenantID, bundle.ID)
	if len(items2) != 0 {
		t.Errorf("items after remove = %d, want 0", len(items2))
	}

	// Delete bundle
	if err := repo.DeleteBundle(ctx, f.TenantID, bundle.ID); err != nil {
		t.Fatalf("DeleteBundle: %v", err)
	}
}
