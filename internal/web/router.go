package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/exedev/docstor/internal/attachments"
	"github.com/exedev/docstor/internal/audit"
	"github.com/exedev/docstor/internal/auth"
	"github.com/exedev/docstor/internal/clients"
	"github.com/exedev/docstor/internal/config"
	"github.com/exedev/docstor/internal/docs"
	"github.com/exedev/docstor/internal/runbooks"
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
	attachments *attachments.Repo
	storage     attachments.Storage
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

	s := &Server{
		db:          db,
		cfg:         cfg,
		sessions:    sessions,
		authMw:      authMw,
		audit:       auditLog,
		clients:     clientsRepo,
		docs:        docsRepo,
		runbooks:    runbooksRepo,
		attachments: attachmentsRepo,
		storage:     localStorage,
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

	staticContent, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticContent))))

	r.Get("/health", s.handleHealth)

	// Public routes
	r.Get("/login", s.handleLoginPage)
	r.Post("/login", s.handleLogin)
	r.Post("/logout", s.handleLogout)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(s.authMw.RequireAuth)

		r.Get("/", s.handleDashboard)

		// Search
		r.Get("/search", s.handleSearch)

		// Runbooks dashboard
		r.Get("/runbooks", s.handleRunbooksDashboard)

		// Documents
		r.Get("/docs", s.handleDocsHomeV2)
		r.Get("/docs/new", s.handleDocNew)
		r.Post("/docs/new", s.handleDocCreate)
		r.Post("/preview", s.handlePreview)
		// Document operations by ID
		r.Get("/docs/id/{id}/edit", s.handleDocEditByID)
		r.Post("/docs/id/{id}/save", s.handleDocSaveByID)
		r.Get("/docs/id/{id}/history", s.handleDocHistoryByID)
		r.Get("/docs/id/{id}/diff", s.handleDocDiffByID)
		r.Get("/docs/id/{id}/revision/{revID}", s.handleDocRevisionByID)
		r.Post("/docs/id/{id}/revert/{revID}", s.handleDocRevertByID)
		r.Post("/docs/id/{id}/verify", s.handleRunbookVerify)
		r.Post("/docs/id/{id}/interval", s.handleRunbookUpdateInterval)
		// Document read by path (must be last)
		r.Get("/docs/*", s.handleDocRead)

		// Document attachments
		r.Get("/docs/id/{id}/attachments", s.handleDocAttachments)
		r.Post("/docs/id/{id}/attachments/{attID}/unlink", s.handleUnlinkAttachment)

		// Attachments
		r.Post("/attachments/upload", s.handleAttachmentUpload)
		r.Get("/attachments/{id}", s.handleAttachmentDownload)

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

		// Clients
		r.Route("/clients", func(r chi.Router) {
			r.Get("/", s.handleClientsList)
			r.Get("/new", s.handleClientNew)
			r.Post("/", s.handleClientCreate)
			r.Get("/{id}", s.handleClientView)
			r.Get("/{id}/edit", s.handleClientEdit)
			r.Post("/{id}", s.handleClientUpdate)
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
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templatesFS,
		"templates/layout/*.html",
		"templates/docs/*.html",
		"templates/auth/*.html",
		"templates/clients/*.html",
		"templates/search/*.html",
		"templates/runbooks/*.html",
		"templates/attachments/*.html",
	)
	if err != nil {
		return err
	}
	s.templates = tmpl
	return nil
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
