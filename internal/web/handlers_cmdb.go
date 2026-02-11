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
	"github.com/exedev/docstor/internal/clients"
	"github.com/exedev/docstor/internal/cmdb"
)

// parseOptionalClientID extracts an optional client_id from the form.
func parseOptionalClientID(r *http.Request) *uuid.UUID {
	cidStr := strings.TrimSpace(r.FormValue("client_id"))
	if cidStr == "" {
		return nil
	}
	cid, err := uuid.Parse(cidStr)
	if err != nil {
		return nil
	}
	return &cid
}

// --- Content structs ---

type SystemsListData struct {
	Systems          []cmdb.System
	Clients          []clients.Client
	SelectedClientID string
}

type SystemFormData struct {
	System  *cmdb.System
	Clients []clients.Client
}

type VendorsListData struct {
	Vendors          []cmdb.Vendor
	Clients          []clients.Client
	SelectedClientID string
}

type VendorFormData struct {
	Vendor  *cmdb.Vendor
	Clients []clients.Client
}

type ContactsListData struct {
	Contacts         []cmdb.Contact
	Clients          []clients.Client
	SelectedClientID string
}

type ContactFormData struct {
	Contact *cmdb.Contact
	Clients []clients.Client
}

type CircuitsListData struct {
	Circuits         []cmdb.Circuit
	Clients          []clients.Client
	SelectedClientID string
}

type CircuitFormData struct {
	Circuit *cmdb.Circuit
	Clients []clients.Client
}

// ===================== SYSTEMS =====================

func (s *Server) handleSystemsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	var clientFilter *uuid.UUID
	if cidStr := r.URL.Query().Get("client_id"); cidStr != "" {
		if cid, err := uuid.Parse(cidStr); err == nil {
			clientFilter = &cid
		}
	}

	systems, err := s.cmdb.ListSystems(ctx, tenant.ID, clientFilter)
	if err != nil {
		slog.Error("failed to list systems", "error", err)
		data := s.newPageData(r)
		data.Title = "Systems - Docstor"
		data.Error = "Failed to load systems"
		s.render(w, r, "systems_list.html", data)
		return
	}

	clientList, _ := s.clients.List(ctx, tenant.ID)

	data := s.newPageData(r)
	data.Title = "Systems - Docstor"
	data.Content = SystemsListData{
		Systems:          systems,
		Clients:          clientList,
		SelectedClientID: r.URL.Query().Get("client_id"),
	}
	s.render(w, r, "systems_list.html", data)
}

func (s *Server) handleSystemNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	clientList, _ := s.clients.List(ctx, tenant.ID)

	data := s.newPageData(r)
	data.Title = "New System - Docstor"
	data.Content = SystemFormData{Clients: clientList}
	s.render(w, r, "system_form.html", data)
}

func (s *Server) handleSystemCreate(w http.ResponseWriter, r *http.Request) {
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
		data.Title = "New System - Docstor"
		data.Error = "Invalid form data"
		data.Content = SystemFormData{Clients: clientList}
		s.render(w, r, "system_form.html", data)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	systemType := strings.TrimSpace(r.FormValue("system_type"))
	environment := strings.TrimSpace(r.FormValue("environment"))
	fqdn := strings.TrimSpace(r.FormValue("fqdn"))
	ip := strings.TrimSpace(r.FormValue("ip"))
	os := strings.TrimSpace(r.FormValue("os"))
	notes := strings.TrimSpace(r.FormValue("notes"))
	clientID := parseOptionalClientID(r)

	if name == "" || systemType == "" || environment == "" {
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "New System - Docstor"
		data.Error = "Name, system type, and environment are required"
		data.Content = SystemFormData{Clients: clientList}
		s.render(w, r, "system_form.html", data)
		return
	}

	sys, err := s.cmdb.CreateSystem(ctx, cmdb.CreateSystemInput{
		TenantID:    tenant.ID,
		ClientID:    clientID,
		SystemType:  systemType,
		Name:        name,
		FQDN:        fqdn,
		IP:          ip,
		OS:          os,
		Environment: environment,
		Notes:       notes,
		OwnerUserID: &user.ID,
	})
	if err != nil {
		slog.Error("failed to create system", "error", err)
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "New System - Docstor"
		data.Error = "Failed to create system"
		data.Content = SystemFormData{Clients: clientList}
		s.render(w, r, "system_form.html", data)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionSystemCreate,
		TargetType:  audit.TargetSystem,
		TargetID:    &sys.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"name": name},
	})

	setFlashSuccess(w, "System created successfully")
	http.Redirect(w, r, "/systems/"+sys.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleSystemView(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	systemID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	sys, err := s.cmdb.GetSystem(ctx, tenant.ID, systemID)
	if errors.Is(err, cmdb.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get system", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := s.newPageData(r)
	data.Title = sys.Name + " - Docstor"
	data.Content = sys
	s.render(w, r, "system_view.html", data)
}

func (s *Server) handleSystemEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	systemID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	sys, err := s.cmdb.GetSystem(ctx, tenant.ID, systemID)
	if errors.Is(err, cmdb.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get system", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	clientList, _ := s.clients.List(ctx, tenant.ID)

	data := s.newPageData(r)
	data.Title = "Edit " + sys.Name + " - Docstor"
	data.Content = SystemFormData{System: sys, Clients: clientList}
	s.render(w, r, "system_form.html", data)
}

func (s *Server) handleSystemUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	systemID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "Edit System - Docstor"
		data.Error = "Invalid form data"
		data.Content = SystemFormData{Clients: clientList}
		s.render(w, r, "system_form.html", data)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	systemType := strings.TrimSpace(r.FormValue("system_type"))
	environment := strings.TrimSpace(r.FormValue("environment"))
	fqdn := strings.TrimSpace(r.FormValue("fqdn"))
	ip := strings.TrimSpace(r.FormValue("ip"))
	os := strings.TrimSpace(r.FormValue("os"))
	notes := strings.TrimSpace(r.FormValue("notes"))
	clientID := parseOptionalClientID(r)

	if name == "" || systemType == "" || environment == "" {
		existing, _ := s.cmdb.GetSystem(ctx, tenant.ID, systemID)
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "Edit System - Docstor"
		data.Error = "Name, system type, and environment are required"
		data.Content = SystemFormData{System: existing, Clients: clientList}
		s.render(w, r, "system_form.html", data)
		return
	}

	sys, err := s.cmdb.UpdateSystem(ctx, tenant.ID, systemID, cmdb.UpdateSystemInput{
		ClientID:    clientID,
		SystemType:  systemType,
		Name:        name,
		FQDN:        fqdn,
		IP:          ip,
		OS:          os,
		Environment: environment,
		Notes:       notes,
	})
	if errors.Is(err, cmdb.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to update system", "error", err)
		existing, _ := s.cmdb.GetSystem(ctx, tenant.ID, systemID)
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "Edit System - Docstor"
		data.Error = "Failed to update system"
		data.Content = SystemFormData{System: existing, Clients: clientList}
		s.render(w, r, "system_form.html", data)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionSystemUpdate,
		TargetType:  audit.TargetSystem,
		TargetID:    &sys.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"name": name},
	})

	setFlashSuccess(w, "System updated successfully")
	http.Redirect(w, r, "/systems/"+sys.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleSystemDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	systemID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Get system name for audit log before deleting
	sys, err := s.cmdb.GetSystem(ctx, tenant.ID, systemID)
	if errors.Is(err, cmdb.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get system for delete", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := s.cmdb.DeleteSystem(ctx, tenant.ID, systemID); err != nil {
		slog.Error("failed to delete system", "error", err)
		http.Error(w, "Failed to delete system", http.StatusInternalServerError)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionSystemDelete,
		TargetType:  audit.TargetSystem,
		TargetID:    &systemID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"name": sys.Name},
	})

	setFlashSuccess(w, "System deleted successfully")
	http.Redirect(w, r, "/systems", http.StatusSeeOther)
}

// ===================== VENDORS =====================

func (s *Server) handleVendorsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	var clientFilter *uuid.UUID
	if cidStr := r.URL.Query().Get("client_id"); cidStr != "" {
		if cid, err := uuid.Parse(cidStr); err == nil {
			clientFilter = &cid
		}
	}

	vendors, err := s.cmdb.ListVendors(ctx, tenant.ID, clientFilter)
	if err != nil {
		slog.Error("failed to list vendors", "error", err)
		data := s.newPageData(r)
		data.Title = "Vendors - Docstor"
		data.Error = "Failed to load vendors"
		s.render(w, r, "vendors_list.html", data)
		return
	}

	clientList, _ := s.clients.List(ctx, tenant.ID)

	data := s.newPageData(r)
	data.Title = "Vendors - Docstor"
	data.Content = VendorsListData{
		Vendors:          vendors,
		Clients:          clientList,
		SelectedClientID: r.URL.Query().Get("client_id"),
	}
	s.render(w, r, "vendors_list.html", data)
}

func (s *Server) handleVendorNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	clientList, _ := s.clients.List(ctx, tenant.ID)

	data := s.newPageData(r)
	data.Title = "New Vendor - Docstor"
	data.Content = VendorFormData{Clients: clientList}
	s.render(w, r, "vendor_form.html", data)
}

func (s *Server) handleVendorCreate(w http.ResponseWriter, r *http.Request) {
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
		data.Title = "New Vendor - Docstor"
		data.Error = "Invalid form data"
		data.Content = VendorFormData{Clients: clientList}
		s.render(w, r, "vendor_form.html", data)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	vendorType := strings.TrimSpace(r.FormValue("vendor_type"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	email := strings.TrimSpace(r.FormValue("email"))
	portalURL := strings.TrimSpace(r.FormValue("portal_url"))
	escalationNotes := strings.TrimSpace(r.FormValue("escalation_notes"))
	notes := strings.TrimSpace(r.FormValue("notes"))
	clientID := parseOptionalClientID(r)

	if name == "" || vendorType == "" {
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "New Vendor - Docstor"
		data.Error = "Name and vendor type are required"
		data.Content = VendorFormData{Clients: clientList}
		s.render(w, r, "vendor_form.html", data)
		return
	}

	v, err := s.cmdb.CreateVendor(ctx, cmdb.CreateVendorInput{
		TenantID:        tenant.ID,
		ClientID:        clientID,
		Name:            name,
		VendorType:      vendorType,
		Phone:           phone,
		Email:           email,
		PortalURL:       portalURL,
		EscalationNotes: escalationNotes,
		Notes:           notes,
	})
	if err != nil {
		slog.Error("failed to create vendor", "error", err)
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "New Vendor - Docstor"
		data.Error = "Failed to create vendor"
		data.Content = VendorFormData{Clients: clientList}
		s.render(w, r, "vendor_form.html", data)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionVendorCreate,
		TargetType:  audit.TargetVendor,
		TargetID:    &v.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"name": name},
	})

	setFlashSuccess(w, "Vendor created successfully")
	http.Redirect(w, r, "/vendors/"+v.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleVendorView(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	v, err := s.cmdb.GetVendor(ctx, tenant.ID, id)
	if errors.Is(err, cmdb.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get vendor", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := s.newPageData(r)
	data.Title = v.Name + " - Docstor"
	data.Content = v
	s.render(w, r, "vendor_view.html", data)
}

func (s *Server) handleVendorEdit(w http.ResponseWriter, r *http.Request) {
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

	v, err := s.cmdb.GetVendor(ctx, tenant.ID, id)
	if errors.Is(err, cmdb.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get vendor", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	clientList, _ := s.clients.List(ctx, tenant.ID)

	data := s.newPageData(r)
	data.Title = "Edit " + v.Name + " - Docstor"
	data.Content = VendorFormData{Vendor: v, Clients: clientList}
	s.render(w, r, "vendor_form.html", data)
}

func (s *Server) handleVendorUpdate(w http.ResponseWriter, r *http.Request) {
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
		data.Title = "Edit Vendor - Docstor"
		data.Error = "Invalid form data"
		data.Content = VendorFormData{Clients: clientList}
		s.render(w, r, "vendor_form.html", data)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	vendorType := strings.TrimSpace(r.FormValue("vendor_type"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	email := strings.TrimSpace(r.FormValue("email"))
	portalURL := strings.TrimSpace(r.FormValue("portal_url"))
	escalationNotes := strings.TrimSpace(r.FormValue("escalation_notes"))
	notes := strings.TrimSpace(r.FormValue("notes"))
	clientID := parseOptionalClientID(r)

	if name == "" || vendorType == "" {
		existing, _ := s.cmdb.GetVendor(ctx, tenant.ID, id)
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "Edit Vendor - Docstor"
		data.Error = "Name and vendor type are required"
		data.Content = VendorFormData{Vendor: existing, Clients: clientList}
		s.render(w, r, "vendor_form.html", data)
		return
	}

	v, err := s.cmdb.UpdateVendor(ctx, tenant.ID, id, cmdb.UpdateVendorInput{
		ClientID:        clientID,
		Name:            name,
		VendorType:      vendorType,
		Phone:           phone,
		Email:           email,
		PortalURL:       portalURL,
		EscalationNotes: escalationNotes,
		Notes:           notes,
	})
	if errors.Is(err, cmdb.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to update vendor", "error", err)
		existing, _ := s.cmdb.GetVendor(ctx, tenant.ID, id)
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "Edit Vendor - Docstor"
		data.Error = "Failed to update vendor"
		data.Content = VendorFormData{Vendor: existing, Clients: clientList}
		s.render(w, r, "vendor_form.html", data)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionVendorUpdate,
		TargetType:  audit.TargetVendor,
		TargetID:    &v.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"name": name},
	})

	setFlashSuccess(w, "Vendor updated successfully")
	http.Redirect(w, r, "/vendors/"+v.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleVendorDelete(w http.ResponseWriter, r *http.Request) {
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

	v, err := s.cmdb.GetVendor(ctx, tenant.ID, id)
	if errors.Is(err, cmdb.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get vendor for delete", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := s.cmdb.DeleteVendor(ctx, tenant.ID, id); err != nil {
		slog.Error("failed to delete vendor", "error", err)
		http.Error(w, "Failed to delete vendor", http.StatusInternalServerError)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionVendorDelete,
		TargetType:  audit.TargetVendor,
		TargetID:    &id,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"name": v.Name},
	})

	setFlashSuccess(w, "Vendor deleted successfully")
	http.Redirect(w, r, "/vendors", http.StatusSeeOther)
}

// ===================== CONTACTS =====================

func (s *Server) handleContactsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	var clientFilter *uuid.UUID
	if cidStr := r.URL.Query().Get("client_id"); cidStr != "" {
		if cid, err := uuid.Parse(cidStr); err == nil {
			clientFilter = &cid
		}
	}

	contacts, err := s.cmdb.ListContacts(ctx, tenant.ID, clientFilter)
	if err != nil {
		slog.Error("failed to list contacts", "error", err)
		data := s.newPageData(r)
		data.Title = "Contacts - Docstor"
		data.Error = "Failed to load contacts"
		s.render(w, r, "contacts_list.html", data)
		return
	}

	clientList, _ := s.clients.List(ctx, tenant.ID)

	data := s.newPageData(r)
	data.Title = "Contacts - Docstor"
	data.Content = ContactsListData{
		Contacts:         contacts,
		Clients:          clientList,
		SelectedClientID: r.URL.Query().Get("client_id"),
	}
	s.render(w, r, "contacts_list.html", data)
}

func (s *Server) handleContactNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	clientList, _ := s.clients.List(ctx, tenant.ID)

	data := s.newPageData(r)
	data.Title = "New Contact - Docstor"
	data.Content = ContactFormData{Clients: clientList}
	s.render(w, r, "contact_form.html", data)
}

func (s *Server) handleContactCreate(w http.ResponseWriter, r *http.Request) {
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
		data.Title = "New Contact - Docstor"
		data.Error = "Invalid form data"
		data.Content = ContactFormData{Clients: clientList}
		s.render(w, r, "contact_form.html", data)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	role := strings.TrimSpace(r.FormValue("role"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	email := strings.TrimSpace(r.FormValue("email"))
	notes := strings.TrimSpace(r.FormValue("notes"))
	clientID := parseOptionalClientID(r)

	if name == "" {
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "New Contact - Docstor"
		data.Error = "Name is required"
		data.Content = ContactFormData{Clients: clientList}
		s.render(w, r, "contact_form.html", data)
		return
	}

	c, err := s.cmdb.CreateContact(ctx, cmdb.CreateContactInput{
		TenantID: tenant.ID,
		ClientID: clientID,
		Name:     name,
		Role:     role,
		Phone:    phone,
		Email:    email,
		Notes:    notes,
	})
	if err != nil {
		slog.Error("failed to create contact", "error", err)
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "New Contact - Docstor"
		data.Error = "Failed to create contact"
		data.Content = ContactFormData{Clients: clientList}
		s.render(w, r, "contact_form.html", data)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionContactCreate,
		TargetType:  audit.TargetContact,
		TargetID:    &c.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"name": name},
	})

	setFlashSuccess(w, "Contact created successfully")
	http.Redirect(w, r, "/contacts/"+c.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleContactView(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	c, err := s.cmdb.GetContact(ctx, tenant.ID, id)
	if errors.Is(err, cmdb.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get contact", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := s.newPageData(r)
	data.Title = c.Name + " - Docstor"
	data.Content = c
	s.render(w, r, "contact_view.html", data)
}

func (s *Server) handleContactEdit(w http.ResponseWriter, r *http.Request) {
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

	c, err := s.cmdb.GetContact(ctx, tenant.ID, id)
	if errors.Is(err, cmdb.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get contact", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	clientList, _ := s.clients.List(ctx, tenant.ID)

	data := s.newPageData(r)
	data.Title = "Edit " + c.Name + " - Docstor"
	data.Content = ContactFormData{Contact: c, Clients: clientList}
	s.render(w, r, "contact_form.html", data)
}

func (s *Server) handleContactUpdate(w http.ResponseWriter, r *http.Request) {
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
		data.Title = "Edit Contact - Docstor"
		data.Error = "Invalid form data"
		data.Content = ContactFormData{Clients: clientList}
		s.render(w, r, "contact_form.html", data)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	role := strings.TrimSpace(r.FormValue("role"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	email := strings.TrimSpace(r.FormValue("email"))
	notes := strings.TrimSpace(r.FormValue("notes"))
	clientID := parseOptionalClientID(r)

	if name == "" {
		existing, _ := s.cmdb.GetContact(ctx, tenant.ID, id)
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "Edit Contact - Docstor"
		data.Error = "Name is required"
		data.Content = ContactFormData{Contact: existing, Clients: clientList}
		s.render(w, r, "contact_form.html", data)
		return
	}

	c, err := s.cmdb.UpdateContact(ctx, tenant.ID, id, cmdb.UpdateContactInput{
		ClientID: clientID,
		Name:     name,
		Role:     role,
		Phone:    phone,
		Email:    email,
		Notes:    notes,
	})
	if errors.Is(err, cmdb.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to update contact", "error", err)
		existing, _ := s.cmdb.GetContact(ctx, tenant.ID, id)
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "Edit Contact - Docstor"
		data.Error = "Failed to update contact"
		data.Content = ContactFormData{Contact: existing, Clients: clientList}
		s.render(w, r, "contact_form.html", data)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionContactUpdate,
		TargetType:  audit.TargetContact,
		TargetID:    &c.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"name": name},
	})

	setFlashSuccess(w, "Contact updated successfully")
	http.Redirect(w, r, "/contacts/"+c.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleContactDelete(w http.ResponseWriter, r *http.Request) {
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

	c, err := s.cmdb.GetContact(ctx, tenant.ID, id)
	if errors.Is(err, cmdb.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get contact for delete", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := s.cmdb.DeleteContact(ctx, tenant.ID, id); err != nil {
		slog.Error("failed to delete contact", "error", err)
		http.Error(w, "Failed to delete contact", http.StatusInternalServerError)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionContactDelete,
		TargetType:  audit.TargetContact,
		TargetID:    &id,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"name": c.Name},
	})

	setFlashSuccess(w, "Contact deleted successfully")
	http.Redirect(w, r, "/contacts", http.StatusSeeOther)
}

// ===================== CIRCUITS =====================

func (s *Server) handleCircuitsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	var clientFilter *uuid.UUID
	if cidStr := r.URL.Query().Get("client_id"); cidStr != "" {
		if cid, err := uuid.Parse(cidStr); err == nil {
			clientFilter = &cid
		}
	}

	circuits, err := s.cmdb.ListCircuits(ctx, tenant.ID, clientFilter)
	if err != nil {
		slog.Error("failed to list circuits", "error", err)
		data := s.newPageData(r)
		data.Title = "Circuits - Docstor"
		data.Error = "Failed to load circuits"
		s.render(w, r, "circuits_list.html", data)
		return
	}

	clientList, _ := s.clients.List(ctx, tenant.ID)

	data := s.newPageData(r)
	data.Title = "Circuits - Docstor"
	data.Content = CircuitsListData{
		Circuits:         circuits,
		Clients:          clientList,
		SelectedClientID: r.URL.Query().Get("client_id"),
	}
	s.render(w, r, "circuits_list.html", data)
}

func (s *Server) handleCircuitNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	clientList, _ := s.clients.List(ctx, tenant.ID)

	data := s.newPageData(r)
	data.Title = "New Circuit - Docstor"
	data.Content = CircuitFormData{Clients: clientList}
	s.render(w, r, "circuit_form.html", data)
}

func (s *Server) handleCircuitCreate(w http.ResponseWriter, r *http.Request) {
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
		data.Title = "New Circuit - Docstor"
		data.Error = "Invalid form data"
		data.Content = CircuitFormData{Clients: clientList}
		s.render(w, r, "circuit_form.html", data)
		return
	}

	provider := strings.TrimSpace(r.FormValue("provider"))
	circuitID := strings.TrimSpace(r.FormValue("circuit_id"))
	circuitType := strings.TrimSpace(r.FormValue("circuit_type"))
	wanIP := strings.TrimSpace(r.FormValue("wan_ip"))
	speed := strings.TrimSpace(r.FormValue("speed"))
	notes := strings.TrimSpace(r.FormValue("notes"))
	clientID := parseOptionalClientID(r)

	if provider == "" || circuitID == "" || circuitType == "" {
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "New Circuit - Docstor"
		data.Error = "Provider, circuit ID, and circuit type are required"
		data.Content = CircuitFormData{Clients: clientList}
		s.render(w, r, "circuit_form.html", data)
		return
	}

	c, err := s.cmdb.CreateCircuit(ctx, cmdb.CreateCircuitInput{
		TenantID:    tenant.ID,
		ClientID:    clientID,
		Provider:    provider,
		CircuitID:   circuitID,
		CircuitType: circuitType,
		WanIP:       wanIP,
		Speed:       speed,
		Notes:       notes,
	})
	if err != nil {
		slog.Error("failed to create circuit", "error", err)
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "New Circuit - Docstor"
		data.Error = "Failed to create circuit"
		data.Content = CircuitFormData{Clients: clientList}
		s.render(w, r, "circuit_form.html", data)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionCircuitCreate,
		TargetType:  audit.TargetCircuit,
		TargetID:    &c.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"provider": provider, "circuit_id": circuitID},
	})

	setFlashSuccess(w, "Circuit created successfully")
	http.Redirect(w, r, "/circuits/"+c.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleCircuitView(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	c, err := s.cmdb.GetCircuit(ctx, tenant.ID, id)
	if errors.Is(err, cmdb.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get circuit", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := s.newPageData(r)
	data.Title = c.Provider + " " + c.CircuitID + " - Docstor"
	data.Content = c
	s.render(w, r, "circuit_view.html", data)
}

func (s *Server) handleCircuitEdit(w http.ResponseWriter, r *http.Request) {
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

	c, err := s.cmdb.GetCircuit(ctx, tenant.ID, id)
	if errors.Is(err, cmdb.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get circuit", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	clientList, _ := s.clients.List(ctx, tenant.ID)

	data := s.newPageData(r)
	data.Title = "Edit Circuit - Docstor"
	data.Content = CircuitFormData{Circuit: c, Clients: clientList}
	s.render(w, r, "circuit_form.html", data)
}

func (s *Server) handleCircuitUpdate(w http.ResponseWriter, r *http.Request) {
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
		data.Title = "Edit Circuit - Docstor"
		data.Error = "Invalid form data"
		data.Content = CircuitFormData{Clients: clientList}
		s.render(w, r, "circuit_form.html", data)
		return
	}

	provider := strings.TrimSpace(r.FormValue("provider"))
	circuitID := strings.TrimSpace(r.FormValue("circuit_id"))
	circuitType := strings.TrimSpace(r.FormValue("circuit_type"))
	wanIP := strings.TrimSpace(r.FormValue("wan_ip"))
	speed := strings.TrimSpace(r.FormValue("speed"))
	notes := strings.TrimSpace(r.FormValue("notes"))
	clientID := parseOptionalClientID(r)

	if provider == "" || circuitID == "" || circuitType == "" {
		existing, _ := s.cmdb.GetCircuit(ctx, tenant.ID, id)
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "Edit Circuit - Docstor"
		data.Error = "Provider, circuit ID, and circuit type are required"
		data.Content = CircuitFormData{Circuit: existing, Clients: clientList}
		s.render(w, r, "circuit_form.html", data)
		return
	}

	c, err := s.cmdb.UpdateCircuit(ctx, tenant.ID, id, cmdb.UpdateCircuitInput{
		ClientID:    clientID,
		Provider:    provider,
		CircuitID:   circuitID,
		CircuitType: circuitType,
		WanIP:       wanIP,
		Speed:       speed,
		Notes:       notes,
	})
	if errors.Is(err, cmdb.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to update circuit", "error", err)
		existing, _ := s.cmdb.GetCircuit(ctx, tenant.ID, id)
		clientList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "Edit Circuit - Docstor"
		data.Error = "Failed to update circuit"
		data.Content = CircuitFormData{Circuit: existing, Clients: clientList}
		s.render(w, r, "circuit_form.html", data)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionCircuitUpdate,
		TargetType:  audit.TargetCircuit,
		TargetID:    &c.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"provider": provider, "circuit_id": circuitID},
	})

	setFlashSuccess(w, "Circuit updated successfully")
	http.Redirect(w, r, "/circuits/"+c.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleCircuitDelete(w http.ResponseWriter, r *http.Request) {
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

	c, err := s.cmdb.GetCircuit(ctx, tenant.ID, id)
	if errors.Is(err, cmdb.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get circuit for delete", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := s.cmdb.DeleteCircuit(ctx, tenant.ID, id); err != nil {
		slog.Error("failed to delete circuit", "error", err)
		http.Error(w, "Failed to delete circuit", http.StatusInternalServerError)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionCircuitDelete,
		TargetType:  audit.TargetCircuit,
		TargetID:    &id,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"provider": c.Provider, "circuit_id": c.CircuitID},
	})

	setFlashSuccess(w, "Circuit deleted successfully")
	http.Redirect(w, r, "/circuits", http.StatusSeeOther)
}
