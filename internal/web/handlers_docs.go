package web

import (
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/exedev/docstor/internal/audit"
	"github.com/exedev/docstor/internal/auth"
	"github.com/exedev/docstor/internal/docs"
)

type DocPageData struct {
	PageData
	Document     *docs.Document
	RenderedBody template.HTML
	Revisions    []docs.Revision
	Clients      []ClientOption
	FromRevision *docs.Revision
	ToRevision   *docs.Revision
	Diff         *docs.DiffResult
}

type ClientOption struct {
	ID       uuid.UUID
	Name     string
	Code     string
	Selected bool
}

func (s *Server) handleDocsHomeV2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	docsList, err := s.docs.List(ctx, tenant.ID, nil, nil)
	if err != nil {
		slog.Error("failed to list documents", "error", err)
		data := s.newPageData(r)
		data.Title = "Documents - Docstor"
		data.Error = "Failed to load documents"
		s.render(w, r, "docs_list.html", data)
		return
	}

	data := s.newPageData(r)
	data.Title = "Documents - Docstor"
	data.Content = docsList
	s.render(w, r, "docs_list.html", data)
}

func (s *Server) handleDocRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	path := chi.URLParam(r, "*")
	if path == "" {
		s.handleDocsHomeV2(w, r)
		return
	}

	doc, err := s.docs.GetByPath(ctx, tenant.ID, path)
	if errors.Is(err, docs.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get document", "error", err, "path", path)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var renderedBody template.HTML
	if doc.CurrentRevision != nil {
		rendered, err := docs.RenderMarkdown(doc.CurrentRevision.BodyMarkdown)
		if err != nil {
			slog.Error("failed to render markdown", "error", err)
		}
		renderedBody = template.HTML(rendered)
	}

	pageData := DocPageData{
		PageData:     s.newPageData(r),
		Document:     doc,
		RenderedBody: renderedBody,
	}
	pageData.Title = doc.Title + " - Docstor"

	s.templates.ExecuteTemplate(w, "doc_read.html", pageData)
}

func (s *Server) handleDocNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Load clients for dropdown
	clientsList, _ := s.clients.List(ctx, tenant.ID)
	var clientOptions []ClientOption
	for _, c := range clientsList {
		clientOptions = append(clientOptions, ClientOption{
			ID:   c.ID,
			Name: c.Name,
			Code: c.Code,
		})
	}

	pageData := DocPageData{
		PageData: s.newPageData(r),
		Clients:  clientOptions,
	}
	pageData.Title = "New Document - Docstor"

	s.templates.ExecuteTemplate(w, "doc_form.html", pageData)
}

func (s *Server) handleDocCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	path := strings.TrimSpace(r.FormValue("path"))
	title := strings.TrimSpace(r.FormValue("title"))
	body := r.FormValue("body")
	docType := docs.DocType(r.FormValue("doc_type"))
	clientIDStr := r.FormValue("client_id")
	message := strings.TrimSpace(r.FormValue("message"))

	if path == "" || title == "" {
		// Load clients for re-render
		clientsList, _ := s.clients.List(ctx, tenant.ID)
		var clientOptions []ClientOption
		for _, c := range clientsList {
			clientOptions = append(clientOptions, ClientOption{ID: c.ID, Name: c.Name, Code: c.Code})
		}

		pageData := DocPageData{
			PageData: s.newPageData(r),
			Clients:  clientOptions,
		}
		pageData.Title = "New Document - Docstor"
		pageData.Error = "Path and title are required"
		s.templates.ExecuteTemplate(w, "doc_form.html", pageData)
		return
	}

	var clientID *uuid.UUID
	if clientIDStr != "" {
		cid, err := uuid.Parse(clientIDStr)
		if err == nil {
			clientID = &cid
		}
	}

	if docType == "" {
		docType = docs.DocTypeDoc
	}

	if message == "" {
		message = "Initial version"
	}

	doc, err := s.docs.Create(ctx, docs.CreateInput{
		TenantID:    tenant.ID,
		ClientID:    clientID,
		Path:        path,
		Title:       title,
		DocType:     docType,
		Sensitivity: docs.SensitivityPublic,
		OwnerUserID: &user.ID,
		CreatedBy:   user.ID,
		Body:        body,
		Message:     message,
	})

	if errors.Is(err, docs.ErrPathConflict) {
		clientsList, _ := s.clients.List(ctx, tenant.ID)
		var clientOptions []ClientOption
		for _, c := range clientsList {
			clientOptions = append(clientOptions, ClientOption{ID: c.ID, Name: c.Name, Code: c.Code})
		}

		pageData := DocPageData{
			PageData: s.newPageData(r),
			Clients:  clientOptions,
		}
		pageData.Title = "New Document - Docstor"
		pageData.Error = "A document with this path already exists"
		s.templates.ExecuteTemplate(w, "doc_form.html", pageData)
		return
	}

	if err != nil {
		slog.Error("failed to create document", "error", err)
		http.Error(w, "Failed to create document", http.StatusInternalServerError)
		return
	}

	// Audit log
	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionDocCreate,
		TargetType:  audit.TargetDocument,
		TargetID:    &doc.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"path": doc.Path, "title": doc.Title},
	})

	http.Redirect(w, r, "/docs/"+doc.Path, http.StatusSeeOther)
}

func (s *Server) handleDocEditByID(w http.ResponseWriter, r *http.Request) {
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

	clientsList, _ := s.clients.List(ctx, tenant.ID)
	var clientOptions []ClientOption
	for _, c := range clientsList {
		selected := doc.ClientID != nil && *doc.ClientID == c.ID
		clientOptions = append(clientOptions, ClientOption{ID: c.ID, Name: c.Name, Code: c.Code, Selected: selected})
	}

	pageData := DocPageData{
		PageData: s.newPageData(r),
		Document: doc,
		Clients:  clientOptions,
	}
	pageData.Title = "Edit " + doc.Title + " - Docstor"

	s.templates.ExecuteTemplate(w, "doc_edit.html", pageData)
}

func (s *Server) handleDocSaveByID(w http.ResponseWriter, r *http.Request) {
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

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	body := r.FormValue("body")
	message := strings.TrimSpace(r.FormValue("message"))
	baseRevIDStr := r.FormValue("base_revision_id")

	if message == "" {
		message = "Updated"
	}

	baseRevID, err := uuid.Parse(baseRevIDStr)
	if err != nil {
		http.Error(w, "Invalid base revision", http.StatusBadRequest)
		return
	}

	doc, err = s.docs.Update(ctx, tenant.ID, doc.ID, docs.UpdateInput{
		Body:           body,
		Message:        message,
		BaseRevisionID: baseRevID,
		UpdatedBy:      user.ID,
	})

	if errors.Is(err, docs.ErrConflict) {
		// Show conflict page
		currentDoc, _ := s.docs.GetByID(ctx, tenant.ID, docID)
		pageData := DocPageData{
			PageData: s.newPageData(r),
			Document: currentDoc,
		}
		pageData.Title = "Conflict - " + currentDoc.Path
		pageData.Error = "This document has been modified by someone else. Please review the changes and try again."
		pageData.Content = map[string]string{
			"yourBody": body,
		}
		s.templates.ExecuteTemplate(w, "doc_conflict.html", pageData)
		return
	}

	if err != nil {
		slog.Error("failed to update document", "error", err)
		http.Error(w, "Failed to save document", http.StatusInternalServerError)
		return
	}

	// Audit log
	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionDocEdit,
		TargetType:  audit.TargetDocument,
		TargetID:    &doc.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"path": doc.Path, "revision_id": doc.CurrentRevisionID.String()},
	})

	http.Redirect(w, r, "/docs/"+doc.Path, http.StatusSeeOther)
}

func (s *Server) handleDocHistoryByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	docID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

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

	revisions, err := s.docs.ListRevisions(ctx, tenant.ID, doc.ID)
	if err != nil {
		slog.Error("failed to list revisions", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	pageData := DocPageData{
		PageData:  s.newPageData(r),
		Document:  doc,
		Revisions: revisions,
	}
	pageData.Title = "History - " + doc.Title

	s.templates.ExecuteTemplate(w, "doc_history.html", pageData)
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	body := r.FormValue("body")
	rendered, err := docs.RenderMarkdown(body)
	if err != nil {
		http.Error(w, "Failed to render markdown", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(rendered))
}

func (s *Server) handleDocRevertByID(w http.ResponseWriter, r *http.Request) {
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

	revID, err := uuid.Parse(chi.URLParam(r, "revID"))
	if err != nil {
		http.Error(w, "Invalid revision ID", http.StatusBadRequest)
		return
	}

	// Perform revert
	doc, err := s.docs.Revert(ctx, tenant.ID, docID, revID, user.ID)
	if errors.Is(err, docs.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to revert document", "error", err)
		http.Error(w, "Failed to revert document", http.StatusInternalServerError)
		return
	}

	// Audit log
	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionDocRevert,
		TargetType:  audit.TargetDocument,
		TargetID:    &doc.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"path": doc.Path, "reverted_to": revID.String(), "new_revision_id": doc.CurrentRevisionID.String()},
	})

	http.Redirect(w, r, "/docs/"+doc.Path, http.StatusSeeOther)
}

func (s *Server) handleDocDiffByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	docID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

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

	// Parse from and to revision IDs
	fromIDStr := r.URL.Query().Get("from")
	toIDStr := r.URL.Query().Get("to")

	if fromIDStr == "" || toIDStr == "" {
		http.Error(w, "Missing from or to parameters", http.StatusBadRequest)
		return
	}

	fromID, err := uuid.Parse(fromIDStr)
	if err != nil {
		http.Error(w, "Invalid from revision ID", http.StatusBadRequest)
		return
	}

	toID, err := uuid.Parse(toIDStr)
	if err != nil {
		http.Error(w, "Invalid to revision ID", http.StatusBadRequest)
		return
	}

	fromRev, err := s.docs.GetRevision(ctx, tenant.ID, fromID)
	if err != nil {
		http.Error(w, "From revision not found", http.StatusNotFound)
		return
	}

	toRev, err := s.docs.GetRevision(ctx, tenant.ID, toID)
	if err != nil {
		http.Error(w, "To revision not found", http.StatusNotFound)
		return
	}

	// Compute diff
	diff := docs.ComputeDiff(fromRev.BodyMarkdown, toRev.BodyMarkdown)

	pageData := DocPageData{
		PageData:     s.newPageData(r),
		Document:     doc,
		FromRevision: fromRev,
		ToRevision:   toRev,
		Diff:         diff,
	}
	pageData.Title = "Diff - " + doc.Title

	s.templates.ExecuteTemplate(w, "doc_diff.html", pageData)
}

func (s *Server) handleDocRevisionByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	docID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	revID, err := uuid.Parse(chi.URLParam(r, "revID"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

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

	rev, err := s.docs.GetRevision(ctx, tenant.ID, revID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if rev.DocumentID != doc.ID {
		http.NotFound(w, r)
		return
	}

	var renderedBody template.HTML
	rendered, err := docs.RenderMarkdown(rev.BodyMarkdown)
	if err != nil {
		slog.Error("failed to render markdown", "error", err)
	}
	renderedBody = template.HTML(rendered)

	pageData := DocPageData{
		PageData:     s.newPageData(r),
		Document:     doc,
		RenderedBody: renderedBody,
		Revisions:    []docs.Revision{*rev},
	}
	pageData.Title = doc.Title + " (Revision) - Docstor"

	s.templates.ExecuteTemplate(w, "doc_revision.html", pageData)
}
