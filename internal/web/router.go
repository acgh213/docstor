package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/justinas/nosurf"

	"github.com/exedev/docstor/internal/attachments"
	"github.com/exedev/docstor/internal/audit"
	"github.com/exedev/docstor/internal/auth"
	"github.com/exedev/docstor/internal/checklists"
	"github.com/exedev/docstor/internal/clients"
	"github.com/exedev/docstor/internal/cmdb"
	"github.com/exedev/docstor/internal/incidents"
	"github.com/exedev/docstor/internal/config"
	"github.com/exedev/docstor/internal/docs"
	"github.com/exedev/docstor/internal/runbooks"
	"github.com/exedev/docstor/internal/doclinks"
	"github.com/exedev/docstor/internal/sites"
	tmplpkg "github.com/exedev/docstor/internal/templates"
)

//go:embed templates
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

type Server struct {
	db          *pgxpool.Pool
	cfg         *config.Config
	templates   *template.Template
	sessions    *auth.SessionManager
	authMw      *auth.Middleware
	audit       *audit.Logger
	clients     *clients.Repository
	docs        *docs.Repository
	runbooks    *runbooks.Repository
	attachments     *attachments.Repo
	storage         attachments.Storage
	templates_repo  *tmplpkg.Repository
	checklists      *checklists.Repository
	cmdb            *cmdb.Repository
	incidents       *incidents.Repository
	sites           *sites.Repository
	doclinks        *doclinks.Repository
	loginLimiter    *auth.RateLimiter
}

func NewRouter(db *pgxpool.Pool, cfg *config.Config) http.Handler {
	sessions := auth.NewSessionManager(db)
	authMw := auth.NewMiddleware(db, sessions)
	auditLog := audit.NewLogger(db)
	clientsRepo := clients.NewRepository(db)
	docsRepo := docs.NewRepository(db)
	runbooksRepo := runbooks.NewRepository(db)

	// Initialize attachments storage
	storagePath := cfg.AttachmentStoragePath
	if storagePath == "" {
		storagePath = "/tmp/docstor-attachments"
	}
	localStorage, err := attachments.NewLocalStorage(storagePath)
	if err != nil {
		slog.Error("failed to initialize attachment storage", "error", err)
		panic(err)
	}
	attachmentsRepo := attachments.NewRepo(db)
	templatesRepo := tmplpkg.NewRepository(db)
	checklistsRepo := checklists.NewRepository(db)
	cmdbRepo := cmdb.NewRepository(db)
	incidentsRepo := incidents.NewRepository(db)
	sitesRepo := sites.NewRepository(db)
	doclinksRepo := doclinks.NewRepository(db)

	s := &Server{
		db:           db,
		cfg:          cfg,
		sessions:     sessions,
		authMw:       authMw,
		audit:        auditLog,
		clients:      clientsRepo,
		docs:         docsRepo,
		runbooks:     runbooksRepo,
		attachments:     attachmentsRepo,
		storage:         localStorage,
		templates_repo:  templatesRepo,
		checklists:      checklistsRepo,
		cmdb:            cmdbRepo,
		incidents:       incidentsRepo,
		sites:           sitesRepo,
		doclinks:        doclinksRepo,
		loginLimiter:    auth.NewRateLimiter(5, time.Minute),
	}

	if err := s.loadTemplates(); err != nil {
		slog.Error("failed to load templates", "error", err)
		panic(err)
	}

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// CSRF protection on all non-static routes
	r.Use(csrfProtect(cfg.IsDevelopment()))

	staticContent, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticContent))))

	r.Get("/health", s.handleHealth)

	// Public routes
	r.Get("/login", s.handleLoginPage)
	r.Post("/login", s.handleLogin)
	r.Post("/logout", s.handleLogout)

	// Home: public landing or authenticated dashboard
	r.Group(func(r chi.Router) {
		r.Use(s.authMw.LoadSession)
		r.Get("/", s.handleHome)
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(s.authMw.RequireAuth)

		// Search
		r.Get("/search", s.handleSearch)

		// Runbooks dashboard
		r.Get("/runbooks", s.handleRunbooksDashboard)

		// Documents
		r.Get("/docs", s.handleDocsHomeV2)
		r.Get("/docs/health", s.handleDocHealth)
		r.Get("/docs/new", s.handleDocNew)
		r.Post("/docs/new", s.handleDocCreate)
		r.Post("/preview", s.handlePreview)
		// Document operations by ID
		r.Get("/docs/id/{id}/edit", s.handleDocEditByID)
		r.Post("/docs/id/{id}/save", s.handleDocSaveByID)
		r.Get("/docs/id/{id}/rename", s.handleDocRenameForm)
		r.Post("/docs/id/{id}/rename", s.handleDocRename)
		r.Post("/docs/id/{id}/delete", s.handleDocDelete)
		r.Get("/docs/id/{id}/history", s.handleDocHistoryByID)
		r.Get("/docs/id/{id}/diff", s.handleDocDiffByID)
		r.Get("/docs/id/{id}/revision/{revID}", s.handleDocRevisionByID)
		r.Post("/docs/id/{id}/revert/{revID}", s.handleDocRevertByID)
		r.Post("/docs/id/{id}/verify", s.handleRunbookVerify)
		r.Post("/docs/id/{id}/interval", s.handleRunbookUpdateInterval)
		r.Post("/docs/id/{id}/metadata", s.handleDocMetadataUpdate)

		// Folder tree (HTMX partial)
		r.Get("/tree", s.handleFolderTree)

		// Document read by path (must be last)
		r.Get("/docs/*", s.handleDocRead)

		// Document attachments
		r.Get("/docs/id/{id}/attachments", s.handleDocAttachments)
		r.Post("/docs/id/{id}/attachments/{attID}/unlink", s.handleUnlinkAttachment)

		// Attachments
		r.Post("/attachments/upload", s.handleAttachmentUpload)
		r.Get("/attachments/{id}", s.handleAttachmentDownload)
		r.Get("/attachments/{id}/preview", s.handleAttachmentPreview)
		r.Post("/attachments/{id}/delete", s.handleAttachmentDelete)
		r.Get("/api/attachments", s.handleAttachmentsAPI)

		// Evidence Bundles
		r.Route("/evidence-bundles", func(r chi.Router) {
			r.Get("/", s.handleBundlesList)
			r.Get("/new", s.handleBundleNew)
			r.Post("/", s.handleBundleCreate)
			r.Get("/{id}", s.handleBundleView)
			r.Post("/{id}/items", s.handleBundleAddItem)
			r.Post("/{id}/items/{attID}/remove", s.handleBundleRemoveItem)
			r.Get("/{id}/export", s.handleBundleExport)
			r.Post("/{id}/delete", s.handleBundleDelete)
		})

		// Templates
		r.Route("/templates", func(r chi.Router) {
			r.Get("/", s.handleTemplatesList)
			r.Get("/new", s.handleTemplateNew)
			r.Post("/", s.handleTemplateCreate)
			r.Get("/{id}", s.handleTemplateView)
			r.Get("/{id}/edit", s.handleTemplateEdit)
			r.Post("/{id}", s.handleTemplateUpdate)
			r.Post("/{id}/delete", s.handleTemplateDelete)
		})
		r.Get("/docs/new/from-template", s.handleDocNewFromTemplate)

		// Checklists
		r.Route("/checklists", func(r chi.Router) {
			r.Get("/", s.handleChecklistsList)
			r.Get("/new", s.handleChecklistNew)
			r.Post("/", s.handleChecklistCreate)
			r.Get("/{id}", s.handleChecklistView)
			r.Get("/{id}/edit", s.handleChecklistEdit)
			r.Post("/{id}", s.handleChecklistUpdate)
			r.Post("/{id}/delete", s.handleChecklistDelete)
		})

		// Checklist Instances
		r.Route("/checklist-instances", func(r chi.Router) {
			r.Get("/", s.handleInstancesList)
			r.Post("/", s.handleInstanceStart)
			r.Get("/{id}", s.handleInstanceView)
			r.Post("/{id}/items/{itemID}/toggle", s.handleInstanceToggleItem)
			r.Post("/{id}/delete", s.handleInstanceDelete)
		})

		// Clients
		r.Route("/clients", func(r chi.Router) {
			r.Get("/", s.handleClientsList)
			r.Get("/new", s.handleClientNew)
			r.Post("/", s.handleClientCreate)
			r.Get("/{id}", s.handleClientView)
			r.Get("/{id}/edit", s.handleClientEdit)
			r.Post("/{id}", s.handleClientUpdate)
		})

		// CMDB - Systems
		r.Route("/systems", func(r chi.Router) {
			r.Get("/", s.handleSystemsList)
			r.Get("/new", s.handleSystemNew)
			r.Post("/", s.handleSystemCreate)
			r.Get("/{id}", s.handleSystemView)
			r.Get("/{id}/edit", s.handleSystemEdit)
			r.Post("/{id}", s.handleSystemUpdate)
			r.Post("/{id}/delete", s.handleSystemDelete)
		})

		// CMDB - Vendors
		r.Route("/vendors", func(r chi.Router) {
			r.Get("/", s.handleVendorsList)
			r.Get("/new", s.handleVendorNew)
			r.Post("/", s.handleVendorCreate)
			r.Get("/{id}", s.handleVendorView)
			r.Get("/{id}/edit", s.handleVendorEdit)
			r.Post("/{id}", s.handleVendorUpdate)
			r.Post("/{id}/delete", s.handleVendorDelete)
		})

		// CMDB - Contacts
		r.Route("/contacts", func(r chi.Router) {
			r.Get("/", s.handleContactsList)
			r.Get("/new", s.handleContactNew)
			r.Post("/", s.handleContactCreate)
			r.Get("/{id}", s.handleContactView)
			r.Get("/{id}/edit", s.handleContactEdit)
			r.Post("/{id}", s.handleContactUpdate)
			r.Post("/{id}/delete", s.handleContactDelete)
		})

		// CMDB - Circuits
		r.Route("/circuits", func(r chi.Router) {
			r.Get("/", s.handleCircuitsList)
			r.Get("/new", s.handleCircuitNew)
			r.Post("/", s.handleCircuitCreate)
			r.Get("/{id}", s.handleCircuitView)
			r.Get("/{id}/edit", s.handleCircuitEdit)
			r.Post("/{id}", s.handleCircuitUpdate)
			r.Post("/{id}/delete", s.handleCircuitDelete)
		})

		// Sites
		r.Route("/sites", func(r chi.Router) {
			r.Get("/", s.handleSitesList)
			r.Get("/new", s.handleSiteNew)
			r.Post("/", s.handleSiteCreate)
			r.Get("/{id}", s.handleSiteView)
			r.Get("/{id}/edit", s.handleSiteEdit)
			r.Post("/{id}", s.handleSiteUpdate)
			r.Post("/{id}/delete", s.handleSiteDelete)
		})

		// Known Issues
		r.Route("/known-issues", func(r chi.Router) {
			r.Get("/", s.handleKnownIssuesList)
			r.Get("/new", s.handleKnownIssueNew)
			r.Post("/", s.handleKnownIssueCreate)
			r.Get("/{id}", s.handleKnownIssueView)
			r.Get("/{id}/edit", s.handleKnownIssueEdit)
			r.Post("/{id}", s.handleKnownIssueUpdate)
			r.Post("/{id}/delete", s.handleKnownIssueDelete)
		})

		// Incidents
		r.Route("/incidents", func(r chi.Router) {
			r.Get("/", s.handleIncidentsList)
			r.Get("/new", s.handleIncidentNew)
			r.Post("/", s.handleIncidentCreate)
			r.Get("/{id}", s.handleIncidentView)
			r.Get("/{id}/edit", s.handleIncidentEdit)
			r.Post("/{id}", s.handleIncidentUpdate)
			r.Post("/{id}/events", s.handleIncidentAddEvent)
			r.Post("/{id}/delete", s.handleIncidentDelete)
		})

		// Admin
		r.Route("/admin", func(r chi.Router) {
			r.Use(s.authMw.RequireRole("admin"))

			r.Get("/users", s.handleAdminUsers)
			r.Get("/users/new", s.handleAdminUserNew)
			r.Post("/users", s.handleAdminUserCreate)
			r.Get("/users/{id}/edit", s.handleAdminUserEdit)
			r.Post("/users/{id}", s.handleAdminUserUpdate)
			r.Post("/users/{id}/delete", s.handleAdminUserDelete)

			r.Get("/audit", s.handleAdminAudit)

			r.Get("/settings", s.handleAdminSettings)
			r.Post("/settings", s.handleAdminSettingsUpdate)
		})
	})

	return r
}

func (s *Server) loadTemplates() error {
	funcMap := template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"formatSize": func(bytes int64) string {
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
		},
		"deref": func(v any) any {
			switch val := v.(type) {
			case *string:
				if val != nil {
					return *val
				}
				return ""
			case *time.Time:
				if val != nil {
					return *val
				}
				return time.Time{}
			case *uuid.UUID:
				if val != nil {
					return val.String()
				}
				return ""
			default:
				return v
			}
		},
		"tof": func(v int) float64 {
			return float64(v)
		},
		"mulf": func(a, b float64) float64 {
			return a * b
		},
		"divf": func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"timeTag": func(t time.Time, format string) template.HTML {
			iso := t.Format(time.RFC3339)
			display := t.Format(format)
			return template.HTML(fmt.Sprintf(`<time datetime="%s">%s</time>`, iso, display))
		},
		"isPreviewable": func(filename, contentType string) bool {
			ext := strings.ToLower(filepath.Ext(filename))
			switch ext {
			case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
				return true
			case ".pdf":
				return true
			}
			return contentType == "application/pdf" || strings.HasPrefix(contentType, "image/")
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templatesFS,
		"templates/layout/*.html",
		"templates/docs/*.html",
		"templates/auth/*.html",
		"templates/clients/*.html",
		"templates/search/*.html",
		"templates/runbooks/*.html",
		"templates/attachments/*.html",
		"templates/admin/*.html",
		"templates/templates/*.html",
		"templates/checklists/*.html",
		"templates/cmdb/*.html",
		"templates/sites/*.html",
		"templates/incidents/*.html",
		"templates/landing.html",
	)
	if err != nil {
		return err
	}
	s.templates = tmpl
	return nil
}

// csrfProtect wraps nosurf for CSRF protection.
// It exempts specific paths that need it (e.g., HTMX preview).
func csrfProtect(isDev bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		csrf := nosurf.New(next)
		csrf.SetBaseCookie(http.Cookie{
			Name:     "csrf_token",
			Path:     "/",
			HttpOnly: true,
			Secure:   !isDev,
			SameSite: http.SameSiteLaxMode,
		})
		// Detect TLS from the actual request (X-Forwarded-Proto or r.TLS)
		csrf.SetIsTLSFunc(func(r *http.Request) bool {
			if r.TLS != nil {
				return true
			}
			return r.Header.Get("X-Forwarded-Proto") == "https"
		})
		// Custom failure handler
		csrf.SetFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slog.Warn("CSRF validation failed",
				"method", r.Method,
				"path", r.URL.Path,
				"reason", nosurf.Reason(r),
				"ip", r.RemoteAddr,
			)
			http.Error(w, "Forbidden - invalid CSRF token", http.StatusForbidden)
		}))
		return csrf
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.db.Ping(ctx); err != nil {
		slog.Error("health check failed", "error", err)
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
