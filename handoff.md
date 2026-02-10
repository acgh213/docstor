# Docstor Handoff Document

## Project Overview

Docstor is a web-first MSP documentation system built with Go. It's server-rendered (no React/SPA), uses HTMX for partial updates, and CodeMirror 6 for the markdown editor.

**URL**: https://switch-dune.exe.xyz:8080/  
**Login**: `admin@example.com` / `admin123`

## Tech Stack

- **Backend**: Go 1.21+, chi router, html/template
- **Database**: PostgreSQL with embedded migrations
- **Frontend**: Server-rendered HTML, HTMX, CodeMirror 6 (bundled)
- **CSS**: Custom minimal CSS (no Tailwind)
- **Markdown**: goldmark + bluemonday sanitizer

## Current Status: Phase 8 Complete

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

## Project Structure

```
switch-dune/
├── cmd/docstor/main.go          # Entry point
├── internal/
│   ├── attachments/             # File attachments & evidence bundles
│   ├── audit/                   # Audit logging
│   ├── auth/                    # Sessions, middleware, password hashing
│   ├── clients/                 # Clients repository
│   ├── config/                  # Config loading
│   ├── db/                      # Database connection, migrations
│   │   └── migrations/          # SQL migration files
│   ├── docs/                    # Documents, revisions, markdown, diff
│   ├── runbooks/                # Runbook verification status
│   └── web/                     # HTTP handlers, templates, static
│       ├── handlers*.go         # Route handlers
│       ├── router.go            # Chi router setup
│       ├── templates/           # Go html/template files
│       └── static/              # CSS, JS
├── editor-bundle/               # CodeMirror 6 esbuild project
│   ├── package.json
│   └── src/editor.js
├── docker-compose.yml
├── Makefile
├── claude.md                    # Build contract (must follow)
├── plan.md                      # Full implementation plan
└── thoughts.md                  # Progress notes
```

## Database Schema (Key Tables)

```sql
-- Core tables
tenants(id, name, created_at)
users(id, email, name, password_hash, created_at, last_login_at)
memberships(id, tenant_id, user_id, role, created_at)  -- role: admin/editor/reader
clients(id, tenant_id, name, code, notes, created_at)

-- Documents
documents(id, tenant_id, client_id, path, title, doc_type, sensitivity,
          owner_user_id, metadata_json, current_revision_id, search_vector,
          created_by, created_at, updated_at)
revisions(id, tenant_id, document_id, body_markdown, created_by, created_at,
          message, base_revision_id)

-- Runbooks
runbook_status(document_id PK, tenant_id, last_verified_at, last_verified_by_user_id,
               verification_interval_days, next_due_at)

-- Audit
audit_log(id, tenant_id, actor_user_id, action, target_type, target_id,
          at, ip, user_agent, metadata_json)

-- Sessions
sessions(id, user_id, tenant_id, token_hash, created_at, expires_at, ip, user_agent)

-- Attachments (Phase 8)
attachments(id, tenant_id, filename, content_type, size_bytes, sha256, storage_key, created_by, created_at)
attachment_links(id, tenant_id, attachment_id, linked_type, linked_id, created_at)
  -- linked_type: 'document' | 'revision' | 'incident' | 'change'
evidence_bundles(id, tenant_id, name, description, created_by, created_at)
evidence_bundle_items(id, tenant_id, bundle_id, attachment_id, note, created_at)
```

## Key Routes

```
# Auth
GET/POST /login, POST /logout

# Documents
GET  /docs                           # List documents
GET  /docs/new                       # New document form
POST /docs/new                       # Create document
GET  /docs/*path                     # Read document by path
GET  /docs/id/{id}/edit              # Edit form
POST /docs/id/{id}/save              # Save (creates revision)
GET  /docs/id/{id}/history           # Revision history
GET  /docs/id/{id}/diff?from=&to=    # Diff view
GET  /docs/id/{id}/revision/{revID}  # View specific revision
POST /docs/id/{id}/revert/{revID}    # Revert to revision
POST /docs/id/{id}/verify            # Verify runbook
POST /docs/id/{id}/interval          # Update verification interval

# Search
GET /search?q=...&client_id=...&doc_type=...

# Runbooks
GET /runbooks                        # Dashboard (overdue, unowned, recent)

# Clients
GET/POST /clients, GET/POST /clients/{id}

# Attachments (Phase 8)
POST /attachments/upload             # Upload file
GET  /attachments/{id}               # Download file
GET  /docs/id/{id}/attachments       # Document attachments page
POST /docs/id/{id}/attachments/{attID}/unlink  # Unlink from document

# Evidence Bundles (Phase 8)
GET  /evidence-bundles               # List bundles
GET  /evidence-bundles/new           # New bundle form
POST /evidence-bundles               # Create bundle
GET  /evidence-bundles/{id}          # View bundle
POST /evidence-bundles/{id}/items    # Add item to bundle
POST /evidence-bundles/{id}/items/{attID}/remove  # Remove item
GET  /evidence-bundles/{id}/export   # Export as ZIP
POST /evidence-bundles/{id}/delete   # Delete bundle

# HTMX
POST /preview                        # Markdown preview
```

## Running Locally

```bash
cd /home/exedev/switch-dune
docker-compose up -d                 # Start PostgreSQL
make dev                             # Or: go run ./cmd/docstor
# Server runs on :8080
```

## Building CodeMirror Bundle

```bash
cd editor-bundle
npm install
npm run build                        # Outputs to internal/web/static/js/codemirror-bundle.js
```

---

# Remaining Phases (Post-MVP)

## Phase 8: Attachments + Evidence Bundles

**Goal**: Upload files, link to docs/revisions, create evidence bundles for export.

**Schema**:
```sql
attachments(id, tenant_id, filename, content_type, size_bytes, sha256,
            storage_key, created_by, created_at)
attachment_links(id, tenant_id, attachment_id, linked_type, linked_id, created_at)
  -- linked_type: 'revision' | 'document' | 'incident' | 'change'
evidence_bundles(id, tenant_id, name, description, created_by, created_at)
evidence_bundle_items(id, tenant_id, bundle_id, attachment_id, note, created_at)
```

**Routes**:
- `POST /attachments/upload` - Upload file
- `GET /attachments/{id}` - Download file
- `GET /docs/id/{id}/attachments` - List document attachments
- `GET /evidence-bundles` - List bundles
- `POST /evidence-bundles` - Create bundle
- `GET /evidence-bundles/{id}/export` - Export as ZIP

**Implementation Notes**:
- Storage: Start with local disk (`/data/attachments/{tenant_id}/{sha256}`)
- Interface for future S3 backend
- Permission checks on download
- Audit logging for uploads/downloads

---

## Phase 9: Templates + Checklists

**Goal**: Create document/runbook templates, checklist library with trackable instances.

**Schema**:
```sql
templates(id, tenant_id, name, template_type, body_markdown, default_metadata_json,
          created_by, created_at)
  -- template_type: 'doc' | 'runbook' | 'incident_rca' | 'change'
checklists(id, tenant_id, name, description, created_by, created_at)
checklist_items(id, tenant_id, checklist_id, position, text)
checklist_instances(id, tenant_id, checklist_id, linked_type, linked_id,
                    status, created_by, created_at, completed_at)
checklist_instance_items(id, tenant_id, instance_id, item_id,
                         done_by_user_id, done_at, note)
```

**Routes**:
- `GET /templates` - Template library
- `POST /docs/new?template={id}` - Create from template
- `GET /checklists` - Checklist library
- `POST /checklist-instances` - Start checklist
- `POST /checklist-instances/{id}/items/{itemId}/toggle` - Check/uncheck item

---

## Phase 10: CMDB-lite + Live Blocks

**Goal**: Lightweight directory of systems/vendors/contacts, with "live block" shortcodes in docs.

**Schema**:
```sql
systems(id, tenant_id, client_id, site_id, system_type, name, fqdn, ip,
        environment, notes, owner_user_id, created_at, updated_at)
  -- system_type: server/firewall/switch/circuit/app/service
vendors(id, tenant_id, client_id, name, type, phone, email, portal_url,
        escalation_notes, created_at)
contacts(id, tenant_id, client_id, site_id, name, role, phone, email, notes)
circuits(id, tenant_id, client_id, site_id, provider, circuit_id, wan_ip,
         speed, notes, created_at, updated_at)
```

**Live Blocks**:
In markdown, support shortcodes like:
- `{{system:123}}` - Renders system name, IP, environment
- `{{vendor:9}}` - Renders vendor contact + portal link
- `{{circuit:55}}` - Renders circuit details

Shortcodes resolved server-side during markdown render. Missing refs show warning.

---

## Phase 11: Known Issues + Incidents

**Goal**: Track known issues board, incident timelines, link to docs.

**Schema**:
```sql
known_issues(id, tenant_id, title, severity, status, client_id,
             description, created_by, created_at, updated_at, linked_document_id)
  -- status: open/investigating/resolved/wont_fix
incidents(id, tenant_id, title, severity, status, client_id,
          started_at, ended_at, summary, created_by, created_at)
incident_events(id, tenant_id, incident_id, at, event_type, detail, actor_user_id)
  -- event_type: detected/acknowledged/investigating/mitigated/resolved/note
```

**Routes**:
- `GET /known-issues` - Known issues board
- `GET /incidents` - Incident list
- `GET /incidents/{id}` - Incident timeline
- `POST /incidents/{id}/events` - Add timeline event

---

## Phase 12: Doc Health Dashboards

**Goal**: Identify stale docs, broken links, popular-but-stale content.

**Features**:
- Stale docs: `updated_at < NOW() - INTERVAL '90 days'`
- Docs without owners: `owner_user_id IS NULL`
- Broken links: Parse internal links from markdown, check if targets exist
- Popular-but-stale: Track view counts (optional), compare to update frequency

**Implementation**:
- Add `doc_links(from_document_id, to_document_id)` table, rebuild on save
- Dashboard queries aggregating document health metrics
- Scheduled job or on-demand scan for link checking

---

## Implementation Guidelines

1. **Always read `claude.md`** - It's the build contract
2. **Tenant scoping** - Every query must filter by tenant_id
3. **Role checks** - Use `membership.IsEditor()` / `membership.IsAdmin()`
4. **Audit logging** - Every write action should call `s.audit.Log()`
5. **Migrations** - Add new SQL files in `internal/db/migrations/`
6. **Templates** - Add to `internal/web/templates/` and update `loadTemplates()`
7. **No SPA behavior** - Full page loads are normal; HTMX for partials only

## Useful Commands

```bash
# Run app
go run ./cmd/docstor

# Build
go build -o docstor ./cmd/docstor

# Test
go test ./...

# Rebuild CM6 bundle
cd editor-bundle && npm run build

# View logs
tail -f /tmp/docstor.log

# Database shell
docker exec -it switch-dune_postgres_1 psql -U docstor -d docstor
```
