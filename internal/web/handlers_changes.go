package web

import (
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/acgh213/docstor/internal/audit"
	"github.com/acgh213/docstor/internal/auth"
	"github.com/acgh213/docstor/internal/changes"
	"github.com/acgh213/docstor/internal/docs"
	"github.com/acgh213/docstor/internal/pagination"
)

type ChangeFormData struct {
	Change  *changes.Change
	Clients []ClientOption
}

type ChangeViewData struct {
	Change          *changes.Change
	Links           []changes.ChangeLink
	DescriptionHTML template.HTML
	RollbackHTML    template.HTML
	ValidationHTML  template.HTML
}

type ChangesListData struct {
	Changes []changes.Change
	Clients []ClientOption
	Status  string
}

func (s *Server) handleChangesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	status := r.URL.Query().Get("status")
	var clientFilter *uuid.UUID
	if cidStr := r.URL.Query().Get("client_id"); cidStr != "" {
		if cid, err := uuid.Parse(cidStr); err == nil {
			clientFilter = &cid
		}
	}

	all, err := s.changes.List(ctx, tenant.ID, status, clientFilter)
	if err != nil {
		slog.Error("failed to list changes", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	pg := pagination.FromRequest(r, 50)
	paged := pagination.ApplyToSlice(&pg, all)

	clientsList, _ := s.clients.List(ctx, tenant.ID)
	var clientOptions []ClientOption
	for _, c := range clientsList {
		clientOptions = append(clientOptions, ClientOption{ID: c.ID, Name: c.Name, Code: c.Code})
	}

	pv := pg.View(r)
	data := s.newPageData(r)
	data.Title = "Changes - Docstor"
	data.Content = ChangesListData{Changes: paged, Clients: clientOptions, Status: status}
	data.Pagination = &pv
	s.render(w, r, "changes_list.html", data)
}

func (s *Server) handleChangeNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)
	mem := auth.MembershipFromContext(ctx)

	if !mem.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	clientsList, _ := s.clients.List(ctx, tenant.ID)
	var clientOptions []ClientOption
	for _, c := range clientsList {
		clientOptions = append(clientOptions, ClientOption{ID: c.ID, Name: c.Name, Code: c.Code})
	}

	data := s.newPageData(r)
	data.Title = "New Change - Docstor"
	data.Content = ChangeFormData{Clients: clientOptions}
	s.render(w, r, "change_form.html", data)
}

func (s *Server) handleChangeCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)
	mem := auth.MembershipFromContext(ctx)

	if !mem.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		clientsList, _ := s.clients.List(ctx, tenant.ID)
		var clientOptions []ClientOption
		for _, c := range clientsList {
			clientOptions = append(clientOptions, ClientOption{ID: c.ID, Name: c.Name, Code: c.Code})
		}
		data := s.newPageData(r)
		data.Title = "New Change - Docstor"
		data.Error = "Title is required"
		data.Content = ChangeFormData{Clients: clientOptions}
		s.render(w, r, "change_form.html", data)
		return
	}

	in := changes.CreateInput{
		TenantID:               tenant.ID,
		ClientID:               parseOptionalClientID(r),
		Title:                  title,
		DescriptionMarkdown:    strings.TrimSpace(r.FormValue("description")),
		RiskLevel:              r.FormValue("risk_level"),
		RollbackPlanMarkdown:   strings.TrimSpace(r.FormValue("rollback_plan")),
		ValidationPlanMarkdown: strings.TrimSpace(r.FormValue("validation_plan")),
		CreatedBy:              user.ID,
	}

	if ws := r.FormValue("window_start"); ws != "" {
		if t, err := time.Parse("2006-01-02T15:04", ws); err == nil {
			in.WindowStart = &t
		}
	}
	if we := r.FormValue("window_end"); we != "" {
		if t, err := time.Parse("2006-01-02T15:04", we); err == nil {
			in.WindowEnd = &t
		}
	}

	if in.RiskLevel == "" {
		in.RiskLevel = "low"
	}

	ch, err := s.changes.Create(ctx, in)
	if err != nil {
		slog.Error("failed to create change", "error", err)
		http.Error(w, "Failed to create change", http.StatusInternalServerError)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:   tenant.ID,
		ActorUserID: &user.ID,
		Action:     "change.create",
		TargetType: "change",
		TargetID:   &ch.ID,
		IP:         r.RemoteAddr,
		UserAgent:  r.UserAgent(),
		Metadata:   map[string]any{"title": ch.Title, "risk_level": ch.RiskLevel},
	})

	setFlashSuccess(w, "Change record created")
	http.Redirect(w, r, "/changes/"+ch.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleChangeView(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	ch, err := s.changes.Get(ctx, tenant.ID, id)
	if errors.Is(err, changes.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get change", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	links, _ := s.changes.ListLinks(ctx, tenant.ID, id)

	descHTML, _ := renderMarkdownSafe(ch.DescriptionMarkdown)
	rollHTML, _ := renderMarkdownSafe(ch.RollbackPlanMarkdown)
	valHTML, _ := renderMarkdownSafe(ch.ValidationPlanMarkdown)

	data := s.newPageData(r)
	data.Title = ch.Title + " - Changes - Docstor"
	data.Content = ChangeViewData{
		Change:          ch,
		Links:           links,
		DescriptionHTML: template.HTML(descHTML),
		RollbackHTML:    template.HTML(rollHTML),
		ValidationHTML:  template.HTML(valHTML),
	}
	s.render(w, r, "change_view.html", data)
}

func (s *Server) handleChangeEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)
	mem := auth.MembershipFromContext(ctx)

	if !mem.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	ch, err := s.changes.Get(ctx, tenant.ID, id)
	if errors.Is(err, changes.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("get change", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	clientsList, _ := s.clients.List(ctx, tenant.ID)
	var clientOptions []ClientOption
	for _, c := range clientsList {
		sel := ch.ClientID != nil && *ch.ClientID == c.ID
		clientOptions = append(clientOptions, ClientOption{ID: c.ID, Name: c.Name, Code: c.Code, Selected: sel})
	}

	data := s.newPageData(r)
	data.Title = "Edit " + ch.Title + " - Docstor"
	data.Content = ChangeFormData{Change: ch, Clients: clientOptions}
	s.render(w, r, "change_form.html", data)
}

func (s *Server) handleChangeUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)
	mem := auth.MembershipFromContext(ctx)

	if !mem.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	in := changes.UpdateInput{
		Title:                  strings.TrimSpace(r.FormValue("title")),
		DescriptionMarkdown:    strings.TrimSpace(r.FormValue("description")),
		ClientID:               parseOptionalClientID(r),
		RiskLevel:              r.FormValue("risk_level"),
		RollbackPlanMarkdown:   strings.TrimSpace(r.FormValue("rollback_plan")),
		ValidationPlanMarkdown: strings.TrimSpace(r.FormValue("validation_plan")),
	}

	if ws := r.FormValue("window_start"); ws != "" {
		if t, err := time.Parse("2006-01-02T15:04", ws); err == nil {
			in.WindowStart = &t
		}
	}
	if we := r.FormValue("window_end"); we != "" {
		if t, err := time.Parse("2006-01-02T15:04", we); err == nil {
			in.WindowEnd = &t
		}
	}

	ch, err := s.changes.Update(ctx, tenant.ID, id, in)
	if errors.Is(err, changes.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("update change", "error", err)
		http.Error(w, "Failed to update change", http.StatusInternalServerError)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:   tenant.ID,
		ActorUserID: &user.ID,
		Action:     "change.update",
		TargetType: "change",
		TargetID:   &ch.ID,
		IP:         r.RemoteAddr,
		UserAgent:  r.UserAgent(),
		Metadata:   map[string]any{"title": ch.Title},
	})

	setFlashSuccess(w, "Change updated")
	http.Redirect(w, r, "/changes/"+ch.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleChangeTransition(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)
	mem := auth.MembershipFromContext(ctx)

	if !mem.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	newStatus := r.FormValue("status")

	ch, err := s.changes.Transition(ctx, tenant.ID, id, user.ID, newStatus)
	if errors.Is(err, changes.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("transition change", "error", err)
		setFlashError(w, "Invalid status transition: "+err.Error())
		http.Redirect(w, r, "/changes/"+id.String(), http.StatusSeeOther)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:   tenant.ID,
		ActorUserID: &user.ID,
		Action:     "change.transition",
		TargetType: "change",
		TargetID:   &ch.ID,
		IP:         r.RemoteAddr,
		UserAgent:  r.UserAgent(),
		Metadata:   map[string]any{"title": ch.Title, "new_status": newStatus},
	})

	setFlashSuccess(w, "Change status updated to "+newStatus)
	http.Redirect(w, r, "/changes/"+ch.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleChangeDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)
	mem := auth.MembershipFromContext(ctx)

	if !mem.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := s.changes.Delete(ctx, tenant.ID, id); err != nil {
		if errors.Is(err, changes.ErrNotFound) {
			setFlashError(w, "Only draft or cancelled changes can be deleted")
		} else {
			slog.Error("delete change", "error", err)
			setFlashError(w, "Failed to delete change")
		}
		http.Redirect(w, r, "/changes/"+id.String(), http.StatusSeeOther)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:   tenant.ID,
		ActorUserID: &user.ID,
		Action:     "change.delete",
		TargetType: "change",
		TargetID:   &id,
		IP:         r.RemoteAddr,
		UserAgent:  r.UserAgent(),
	})

	setFlashSuccess(w, "Change deleted")
	http.Redirect(w, r, "/changes", http.StatusSeeOther)
}

func (s *Server) handleChangeLinkAdd(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)
	mem := auth.MembershipFromContext(ctx)

	if !mem.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	linkedType := r.FormValue("linked_type")
	linkedID, err := uuid.Parse(r.FormValue("linked_id"))
	if err != nil {
		setFlashError(w, "Invalid linked item")
		http.Redirect(w, r, "/changes/"+id.String(), http.StatusSeeOther)
		return
	}

	if err := s.changes.AddLink(ctx, tenant.ID, id, linkedType, linkedID); err != nil {
		slog.Error("add change link", "error", err)
		setFlashError(w, "Failed to add link")
		http.Redirect(w, r, "/changes/"+id.String(), http.StatusSeeOther)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:   tenant.ID,
		ActorUserID: &user.ID,
		Action:     "change.link_add",
		TargetType: "change",
		TargetID:   &id,
		IP:         r.RemoteAddr,
		UserAgent:  r.UserAgent(),
		Metadata:   map[string]any{"linked_type": linkedType, "linked_id": linkedID.String()},
	})

	http.Redirect(w, r, "/changes/"+id.String(), http.StatusSeeOther)
}

func (s *Server) handleChangeLinkRemove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)
	mem := auth.MembershipFromContext(ctx)

	if !mem.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id, _ := uuid.Parse(chi.URLParam(r, "id"))
	linkID, err := uuid.Parse(chi.URLParam(r, "linkID"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	_ = s.changes.RemoveLink(ctx, tenant.ID, linkID)

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:   tenant.ID,
		ActorUserID: &user.ID,
		Action:     "change.link_remove",
		TargetType: "change",
		TargetID:   &id,
		IP:         r.RemoteAddr,
		UserAgent:  r.UserAgent(),
		Metadata:   map[string]any{"link_id": linkID.String()},
	})

	http.Redirect(w, r, "/changes/"+id.String(), http.StatusSeeOther)
}

// renderMarkdownSafe renders markdown and returns empty string on error.
func renderMarkdownSafe(md string) (string, error) {
	if md == "" {
		return "", nil
	}
	return docs.RenderMarkdown(md)
}
