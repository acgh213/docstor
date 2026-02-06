# Docstor Implementation Handoff

## Project Status: Phase 3 Complete

Docstor is a web-first MSP documentation system. The core foundation is complete and the documents module is partially implemented.

---

## What's Been Built

### Phase 0: Repository Skeleton ✅
- Go module with chi router, pgx/v5, golang-migrate
- Docker Compose for Postgres
- Makefile with `make dev`, `make seed`, `make reset` commands
- Embedded migrations and templates
- Health endpoint at `/health`
- Basic CSS with clean, minimal design

### Phase 1: Auth + Tenancy + Role Gating ✅
- **Auth package** (`internal/auth/`):
  - Password hashing with bcrypt
  - Session management with secure tokens (stored in DB)
  - Context helpers for User, Tenant, Membership
  - `RequireAuth` middleware
  - `RequireRole` middleware for admin/editor/reader gating
- **Audit logging** (`internal/audit/`):
  - Append-only audit log with metadata
  - Login success/failure tracking
- **Seed command** (`cmd/seed/`):
  - Creates default tenant and admin user
  - Login: `admin@example.com` / `admin123`

### Phase 2: Clients ✅
- **Clients CRUD** (`internal/clients/`):
  - List, Get, Create, Update operations
  - Tenant-scoped queries
  - Audit logging for create/update
- **UI**:
  - Client list, view, create, edit pages
  - All operations permission-gated

### Phase 3: Documents ✅
- **Documents package** (`internal/docs/`):
  - Full data model: documents, revisions
  - Repository with List, GetByPath, GetByID, Create, Update
  - Revision tracking with base_revision_id for conflict detection
  - Markdown rendering with goldmark + bluemonday sanitization
- **Handlers** (`internal/web/handlers_docs.go`):
  - Document list, read, create, edit, history views
  - Audit logging for document operations
  - Conflict detection on save
- **Templates**:
  - `docs_list.html` - Document listing
  - `doc_read.html` - Document view with rendered markdown
  - `doc_form.html` - Create new document
  - `doc_edit.html` - Edit document with preview support
  - `doc_history.html` - Revision history
  - `doc_conflict.html` - Conflict resolution page

---

## Current File Structure

```
/home/exedev/switch-dune/
├── cmd/
│   ├── docstor/main.go       # Main entry point
│   └── seed/main.go          # Seed command
├── internal/
│   ├── audit/audit.go        # Audit logging
│   ├── auth/
│   │   ├── context.go        # Context helpers
│   │   ├── middleware.go     # Auth middleware
│   │   ├── password.go       # Password hashing
│   │   └── session.go        # Session management
│   ├── clients/clients.go    # Clients repository
│   ├── config/config.go      # Configuration
│   ├── db/
│   │   ├── db.go             # Database connection
│   │   ├── migrations.go     # Migration runner
│   │   └── migrations/       # SQL migrations
│   ├── docs/
│   │   ├── docs.go           # Documents repository
│   │   └── markdown.go       # Markdown rendering
│   └── web/
│       ├── handlers.go       # Auth, dashboard, client handlers
│       ├── handlers_docs.go  # Document handlers
│       ├── router.go         # HTTP router
│       ├── static/css/       # Stylesheets
│       └── templates/        # HTML templates
├── docker-compose.yml
├── Makefile
├── go.mod / go.sum
├── .env / .env.example
├── plan.md                   # Full implementation plan
├── claude.md                 # Build contract/rules
└── thoughts.md               # Implementation notes
```

---

## Database Schema

The initial migration (`001_initial_schema.up.sql`) creates:

- `tenants` - Multi-tenancy support
- `users` - User accounts with password hashes
- `memberships` - Tenant-user-role relationships
- `clients` - Client organizations within a tenant
- `documents` - Documents with path, title, type, sensitivity
- `revisions` - Immutable revision history
- `audit_log` - Append-only audit trail
- `runbook_status` - Verification tracking for runbooks
- `sessions` - Server-side session storage

---

## How to Run

```bash
# Start Postgres
make db-up

# Seed initial data (creates admin user)
make seed

# Run the application
make dev

# Access at http://localhost:8080
# Login: admin@example.com / admin123
```

---

## What's Left to Complete

### Phase 3 Remaining (Documents MVP) - COMPLETED
- [x] Fixed document creation 500 error (nil pointer in GetRevision)
- [x] Fixed template rendering issue (global "content" block conflict)
- [x] Fixed history page truncation (uuid pointer comparison issue)
- [x] Created `doc_conflict.html` template for revision conflicts
- [x] Added CSS for conflict page

### Phase 4: Trust Layer (Revisions)
- [ ] Implement revert functionality (creates new revision from old)
- [ ] Add diff view between revisions
- [ ] Test conflict detection when two users edit simultaneously

### Phase 5: Editor Island
- [ ] Add CodeMirror 6 on edit pages (with textarea fallback)
- [ ] Implement HTMX preview endpoint properly

### Phase 6: Search
- [ ] Add full-text search using Postgres tsvector
- [ ] Search UI with filters (client, doc_type, owner)

### Phase 7: Living Runbooks
- [ ] Runbook verification workflow
- [ ] Overdue runbooks dashboard
- [ ] Verification interval tracking

---

## Key Technical Decisions

1. **Routing**: Document operations (edit/save/history) use `/docs/id/{id}/...` to avoid chi wildcard limitations
2. **Sessions**: Stored in database (not cookies) for audit trail and server-side invalidation
3. **Markdown**: Server-side rendering with goldmark, sanitized with bluemonday
4. **Templates**: Go html/template with embedded FS, partials in layout/
5. **Revisions**: Conflict detection via `base_revision_id` check before save

---

## Known Issues / Notes

1. Preview button uses HTMX but the JS show/hide is a quick fallback
2. Runbooks dashboard and verification not yet implemented
3. Search not yet implemented

**Priority for next session**: Implement Phase 4 (Revert functionality, diff view) or Phase 5 (CodeMirror editor).

---

## Testing the Current Build

```bash
# After running make dev, test these endpoints:

# Health check
curl http://localhost:8080/health

# Login (returns 303 redirect)
curl -c cookies.txt -X POST -d "email=admin@example.com&password=admin123" http://localhost:8080/login

# View docs list
curl -b cookies.txt http://localhost:8080/docs

# Create a document
curl -b cookies.txt -X POST -d "path=test/hello&title=Hello World&body=# Hello\n\nThis is a test." http://localhost:8080/docs/new

# View the document
curl -b cookies.txt http://localhost:8080/docs/test/hello
```

---

## Reference Documents

- `plan.md` - Full phased implementation plan with PR-sized tasks
- `claude.md` - Build contract with non-negotiables and rules
- `thoughts.md` - Implementation progress tracking

---

*Handoff updated: 2026-02-06* - Phase 3 completed, document CRUD fully functional
