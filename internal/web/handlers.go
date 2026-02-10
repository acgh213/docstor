package web

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/exedev/docstor/internal/audit"
	"github.com/exedev/docstor/internal/auth"
	"github.com/exedev/docstor/internal/clients"
)

type PageData struct {
	Title      string
	User       *auth.User
	Tenant     *auth.Tenant
	Membership *auth.Membership
	Content    any
	Error      string
	Success    string
}

func (s *Server) newPageData(r *http.Request) PageData {
	return PageData{
		Title:      "Docstor",
		User:       auth.UserFromContext(r.Context()),
		Tenant:     auth.TenantFromContext(r.Context()),
		Membership: auth.MembershipFromContext(r.Context()),
	}
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data PageData) {
	// Read flash messages
	if flash := getFlash(w, r, flashSuccessCookie); flash != "" && data.Success == "" {
		data.Success = flash
	}
	if flash := getFlash(w, r, flashErrorCookie); flash != "" && data.Error == "" {
		data.Error = flash
	}
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("template render error", "template", name, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// Auth handlers

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	data := PageData{Title: "Login - Docstor"}
	s.render(w, r, "login.html", data)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		data := PageData{Title: "Login - Docstor", Error: "Invalid form data"}
		s.render(w, r, "login.html", data)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	if email == "" || password == "" {
		data := PageData{Title: "Login - Docstor", Error: "Email and password are required"}
		s.render(w, r, "login.html", data)
		return
	}

	// Find user by email
	var userID uuid.UUID
	var passwordHash string
	err := s.db.QueryRow(ctx, `
		SELECT id, password_hash FROM users WHERE email = $1
	`, email).Scan(&userID, &passwordHash)

	if errors.Is(err, pgx.ErrNoRows) {
		data := PageData{Title: "Login - Docstor", Error: "Invalid email or password"}
		s.render(w, r, "login.html", data)
		return
	}
	if err != nil {
		slog.Error("failed to query user", "error", err)
		data := PageData{Title: "Login - Docstor", Error: "An error occurred. Please try again."}
		s.render(w, r, "login.html", data)
		return
	}

	// Check password
	if err := auth.CheckPassword(passwordHash, password); err != nil {
		// Log failed attempt
		var tenantID uuid.UUID
		_ = s.db.QueryRow(ctx, `
			SELECT tenant_id FROM memberships WHERE user_id = $1 LIMIT 1
		`, userID).Scan(&tenantID)

		if tenantID != uuid.Nil {
			_ = s.audit.Log(ctx, audit.Entry{
				TenantID:    tenantID,
				ActorUserID: &userID,
				Action:      audit.ActionLoginFailed,
				TargetType:  audit.TargetUser,
				TargetID:    &userID,
				IP:          r.RemoteAddr,
				UserAgent:   r.UserAgent(),
				Metadata:    map[string]any{"email": email},
			})
		}

		data := PageData{Title: "Login - Docstor", Error: "Invalid email or password"}
		s.render(w, r, "login.html", data)
		return
	}

	// Get user's tenant membership (for MVP, use first/only membership)
	var tenantID uuid.UUID
	err = s.db.QueryRow(ctx, `
		SELECT tenant_id FROM memberships WHERE user_id = $1 LIMIT 1
	`, userID).Scan(&tenantID)

	if errors.Is(err, pgx.ErrNoRows) {
		data := PageData{Title: "Login - Docstor", Error: "No tenant membership found"}
		s.render(w, r, "login.html", data)
		return
	}
	if err != nil {
		slog.Error("failed to query membership", "error", err)
		data := PageData{Title: "Login - Docstor", Error: "An error occurred. Please try again."}
		s.render(w, r, "login.html", data)
		return
	}

	// Create session
	token, err := s.sessions.Create(ctx, userID, tenantID, r.RemoteAddr, r.UserAgent())
	if err != nil {
		slog.Error("failed to create session", "error", err)
		data := PageData{Title: "Login - Docstor", Error: "An error occurred. Please try again."}
		s.render(w, r, "login.html", data)
		return
	}

	// Update last login
	_, _ = s.db.Exec(ctx, `UPDATE users SET last_login_at = $1 WHERE id = $2`, time.Now(), userID)

	// Log successful login
	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenantID,
		ActorUserID: &userID,
		Action:      audit.ActionLoginSuccess,
		TargetType:  audit.TargetUser,
		TargetID:    &userID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"email": email},
	})

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.Env != "development",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(auth.SessionDuration.Seconds()),
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err == nil {
		_ = s.sessions.Delete(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// Dashboard handlers

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	data := s.newPageData(r)
	data.Title = "Dashboard - Docstor"
	s.render(w, r, "index.html", data)
}

func (s *Server) handleDocsHome(w http.ResponseWriter, r *http.Request) {
	data := s.newPageData(r)
	data.Title = "Documents - Docstor"
	s.render(w, r, "index.html", data)
}

// Client handlers

func (s *Server) handleClientsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	clientList, err := s.clients.List(ctx, tenant.ID)
	if err != nil {
		slog.Error("failed to list clients", "error", err)
		data := s.newPageData(r)
		data.Title = "Clients - Docstor"
		data.Error = "Failed to load clients"
		s.render(w, r, "clients_list.html", data)
		return
	}

	data := s.newPageData(r)
	data.Title = "Clients - Docstor"
	data.Content = clientList
	s.render(w, r, "clients_list.html", data)
}

func (s *Server) handleClientNew(w http.ResponseWriter, r *http.Request) {
	membership := auth.MembershipFromContext(r.Context())
	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	data := s.newPageData(r)
	data.Title = "New Client - Docstor"
	s.render(w, r, "client_form.html", data)
}

func (s *Server) handleClientCreate(w http.ResponseWriter, r *http.Request) {
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
		data.Title = "New Client - Docstor"
		data.Error = "Invalid form data"
		s.render(w, r, "client_form.html", data)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	code := strings.TrimSpace(r.FormValue("code"))
	notes := strings.TrimSpace(r.FormValue("notes"))

	if name == "" || code == "" {
		data := s.newPageData(r)
		data.Title = "New Client - Docstor"
		data.Error = "Name and code are required"
		s.render(w, r, "client_form.html", data)
		return
	}

	client, err := s.clients.Create(ctx, clients.CreateInput{
		TenantID: tenant.ID,
		Name:     name,
		Code:     code,
		Notes:    notes,
	})
	if err != nil {
		slog.Error("failed to create client", "error", err)
		data := s.newPageData(r)
		data.Title = "New Client - Docstor"
		data.Error = "Failed to create client. The code might already be in use."
		s.render(w, r, "client_form.html", data)
		return
	}

	// Audit log
	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionClientCreate,
		TargetType:  audit.TargetClient,
		TargetID:    &client.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"name": name, "code": code},
	})

	setFlashSuccess(w, "Client created successfully")
	http.Redirect(w, r, "/clients/"+client.ID.String(), http.StatusSeeOther)
}

func (s *Server) handleClientView(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	clientID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	client, err := s.clients.Get(ctx, tenant.ID, clientID)
	if errors.Is(err, clients.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get client", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := s.newPageData(r)
	data.Title = client.Name + " - Docstor"
	data.Content = client
	s.render(w, r, "client_view.html", data)
}

func (s *Server) handleClientEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	clientID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	client, err := s.clients.Get(ctx, tenant.ID, clientID)
	if errors.Is(err, clients.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to get client", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := s.newPageData(r)
	data.Title = "Edit " + client.Name + " - Docstor"
	data.Content = client
	s.render(w, r, "client_form.html", data)
}

func (s *Server) handleClientUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	membership := auth.MembershipFromContext(ctx)
	tenant := auth.TenantFromContext(ctx)
	user := auth.UserFromContext(ctx)

	if !membership.IsEditor() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	clientID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		data := s.newPageData(r)
		data.Title = "Edit Client - Docstor"
		data.Error = "Invalid form data"
		s.render(w, r, "client_form.html", data)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	code := strings.TrimSpace(r.FormValue("code"))
	notes := strings.TrimSpace(r.FormValue("notes"))

	if name == "" || code == "" {
		client, _ := s.clients.Get(ctx, tenant.ID, clientID)
		data := s.newPageData(r)
		data.Title = "Edit Client - Docstor"
		data.Error = "Name and code are required"
		data.Content = client
		s.render(w, r, "client_form.html", data)
		return
	}

	client, err := s.clients.Update(ctx, tenant.ID, clientID, clients.UpdateInput{
		Name:  name,
		Code:  code,
		Notes: notes,
	})
	if errors.Is(err, clients.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("failed to update client", "error", err)
		existingClient, _ := s.clients.Get(ctx, tenant.ID, clientID)
		data := s.newPageData(r)
		data.Title = "Edit Client - Docstor"
		data.Error = "Failed to update client"
		data.Content = existingClient
		s.render(w, r, "client_form.html", data)
		return
	}

	// Audit log
	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &user.ID,
		Action:      audit.ActionClientUpdate,
		TargetType:  audit.TargetClient,
		TargetID:    &client.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"name": name, "code": code},
	})

	setFlashSuccess(w, "Client updated successfully")
	http.Redirect(w, r, "/clients/"+client.ID.String(), http.StatusSeeOther)
}
