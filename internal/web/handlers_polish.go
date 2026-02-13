package web

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/acgh213/docstor/internal/audit"
	"github.com/acgh213/docstor/internal/auth"
	"github.com/acgh213/docstor/internal/docs"
)

// SVG icons for folder tree (inline, no external deps)
const (
	iconChevron = `<svg class="tree-icon tree-chevron" viewBox="0 0 16 16" fill="currentColor"><path d="M6.22 3.22a.75.75 0 0 1 1.06 0l4.25 4.25a.75.75 0 0 1 0 1.06l-4.25 4.25a.75.75 0 0 1-1.06-1.06L9.94 8 6.22 4.28a.75.75 0 0 1 0-1.06Z"/></svg>`
	iconFolder  = `<svg class="tree-icon tree-icon-folder" viewBox="0 0 16 16" fill="currentColor"><path d="M1.75 1A1.75 1.75 0 0 0 0 2.75v10.5C0 14.216.784 15 1.75 15h12.5A1.75 1.75 0 0 0 16 13.25v-8.5A1.75 1.75 0 0 0 14.25 3H7.5a.25.25 0 0 1-.2-.1l-.9-1.2C6.07 1.26 5.55 1 5 1H1.75Z"/></svg>`
	iconDoc     = `<svg class="tree-icon tree-icon-doc" viewBox="0 0 16 16" fill="currentColor"><path d="M3.75 1.5a.25.25 0 0 0-.25.25v12.5c0 .138.112.25.25.25h8.5a.25.25 0 0 0 .25-.25V6H9.75A1.75 1.75 0 0 1 8 4.25V1.5H3.75Zm5.75.56v2.19c0 .138.112.25.25.25h2.19L9.5 2.06ZM2 1.75C2 .784 2.784 0 3.75 0h5.086c.464 0 .909.184 1.237.513l3.414 3.414c.329.328.513.773.513 1.237v9.086A1.75 1.75 0 0 1 12.25 16h-8.5A1.75 1.75 0 0 1 2 14.25V1.75Z"/></svg>`
	iconRunbook = `<svg class="tree-icon tree-icon-runbook" viewBox="0 0 16 16" fill="currentColor"><path d="M2 1.75C2 .784 2.784 0 3.75 0h8.5C13.216 0 14 .784 14 1.75v12.5A1.75 1.75 0 0 1 12.25 16h-8.5A1.75 1.75 0 0 1 2 14.25V1.75Zm3.5 3a.75.75 0 0 0 0 1.5h5a.75.75 0 0 0 0-1.5h-5Zm0 3a.75.75 0 0 0 0 1.5h5a.75.75 0 0 0 0-1.5h-5Zm0 3a.75.75 0 0 0 0 1.5h3a.75.75 0 0 0 0-1.5h-3Z"/></svg>`
)

// handleFolderTree returns an HTML partial for the folder tree at a given prefix.
// GET /tree?folder=...
func (s *Server) handleFolderTree(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)
	mem := auth.MembershipFromContext(ctx)

	folder := r.URL.Query().Get("folder")
	if folder != "" && !strings.HasSuffix(folder, "/") {
		folder += "/"
	}

	folders, directDocs, err := s.docs.ListFolders(ctx, tenant.ID, folder)
	if err != nil {
		slog.Error("failed to list folders", "error", err)
		http.Error(w, "Failed to load tree", http.StatusInternalServerError)
		return
	}

	// Filter docs by sensitivity
	var filteredDocs []docs.Document
	for _, d := range directDocs {
		if docs.CanAccess(mem.Role, d.Sensitivity) {
			filteredDocs = append(filteredDocs, d)
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	for _, f := range folders {
		name := strings.TrimSuffix(f, "/")
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		fmt.Fprintf(w, `<div class="tree-folder">`)
		fmt.Fprintf(w, `<div class="tree-folder-header" hx-get="/tree?folder=%s" hx-target="next .tree-children" hx-swap="innerHTML" onclick="this.parentElement.classList.toggle('open')">`, f)
		fmt.Fprintf(w, `%s%s<span class="tree-label">%s</span></div>`, iconChevron, iconFolder, name)
		fmt.Fprintf(w, `<div class="tree-children"></div>`)
		fmt.Fprintf(w, `</div>`)
	}

	for _, d := range filteredDocs {
		icon := iconDoc
		if d.DocType == docs.DocTypeRunbook {
			icon = iconRunbook
		}
		fmt.Fprintf(w, `<div class="tree-doc"><a href="/docs/%s">%s<span class="tree-label">%s</span></a></div>`, d.Path, icon, d.Title)
	}

	if len(folders) == 0 && len(filteredDocs) == 0 {
		fmt.Fprintf(w, `<div class="tree-empty">No items in this folder</div>`)
	}
}

// handleDocMetadataUpdate handles inline metadata edits via HTMX.
// POST /docs/id/{id}/metadata
func (s *Server) handleDocMetadataUpdate(w http.ResponseWriter, r *http.Request) {
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
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	var update docs.MetadataUpdate
	changes := map[string]any{}

	if v := r.FormValue("doc_type"); v != "" {
		dt := docs.DocType(v)
		if dt == docs.DocTypeDoc || dt == docs.DocTypeRunbook {
			update.DocType = &dt
			changes["doc_type"] = v
		}
	}

	if v := r.FormValue("sensitivity"); v != "" {
		se := docs.Sensitivity(v)
		if se == docs.SensitivityPublic || se == docs.SensitivityRestricted || se == docs.SensitivityConfidential {
			update.Sensitivity = &se
			changes["sensitivity"] = v
		}
	}

	if v, ok := r.Form["owner_user_id"]; ok && len(v) > 0 {
		if v[0] == "" {
			nil_id := uuid.Nil
			update.OwnerUserID = &nil_id
			changes["owner"] = "cleared"
		} else if ownerID, err := uuid.Parse(v[0]); err == nil {
			update.OwnerUserID = &ownerID
			changes["owner_user_id"] = ownerID.String()
		}
	}

	if v, ok := r.Form["client_id"]; ok && len(v) > 0 {
		if v[0] == "" {
			nil_id := uuid.Nil
			update.ClientID = &nil_id
			changes["client"] = "cleared"
		} else if clientID, err := uuid.Parse(v[0]); err == nil {
			update.ClientID = &clientID
			changes["client_id"] = clientID.String()
		}
	}

	if len(changes) == 0 {
		http.Error(w, "No changes", http.StatusBadRequest)
		return
	}

	if err := s.docs.UpdateMetadata(ctx, tenant.ID, docID, update); err != nil {
		slog.Error("failed to update metadata", "error", err)
		http.Error(w, "Failed to update", http.StatusInternalServerError)
		return
	}

	changes["path"] = doc.Path
	changes["title"] = doc.Title

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionDocMetadata,
		TargetType:  audit.TargetDocument,
		TargetID:    &docID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    changes,
	})

	// If HTMX request, redirect so the page reloads with updated data
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/docs/"+doc.Path)
		w.WriteHeader(http.StatusOK)
		return
	}

	setFlashSuccess(w, "Document metadata updated")
	http.Redirect(w, r, "/docs/"+doc.Path, http.StatusSeeOther)
}

// handleAttachmentPreview serves an inline preview for image/PDF attachments.
// GET /attachments/{id}/preview
func (s *Server) handleAttachmentPreview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	att, err := s.attachments.GetAttachment(ctx, tenant.ID, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Only allow preview for images and PDFs
	ext := strings.ToLower(filepath.Ext(att.Filename))
	isImage := ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".webp" || ext == ".svg"
	isPDF := ext == ".pdf" || att.ContentType == "application/pdf"

	if !isImage && !isPDF {
		http.Error(w, "Preview not available for this file type", http.StatusBadRequest)
		return
	}

	// Serve the file inline (not as download)
	rc, err := s.storage.Retrieve(att.StorageKey)
	if err != nil {
		slog.Error("failed to read attachment", "error", err)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", att.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", att.Filename))
	io.Copy(w, rc)
}
