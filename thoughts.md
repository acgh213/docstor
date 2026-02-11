# Docstor Implementation Progress

## Current Status: All Feature Phases Complete (0–12)

Last updated: February 11, 2026

---

## Completed Phases

### Phase 0 ✅ — Project Skeleton
- Go module, Makefile, Docker Compose
- Migrations framework with embedded SQL
- Base layout, CSS, health endpoint

### Phase 1 ✅ — Auth & Tenancy
- Password hashing (bcrypt), sessions (token + hash in Postgres)
- Tenant-scoped middleware (every request filtered by tenant_id)
- Role gating: admin/editor/reader
- Audit logging foundation

### Phase 2 ✅ — Clients
- Clients CRUD with full UI
- Audit logging for client operations
- Permission gating on create/edit

### Phase 3 ✅ — Documents & Revisions
- Documents repository: List, GetByPath, GetByID, Create, Update
- Revisions table with conflict detection via base_revision_id
- Markdown rendering with goldmark + bluemonday
- Templates: list, read, form, edit, history, conflict
- Route structure: `/docs/*` for read, `/docs/id/{id}/...` for operations

### Phase 4 ✅ — History, Diff, Revert
- Revert creates new revision from old revision's content (never deletes)
- Line-by-line diff between any two revisions (go-diff/diffmatchpatch)
- View any historical revision with rendered markdown
- History UI: compare revisions dropdown, View/Revert buttons

### Phase 5 ✅ — Editor Island
- Enhanced textarea with monospace font (JS-disabled fallback)
- localStorage draft saving with recovery
- Tab key indentation support
- HTMX markdown preview

### Phase 6 ✅ — Search
- tsvector column with GIN index
- Trigger auto-updates search vector on document changes
- Weighted search (title A, path B, body C)
- Filters: client_id, doc_type, owner_id
- ts_headline for highlighted snippets

### Phase 7 ✅ — Living Runbooks
- runbook_status table: verification cadence, last_verified_at, next_due_at
- Verify action stamps timestamp, computes next due date
- Runbooks dashboard: Overdue / Unowned / Recently Verified
- Runbook Status card on doc view page

### Phase 7.5 ✅ — CodeMirror 6 Editor
- esbuild bundle for CM6 (editor-bundle/)
- Markdown syntax highlighting with custom theme
- Vim mode toggle (persisted in localStorage)
- Line numbers, code folding, bracket matching
- Search (Ctrl+F), history (undo/redo)
- Falls back to textarea if bundle fails

### Phase 8 ✅ — Attachments + Evidence Bundles
- `attachments` table with SHA256 deduplication
- `attachment_links` for polymorphic linking (doc, revision, incident)
- `evidence_bundles` + `evidence_bundle_items` tables
- Upload, download, link-to-doc, unlink handlers
- Bundle CRUD + ZIP export
- Attachment picker (AJAX select dropdown)
- Local file storage backend (S3 interface planned)
- Audit logging for all attachment operations

### Phase 9 ✅ — Templates + Checklists
- `templates` table (doc/runbook types)
- Template library with create/edit/delete + create-doc-from-template flow
- `checklists`, `checklist_items`, `checklist_instances`, `checklist_instance_items` tables
- Checklist instances linked to documents, HTMX toggle for items
- Migration 004

### Phase 10 ✅ — CMDB-lite + Live Blocks
- `systems`, `vendors`, `contacts`, `circuits` tables with full CRUD
- Client filtering on all list views
- Shortcode rendering: `{{system:uuid}}`, `{{vendor:uuid}}`, `{{contact:uuid}}`, `{{circuit:uuid}}`
- Shortcodes resolved server-side after markdown render; missing refs show warning spans
- 12 templates (list/form/view for each entity type)
- Migration 005

### Phase 11 ✅ — Known Issues + Incidents
- `known_issues` table with severity (low/medium/high/critical) and status (open/investigating/resolved/wont_fix)
- `incidents` table with timeline via `incident_events`
- Event types: detected/acknowledged/investigating/mitigated/resolved/note
- Client and status filtering on list views
- 6 templates (list/form/view for known issues + incidents)
- Migration 006

### Polish ✅ — UX Refinements
- Drag-and-drop file upload zone on attachments page (auto-submit on drop)
- HTMX folder tree navigation on docs list (`GET /tree?folder=...`)
- Image/PDF inline preview on attachments page (`GET /attachments/{id}/preview`)
- Quick-edit doc metadata from view page (type, sensitivity, owner, client via inline HTMX form)
- `docs.UpdateMetadata()` and `docs.ListFolders()` repo methods
- `isPreviewable` template function

### Phase 13 ✅ — Sites
- `sites` table with client_id FK, indexes
- site_id added to systems, contacts, circuits tables
- Full CRUD handlers + 3 templates (list/form/view)
- Client view updated: shows sites table, docs table, CMDB quick links
- Sidebar nav: Sites between Checklists and Systems
- Audit logging for site create/update/delete
- Migration 007

### Security Hardening ✅
- **CSRF protection**: nosurf middleware with form tokens + HTMX header injection
- **Login rate limiting**: 5 attempts/60s per IP, in-memory with auto-cleanup
- **Sensitivity gating**: public-internal for all; restricted/confidential for admin+editor only
- Proper TLS detection for exe.dev proxy (X-Forwarded-Proto)

### UX/UI Overhaul ✅
- **Phase A (Critical)**: Attachments button on doc view, mobile responsive layout with hamburger sidebar, form submission fix, bundle attachment picker
- **Phase B (High)**: Sidebar active state (URL-based), cookie-based flash messages (auto-dismiss + dismiss button), unified search UX
- **Phase C (Polish)**: Breadcrumbs on all pages, table zebra striping + hover, empty state icons + CTAs, loading states on submit, confirm dialogs, favicon, relative time display
- **Phase D (Editor)**: Custom markdown-aware HighlightStyle (headings blue, bold orange, italic purple, code red, links blue), improved rendered code blocks (dark theme, copy button), blockquote/table/hr styling, Ctrl+K search + Ctrl+S save shortcuts, CodeMirror on new doc form

---

## Remaining Work

### Optional: Hardening Phase
- [ ] S3-compatible storage backend
- [ ] PDF packet export
- [ ] Break-glass access for confidential docs
- [ ] Tamper-evident audit (hash chain)

All feature phases (0–13) and polish items are complete.

---

## Architecture Notes

### Key Decisions
1. **No React/SPA** — Server-rendered HTML + HTMX for partials (per claude.md §1)
2. **CodeMirror 6** — Only JS "island" allowed; bundled with esbuild
3. **Postgres FTS** — tsvector + GIN index; no external search service
4. **nosurf CSRF** — Cookie-based double-submit; compatible with HTMX via X-CSRF-Token header
5. **Sensitivity gating** — Role-only in MVP (no per-doc allowlists)
6. **Immutable revisions** — Revert creates new revision; never delete history

### File Structure
```
cmd/docstor/main.go              # Entry point
internal/
  attachments/                    # File storage + repo
  audit/                          # Append-only audit log
  auth/                           # Sessions, passwords, middleware, rate limiter
  clients/                        # Clients CRUD
  config/                         # Env-based config
  db/migrations/                  # SQL migrations (001-003)
  docs/                           # Documents, revisions, markdown, diff, sensitivity
  runbooks/                       # Verification workflow
  search/                         # FTS repository
  web/                            # Handlers, templates, router, flash
editor-bundle/                    # CM6 esbuild source
web/templates/                    # Go html/template files
web/static/                       # CSS, JS, favicon
```

See `handoff.md` for complete reference.
