package web

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/exedev/docstor/internal/audit"
	"github.com/exedev/docstor/internal/auth"
	"github.com/exedev/docstor/internal/docs"
	"github.com/exedev/docstor/internal/runbooks"
)

type RunbooksPageData struct {
	PageData
	Overdue          []runbooks.RunbookWithStatus
	Unowned          []runbooks.RunbookWithStatus
	RecentlyVerified []runbooks.RunbookWithStatus
	AllRunbooks      []runbooks.RunbookWithStatus
}

func (s *Server) handleRunbooksDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	pageData := RunbooksPageData{
		PageData: s.newPageData(r),
	}
	pageData.Title = "Runbooks - Docstor"

	// Load all lists
	overdue, err := s.runbooks.ListOverdue(ctx, tenant.ID)
	if err != nil {
		slog.Error("failed to load overdue runbooks", "error", err)
	}
	pageData.Overdue = overdue

	unowned, err := s.runbooks.ListUnowned(ctx, tenant.ID)
	if err != nil {
		slog.Error("failed to load unowned runbooks", "error", err)
	}
	pageData.Unowned = unowned

	recent, err := s.runbooks.ListRecentlyVerified(ctx, tenant.ID)
	if err != nil {
		slog.Error("failed to load recently verified runbooks", "error", err)
	}
	pageData.RecentlyVerified = recent

	s.templates.ExecuteTemplate(w, "runbooks_dashboard.html", pageData)
}

func (s *Server) handleRunbookVerify(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	docID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Get the document to verify it's a runbook
	doc, err := s.docs.GetByID(ctx, tenant.ID, docID)
	if errors.Is(err, docs.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get document", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if doc.DocType != docs.DocTypeRunbook {
		http.Error(w, "Document is not a runbook", http.StatusBadRequest)
		return
	}

	// Ensure runbook_status exists
	if err := s.runbooks.EnsureStatus(ctx, tenant.ID, docID, 90); err != nil {
		slog.Error("failed to ensure runbook status", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Perform verification
	status, err := s.runbooks.Verify(ctx, tenant.ID, docID, user.ID)
	if err != nil {
		slog.Error("failed to verify runbook", "error", err)
		http.Error(w, "Failed to verify runbook", http.StatusInternalServerError)
		return
	}

	// Audit log
	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionRunbookVerify,
		TargetType:  audit.TargetDocument,
		TargetID:    &docID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata: map[string]any{
			"path":           doc.Path,
			"title":          doc.Title,
			"next_due_at":    status.NextDueAt,
			"interval_days":  status.VerificationIntervalDays,
		},
	})

	// Redirect back to the document
	setFlashSuccess(w, "Runbook verified successfully")
	http.Redirect(w, r, "/docs/"+doc.Path, http.StatusSeeOther)
}

func (s *Server) handleRunbookUpdateInterval(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	docID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	intervalStr := r.FormValue("interval_days")
	interval, err := strconv.Atoi(intervalStr)
	if err != nil || interval < 1 || interval > 365 {
		http.Error(w, "Invalid interval (must be 1-365 days)", http.StatusBadRequest)
		return
	}

	// Ensure status exists first
	if err := s.runbooks.EnsureStatus(ctx, tenant.ID, docID, interval); err != nil {
		slog.Error("failed to ensure runbook status", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Update interval
	if err := s.runbooks.UpdateInterval(ctx, tenant.ID, docID, interval); err != nil {
		slog.Error("failed to update interval", "error", err)
		http.Error(w, "Failed to update interval", http.StatusInternalServerError)
		return
	}

	// Get doc for redirect
	doc, err := s.docs.GetByID(ctx, tenant.ID, docID)
	if err != nil {
		http.Redirect(w, r, "/runbooks", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/docs/"+doc.Path, http.StatusSeeOther)
}
