package auth

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

const SessionCookieName = "docstor_session"

type Middleware struct {
	db       *pgxpool.Pool
	sessions *SessionManager
}

func NewMiddleware(db *pgxpool.Pool, sessions *SessionManager) *Middleware {
	return &Middleware{
		db:       db,
		sessions: sessions,
	}
}

func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		session, err := m.sessions.Validate(r.Context(), cookie.Value)
		if err != nil {
			slog.Debug("invalid session", "error", err)
			http.SetCookie(w, &http.Cookie{
				Name:     SessionCookieName,
				Value:    "",
				Path:     "/",
				MaxAge:   -1,
				HttpOnly: true,
			})
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		ctx := r.Context()

		// Load user
		var user User
		err = m.db.QueryRow(ctx, `
			SELECT id, email, name FROM users WHERE id = $1
		`, session.UserID).Scan(&user.ID, &user.Email, &user.Name)
		if err != nil {
			slog.Error("failed to load user", "error", err, "user_id", session.UserID)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		ctx = ContextWithUser(ctx, &user)

		// Load tenant
		var tenant Tenant
		err = m.db.QueryRow(ctx, `
			SELECT id, name FROM tenants WHERE id = $1
		`, session.TenantID).Scan(&tenant.ID, &tenant.Name)
		if err != nil {
			slog.Error("failed to load tenant", "error", err, "tenant_id", session.TenantID)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		ctx = ContextWithTenant(ctx, &tenant)

		// Load membership
		var membership Membership
		err = m.db.QueryRow(ctx, `
			SELECT id, tenant_id, user_id, role
			FROM memberships
			WHERE tenant_id = $1 AND user_id = $2
		`, session.TenantID, session.UserID).Scan(&membership.ID, &membership.TenantID, &membership.UserID, &membership.Role)
		if err != nil {
			slog.Error("failed to load membership", "error", err, "tenant_id", session.TenantID, "user_id", session.UserID)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		ctx = ContextWithMembership(ctx, &membership)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *Middleware) RequireRole(minRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			membership := MembershipFromContext(r.Context())
			if membership == nil {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			allowed := false
			switch minRole {
			case "admin":
				allowed = membership.IsAdmin()
			case "editor":
				allowed = membership.IsEditor()
			case "reader":
				allowed = membership.IsReader()
			}

			if !allowed {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Helper that loads session context if available but doesn't require auth
func (m *Middleware) LoadSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		session, err := m.sessions.Validate(r.Context(), cookie.Value)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		ctx := r.Context()

		var user User
		err = m.db.QueryRow(ctx, `
			SELECT id, email, name FROM users WHERE id = $1
		`, session.UserID).Scan(&user.ID, &user.Email, &user.Name)
		if err == nil {
			ctx = ContextWithUser(ctx, &user)
		}

		var tenant Tenant
		err = m.db.QueryRow(ctx, `
			SELECT id, name FROM tenants WHERE id = $1
		`, session.TenantID).Scan(&tenant.ID, &tenant.Name)
		if err == nil {
			ctx = ContextWithTenant(ctx, &tenant)
		}

		var membership Membership
		err = m.db.QueryRow(ctx, `
			SELECT id, tenant_id, user_id, role
			FROM memberships
			WHERE tenant_id = $1 AND user_id = $2
		`, session.TenantID, session.UserID).Scan(&membership.ID, &membership.TenantID, &membership.UserID, &membership.Role)
		if err == nil {
			ctx = ContextWithMembership(ctx, &membership)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
