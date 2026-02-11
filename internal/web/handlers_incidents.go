package web

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/exedev/docstor/internal/audit"
	"github.com/exedev/docstor/internal/auth"
	"github.com/exedev/docstor/internal/clients"
	"github.com/exedev/docstor/internal/incidents"
)

// --- Content structs ---

type KnownIssuesListData struct {
	Issues           []incidents.KnownIssue
	Clients          []clients.Client
	SelectedClientID string
	SelectedStatus   string
}

type KnownIssueFormData struct {
	Issue   *incidents.KnownIssue
	Clients []clients.Client
}

type IncidentsListData struct {
	Incidents        []incidents.Incident
	Clients          []clients.Client
	SelectedClientID string
	SelectedStatus   string
}

type IncidentFormData struct {
	Incident *incidents.Incident
	Clients  []clients.Client
}

type IncidentViewData struct {
	Incident *incidents.Incident
	Events   []incidents.IncidentEvent
}

// ===================== KNOWN ISSUES =====================

func (s *Server) handleKnownIssuesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	status := r.URL.Query().Get("status")
	var clientFilter *uuid.UUID
	if cidStr := r.URL.Query().Get("client_id"); cidStr != "" {
		if cid, err := uuid.Parse(cidStr); err == nil {
			clientFilter = &cid
		}
	}

	issues, err := s.incidents.ListKnownIssues(ctx, tenant.ID, status, clientFilter)
	if err != nil {
		slog.Error("failed to list known issues", "error", err)
		data := s.newPageData(r)
		data.Title = "Known Issues - Docstor"
		data.Error = "Failed to load known issues"
		s.render(w, r, "known_issues_list.html", data)
		return
	}

	clientList, _ := s.clients.List(ctx, tenant.ID)

	data := s.newPageData(r)
	data.Title = "Known Issues - Docstor"
	data.Content = KnownIssuesListData{
		Issues:           issues,
		Clients:          clientList,
		SelectedClientID: r.URL.Query().Get("client_id"),
		SelectedStatus:   status,
	}
	s.render(w, r, "known_issues_list.html", data)
}

func (s *Server) handleKnownIssueNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	clientList, _ := s.clients.List(ctx, tenant.ID)

	data := s.newPageData(r)
	data.Title = "New Known Issue - Docstor"
	data.Content = KnownIssueFormData{Clients: clientList}
	s.render(w, r, "known_issue_form.html", data)
}

func (s *Server) handleKnownIssueCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "New Known Issue - Docstor"
		data.Error = "Invalid form data"
		data.Content = KnownIssueFormData{Clients: clientList}
		s.render(w, r, "known_issue_form.html", data)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	severity := strings.TrimSpace(r.FormValue("severity"))
	statusVal := strings.TrimSpace(r.FormValue("status"))
	description := strings.TrimSpace(r.FormValue("description"))
	workaround := strings.TrimSpace(r.FormValue("workaround"))
	clientID := parseOptionalClientID(r)

	var linkedDocID *uuid.UUID
	if ldStr := strings.TrimSpace(r.FormValue("linked_document_id")); ldStr != "" {
		if ld, err := uuid.Parse(ldStr); err == nil {
			linkedDocID = &ld
		}
	}

	if title == "" || severity == "" || statusVal == "" {
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "New Known Issue - Docstor"
		data.Error = "Title, severity, and status are required"
		data.Content = KnownIssueFormData{Clients: clientList}
		s.render(w, r, "known_issue_form.html", data)
		return
	}

	ki, err := s.incidents.CreateKnownIssue(ctx, incidents.CreateKnownIssueInput{
		TenantID:         tenant.ID,
		ClientID:         clientID,
		Title:            title,
		Severity:         severity,
		Status:           statusVal,
		Description:      description,
		Workaround:       workaround,
		LinkedDocumentID: linkedDocID,
		CreatedBy:        user.ID,
	})
	if err != nil {
		slog.Error("failed to create known issue", "error", err)
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "New Known Issue - Docstor"
		data.Error = "Failed to create known issue"
		data.Content = KnownIssueFormData{Clients: clientList}
		s.render(w, r, "known_issue_form.html", data)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionKnownIssueCreate,
		TargetType:  audit.TargetKnownIssue,
		TargetID:    &ki.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"title": title, "severity": severity},
	})

	setFlashSuccess(w, "Known issue created successfully")
	http.Redirect(w, r, "/known-issues/"+ki.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleKnownIssueView(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	ki, err := s.incidents.GetKnownIssue(ctx, tenant.ID, id)
	if errors.Is(err, incidents.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get known issue", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := s.newPageData(r)
	data.Title = ki.Title + " - Docstor"
	data.Content = ki
	s.render(w, r, "known_issue_view.html", data)
}

func (s *Server) handleKnownIssueEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	ki, err := s.incidents.GetKnownIssue(ctx, tenant.ID, id)
	if errors.Is(err, incidents.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get known issue", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	clientList, _ := s.clients.List(ctx, tenant.ID)

	data := s.newPageData(r)
	data.Title = "Edit " + ki.Title + " - Docstor"
	data.Content = KnownIssueFormData{Issue: ki, Clients: clientList}
	s.render(w, r, "known_issue_form.html", data)
}

func (s *Server) handleKnownIssueUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "Edit Known Issue - Docstor"
		data.Error = "Invalid form data"
		data.Content = KnownIssueFormData{Clients: clientList}
		s.render(w, r, "known_issue_form.html", data)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	severity := strings.TrimSpace(r.FormValue("severity"))
	statusVal := strings.TrimSpace(r.FormValue("status"))
	description := strings.TrimSpace(r.FormValue("description"))
	workaround := strings.TrimSpace(r.FormValue("workaround"))
	clientID := parseOptionalClientID(r)

	var linkedDocID *uuid.UUID
	if ldStr := strings.TrimSpace(r.FormValue("linked_document_id")); ldStr != "" {
		if ld, err := uuid.Parse(ldStr); err == nil {
			linkedDocID = &ld
		}
	}

	if title == "" || severity == "" || statusVal == "" {
		existing, _ := s.incidents.GetKnownIssue(ctx, tenant.ID, id)
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "Edit Known Issue - Docstor"
		data.Error = "Title, severity, and status are required"
		data.Content = KnownIssueFormData{Issue: existing, Clients: clientList}
		s.render(w, r, "known_issue_form.html", data)
		return
	}

	ki, err := s.incidents.UpdateKnownIssue(ctx, tenant.ID, id, incidents.UpdateKnownIssueInput{
		ClientID:         clientID,
		Title:            title,
		Severity:         severity,
		Status:           statusVal,
		Description:      description,
		Workaround:       workaround,
		LinkedDocumentID: linkedDocID,
	})
	if errors.Is(err, incidents.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to update known issue", "error", err)
		existing, _ := s.incidents.GetKnownIssue(ctx, tenant.ID, id)
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "Edit Known Issue - Docstor"
		data.Error = "Failed to update known issue"
		data.Content = KnownIssueFormData{Issue: existing, Clients: clientList}
		s.render(w, r, "known_issue_form.html", data)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionKnownIssueUpdate,
		TargetType:  audit.TargetKnownIssue,
		TargetID:    &ki.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"title": title, "severity": severity, "status": statusVal},
	})

	setFlashSuccess(w, "Known issue updated successfully")
	http.Redirect(w, r, "/known-issues/"+ki.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleKnownIssueDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	ki, err := s.incidents.GetKnownIssue(ctx, tenant.ID, id)
	if errors.Is(err, incidents.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get known issue for delete", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := s.incidents.DeleteKnownIssue(ctx, tenant.ID, id); err != nil {
		slog.Error("failed to delete known issue", "error", err)
		http.Error(w, "Failed to delete known issue", http.StatusInternalServerError)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionKnownIssueDelete,
		TargetType:  audit.TargetKnownIssue,
		TargetID:    &id,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"title": ki.Title},
	})

	setFlashSuccess(w, "Known issue deleted successfully")
	http.Redirect(w, r, "/known-issues", http.StatusSeeOther)
}

// ===================== INCIDENTS =====================

func (s *Server) handleIncidentsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	status := r.URL.Query().Get("status")
	var clientFilter *uuid.UUID
	if cidStr := r.URL.Query().Get("client_id"); cidStr != "" {
		if cid, err := uuid.Parse(cidStr); err == nil {
			clientFilter = &cid
		}
	}

	incidentList, err := s.incidents.ListIncidents(ctx, tenant.ID, status, clientFilter)
	if err != nil {
		slog.Error("failed to list incidents", "error", err)
		data := s.newPageData(r)
		data.Title = "Incidents - Docstor"
		data.Error = "Failed to load incidents"
		s.render(w, r, "incidents_list.html", data)
		return
	}

	clientList, _ := s.clients.List(ctx, tenant.ID)

	data := s.newPageData(r)
	data.Title = "Incidents - Docstor"
	data.Content = IncidentsListData{
		Incidents:        incidentList,
		Clients:          clientList,
		SelectedClientID: r.URL.Query().Get("client_id"),
		SelectedStatus:   status,
	}
	s.render(w, r, "incidents_list.html", data)
}

func (s *Server) handleIncidentNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	clientList, _ := s.clients.List(ctx, tenant.ID)

	data := s.newPageData(r)
	data.Title = "New Incident - Docstor"
	data.Content = IncidentFormData{Clients: clientList}
	s.render(w, r, "incident_form.html", data)
}

func (s *Server) handleIncidentCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "New Incident - Docstor"
		data.Error = "Invalid form data"
		data.Content = IncidentFormData{Clients: clientList}
		s.render(w, r, "incident_form.html", data)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	severity := strings.TrimSpace(r.FormValue("severity"))
	statusVal := strings.TrimSpace(r.FormValue("status"))
	summary := strings.TrimSpace(r.FormValue("summary"))
	clientID := parseOptionalClientID(r)

	startedAt := time.Now()
	if saStr := strings.TrimSpace(r.FormValue("started_at")); saStr != "" {
		if parsed, err := time.Parse("2006-01-02T15:04", saStr); err == nil {
			startedAt = parsed
		}
	}

	if title == "" || severity == "" || statusVal == "" {
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "New Incident - Docstor"
		data.Error = "Title, severity, and status are required"
		data.Content = IncidentFormData{Clients: clientList}
		s.render(w, r, "incident_form.html", data)
		return
	}

	inc, err := s.incidents.CreateIncident(ctx, incidents.CreateIncidentInput{
		TenantID:  tenant.ID,
		ClientID:  clientID,
		Title:     title,
		Severity:  severity,
		Status:    statusVal,
		StartedAt: startedAt,
		Summary:   summary,
		CreatedBy: user.ID,
	})
	if err != nil {
		slog.Error("failed to create incident", "error", err)
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "New Incident - Docstor"
		data.Error = "Failed to create incident"
		data.Content = IncidentFormData{Clients: clientList}
		s.render(w, r, "incident_form.html", data)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionIncidentCreate,
		TargetType:  audit.TargetIncident,
		TargetID:    &inc.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"title": title, "severity": severity},
	})

	setFlashSuccess(w, "Incident created successfully")
	http.Redirect(w, r, "/incidents/"+inc.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleIncidentView(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	inc, err := s.incidents.GetIncident(ctx, tenant.ID, id)
	if errors.Is(err, incidents.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get incident", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	events, err := s.incidents.ListEvents(ctx, tenant.ID, id)
	if err != nil {
		slog.Error("failed to list incident events", "error", err)
	}

	data := s.newPageData(r)
	data.Title = inc.Title + " - Docstor"
	data.Content = IncidentViewData{Incident: inc, Events: events}
	s.render(w, r, "incident_view.html", data)
}

func (s *Server) handleIncidentEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	inc, err := s.incidents.GetIncident(ctx, tenant.ID, id)
	if errors.Is(err, incidents.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get incident", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	clientList, _ := s.clients.List(ctx, tenant.ID)

	data := s.newPageData(r)
	data.Title = "Edit " + inc.Title + " - Docstor"
	data.Content = IncidentFormData{Incident: inc, Clients: clientList}
	s.render(w, r, "incident_form.html", data)
}

func (s *Server) handleIncidentUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "Edit Incident - Docstor"
		data.Error = "Invalid form data"
		data.Content = IncidentFormData{Clients: clientList}
		s.render(w, r, "incident_form.html", data)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	severity := strings.TrimSpace(r.FormValue("severity"))
	statusVal := strings.TrimSpace(r.FormValue("status"))
	summary := strings.TrimSpace(r.FormValue("summary"))
	clientID := parseOptionalClientID(r)

	startedAt := time.Now()
	if saStr := strings.TrimSpace(r.FormValue("started_at")); saStr != "" {
		if parsed, err := time.Parse("2006-01-02T15:04", saStr); err == nil {
			startedAt = parsed
		}
	}

	var endedAt *time.Time
	if eaStr := strings.TrimSpace(r.FormValue("ended_at")); eaStr != "" {
		if parsed, err := time.Parse("2006-01-02T15:04", eaStr); err == nil {
			endedAt = &parsed
		}
	}
	// Auto-set ended_at when resolved
	if statusVal == "resolved" && endedAt == nil {
		now := time.Now()
		endedAt = &now
	}

	if title == "" || severity == "" || statusVal == "" {
		existing, _ := s.incidents.GetIncident(ctx, tenant.ID, id)
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "Edit Incident - Docstor"
		data.Error = "Title, severity, and status are required"
		data.Content = IncidentFormData{Incident: existing, Clients: clientList}
		s.render(w, r, "incident_form.html", data)
		return
	}

	inc, err := s.incidents.UpdateIncident(ctx, tenant.ID, id, incidents.UpdateIncidentInput{
		ClientID:  clientID,
		Title:     title,
		Severity:  severity,
		Status:    statusVal,
		StartedAt: startedAt,
		EndedAt:   endedAt,
		Summary:   summary,
	})
	if errors.Is(err, incidents.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to update incident", "error", err)
		existing, _ := s.incidents.GetIncident(ctx, tenant.ID, id)
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "Edit Incident - Docstor"
		data.Error = "Failed to update incident"
		data.Content = IncidentFormData{Incident: existing, Clients: clientList}
		s.render(w, r, "incident_form.html", data)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionIncidentUpdate,
		TargetType:  audit.TargetIncident,
		TargetID:    &inc.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"title": title, "severity": severity, "status": statusVal},
	})

	setFlashSuccess(w, "Incident updated successfully")
	http.Redirect(w, r, "/incidents/"+inc.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleIncidentAddEvent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		setFlashError(w, "Invalid form data")
		http.Redirect(w, r, "/incidents/"+id.String(), http.StatusSeeOther)
		return
	}

	eventType := strings.TrimSpace(r.FormValue("event_type"))
	detail := strings.TrimSpace(r.FormValue("detail"))

	if eventType == "" || detail == "" {
		setFlashError(w, "Event type and detail are required")
		http.Redirect(w, r, "/incidents/"+id.String(), http.StatusSeeOther)
		return
	}

	event, err := s.incidents.CreateEvent(ctx, incidents.CreateEventInput{
		TenantID:    tenant.ID,
		IncidentID:  id,
		EventType:   eventType,
		Detail:      detail,
		ActorUserID: user.ID,
	})
	if err != nil {
		slog.Error("failed to create incident event", "error", err)
		setFlashError(w, "Failed to add event")
		http.Redirect(w, r, "/incidents/"+id.String(), http.StatusSeeOther)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionIncidentEvent,
		TargetType:  audit.TargetIncident,
		TargetID:    &id,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"event_id": event.ID.String(), "event_type": eventType},
	})

	setFlashSuccess(w, "Event added successfully")
	http.Redirect(w, r, "/incidents/"+id.String(), http.StatusSeeOther)
}

func (s *Server) handleIncidentDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	inc, err := s.incidents.GetIncident(ctx, tenant.ID, id)
	if errors.Is(err, incidents.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get incident for delete", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := s.incidents.DeleteIncident(ctx, tenant.ID, id); err != nil {
		slog.Error("failed to delete incident", "error", err)
		http.Error(w, "Failed to delete incident", http.StatusInternalServerError)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionIncidentDelete,
		TargetType:  audit.TargetIncident,
		TargetID:    &id,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"title": inc.Title},
	})

	setFlashSuccess(w, "Incident deleted successfully")
	http.Redirect(w, r, "/incidents", http.StatusSeeOther)
}
