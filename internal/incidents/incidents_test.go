package incidents_test

import (
	"context"
	"testing"
	"time"

	"github.com/acgh213/docstor/internal/incidents"
	"github.com/acgh213/docstor/internal/testutil"
)

func TestKnownIssues_CRUD(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := incidents.NewRepository(pool)
	ctx := context.Background()
	f := testutil.QuickFixture(t, pool, "KITest", "ki@inc.com", "editor")

	// Create
	ki, err := repo.CreateKnownIssue(ctx, incidents.CreateKnownIssueInput{
		TenantID:    f.TenantID,
		CreatedBy:   f.UserID,
		Title:       "DNS flapping",
		Severity:    "high",
		Status:      "open",
		Description: "DNS intermittently fails",
	})
	if err != nil {
		t.Fatalf("CreateKnownIssue: %v", err)
	}
	if ki.Title != "DNS flapping" {
		t.Errorf("Title = %q, want DNS flapping", ki.Title)
	}

	// Get
	got, err := repo.GetKnownIssue(ctx, f.TenantID, ki.ID)
	if err != nil {
		t.Fatalf("GetKnownIssue: %v", err)
	}
	if got.Severity != "high" {
		t.Errorf("Severity = %q, want high", got.Severity)
	}

	// Update
	updated, err := repo.UpdateKnownIssue(ctx, f.TenantID, ki.ID, incidents.UpdateKnownIssueInput{
		Title:       "DNS flapping",
		Severity:    "medium",
		Status:      "mitigated",
		Description: "Fixed with secondary DNS",
	})
	if err != nil {
		t.Fatalf("UpdateKnownIssue: %v", err)
	}
	if updated.Status != "mitigated" {
		t.Errorf("Status = %q, want mitigated", updated.Status)
	}

	// List
	list, err := repo.ListKnownIssues(ctx, f.TenantID, "", nil)
	if err != nil {
		t.Fatalf("ListKnownIssues: %v", err)
	}
	found := false
	for _, item := range list {
		if item.ID == ki.ID {
			found = true
		}
	}
	if !found {
		t.Error("known issue not found in list")
	}

	// Delete
	if err := repo.DeleteKnownIssue(ctx, f.TenantID, ki.ID); err != nil {
		t.Fatalf("DeleteKnownIssue: %v", err)
	}
	_, err = repo.GetKnownIssue(ctx, f.TenantID, ki.ID)
	if err != incidents.ErrNotFound {
		t.Errorf("after delete: got %v, want ErrNotFound", err)
	}
}

func TestIncidents_CRUDWithEvents(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := incidents.NewRepository(pool)
	ctx := context.Background()
	f := testutil.QuickFixture(t, pool, "IncTest", "inc@inc.com", "editor")

	// Create incident
	inc, err := repo.CreateIncident(ctx, incidents.CreateIncidentInput{
		TenantID:  f.TenantID,
		CreatedBy: f.UserID,
		Title:     "Outage",
		Severity:  "critical",
		Status:    "active",
		Summary:   "Full outage",
		StartedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}

	// Add events
	ev1, err := repo.CreateEvent(ctx, incidents.CreateEventInput{
		TenantID:    f.TenantID,
		IncidentID:  inc.ID,
		ActorUserID: f.UserID,
		EventType:   "detected",
		Detail:      "Monitoring alert fired",
	})
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}

	ev2, err := repo.CreateEvent(ctx, incidents.CreateEventInput{
		TenantID:    f.TenantID,
		IncidentID:  inc.ID,
		ActorUserID: f.UserID,
		EventType:   "mitigated",
		Detail:      "Failover activated",
	})
	if err != nil {
		t.Fatalf("CreateEvent 2: %v", err)
	}
	_ = ev1
	_ = ev2

	// List events
	events, err := repo.ListEvents(ctx, f.TenantID, inc.ID)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("event count = %d, want 2", len(events))
	}

	// Delete incident
	if err := repo.DeleteIncident(ctx, f.TenantID, inc.ID); err != nil {
		t.Fatalf("DeleteIncident: %v", err)
	}
}

func TestIncidents_TenantIsolation(t *testing.T) {
	pool := testutil.SetupDB(t)
	repo := incidents.NewRepository(pool)
	ctx := context.Background()

	fA := testutil.QuickFixture(t, pool, "IncIsoA", "a@inc.com", "editor")
	fB := testutil.QuickFixture(t, pool, "IncIsoB", "b@inc.com", "editor")

	ki, err := repo.CreateKnownIssue(ctx, incidents.CreateKnownIssueInput{
		TenantID: fA.TenantID, CreatedBy: fA.UserID,
		Title: "Secret issue", Severity: "low", Status: "open",
	})
	if err != nil {
		t.Fatalf("CreateKnownIssue: %v", err)
	}

	_, err = repo.GetKnownIssue(ctx, fB.TenantID, ki.ID)
	if err != incidents.ErrNotFound {
		t.Errorf("tenant B GetKnownIssue: got %v, want ErrNotFound", err)
	}

	inc, err := repo.CreateIncident(ctx, incidents.CreateIncidentInput{
		TenantID: fA.TenantID, CreatedBy: fA.UserID,
		Title: "Secret incident", Severity: "high", Status: "active",
		StartedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}

	_, err = repo.GetIncident(ctx, fB.TenantID, inc.ID)
	if err != incidents.ErrNotFound {
		t.Errorf("tenant B GetIncident: got %v, want ErrNotFound", err)
	}

	listB, err := repo.ListIncidents(ctx, fB.TenantID, "", nil)
	if err != nil {
		t.Fatalf("ListIncidents B: %v", err)
	}
	for _, item := range listB {
		if item.ID == inc.ID {
			t.Error("tenant B can see tenant A's incident")
		}
	}
}
