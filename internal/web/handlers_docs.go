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

	"github.com/exedev/docstor/internal/audit"
	"github.com/exedev/docstor/internal/auth"
	"github.com/exedev/docstor/internal/docs"
	"github.com/exedev/docstor/internal/runbooks"
)

// checkSensitivity returns true if the user has access. If not, it writes a 403.
func (s *Server) checkSensitivity(w http.ResponseWriter, r *http.Request, doc *docs.Document) bool {
	mem := auth.MembershipFromContext(r.Context())
	if mem == nil || !docs.CanAccess(mem.Role, doc.Sensitivity) {
		slog.Warn("sensitivity access denied",
			"doc_id", doc.ID, "sensitivity", doc.Sensitivity,
			"role", mem.Role, "user_id", mem.UserID)
		data := s.newPageData(r)
		data.Title = "Access Denied"
		data.Error = "You do not have permission to view this document."
		w.WriteHeader(http.StatusForbidden)
		s.render(w, r, "docs_list.html", data)
		return false
	}
	return true
}

// getDocAndCheckAccess loads a doc by ID, checks sensitivity, and returns it.
// Returns nil if an error/forbidden response was written.
func (s *Server) getDocAndCheckAccess(w http.ResponseWriter, r *http.Request) *docs.Document {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	docID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return nil
	}

	doc, err := s.docs.GetByID(ctx, tenant.ID, docID)
	if errors.Is(err, docs.ErrNotFound) {
		http.NotFound(w, r)
		return nil
	}
	if err != nil {
		slog.Error("failed to get document", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return nil
	}

	if !s.checkSensitivity(w, r, doc) {
		return nil
	}

	return doc
}

type DocPageData struct {
	PageData
	Document       *docs.Document
	RenderedBody   template.HTML
	Revisions      []docs.Revision
	Clients        []ClientOption
	FromRevision   *docs.Revision
	ToRevision     *docs.Revision
	Diff           *docs.DiffResult
	RunbookStatus  *runbooks.Status
	RunbookOverdue bool
	DefaultDocType string
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
	mem := auth.MembershipFromContext(ctx)

	docsList, err := s.docs.List(ctx, tenant.ID, nil, nil)
	if err != nil {
		slog.Error("failed to list documents", "error", err)
		data := s.newPageData(r)
		data.Title = "Documents - Docstor"
		data.Error = "Failed to load documents"
		s.render(w, r, "docs_list.html", data)
		return
	}

	// Filter by sensitivity for the current user's role
	if mem != nil {
		var filtered []docs.Document
		for _, d := range docsList {
			if docs.CanAccess(mem.Role, d.Sensitivity) {
				filtered = append(filtered, d)
			}
		}
		docsList = filtered
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

	if !s.checkSensitivity(w, r, doc) {
		return
	}

	var renderedBody template.HTML
	if doc.CurrentRevision != nil {
		rendered, err := docs.RenderMarkdown(doc.CurrentRevision.BodyMarkdown)
		if err != nil {
			slog.Error("failed to render markdown", "error", err)
		}
		// Resolve CMDB shortcodes in rendered HTML
		rendered = s.cmdb.RenderShortcodes(ctx, tenant.ID, rendered)
		renderedBody = template.HTML(rendered)
	}

	pageData := DocPageData{
		PageData:     s.newPageData(r),
		Document:     doc,
		RenderedBody: renderedBody,
	}
	pageData.Title = doc.Title + " - Docstor"

	// Load runbook status if this is a runbook
	if doc.DocType == docs.DocTypeRunbook {
		status, err := s.runbooks.GetStatus(ctx, tenant.ID, doc.ID)
		if err == nil {
			pageData.RunbookStatus = status
			if status.NextDueAt != nil && status.NextDueAt.Before(time.Now()) {
				pageData.RunbookOverdue = true
			}
		}
	}

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

	// Check for type query param
	defaultType := r.URL.Query().Get("type")
	if defaultType != "runbook" {
		defaultType = "doc"
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
		PageData:       s.newPageData(r),
		Clients:        clientOptions,
		DefaultDocType: defaultType,
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

	setFlashSuccess(w, "Document created successfully")
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

	if !s.checkSensitivity(w, r, doc) {
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

	doc := s.getDocAndCheckAccess(w, r)
	if doc == nil {
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
		currentDoc, _ := s.docs.GetByID(ctx, tenant.ID, doc.ID)
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

	setFlashSuccess(w, "Changes saved successfully")
	http.Redirect(w, r, "/docs/"+doc.Path, http.StatusSeeOther)
}

func (s *Server) handleDocHistoryByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	doc := s.getDocAndCheckAccess(w, r)
	if doc == nil {
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

	// Resolve CMDB shortcodes in preview
	ctx := r.Context()
	if tenant := auth.TenantFromContext(ctx); tenant != nil {
		rendered = s.cmdb.RenderShortcodes(ctx, tenant.ID, rendered)
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

	preDoc := s.getDocAndCheckAccess(w, r)
	if preDoc == nil {
		return
	}

	revID, err := uuid.Parse(chi.URLParam(r, "revID"))
	if err != nil {
		http.Error(w, "Invalid revision ID", http.StatusBadRequest)
		return
	}

	// Perform revert
	doc, err := s.docs.Revert(ctx, tenant.ID, preDoc.ID, revID, user.ID)
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

	setFlashSuccess(w, "Document reverted successfully")
	http.Redirect(w, r, "/docs/"+doc.Path, http.StatusSeeOther)
}

func (s *Server) handleDocDiffByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	doc := s.getDocAndCheckAccess(w, r)
	if doc == nil {
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

	doc := s.getDocAndCheckAccess(w, r)
	if doc == nil {
		return
	}

	revID, err := uuid.Parse(chi.URLParam(r, "revID"))
	if err != nil {
		http.NotFound(w, r)
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
	rendered = s.cmdb.RenderShortcodes(ctx, tenant.ID, rendered)
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

func (s *Server) handleDocRenameForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	doc := s.getDocAndCheckAccess(w, r)
	if doc == nil {
		return
	}

	pageData := DocPageData{
		PageData: s.newPageData(r),
		Document: doc,
	}
	pageData.Title = "Rename " + doc.Title + " - Docstor"

	s.templates.ExecuteTemplate(w, "doc_rename.html", pageData)
}

func (s *Server) handleDocRename(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	doc := s.getDocAndCheckAccess(w, r)
	if doc == nil {
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	newPath := strings.TrimSpace(r.FormValue("new_path"))
	newTitle := strings.TrimSpace(r.FormValue("new_title"))

	if newPath == "" || newTitle == "" {
		pageData := DocPageData{
			PageData: s.newPageData(r),
			Document: doc,
		}
		pageData.Title = "Rename " + doc.Title + " - Docstor"
		pageData.Error = "Path and title are required"
		s.templates.ExecuteTemplate(w, "doc_rename.html", pageData)
		return
	}

	oldPath := doc.Path
	oldTitle := doc.Title

	updated, err := s.docs.Rename(ctx, tenant.ID, doc.ID, newPath, newTitle, user.ID)
	if errors.Is(err, docs.ErrPathConflict) {
		pageData := DocPageData{
			PageData: s.newPageData(r),
			Document: doc,
		}
		// Show the user's attempted values so they can adjust
		pageData.Document.Path = newPath
		pageData.Document.Title = newTitle
		pageData.Title = "Rename " + oldTitle + " - Docstor"
		pageData.Error = "A document with this path already exists"
		s.templates.ExecuteTemplate(w, "doc_rename.html", pageData)
		return
	}
	if err != nil {
		slog.Error("failed to rename document", "error", err)
		http.Error(w, "Failed to rename document", http.StatusInternalServerError)
		return
	}

	// Audit log
	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionDocMove,
		TargetType:  audit.TargetDocument,
		TargetID:    &updated.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata: map[string]any{
			"old_path":  oldPath,
			"new_path":  updated.Path,
			"old_title": oldTitle,
			"new_title": updated.Title,
		},
	})

	setFlashSuccess(w, "Document renamed successfully")
	http.Redirect(w, r, "/docs/"+updated.Path, http.StatusSeeOther)
}

func (s *Server) handleDocDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	doc := s.getDocAndCheckAccess(w, r)
	if doc == nil {
		return
	}

	docID := doc.ID
	docPath := doc.Path
	docTitle := doc.Title

	if err := s.docs.Delete(ctx, tenant.ID, docID); err != nil {
		slog.Error("failed to delete document", "error", err)
		http.Error(w, "Failed to delete document", http.StatusInternalServerError)
		return
	}

	// Audit log
	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionDocDelete,
		TargetType:  audit.TargetDocument,
		TargetID:    &docID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"path": docPath, "title": docTitle},
	})

	setFlashSuccess(w, "Document \"" + docTitle + "\" deleted")
	http.Redirect(w, r, "/docs", http.StatusSeeOther)
}
