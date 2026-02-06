# Docstor Implementation Handoff

## Project Status: Phase 4 Complete

Docstor is a web-first MSP documentation system. The core foundation is complete through Phase 4 (Trust Layer with revisions, revert, and diff).

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

### Phase 4: Trust Layer (Revisions) ✅
- **Revert functionality** (`internal/docs/docs.go`):
  - `Revert()` method creates new revision from old revision's body
  - Never deletes history - always creates a new revision
  - Auto-generated commit message indicating source revision
- **Diff view** (`internal/docs/diff.go`):
  - Line-by-line diff using go-diff/diffmatchpatch
  - Returns additions/deletions counts
  - HTML-escaped output for safe rendering
- **Handlers**:
  - `handleDocRevertByID` - Revert to a specific revision
  - `handleDocDiffByID` - Compare two revisions
  - `handleDocRevisionByID` - View a specific revision
- **Templates**:
  - `doc_diff.html` - Side-by-side diff view with line numbers
  - `doc_revision.html` - View historical revision with revert option
  - Enhanced `doc_history.html` with compare dropdown and View/Revert buttons
- **Audit logging**:
  - Revert actions logged with `doc.revert` action
  - Metadata includes: path, reverted_to revision ID, new revision ID

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
│   │   ├── diff.go           # Diff computation
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

## Routes Summary

### Auth
- `GET /login` - Login page
- `POST /login` - Authenticate
- `POST /logout` - Log out

### Documents
- `GET /docs` - Document list
- `GET /docs/new` - Create form
- `POST /docs/new` - Create document
- `GET /docs/*` - Read document by path
- `GET /docs/id/{id}/edit` - Edit form
- `POST /docs/id/{id}/save` - Save changes
- `GET /docs/id/{id}/history` - Revision history
- `GET /docs/id/{id}/diff?from=...&to=...` - Compare revisions
- `GET /docs/id/{id}/revision/{revID}` - View specific revision
- `POST /docs/id/{id}/revert/{revID}` - Revert to revision

### Clients
- `GET /clients` - Client list
- `GET /clients/new` - Create form
- `POST /clients` - Create client
- `GET /clients/{id}` - View client
- `GET /clients/{id}/edit` - Edit form
- `POST /clients/{id}` - Update client

### Utilities
- `GET /health` - Health check
- `POST /preview` - Markdown preview (HTMX)

---

## What's Left to Complete

### Phase 5: Editor Island
- [ ] Add CodeMirror 6 on edit pages (with textarea fallback)
- [ ] Improve HTMX preview endpoint

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
6. **Diff**: Uses go-diff/diffmatchpatch for line-based comparison
7. **Revert**: Always creates new revision, never deletes history

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
curl -b cookies.txt -X POST -d "path=test/hello&title=Hello World&body=# Hello" http://localhost:8080/docs/new

# View revision history
curl -b cookies.txt http://localhost:8080/docs/id/{DOC_ID}/history

# Compare revisions
curl -b cookies.txt "http://localhost:8080/docs/id/{DOC_ID}/diff?from={REV1_ID}&to={REV2_ID}"

# Revert to a revision
curl -b cookies.txt -X POST http://localhost:8080/docs/id/{DOC_ID}/revert/{REV_ID}
```

---

## Reference Documents

- `plan.md` - Full phased implementation plan with PR-sized tasks
- `claude.md` - Build contract with non-negotiables and rules
- `thoughts.md` - Implementation progress tracking

---

*Handoff updated: 2026-02-06* - Phase 4 completed, revert and diff functionality working
