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
	"github.com/exedev/docstor/internal/sites"
)

type SiteFormData struct {
	Site    *sites.Site
	Clients []clients.Client
}

type SiteViewData struct {
	Site *sites.Site
}

func (s *Server) handleSitesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	var clientFilter *uuid.UUID
	if cid := r.URL.Query().Get("client_id"); cid != "" {
		if id, err := uuid.Parse(cid); err == nil {
			clientFilter = &id
		}
	}

	sitesList, err := s.sites.List(ctx, tenant.ID, clientFilter)
	if err != nil {
		slog.Error("failed to list sites", "error", err)
		data := s.newPageData(r)
		data.Title = "Sites - Docstor"
		data.Error = "Failed to load sites"
		s.render(w, r, "sites_list.html", data)
		return
	}

	data := s.newPageData(r)
	data.Title = "Sites - Docstor"
	data.Content = sitesList
	s.render(w, r, "sites_list.html", data)
}

func (s *Server) handleSiteNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	clientsList, _ := s.clients.List(ctx, tenant.ID)

	data := s.newPageData(r)
	data.Title = "New Site - Docstor"
	data.Content = SiteFormData{Clients: clientsList}

	// Pre-select client from query param
	if cid := r.URL.Query().Get("client_id"); cid != "" {
		data.Content = SiteFormData{
			Clients: clientsList,
			Site:    &sites.Site{ClientID: uuid.MustParse(cid)},
		}
	}

	s.render(w, r, "site_form.html", data)
}

func (s *Server) handleSiteCreate(w http.ResponseWriter, r *http.Request) {
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

	name := strings.TrimSpace(r.FormValue("name"))
	clientIDStr := r.FormValue("client_id")
	address := strings.TrimSpace(r.FormValue("address"))
	notes := strings.TrimSpace(r.FormValue("notes"))

	clientID, err := uuid.Parse(clientIDStr)
	if err != nil || name == "" {
		clientsList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "New Site - Docstor"
		data.Error = "Name and Client are required"
		data.Content = SiteFormData{Clients: clientsList}
		s.render(w, r, "site_form.html", data)
		return
	}

	site, err := s.sites.Create(ctx, sites.CreateInput{
		TenantID: tenant.ID,
		ClientID: clientID,
		Name:     name,
		Address:  address,
		Notes:    notes,
	})
	if err != nil {
		slog.Error("failed to create site", "error", err)
		clientsList, _ := s.clients.List(ctx, tenant.ID)
		data := s.newPageData(r)
		data.Title = "New Site - Docstor"
		data.Error = "Failed to create site"
		data.Content = SiteFormData{Clients: clientsList}
		s.render(w, r, "site_form.html", data)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionSiteCreate,
		TargetType:  audit.TargetSite,
		TargetID:    &site.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"name": name, "client_id": clientID.String()},
	})

	setFlashSuccess(w, "Site created successfully")
	http.Redirect(w, r, "/sites/"+site.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleSiteView(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	site, err := s.sites.Get(ctx, tenant.ID, id)
	if errors.Is(err, sites.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get site", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Load related CMDB items for this site
	systems, _ := s.cmdb.ListSystems(ctx, tenant.ID, nil)
	vendors, _ := s.cmdb.ListVendors(ctx, tenant.ID, &site.ClientID)
	contacts, _ := s.cmdb.ListContacts(ctx, tenant.ID, nil)
	circuits, _ := s.cmdb.ListCircuits(ctx, tenant.ID, nil)

	// Filter by site_id (systems, contacts, circuits have site_id now)
	// For now, show all items for this client since site_id filtering
	// needs the CMDB repos updated. We'll show the client's items.

	data := s.newPageData(r)
	data.Title = site.Name + " - Docstor"
	data.Content = map[string]any{
		"Site":     site,
		"Systems":  systems,
		"Vendors":  vendors,
		"Contacts": contacts,
		"Circuits": circuits,
	}
	s.render(w, r, "site_view.html", data)
}

func (s *Server) handleSiteEdit(w http.ResponseWriter, r *http.Request) {
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

	site, err := s.sites.Get(ctx, tenant.ID, id)
	if errors.Is(err, sites.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	clientsList, _ := s.clients.List(ctx, tenant.ID)

	data := s.newPageData(r)
	data.Title = "Edit " + site.Name + " - Docstor"
	data.Content = SiteFormData{Site: site, Clients: clientsList}
	s.render(w, r, "site_form.html", data)
}

func (s *Server) handleSiteUpdate(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	clientIDStr := r.FormValue("client_id")
	address := strings.TrimSpace(r.FormValue("address"))
	notes := strings.TrimSpace(r.FormValue("notes"))

	clientID, err := uuid.Parse(clientIDStr)
	if err != nil || name == "" {
		clientsList, _ := s.clients.List(ctx, tenant.ID)
		site, _ := s.sites.Get(ctx, tenant.ID, id)
		data := s.newPageData(r)
		data.Title = "Edit Site - Docstor"
		data.Error = "Name and Client are required"
		data.Content = SiteFormData{Site: site, Clients: clientsList}
		s.render(w, r, "site_form.html", data)
		return
	}

	site, err := s.sites.Update(ctx, tenant.ID, id, sites.UpdateInput{
		ClientID: clientID,
		Name:     name,
		Address:  address,
		Notes:    notes,
	})
	if errors.Is(err, sites.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to update site", "error", err)
		http.Error(w, "Failed to update site", http.StatusInternalServerError)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionSiteUpdate,
		TargetType:  audit.TargetSite,
		TargetID:    &id,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"name": name},
	})

	setFlashSuccess(w, "Site updated successfully")
	http.Redirect(w, r, "/sites/"+site.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleSiteDelete(w http.ResponseWriter, r *http.Request) {
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

	site, _ := s.sites.Get(ctx, tenant.ID, id)

	if err := s.sites.Delete(ctx, tenant.ID, id); err != nil {
		if errors.Is(err, sites.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		slog.Error("failed to delete site", "error", err)
		http.Error(w, "Failed to delete site", http.StatusInternalServerError)
		return
	}

	metadata := map[string]any{"id": id.String()}
	if site != nil {
		metadata["name"] = site.Name
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionSiteDelete,
		TargetType:  audit.TargetSite,
		TargetID:    &id,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    metadata,
	})

	setFlashSuccess(w, "Site deleted")
	http.Redirect(w, r, "/sites", http.StatusSeeOther)
}
