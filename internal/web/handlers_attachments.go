package web

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/exedev/docstor/internal/attachments"
	"github.com/exedev/docstor/internal/audit"
	"github.com/exedev/docstor/internal/auth"
)

const maxUploadSize = 50 * 1024 * 1024 // 50MB

// handleAttachmentUpload handles file uploads
// POST /attachments/upload?document_id=...
func (s *Server) handleAttachmentUpload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mem := auth.MembershipFromContext(ctx)
	if mem == nil || !mem.IsEditor() {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "file too large", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "no file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	documentID := r.FormValue("document_id")

	// Read file and compute hash
	hash, content, err := attachments.ComputeSHA256(file)
	if err != nil {
		slog.Error("compute hash", "error", err)
		http.Error(w, "upload failed", http.StatusInternalServerError)
		return
	}

	// Check for existing attachment with same hash (deduplication)
	existing, err := s.attachments.FindBySHA256(ctx, mem.TenantID, hash)
	if err != nil {
		slog.Error("find by sha256", "error", err)
		http.Error(w, "upload failed", http.StatusInternalServerError)
		return
	}

	var att *attachments.Attachment
	if existing != nil {
		// Reuse existing attachment
		att = existing
	} else {
		// Store the file
		storageKey, err := s.storage.Store(mem.TenantID.String(), hash, bytes.NewReader(content))
		if err != nil {
			slog.Error("store file", "error", err)
			http.Error(w, "upload failed", http.StatusInternalServerError)
			return
		}

		// Create attachment record
		att = &attachments.Attachment{
			TenantID:    mem.TenantID,
			Filename:    header.Filename,
			ContentType: header.Header.Get("Content-Type"),
			SizeBytes:   int64(len(content)),
			SHA256:      hash,
			StorageKey:  storageKey,
			CreatedBy:   mem.UserID,
		}
		if att.ContentType == "" {
			att.ContentType = "application/octet-stream"
		}

		if err := s.attachments.CreateAttachment(ctx, att); err != nil {
			slog.Error("create attachment", "error", err)
			http.Error(w, "upload failed", http.StatusInternalServerError)
			return
		}
	}

	// Link to document if provided
	if documentID != "" {
		docUUID, _ := uuid.Parse(documentID)
		link := &attachments.AttachmentLink{
			TenantID:     mem.TenantID,
			AttachmentID: att.ID,
			LinkedType:   "document",
			LinkedID:     docUUID,
		}
		if err := s.attachments.CreateLink(ctx, link); err != nil {
			slog.Error("create link", "error", err)
			// Don't fail - attachment was created
		}
	}

	// Audit log
	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    mem.TenantID,
		ActorUserID: &mem.UserID,
		Action:      "attachment.upload",
		TargetType:  "attachment",
		TargetID:    &att.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata: map[string]any{
			"filename":    att.Filename,
			"size_bytes":  att.SizeBytes,
			"document_id": documentID,
		},
	})

	// Respond with HTMX partial or redirect
	if r.Header.Get("HX-Request") == "true" {
		// Return partial for HTMX
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<div class="attachment-item" id="att-%s">
			<a href="/attachments/%s">%s</a>
			<span class="size">(%s)</span>
		</div>`, att.ID, att.ID, att.Filename, formatSize(att.SizeBytes))
		return
	}

	// Regular form submission - redirect back
	if documentID != "" {
		http.Redirect(w, r, "/docs/id/"+documentID+"/attachments", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/attachments", http.StatusSeeOther)
	}
}

// handleAttachmentDownload serves an attachment file
// GET /attachments/{id}
func (s *Server) handleAttachmentDownload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mem := auth.MembershipFromContext(ctx)
	if mem == nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "id")
	idUUID, err := uuid.Parse(id)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	att, err := s.attachments.GetAttachment(ctx, mem.TenantID, idUUID)
	if err != nil {
		slog.Error("get attachment", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if att == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Get file from storage
	reader, err := s.storage.Retrieve(att.StorageKey)
	if err != nil {
		slog.Error("retrieve file", "error", err)
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	defer reader.Close()

	// Audit download
	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    mem.TenantID,
		ActorUserID: &mem.UserID,
		Action:      "attachment.download",
		TargetType:  "attachment",
		TargetID:    &att.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"filename": att.Filename},
	})

	// Set headers
	w.Header().Set("Content-Type", att.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, att.Filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", att.SizeBytes))

	io.Copy(w, reader)
}

// handleDocAttachments shows attachments for a document
// GET /docs/id/{id}/attachments
func (s *Server) handleDocAttachments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mem := auth.MembershipFromContext(ctx)
	if mem == nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	docID := chi.URLParam(r, "id")
	docUUID, err := uuid.Parse(docID)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	doc, err := s.docs.GetByID(ctx, mem.TenantID, docUUID)
	if err != nil || doc == nil {
		http.Error(w, "document not found", http.StatusNotFound)
		return
	}

	attachmentList, err := s.attachments.ListByDocument(ctx, mem.TenantID, docUUID)
	if err != nil {
		slog.Error("list attachments", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := s.newPageData(r)
	data.Title = "Attachments - " + doc.Title
	data.Content = map[string]any{
		"Document":    doc,
		"Attachments": attachmentList,
	}
	s.render(w, r, "doc_attachments.html", data)
}

// handleUnlinkAttachment removes an attachment link from a document
// POST /docs/id/{id}/attachments/{attID}/unlink
func (s *Server) handleUnlinkAttachment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mem := auth.MembershipFromContext(ctx)
	if mem == nil || !mem.IsEditor() {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	docID := chi.URLParam(r, "id")
	attID := chi.URLParam(r, "attID")

	docUUID, _ := uuid.Parse(docID)
	attUUID, _ := uuid.Parse(attID)

	// Find and delete the link
	link, err := s.attachments.GetLinkByAttachmentAndDoc(ctx, mem.TenantID, attUUID, docUUID)
	if err != nil || link == nil {
		http.Error(w, "link not found", http.StatusNotFound)
		return
	}

	if err := s.attachments.DeleteLink(ctx, mem.TenantID, link.ID); err != nil {
		slog.Error("delete link", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    mem.TenantID,
		ActorUserID: &mem.UserID,
		Action:      "attachment.unlink",
		TargetType:  "document",
		TargetID:    &docUUID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"attachment_id": attID},
	})

	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/docs/id/"+docID+"/attachments", http.StatusSeeOther)
}

// --- Evidence Bundles ---

// handleBundlesList shows all evidence bundles
// GET /evidence-bundles
func (s *Server) handleBundlesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mem := auth.MembershipFromContext(ctx)
	if mem == nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	bundles, err := s.attachments.ListBundles(ctx, mem.TenantID)
	if err != nil {
		slog.Error("list bundles", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := s.newPageData(r)
	data.Title = "Evidence Bundles"
	data.Content = map[string]any{"Bundles": bundles}
	s.render(w, r, "bundles_list.html", data)
}

// handleBundleNew shows the new bundle form
// GET /evidence-bundles/new
func (s *Server) handleBundleNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mem := auth.MembershipFromContext(ctx)
	if mem == nil || !mem.IsEditor() {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	data := s.newPageData(r)
	data.Title = "New Evidence Bundle"
	s.render(w, r, "bundle_new.html", data)
}

// handleBundleCreate creates a new evidence bundle
// POST /evidence-bundles
func (s *Server) handleBundleCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mem := auth.MembershipFromContext(ctx)
	if mem == nil || !mem.IsEditor() {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	desc := strings.TrimSpace(r.FormValue("description"))

	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}

	bundle := &attachments.EvidenceBundle{
		TenantID:    mem.TenantID,
		Name:        name,
		Description: desc,
		CreatedBy:   mem.UserID,
	}

	if err := s.attachments.CreateBundle(ctx, bundle); err != nil {
		slog.Error("create bundle", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    mem.TenantID,
		ActorUserID: &mem.UserID,
		Action:      "bundle.create",
		TargetType:  "evidence_bundle",
		TargetID:    &bundle.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"name": name},
	})

	http.Redirect(w, r, "/evidence-bundles/"+bundle.ID.String(), http.StatusSeeOther)
}

// handleBundleView shows a single bundle
// GET /evidence-bundles/{id}
func (s *Server) handleBundleView(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mem := auth.MembershipFromContext(ctx)
	if mem == nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "id")
	idUUID, err := uuid.Parse(id)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	bundle, err := s.attachments.GetBundle(ctx, mem.TenantID, idUUID)
	if err != nil || bundle == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	items, err := s.attachments.ListBundleItems(ctx, mem.TenantID, idUUID)
	if err != nil {
		slog.Error("list bundle items", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := s.newPageData(r)
	data.Title = bundle.Name + " - Evidence Bundle"
	data.Content = map[string]any{"Bundle": bundle, "Items": items}
	s.render(w, r, "bundle_view.html", data)
}

// handleBundleAddItem adds an attachment to a bundle
// POST /evidence-bundles/{id}/items
func (s *Server) handleBundleAddItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mem := auth.MembershipFromContext(ctx)
	if mem == nil || !mem.IsEditor() {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	bundleID := chi.URLParam(r, "id")
	attachmentID := r.FormValue("attachment_id")
	note := strings.TrimSpace(r.FormValue("note"))

	if attachmentID == "" {
		http.Error(w, "attachment_id required", http.StatusBadRequest)
		return
	}

	bundleUUID, _ := uuid.Parse(bundleID)
	attUUID, _ := uuid.Parse(attachmentID)

	item := &attachments.EvidenceBundleItem{
		TenantID:     mem.TenantID,
		BundleID:     bundleUUID,
		AttachmentID: attUUID,
		Note:         note,
	}

	if err := s.attachments.AddBundleItem(ctx, item); err != nil {
		slog.Error("add bundle item", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    mem.TenantID,
		ActorUserID: &mem.UserID,
		Action:      "bundle.add_item",
		TargetType:  "evidence_bundle",
		TargetID:    &bundleUUID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"attachment_id": attachmentID},
	})

	http.Redirect(w, r, "/evidence-bundles/"+bundleID, http.StatusSeeOther)
}

// handleBundleRemoveItem removes an attachment from a bundle
// POST /evidence-bundles/{id}/items/{attID}/remove
func (s *Server) handleBundleRemoveItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mem := auth.MembershipFromContext(ctx)
	if mem == nil || !mem.IsEditor() {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	bundleID := chi.URLParam(r, "id")
	attID := chi.URLParam(r, "attID")

	bundleUUID, _ := uuid.Parse(bundleID)
	attUUID, _ := uuid.Parse(attID)

	if err := s.attachments.RemoveBundleItem(ctx, mem.TenantID, bundleUUID, attUUID); err != nil {
		slog.Error("remove bundle item", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    mem.TenantID,
		ActorUserID: &mem.UserID,
		Action:      "bundle.remove_item",
		TargetType:  "evidence_bundle",
		TargetID:    &bundleUUID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"attachment_id": attID},
	})

	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/evidence-bundles/"+bundleID, http.StatusSeeOther)
}

// handleBundleExport exports a bundle as a ZIP file
// GET /evidence-bundles/{id}/export
func (s *Server) handleBundleExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mem := auth.MembershipFromContext(ctx)
	if mem == nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "id")
	idUUID, err := uuid.Parse(id)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	bundle, err := s.attachments.GetBundle(ctx, mem.TenantID, idUUID)
	if err != nil || bundle == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	items, err := s.attachments.ListBundleItems(ctx, mem.TenantID, idUUID)
	if err != nil {
		slog.Error("list bundle items", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Create ZIP file
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	// Track filenames for deduplication
	usedNames := make(map[string]int)

	for _, item := range items {
		att := item.Attachment

		// Handle duplicate filenames
		filename := att.Filename
		if count, exists := usedNames[filename]; exists {
			ext := filepath.Ext(filename)
			base := strings.TrimSuffix(filename, ext)
			filename = fmt.Sprintf("%s_%d%s", base, count+1, ext)
		}
		usedNames[att.Filename]++

		// Get file content
		reader, err := s.storage.Retrieve(att.StorageKey)
		if err != nil {
			slog.Error("retrieve for export", "error", err, "attachment_id", att.ID)
			continue
		}

		// Add to ZIP
		fw, err := zipWriter.Create(filename)
		if err != nil {
			reader.Close()
			continue
		}

		io.Copy(fw, reader)
		reader.Close()
	}

	// Add manifest
	manifest, _ := zipWriter.Create("_manifest.txt")
	fmt.Fprintf(manifest, "Evidence Bundle: %s\n", bundle.Name)
	fmt.Fprintf(manifest, "Description: %s\n", bundle.Description)
	fmt.Fprintf(manifest, "Created: %s\n", bundle.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(manifest, "Exported: %s\n\n", r.Context().Value("now"))
	fmt.Fprintf(manifest, "Files:\n")
	for _, item := range items {
		fmt.Fprintf(manifest, "- %s (SHA256: %s)\n", item.Attachment.Filename, item.Attachment.SHA256)
		if item.Note != "" {
			fmt.Fprintf(manifest, "  Note: %s\n", item.Note)
		}
	}

	zipWriter.Close()

	// Audit export
	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    mem.TenantID,
		ActorUserID: &mem.UserID,
		Action:      "bundle.export",
		TargetType:  "evidence_bundle",
		TargetID:    &bundle.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"name": bundle.Name, "item_count": len(items)},
	})

	// Send ZIP
	safeFilename := strings.ReplaceAll(bundle.Name, " ", "_")
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, safeFilename))
	w.Write(buf.Bytes())
}

// handleBundleDelete deletes a bundle
// POST /evidence-bundles/{id}/delete
func (s *Server) handleBundleDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mem := auth.MembershipFromContext(ctx)
	if mem == nil || !mem.IsEditor() {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	id := chi.URLParam(r, "id")
	idUUID, err := uuid.Parse(id)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	bundle, err := s.attachments.GetBundle(ctx, mem.TenantID, idUUID)
	if err != nil || bundle == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if err := s.attachments.DeleteBundle(ctx, mem.TenantID, idUUID); err != nil {
		slog.Error("delete bundle", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    mem.TenantID,
		ActorUserID: &mem.UserID,
		Action:      "bundle.delete",
		TargetType:  "evidence_bundle",
		TargetID:    &idUUID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"name": bundle.Name},
	})

	http.Redirect(w, r, "/evidence-bundles", http.StatusSeeOther)
}

// Helper to format file sizes
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// handleAttachmentsAPI returns a JSON list of all attachments for the tenant (for picker UI)
// GET /api/attachments
func (s *Server) handleAttachmentsAPI(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mem := auth.MembershipFromContext(ctx)
	if mem == nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	atts, err := s.attachments.ListAll(ctx, mem.TenantID)
	if err != nil {
		slog.Error("list attachments", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	type attJSON struct {
		ID          string `json:"id"`
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
		SizeBytes   int64  `json:"size_bytes"`
		CreatedAt   string `json:"created_at"`
	}

	result := make([]attJSON, 0, len(atts))
	for _, a := range atts {
		result = append(result, attJSON{
			ID:          a.ID.String(),
			Filename:    a.Filename,
			ContentType: a.ContentType,
			SizeBytes:   a.SizeBytes,
			CreatedAt:   a.CreatedAt.Format("Jan 2, 2006 3:04 PM"),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
