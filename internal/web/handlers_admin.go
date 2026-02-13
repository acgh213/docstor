package web

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/acgh213/docstor/internal/audit"
	"github.com/acgh213/docstor/internal/auth"
	"github.com/acgh213/docstor/internal/pagination"
)

// ── Data types for admin templates ──────────────────────────────────

type adminUser struct {
	ID          uuid.UUID
	Email       string
	Name        string
	Role        string
	CreatedAt   time.Time
	LastLoginAt *time.Time
	MemberID    uuid.UUID
}

type adminAuditRow struct {
	ID         uuid.UUID
	Action     string
	TargetType string
	TargetID   *uuid.UUID
	At         time.Time
	IP         string
	ActorName  *string
	ActorEmail *string
	Metadata   string
}

type adminSettingsData struct {
	TenantID    uuid.UUID
	TenantName  string
	MemberCount int
}

type adminUserFormData struct {
	User   *adminUser
	IsEdit bool
}

type adminAuditPageData struct {
	Entries []adminAuditRow
	Filter  string
}

// ── Users list ──────────────────────────────────────────────────────

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	rows, err := s.db.Query(ctx, `
		SELECT u.id, u.email, u.name, m.role, u.created_at, u.last_login_at, m.id
		FROM memberships m
		JOIN users u ON u.id = m.user_id
		WHERE m.tenant_id = $1
		ORDER BY u.name
	`, tenant.ID)
	if err != nil {
		slog.Error("admin: failed to list users", "error", err)
		data := s.newPageData(r)
		data.Title = "Admin · Users"
		data.Error = "Failed to load users"
		s.render(w, r, "admin_users.html", data)
		return
	}
	defer rows.Close()

	var users []adminUser
	for rows.Next() {
		var u adminUser
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.CreatedAt, &u.LastLoginAt, &u.MemberID); err != nil {
			slog.Error("admin: scan user", "error", err)
			continue
		}
		users = append(users, u)
	}

	pg := pagination.FromRequest(r, pagination.DefaultPerPage)
	paged := pagination.ApplyToSlice(&pg, users)
	pv := pg.View(r)

	data := s.newPageData(r)
	data.Title = "Admin · Users"
	data.Pagination = &pv
	data.Content = paged
	s.render(w, r, "admin_users.html", data)
}

// ── New user form ───────────────────────────────────────────────────

func (s *Server) handleAdminUserNew(w http.ResponseWriter, r *http.Request) {
	data := s.newPageData(r)
	data.Title = "Admin · New User"
	data.Content = adminUserFormData{IsEdit: false}
	s.render(w, r, "admin_user_form.html", data)
}

// ── Create user ─────────────────────────────────────────────────────

func (s *Server) handleAdminUserCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)
	actor := auth.UserFromContext(ctx)

	if err := r.ParseForm(); err != nil {
		data := s.newPageData(r)
		data.Title = "Admin · New User"
		data.Error = "Invalid form data"
		data.Content = adminUserFormData{IsEdit: false}
		s.render(w, r, "admin_user_form.html", data)
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	name := strings.TrimSpace(r.FormValue("name"))
	password := r.FormValue("password")
	role := r.FormValue("role")

	// Validate
	if email == "" {
		s.renderUserFormError(w, r, "Email is required", nil)
		return
	}
	if password == "" || len(password) < 8 {
		s.renderUserFormError(w, r, "Password must be at least 8 characters", nil)
		return
	}
	if role != "admin" && role != "editor" && role != "reader" {
		s.renderUserFormError(w, r, "Invalid role", nil)
		return
	}
	if name == "" {
		name = email
	}

	// Check duplicate email
	var exists bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists)
	if err != nil {
		slog.Error("admin: check email", "error", err)
		s.renderUserFormError(w, r, "Database error", nil)
		return
	}
	if exists {
		s.renderUserFormError(w, r, "A user with that email already exists", nil)
		return
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		slog.Error("admin: hash password", "error", err)
		s.renderUserFormError(w, r, "Internal error", nil)
		return
	}

	// Insert user
	var userID uuid.UUID
	err = s.db.QueryRow(ctx, `
		INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3) RETURNING id
	`, email, name, hash).Scan(&userID)
	if err != nil {
		slog.Error("admin: insert user", "error", err)
		s.renderUserFormError(w, r, "Failed to create user", nil)
		return
	}

	// Insert membership
	var memberID uuid.UUID
	err = s.db.QueryRow(ctx, `
		INSERT INTO memberships (tenant_id, user_id, role) VALUES ($1, $2, $3) RETURNING id
	`, tenant.ID, userID, role).Scan(&memberID)
	if err != nil {
		slog.Error("admin: insert membership", "error", err)
		s.renderUserFormError(w, r, "Failed to create membership", nil)
		return
	}

	// Audit
	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &actor.ID,
		Action:      audit.ActionMembershipAdd,
		TargetType:  audit.TargetMembership,
		TargetID:    &memberID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"email": email, "role": role, "user_id": userID.String()},
	})

	setFlashSuccess(w, "User "+email+" created successfully")
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (s *Server) renderUserFormError(w http.ResponseWriter, r *http.Request, msg string, u *adminUser) {
	data := s.newPageData(r)
	data.Title = "Admin · User"
	data.Error = msg
	data.Content = adminUserFormData{User: u, IsEdit: u != nil}
	s.render(w, r, "admin_user_form.html", data)
}

// ── Edit user form ──────────────────────────────────────────────────

func (s *Server) handleAdminUserEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	u, err := s.adminGetUser(ctx, tenant.ID, userID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	data := s.newPageData(r)
	data.Title = "Admin · Edit " + u.Name
	data.Content = adminUserFormData{User: u, IsEdit: true}
	s.render(w, r, "admin_user_form.html", data)
}

// ── Update user ─────────────────────────────────────────────────────

func (s *Server) handleAdminUserUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)
	actor := auth.UserFromContext(ctx)

	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	u, err := s.adminGetUser(ctx, tenant.ID, userID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.renderUserFormError(w, r, "Invalid form data", u)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	role := r.FormValue("role")
	newPassword := r.FormValue("password")

	if role != "admin" && role != "editor" && role != "reader" {
		s.renderUserFormError(w, r, "Invalid role", u)
		return
	}
	if name == "" {
		s.renderUserFormError(w, r, "Name is required", u)
		return
	}

	// Prevent self-demotion
	if userID == actor.ID && role != "admin" {
		s.renderUserFormError(w, r, "You cannot remove your own admin role", u)
		return
	}

	if newPassword != "" && len(newPassword) < 8 {
		s.renderUserFormError(w, r, "Password must be at least 8 characters", u)
		return
	}

	// Update user name (and password if provided)
	if newPassword != "" {
		hash, herr := auth.HashPassword(newPassword)
		if herr != nil {
			slog.Error("admin: hash password", "error", herr)
			s.renderUserFormError(w, r, "Internal error", u)
			return
		}
		_, err = s.db.Exec(ctx, `UPDATE users SET name = $1, password_hash = $2 WHERE id = $3`, name, hash, userID)
	} else {
		_, err = s.db.Exec(ctx, `UPDATE users SET name = $1 WHERE id = $2`, name, userID)
	}
	if err != nil {
		slog.Error("admin: update user", "error", err)
		s.renderUserFormError(w, r, "Failed to update user", u)
		return
	}

	// Update role
	oldRole := u.Role
	_, err = s.db.Exec(ctx, `UPDATE memberships SET role = $1 WHERE tenant_id = $2 AND user_id = $3`, role, tenant.ID, userID)
	if err != nil {
		slog.Error("admin: update membership", "error", err)
		s.renderUserFormError(w, r, "Failed to update role", u)
		return
	}

	// Audit
	meta := map[string]any{"name": name, "user_id": userID.String()}
	if oldRole != role {
		meta["old_role"] = oldRole
		meta["new_role"] = role
	}
	if newPassword != "" {
		meta["password_changed"] = true
	}
	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &actor.ID,
		Action:      audit.ActionMembershipEdit,
		TargetType:  audit.TargetMembership,
		TargetID:    &u.MemberID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    meta,
	})

	setFlashSuccess(w, "User "+name+" updated")
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

// ── Delete user (remove membership) ─────────────────────────────────

func (s *Server) handleAdminUserDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)
	actor := auth.UserFromContext(ctx)

	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Cannot delete yourself
	if userID == actor.ID {
		setFlashError(w, "You cannot remove yourself")
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}

	u, err := s.adminGetUser(ctx, tenant.ID, userID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Delete sessions for this tenant
	_, _ = s.db.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1 AND tenant_id = $2`, userID, tenant.ID)

	// Delete membership
	_, err = s.db.Exec(ctx, `DELETE FROM memberships WHERE tenant_id = $1 AND user_id = $2`, tenant.ID, userID)
	if err != nil {
		slog.Error("admin: delete membership", "error", err)
		setFlashError(w, "Failed to remove user")
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}

	// Audit
	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &actor.ID,
		Action:      audit.ActionMembershipDel,
		TargetType:  audit.TargetMembership,
		TargetID:    &u.MemberID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"email": u.Email, "role": u.Role, "user_id": userID.String()},
	})

	setFlashSuccess(w, "User "+u.Email+" removed from tenant")
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

// ── Audit log viewer ────────────────────────────────────────────────

func (s *Server) handleAdminAudit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	filter := r.URL.Query().Get("action")

	query := `
		SELECT a.id, a.action, a.target_type, a.target_id, a.at, COALESCE(a.ip,''),
		       u.name, u.email, COALESCE(a.metadata_json::text,'{}')
		FROM audit_log a
		LEFT JOIN users u ON u.id = a.actor_user_id
		WHERE a.tenant_id = $1
	`
	args := []any{tenant.ID}

	if filter != "" {
		query += ` AND a.action = $2`
		args = append(args, filter)
	}
	// Count total for pagination
	countQuery := `SELECT COUNT(*) FROM audit_log a WHERE a.tenant_id = $1`
	countArgs := []any{tenant.ID}
	if filter != "" {
		countQuery += ` AND a.action = $2`
		countArgs = append(countArgs, filter)
	}
	var total int
	_ = s.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total)

	pg := pagination.FromRequest(r, pagination.AuditPerPage)
	pg.Apply(total)
	pv := pg.View(r)

	query += fmt.Sprintf(` ORDER BY a.at DESC LIMIT %d OFFSET %d`, pg.PerPage, pg.Offset())

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		slog.Error("admin: query audit log", "error", err)
		data := s.newPageData(r)
		data.Title = "Admin · Audit Log"
		data.Error = "Failed to load audit log"
		data.Content = adminAuditPageData{Filter: filter}
		s.render(w, r, "admin_audit.html", data)
		return
	}
	defer rows.Close()

	var entries []adminAuditRow
	for rows.Next() {
		var e adminAuditRow
		if err := rows.Scan(&e.ID, &e.Action, &e.TargetType, &e.TargetID, &e.At, &e.IP, &e.ActorName, &e.ActorEmail, &e.Metadata); err != nil {
			slog.Error("admin: scan audit row", "error", err)
			continue
		}
		entries = append(entries, e)
	}

	data := s.newPageData(r)
	data.Title = "Admin · Audit Log"
	data.Pagination = &pv
	data.Content = adminAuditPageData{Entries: entries, Filter: filter}
	s.render(w, r, "admin_audit.html", data)
}

// ── Tenant settings ─────────────────────────────────────────────────

func (s *Server) handleAdminSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)

	var count int
	_ = s.db.QueryRow(ctx, `SELECT COUNT(*) FROM memberships WHERE tenant_id = $1`, tenant.ID).Scan(&count)

	data := s.newPageData(r)
	data.Title = "Admin · Settings"
	data.Content = adminSettingsData{
		TenantID:    tenant.ID,
		TenantName:  tenant.Name,
		MemberCount: count,
	}
	s.render(w, r, "admin_settings.html", data)
}

func (s *Server) handleAdminSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant := auth.TenantFromContext(ctx)
	actor := auth.UserFromContext(ctx)

	if err := r.ParseForm(); err != nil {
		setFlashError(w, "Invalid form data")
		http.Redirect(w, r, "/admin/settings", http.StatusSeeOther)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		setFlashError(w, "Tenant name is required")
		http.Redirect(w, r, "/admin/settings", http.StatusSeeOther)
		return
	}

	_, err := s.db.Exec(ctx, `UPDATE tenants SET name = $1 WHERE id = $2`, name, tenant.ID)
	if err != nil {
		slog.Error("admin: update tenant", "error", err)
		setFlashError(w, "Failed to update settings")
		http.Redirect(w, r, "/admin/settings", http.StatusSeeOther)
		return
	}

	_ = s.audit.Log(ctx, audit.Entry{
		TenantID:    tenant.ID,
		ActorUserID: &actor.ID,
		Action:      "tenant.update",
		TargetType:  audit.TargetTenant,
		TargetID:    &tenant.ID,
		IP:          r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Metadata:    map[string]any{"new_name": name},
	})

	setFlashSuccess(w, "Settings updated")
	http.Redirect(w, r, "/admin/settings", http.StatusSeeOther)
}

// ── Helper ──────────────────────────────────────────────────────────

func (s *Server) adminGetUser(ctx context.Context, tenantID, userID uuid.UUID) (*adminUser, error) {
	var u adminUser
	err := s.db.QueryRow(ctx, `
		SELECT u.id, u.email, u.name, m.role, u.created_at, u.last_login_at, m.id
		FROM memberships m
		JOIN users u ON u.id = m.user_id
		WHERE m.tenant_id = $1 AND u.id = $2
	`, tenantID, userID).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.CreatedAt, &u.LastLoginAt, &u.MemberID)
	return &u, err
}
