package web

import (
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/exedev/docstor/internal/audit"
	"github.com/exedev/docstor/internal/auth"
	"github.com/exedev/docstor/internal/clients"
	"github.com/exedev/docstor/internal/config"
	"github.com/exedev/docstor/internal/docs"
)

//go:embed templates
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

type Server struct {
	db        *pgxpool.Pool
	cfg       *config.Config
	templates *template.Template
	sessions  *auth.SessionManager
	authMw    *auth.Middleware
	audit     *audit.Logger
	clients   *clients.Repository
	docs      *docs.Repository
}

func NewRouter(db *pgxpool.Pool, cfg *config.Config) http.Handler {
	sessions := auth.NewSessionManager(db)
	authMw := auth.NewMiddleware(db, sessions)
	auditLog := audit.NewLogger(db)
	clientsRepo := clients.NewRepository(db)
	docsRepo := docs.NewRepository(db)

	s := &Server{
		db:       db,
		cfg:      cfg,
		sessions: sessions,
		authMw:   authMw,
		audit:    auditLog,
		clients:  clientsRepo,
		docs:     docsRepo,
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

		// Documents
		r.Get("/docs", s.handleDocsHomeV2)
		r.Get("/docs/new", s.handleDocNew)
		r.Post("/docs/new", s.handleDocCreate)
		r.Post("/preview", s.handlePreview)
		// Document operations by ID
		r.Get("/docs/id/{id}/edit", s.handleDocEditByID)
		r.Post("/docs/id/{id}/save", s.handleDocSaveByID)
		r.Get("/docs/id/{id}/history", s.handleDocHistoryByID)
		// Document read by path (must be last)
		r.Get("/docs/*", s.handleDocRead)

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
	tmpl, err := template.ParseFS(templatesFS,
		"templates/layout/*.html",
		"templates/docs/*.html",
		"templates/auth/*.html",
		"templates/clients/*.html",
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
