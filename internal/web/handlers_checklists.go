package web

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/exedev/docstor/internal/audit"
	"github.com/exedev/docstor/internal/auth"
	"github.com/exedev/docstor/internal/checklists"
	"github.com/exedev/docstor/internal/pagination"
)

func (s *Server) handleChecklistsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	cls, err := s.checklists.List(ctx, tenant.ID)
	if err != nil {
		slog.Error("failed to list checklists", "error", err)
		data := s.newPageData(r)
		data.Title = "Checklists - Docstor"
		data.Error = "Failed to load checklists"
		s.render(w, r, "checklists_list.html", data)
		return
	}

	pg := pagination.FromRequest(r, pagination.DefaultPerPage)
	paged := pagination.ApplyToSlice(&pg, cls)
	pv := pg.View(r)

	data := s.newPageData(r)
	data.Title = "Checklists - Docstor"
	data.Pagination = &pv
	data.Content = paged
	s.render(w, r, "checklists_list.html", data)
}

func (s *Server) handleChecklistNew(w http.ResponseWriter, r *http.Request) {
	membership := auth.MembershipFromContext(r.Context())
	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	data := s.newPageData(r)
	data.Title = "New Checklist - Docstor"
	s.render(w, r, "checklist_form.html", data)
}

func (s *Server) handleChecklistCreate(w http.ResponseWriter, r *http.Request) {
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
		data.Title = "New Checklist - Docstor"
		data.Error = "Invalid form data"
		s.render(w, r, "checklist_form.html", data)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	itemsRaw := r.FormValue("items")

	if name == "" {
		data := s.newPageData(r)
		data.Title = "New Checklist - Docstor"
		data.Error = "Name is required"
		s.render(w, r, "checklist_form.html", data)
		return
	}

	// Parse items: one per line
	var items []string
	for _, line := range strings.Split(itemsRaw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			items = append(items, line)
		}
	}

	if len(items) == 0 {
		data := s.newPageData(r)
		data.Title = "New Checklist - Docstor"
		data.Error = "At least one checklist item is required"
		s.render(w, r, "checklist_form.html", data)
		return
	}

	cl, err := s.checklists.Create(ctx, checklists.CreateInput{
		TenantID:    tenant.ID,
		Name:        name,
		Description: description,
		CreatedBy:   user.ID,
		Items:       items,
	})
	if err != nil {
		slog.Error("failed to create checklist", "error", err)
		data := s.newPageData(r)
		data.Title = "New Checklist - Docstor"
		data.Error = "Failed to create checklist"
		s.render(w, r, "checklist_form.html", data)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionChecklistCreate,
		TargetType:  audit.TargetChecklist,
		TargetID:    &cl.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"name": name, "item_count": len(items)},
	})

	setFlashSuccess(w, "Checklist created successfully")
	http.Redirect(w, r, "/checklists/"+cl.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleChecklistView(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	clID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	cl, err := s.checklists.Get(ctx, tenant.ID, clID)
	if errors.Is(err, checklists.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get checklist", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Get instances using this checklist
	instances, _ := s.checklists.ListInstances(ctx, tenant.ID, "")
	var relatedInstances []checklists.Instance
	for _, inst := range instances {
		if inst.ChecklistID == clID {
			relatedInstances = append(relatedInstances, inst)
		}
	}

	data := s.newPageData(r)
	data.Title = cl.Name + " - Docstor"
	data.Content = map[string]any{
		"Checklist": cl,
		"Instances": relatedInstances,
	}
	s.render(w, r, "checklist_view.html", data)
}

func (s *Server) handleChecklistEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	clID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	cl, err := s.checklists.Get(ctx, tenant.ID, clID)
	if errors.Is(err, checklists.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get checklist", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := s.newPageData(r)
	data.Title = "Edit " + cl.Name + " - Docstor"
	data.Content = cl
	s.render(w, r, "checklist_form.html", data)
}

func (s *Server) handleChecklistUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	clID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		data := s.newPageData(r)
		data.Title = "Edit Checklist - Docstor"
		data.Error = "Invalid form data"
		s.render(w, r, "checklist_form.html", data)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	itemsRaw := r.FormValue("items")

	if name == "" {
		cl, _ := s.checklists.Get(ctx, tenant.ID, clID)
		data := s.newPageData(r)
		data.Title = "Edit Checklist - Docstor"
		data.Error = "Name is required"
		data.Content = cl
		s.render(w, r, "checklist_form.html", data)
		return
	}

	var items []string
	for _, line := range strings.Split(itemsRaw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			items = append(items, line)
		}
	}

	cl, err := s.checklists.Update(ctx, tenant.ID, clID, checklists.UpdateInput{
		Name:        name,
		Description: description,
		Items:       items,
	})
	if errors.Is(err, checklists.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to update checklist", "error", err)
		data := s.newPageData(r)
		data.Title = "Edit Checklist - Docstor"
		data.Error = "Failed to update checklist"
		s.render(w, r, "checklist_form.html", data)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionChecklistUpdate,
		TargetType:  audit.TargetChecklist,
		TargetID:    &cl.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"name": name},
	})

	setFlashSuccess(w, "Checklist updated successfully")
	http.Redirect(w, r, "/checklists/"+cl.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleChecklistDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	clID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	cl, _ := s.checklists.Get(ctx, tenant.ID, clID)

	if err := s.checklists.Delete(ctx, tenant.ID, clID); err != nil {
		if errors.Is(err, checklists.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("failed to delete checklist", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	metadata := map[string]any{}
	if cl != nil {
		metadata["name"] = cl.Name
	}
	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionChecklistDelete,
		TargetType:  audit.TargetChecklist,
		TargetID:    &clID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    metadata,
	})

	setFlashSuccess(w, "Checklist deleted")
	http.Redirect(w, r, "/checklists", http.StatusSeeOther)
}

// --- Instances ---

func (s *Server) handleInstancesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	status := r.URL.Query().Get("status")

	instances, err := s.checklists.ListInstances(ctx, tenant.ID, status)
	if err != nil {
		slog.Error("failed to list instances", "error", err)
		data := s.newPageData(r)
		data.Title = "Checklist Runs - Docstor"
		data.Error = "Failed to load checklist runs"
		s.render(w, r, "instances_list.html", data)
		return
	}

	pg := pagination.FromRequest(r, pagination.DefaultPerPage)
	paged := pagination.ApplyToSlice(&pg, instances)
	pv := pg.View(r)

	data := s.newPageData(r)
	data.Title = "Checklist Runs - Docstor"
	data.Pagination = &pv
	data.Content = map[string]any{
		"Instances": paged,
		"Status":    status,
	}
	s.render(w, r, "instances_list.html", data)
}

func (s *Server) handleInstanceStart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	checklistIDStr := r.FormValue("checklist_id")
	checklistID, err := uuid.Parse(checklistIDStr)
	if err != nil {
		http.Error(w, "Invalid checklist ID", http.StatusBadRequest)
		return
	}

	input := checklists.StartInput{
		TenantID:    tenant.ID,
		ChecklistID: checklistID,
		CreatedBy:   user.ID,
	}

	// Optional doc link
	if docIDStr := r.FormValue("doc_id"); docIDStr != "" {
		if docID, err := uuid.Parse(docIDStr); err == nil {
			linkedType := "document"
			input.LinkedType = &linkedType
			input.LinkedID = &docID
		}
	}

	inst, err := s.checklists.StartInstance(ctx, input)
	if err != nil {
		slog.Error("failed to start checklist instance", "error", err)
		setFlashError(w, "Failed to start checklist")
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionChecklistStart,
		TargetType:  audit.TargetCLInstance,
		TargetID:    &inst.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"checklist_id": checklistID.String()},
	})

	setFlashSuccess(w, "Checklist started")
	http.Redirect(w, r, "/checklist-instances/"+inst.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleInstanceView(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	instID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	inst, err := s.checklists.GetInstance(ctx, tenant.ID, instID)
	if errors.Is(err, checklists.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get instance", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := s.newPageData(r)
	data.Title = "Checklist Run - Docstor"
	data.Content = inst
	s.render(w, r, "instance_view.html", data)
}

func (s *Server) handleInstanceToggleItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	instID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	itemID, err := uuid.Parse(chi.URLParam(r, "itemID"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	result, err := s.checklists.ToggleItem(ctx, tenant.ID, instID, itemID, user.ID)
	if errors.Is(err, checklists.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to toggle checklist item", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionChecklistToggle,
		TargetType:  audit.TargetCLInstance,
		TargetID:    &instID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"item_id": itemID.String(), "done": result.Done, "text": result.Text},
	})

	// If HTMX request, return partial
	if r.Header.Get("HX-Request") == "true" {
		// Reload full instance for updated counts
		inst, _ := s.checklists.GetInstance(ctx, tenant.ID, instID)
		data := s.newPageData(r)
		data.Content = inst
		s.render(w, r, "instance_items_partial.html", data)
		return
	}

	http.Redirect(w, r, "/checklist-instances/"+instID.String(), http.StatusSeeOther)
}

func (s *Server) handleInstanceDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	instID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := s.checklists.DeleteInstance(ctx, tenant.ID, instID); err != nil {
		if errors.Is(err, checklists.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("failed to delete instance", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionChecklistDelete,
		TargetType:  audit.TargetCLInstance,
		TargetID:    &instID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
	})

	setFlashSuccess(w, "Checklist run deleted")
	http.Redirect(w, r, "/checklist-instances", http.StatusSeeOther)
}
