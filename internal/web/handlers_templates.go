package web

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/acgh213/docstor/internal/audit"
	"github.com/acgh213/docstor/internal/auth"
	"github.com/acgh213/docstor/internal/docs"
	"github.com/acgh213/docstor/internal/pagination"
	tmplpkg "github.com/acgh213/docstor/internal/templates"
)

type TemplatePageData struct {
	PageData
	Template  *tmplpkg.Template
	Templates []tmplpkg.Template
	Preview   string
}

func (s *Server) handleTemplatesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	templates, err := s.templates_repo.List(ctx, tenant.ID)
	if err != nil {
		slog.Error("failed to list templates", "error", err)
		data := s.newPageData(r)
		data.Title = "Templates - Docstor"
		data.Error = "Failed to load templates"
		s.render(w, r, "templates_list.html", data)
		return
	}

	pg := pagination.FromRequest(r, pagination.DefaultPerPage)
	paged := pagination.ApplyToSlice(&pg, templates)
	pv := pg.View(r)

	data := s.newPageData(r)
	data.Title = "Templates - Docstor"
	data.Pagination = &pv
	data.Content = paged
	s.render(w, r, "templates_list.html", data)
}

func (s *Server) handleTemplateNew(w http.ResponseWriter, r *http.Request) {
	membership := auth.MembershipFromContext(r.Context())
	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	data := s.newPageData(r)
	data.Title = "New Template - Docstor"
	s.render(w, r, "template_form.html", data)
}

func (s *Server) handleTemplateCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		data := s.newPageData(r)
		data.Title = "New Template - Docstor"
		data.Error = "Invalid form data"
		s.render(w, r, "template_form.html", data)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	templateType := r.FormValue("template_type")
	body := r.FormValue("body")

	if name == "" {
		data := s.newPageData(r)
		data.Title = "New Template - Docstor"
		data.Error = "Name is required"
		s.render(w, r, "template_form.html", data)
		return
	}

	tmpl, err := s.templates_repo.Create(ctx, tmplpkg.CreateInput{
		TenantID:     tenant.ID,
		Name:         name,
		TemplateType: templateType,
		BodyMarkdown: body,
		CreatedBy:    user.ID,
	})
	if err != nil {
		slog.Error("failed to create template", "error", err)
		data := s.newPageData(r)
		data.Title = "New Template - Docstor"
		data.Error = "Failed to create template"
		s.render(w, r, "template_form.html", data)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionTemplateCreate,
		TargetType:  audit.TargetTemplate,
		TargetID:    &tmpl.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"name": name, "type": templateType},
	})

	setFlashSuccess(w, "Template created successfully")
	http.Redirect(w, r, "/templates/"+tmpl.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleTemplateView(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	tmplID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	tmpl, err := s.templates_repo.Get(ctx, tenant.ID, tmplID)
	if errors.Is(err, tmplpkg.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Render preview
	preview, _ := docs.RenderMarkdown(tmpl.BodyMarkdown)

	data := s.newPageData(r)
	data.Title = tmpl.Name + " - Docstor"
	data.Content = TemplatePageData{
		PageData: data,
		Template: tmpl,
		Preview:  preview,
	}
	s.render(w, r, "template_view.html", data)
}

func (s *Server) handleTemplateEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	tmplID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	tmpl, err := s.templates_repo.Get(ctx, tenant.ID, tmplID)
	if errors.Is(err, tmplpkg.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := s.newPageData(r)
	data.Title = "Edit " + tmpl.Name + " - Docstor"
	data.Content = tmpl
	s.render(w, r, "template_form.html", data)
}

func (s *Server) handleTemplateUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	tmplID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		data := s.newPageData(r)
		data.Title = "Edit Template - Docstor"
		data.Error = "Invalid form data"
		s.render(w, r, "template_form.html", data)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	templateType := r.FormValue("template_type")
	body := r.FormValue("body")

	if name == "" {
		tmpl, _ := s.templates_repo.Get(ctx, tenant.ID, tmplID)
		data := s.newPageData(r)
		data.Title = "Edit Template - Docstor"
		data.Error = "Name is required"
		data.Content = tmpl
		s.render(w, r, "template_form.html", data)
		return
	}

	tmpl, err := s.templates_repo.Update(ctx, tenant.ID, tmplID, tmplpkg.UpdateInput{
		Name:         name,
		TemplateType: templateType,
		BodyMarkdown: body,
	})
	if errors.Is(err, tmplpkg.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to update template", "error", err)
		data := s.newPageData(r)
		data.Title = "Edit Template - Docstor"
		data.Error = "Failed to update template"
		s.render(w, r, "template_form.html", data)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionTemplateUpdate,
		TargetType:  audit.TargetTemplate,
		TargetID:    &tmpl.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"name": name, "type": templateType},
	})

	setFlashSuccess(w, "Template updated successfully")
	http.Redirect(w, r, "/templates/"+tmpl.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleTemplateDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	tmplID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Get template info for audit before deleting
	tmpl, _ := s.templates_repo.Get(ctx, tenant.ID, tmplID)

	if err := s.templates_repo.Delete(ctx, tenant.ID, tmplID); err != nil {
		if errors.Is(err, tmplpkg.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("failed to delete template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	metadata := map[string]any{}
	if tmpl != nil {
		metadata["name"] = tmpl.Name
	}
	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionTemplateDelete,
		TargetType:  audit.TargetTemplate,
		TargetID:    &tmplID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    metadata,
	})

	setFlashSuccess(w, "Template deleted")
	http.Redirect(w, r, "/templates", http.StatusSeeOther)
}

// handleDocNewFromTemplate pre-fills the new doc form with template content.
func (s *Server) handleDocNewFromTemplate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)
	membership := auth.MembershipFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	tmplIDStr := r.URL.Query().Get("template")
	if tmplIDStr == "" {
		http.Redirect(w, r, "/docs/new", http.StatusSeeOther)
		return
	}

	tmplID, err := uuid.Parse(tmplIDStr)
	if err != nil {
		http.Redirect(w, r, "/docs/new", http.StatusSeeOther)
		return
	}

	tmpl, err := s.templates_repo.Get(ctx, tenant.ID, tmplID)
	if err != nil {
		http.Redirect(w, r, "/docs/new", http.StatusSeeOther)
		return
	}

	// Load clients for the form
	clientList, _ := s.clients.List(ctx, tenant.ID)
	var clientOpts []ClientOption
	for _, c := range clientList {
		clientOpts = append(clientOpts, ClientOption{ID: c.ID, Name: c.Name, Code: c.Code})
	}

	data := s.newPageData(r)
	data.Title = "New Document from Template - Docstor"
	data.Content = DocPageData{
		PageData:       data,
		Clients:        clientOpts,
		DefaultDocType: tmpl.TemplateType,
		Document: &docs.Document{
			CurrentRevision: &docs.Revision{
				BodyMarkdown: tmpl.BodyMarkdown,
			},
		},
	}
	s.render(w, r, "doc_from_template.html", data)
}
