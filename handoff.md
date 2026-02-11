# Docstor Handoff Document

## Project Overview

Docstor is a web-first MSP documentation system built with Go. It's server-rendered (no React/SPA), uses HTMX for partial updates, and CodeMirror 6 for the markdown editor.

**URL**: https://switch-dune.exe.xyz:8000/  
**Login**: `admin@example.com` / `admin123`

## Tech Stack

- **Backend**: Go 1.21+, chi router, html/template
- **Database**: PostgreSQL with embedded migrations
- **Frontend**: Server-rendered HTML, HTMX, CodeMirror 6 (bundled)
- **CSS**: Custom minimal CSS (no Tailwind)
- **Markdown**: goldmark + bluemonday sanitizer
- **CSRF**: justinas/nosurf v1.2

## Current Status

### Completed Phases

| Phase | Description | Status |
|-------|-------------|--------|
| 0 | Repo skeleton, Docker Compose, migrations | ✅ |
| 1 | Auth, tenancy, role gating (admin/editor/reader) | ✅ |
| 2 | Clients CRUD | ✅ |
| 3 | Documents (CRUD, revisions, markdown rendering) | ✅ |
| 4 | History, diff view, revert | ✅ |
| 5 | Editor (enhanced textarea, draft saving) | ✅ |
| 6 | Full-text search (PostgreSQL tsvector) | ✅ |
| 7 | Living Runbooks (verification workflow, dashboard) | ✅ |
| 7.5 | CodeMirror 6 editor with vim mode | ✅ |
| 8 | Attachments + Evidence Bundles | ✅ |
| Sec | CSRF, login rate limiting, sensitivity gating | ✅ |
| UX A-D | Mobile, flash messages, breadcrumbs, editor polish | ✅ |
| Tests | Tenant isolation, role gating, XSS, conflict, audit | ✅ |
| Admin | User management, audit viewer, tenant settings | ✅ |
| Landing | Public landing page + improved dashboard | ✅ |
| Doc Ops | Rename/move/delete documents | ✅ |
| 12 | Doc Health Dashboard (stale, unowned) | ✅ |

### Remaining Phases

| Phase | Description | Est. |
|-------|-------------|------|
| 9 | Templates + Checklists | 3-4h |
| 10 | CMDB-lite + Live Blocks | 4-6h |
| 11 | Known Issues + Incidents | 3-4h |
| Polish | Drag-drop upload, HTMX folder tree, image preview | 2-3h |

## Project Structure

```
switch-dune/
├── cmd/docstor/main.go          # Entry point
├── cmd/seed/main.go             # Database seeder
├── internal/
│   ├── attachments/             # File attachments & evidence bundles
│   ├── audit/                   # Audit logging (audit_test.go)
│   ├── auth/                    # Sessions, middleware, password hashing, rate limiter
│   ├── clients/                 # Clients repository
│   ├── config/                  # Config loading
│   ├── db/                      # Database connection, migrations
│   │   └── migrations/          # SQL migration files (001-003)
│   ├── docs/                    # Documents, revisions, markdown, diff, sensitivity
│   │   │                        # (docs_test.go, markdown_test.go)
│   ├── runbooks/                # Runbook verification status
│   ├── testutil/                # Test helpers (DB setup, fixtures)
│   └── web/                     # HTTP handlers, templates, static
│       ├── handlers.go          # Auth, clients, dashboard, home
│       ├── handlers_admin.go    # Admin section (users, audit, settings)
│       ├── handlers_docs.go     # Doc CRUD, rename, delete
│       ├── handlers_health.go   # Doc health dashboard
│       ├── handlers_runbooks.go # Runbook verification
│       ├── handlers_search.go   # Search
│       ├── handlers_attachments.go # Attachments & bundles
│       ├── handlers_test.go     # Integration tests (role/sensitivity gating)
│       ├── router.go            # Chi router, template loading, CSRF
│       ├── templates/           # Go html/template files
│       │   ├── layout/          # base.html (sidebar, topbar, scripts)
│       │   ├── auth/            # login.html
│       │   ├── docs/            # doc_read, edit, list, form, history, diff, rename, health
│       │   ├── admin/           # admin_users, user_form, audit, settings
│       │   ├── clients/         # clients_list, client_form, client_view
│       │   ├── runbooks/        # runbooks_dashboard
│       │   ├── search/          # search
│       │   ├── attachments/     # doc_attachments, bundles
│       │   └── landing.html     # Public landing page
│       └── static/              # CSS, JS, favicon
├── editor-bundle/               # CodeMirror 6 esbuild project
├── docker-compose.yml
├── claude.md                    # Build contract (must follow)
├── plan.md                      # Full implementation plan
└── thoughts.md                  # Progress notes
```

## Key Routes

```
# Public
GET  /                               # Landing page (unauth) or dashboard (auth)
GET  /login                          # Login form
POST /login                          # Login submit
POST /logout                         # Logout
GET  /health                         # Health check

# Documents
GET  /docs                           # List documents
GET  /docs/new                       # New document form
POST /docs/new                       # Create document
GET  /docs/health                    # Doc health dashboard
GET  /docs/*path                     # Read document by path
GET  /docs/id/{id}/edit              # Edit form
POST /docs/id/{id}/save              # Save (creates revision)
GET  /docs/id/{id}/history           # Revision history
GET  /docs/id/{id}/diff?from=&to=    # Diff view
GET  /docs/id/{id}/revision/{revID}  # View specific revision
POST /docs/id/{id}/revert/{revID}    # Revert to revision
GET  /docs/id/{id}/rename            # Rename form
POST /docs/id/{id}/rename            # Rename submit
POST /docs/id/{id}/delete            # Delete document
POST /docs/id/{id}/verify            # Verify runbook
POST /docs/id/{id}/interval          # Update verification interval

# Search
GET /search?q=...&client_id=...&doc_type=...

# Runbooks
GET /runbooks                        # Dashboard (overdue, unowned, recent)

# Clients
GET/POST /clients, GET/POST /clients/{id}

# Attachments & Evidence Bundles
POST /attachments/upload
GET  /attachments/{id}
GET  /docs/id/{id}/attachments
/evidence-bundles/*

# Admin (admin role only)
GET  /admin/users                    # User list
GET  /admin/users/new                # Add user form
POST /admin/users                    # Create user
GET  /admin/users/{id}/edit          # Edit user
POST /admin/users/{id}               # Update user
POST /admin/users/{id}/delete        # Remove from tenant
GET  /admin/audit                    # Audit log viewer
GET  /admin/settings                 # Tenant settings
POST /admin/settings                 # Update settings

# HTMX
POST /preview                        # Markdown preview
```

## Security Summary

| Feature | Implementation |
|---------|---------------|
| CSRF | nosurf v1.2, form tokens + HTMX header injection |
| Rate limiting | 5 attempts/60s per IP on login |
| Sensitivity | public-internal (all), restricted/confidential (admin+editor) |
| Sessions | SHA-256 hashed tokens, 7-day expiry, HttpOnly/SameSite |
| Passwords | bcrypt cost=12 |
| Markdown | bluemonday sanitizer, no raw HTML |
| Audit | Append-only log for all meaningful writes |

## Test Suite (33 tests, all pass with -race)

| File | What it tests |
|------|---------------|
| `internal/docs/docs_test.go` | Tenant isolation (docs, revisions, clients, search), conflict detection, revert, CanAccess |
| `internal/docs/markdown_test.go` | XSS regression (script, onerror, javascript:), normal rendering, GFM |
| `internal/audit/audit_test.go` | Round-trip, append-only contract |
| `internal/web/handlers_test.go` | Role gating (reader/editor/admin), sensitivity via HTTP |
| `internal/auth/ratelimit_test.go` | Rate limiter logic |

```bash
go test -race ./...    # Run all tests
go test -short ./...   # Skip integration tests
```

## Running Locally

```bash
cd /home/exedev/switch-dune
docker-compose up -d                 # Start PostgreSQL
PORT=8000 go run ./cmd/docstor       # Server on :8000
```

## Building CodeMirror Bundle

```bash
cd editor-bundle
npm install
npm run build   # → internal/web/static/js/codemirror-bundle.js
```

## Implementation Guidelines

1. **Always read `claude.md`** — It's the build contract
2. **Tenant scoping** — Every query must filter by tenant_id
3. **Role checks** — `membership.IsEditor()` / `membership.IsAdmin()`
4. **Audit logging** — Every write action → `s.audit.Log()`
5. **CSRF** — Every POST form → `{{.CSRFField}}`
6. **Migrations** — New SQL in `internal/db/migrations/`
7. **Templates** — Add to `internal/web/templates/` and update `loadTemplates()` glob
8. **No SPA** — Full page loads normal; HTMX for partials only
