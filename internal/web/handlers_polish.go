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

	"github.com/exedev/docstor/internal/audit"
	"github.com/exedev/docstor/internal/auth"
	"github.com/exedev/docstor/internal/docs"
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
		// Display name is the last segment
		name := strings.TrimSuffix(f, "/")
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		fmt.Fprintf(w, `<div class="tree-folder">`)
		fmt.Fprintf(w, `<div class="tree-folder-header" hx-get="/tree?folder=%s" hx-target="next .tree-children" hx-swap="innerHTML" onclick="this.parentElement.classList.toggle('open')">`, f)
		fmt.Fprintf(w, `<span class="tree-arrow">▸</span> 📁 %s</div>`, name)
		fmt.Fprintf(w, `<div class="tree-children"></div>`)
		fmt.Fprintf(w, `</div>`)
	}

	for _, d := range filteredDocs {
		icon := "📄"
		if d.DocType == docs.DocTypeRunbook {
			icon = "📋"
		}
		fmt.Fprintf(w, `<div class="tree-doc"><a href="/docs/%s">%s %s</a></div>`, d.Path, icon, d.Title)
	}

	if len(folders) == 0 && len(filteredDocs) == 0 {
		fmt.Fprintf(w, `<div class="tree-empty">Empty folder</div>`)
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
