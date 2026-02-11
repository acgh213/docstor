# Handoff: Complete Phase 10 (CMDB-lite + Live Blocks) and Phase 11 (Known Issues + Incidents)

## Context

Docstor is a Go server-rendered MSP documentation system. See `claude.md` for the build contract (no React, HTMX allowed, minimal JS). See `plan.md` for full plan.

**App URL**: https://switch-dune.exe.xyz:8000/  
**Login**: admin@example.com / admin123  
**DB**: postgres://docstor:docstor@localhost:5432/docstor?sslmode=disable  
**Process**: `PORT=8000 /tmp/docstor` (no systemd; kill old, rebuild, restart)  
**Migration state**: schema_migrations version=4. Migrations 005 and 006 have NOT been applied yet.

## What's Done (committed at d91de70)

### Migrations (not yet applied to DB)
- `internal/db/migrations/005_cmdb.up.sql` — systems, vendors, contacts, circuits tables with indexes
- `internal/db/migrations/006_incidents.up.sql` — known_issues, incidents, incident_events tables with indexes
- Both have corresponding `.down.sql` files

### Repository layers (compile clean, not wired in)
- `internal/cmdb/cmdb.go` — Full CRUD for System, Vendor, Contact, Circuit. Each has List/Get/Create/Update/Delete. List methods accept optional `*uuid.UUID` clientID filter.
- `internal/cmdb/shortcodes.go` — `RenderShortcodes(ctx, tenantID, html)` replaces `{{system:UUID}}`, `{{vendor:UUID}}`, `{{contact:UUID}}`, `{{circuit:UUID}}` in rendered HTML with live info blocks. Missing refs show warning spans.
- `internal/incidents/incidents.go` — Full CRUD for KnownIssue, Incident, IncidentEvent. Includes joined fields (ClientName, CreatedByName, ActorName). Events are timeline entries for incidents.

### Audit constants
- `internal/audit/audit.go` — Added action/target constants for all 7 new entity types (system, vendor, contact, circuit, known_issue, incident, incident_event)

## What's NOT Done (the actual work)

### 1. Handlers — `internal/web/handlers_cmdb.go`
Needs CRUD handlers for each of 4 CMDB types. Follow exact pattern from `handlers.go` client handlers (handleClientsList, handleClientNew, handleClientCreate, handleClientView, handleClientEdit, handleClientUpdate) plus handleXDelete.

For each entity type (System, Vendor, Contact, Circuit):
- List handler: support `?client_id=UUID` query param filter
- New/Edit form handlers: load client list for dropdown via `s.clients.List(ctx, tenant.ID)`
- Create/Update: parse form, validate required fields, audit log, flash message, redirect
- Delete: editor+ only, audit log, flash, redirect to list
- All writes require `membership.IsEditor()`

Form data struct pattern:
```go
type SystemFormData struct {
    System  *cmdb.System      // nil for new
    Clients []clients.Client
}
```

Template names: `systems_list.html`, `system_form.html`, `system_view.html` (same pattern for vendors, contacts, circuits)

### 2. Handlers — `internal/web/handlers_incidents.go`
Needs CRUD for known issues + incidents + event timeline.

Known Issues:
- List with status filter (`?status=open`) and client filter
- New/Edit form with client dropdown, severity select (low/medium/high/critical), status select (open/investigating/resolved/wont_fix)
- Description and workaround as textareas
- Optional linked_document_id field

Incidents:
- List with status filter (`?status=investigating`) and client filter  
- New/Edit form with severity, status (detected/investigating/mitigated/resolved), started_at datetime input
- View page shows timeline of events
- POST `/{id}/events` adds timeline event (event_type select + detail textarea) — can use HTMX to append
- ended_at set when status changes to resolved

Template names: `known_issues_list.html`, `known_issue_form.html`, `known_issue_view.html`, `incidents_list.html`, `incident_form.html`, `incident_view.html`

### 3. HTML Templates
12 CMDB templates + 6 incident templates = 18 files total.

Directories to create:
- `internal/web/templates/cmdb/` (systems_list, system_form, system_view, vendors_list, vendor_form, vendor_view, contacts_list, contact_form, contact_view, circuits_list, circuit_form, circuit_view)
- `internal/web/templates/incidents/` (known_issues_list, known_issue_form, known_issue_view, incidents_list, incident_form, incident_view)

Follow existing template patterns (see `clients_list.html`, `client_form.html`, `client_view.html` for reference). Each template must:
- Use `{{define "template_name.html"}}` wrapper
- Include `head_common`, `sidebar`, `topbar`, `alerts`, `page_scripts` blocks
- Use `{{.CSRFField}}` in all POST forms
- Use breadcrumbs
- List pages: table with filter dropdowns, links to detail pages
- Form pages: proper labels, select dropdowns for enums, client dropdown
- View pages: detail card, edit/delete buttons

Incident view page should have a timeline section showing events chronologically with event_type badges and an "Add Event" form (can be HTMX).

### 4. Router Wiring — `internal/web/router.go`

Changes needed:

**Imports** — add:
```go
"github.com/exedev/docstor/internal/cmdb"
"github.com/exedev/docstor/internal/incidents"
```

**Server struct** — add fields:
```go
cmdb      *cmdb.Repository
incidents *incidents.Repository
```

**NewRouter()** — initialize repos:
```go
cmdbRepo := cmdb.NewRepository(db)
incidentsRepo := incidents.NewRepository(db)
```
And assign to Server struct.

**Routes** — add inside the protected group:
```go
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
// Same pattern for /vendors, /contacts, /circuits

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
```

**Template loader** — add to ParseFS patterns:
```go
"templates/cmdb/*.html",
"templates/incidents/*.html",
```

### 5. Sidebar — `internal/web/templates/layout/base.html`
Add nav items (after Checklists, before Evidence Bundles):
```html
<a href="/systems" class="nav-item" data-nav="systems">🖥 Systems</a>
<a href="/vendors" class="nav-item" data-nav="vendors">🏢 Vendors</a>
<a href="/contacts" class="nav-item" data-nav="contacts">👤 Contacts</a>
<a href="/circuits" class="nav-item" data-nav="circuits">🔌 Circuits</a>
<a href="/known-issues" class="nav-item" data-nav="known-issues">⚠️ Known Issues</a>
<a href="/incidents" class="nav-item" data-nav="incidents">🚨 Incidents</a>
```

### 6. Shortcode Integration
In `internal/web/handlers_docs.go`, the doc read handler renders markdown. After `docs.RenderMarkdown(body)`, call `s.cmdb.RenderShortcodes(ctx, tenantID, rendered)` to resolve live blocks.

Same for the preview handler in `handlers_docs.go` or wherever `/preview` is handled.

Find the exact location by searching for `RenderMarkdown` calls in handlers.

### 7. CSS
Append to `internal/web/static/css/main.css`:
- `.shortcode-block` — inline-block, padding, border-radius, background, small font
- `.shortcode-system` / `.shortcode-vendor` / `.shortcode-contact` / `.shortcode-circuit` — subtle color variations
- `.shortcode-warning` — yellow/orange warning style
- `.timeline` — vertical timeline for incident events
- `.timeline-event` — event cards with type badges
- `.severity-badge` — colored badges for low/medium/high/critical
- `.status-badge` — colored badges for various statuses

### 8. Apply Migrations
```bash
psql postgres://docstor:docstor@localhost:5432/docstor?sslmode=disable -f internal/db/migrations/005_cmdb.up.sql
psql postgres://docstor:docstor@localhost:5432/docstor?sslmode=disable -f internal/db/migrations/006_incidents.up.sql
psql postgres://docstor:docstor@localhost:5432/docstor?sslmode=disable -c "UPDATE schema_migrations SET version = 6, dirty = false"
```

### 9. Build + Test
```bash
cd /home/exedev/switch-dune
go build -o /tmp/docstor ./cmd/docstor/
pkill -f '/tmp/docstor'; sleep 1
PORT=8000 nohup /tmp/docstor > /tmp/docstor.log 2>&1 &
go test ./... -count=1
```

## Key Patterns to Follow

**Handler pattern** (see `handlers.go` handleClientCreate for canonical example):
1. Check `membership.IsEditor()` for writes
2. Parse form
3. Validate required fields
4. Call repo method
5. Audit log with `s.audit.Log(ctx, audit.Entry{...})`
6. `setFlashSuccess(w, "...")` 
7. `http.Redirect(w, r, "...", http.StatusSeeOther)`

**Template pattern** (see `clients_list.html` for canonical example):
- `{{define "name.html"}}` ... full HTML doc ... `{{end}}`
- Uses `{{.Content}}` for the page-specific data
- Flash alerts via `{{template "alerts" .}}`

**PageData** (defined in `handlers.go`):
```go
type PageData struct {
    Title, Error, Success string
    User, Tenant, Membership (from auth context)
    Content any
    CSRFToken, CSRFField
}
```

## File Reference

| File | Purpose |
|------|--------|
| `internal/web/router.go` | All routes, Server struct, template loader |
| `internal/web/handlers.go` | PageData, render(), auth handlers, client handlers |
| `internal/web/handlers_docs.go` | Doc CRUD (find RenderMarkdown call here) |
| `internal/web/handlers_templates.go` | Templates CRUD (good pattern ref) |
| `internal/web/handlers_checklists.go` | Checklists CRUD (good pattern ref) |
| `internal/web/templates/layout/base.html` | Sidebar, topbar, alerts |
| `internal/web/templates/clients/` | Client templates (canonical reference) |
| `internal/web/static/css/main.css` | All styles (~1850 lines) |
| `internal/audit/audit.go` | Audit logger + constants |
| `plan.md` | Full plan with Phase 10/11 specs |
| `claude.md` | Build contract — MUST follow |
