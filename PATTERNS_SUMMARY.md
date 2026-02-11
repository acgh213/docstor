# Codebase Patterns Summary

This document captures codebase patterns for adding new features to Docstor. Originally written for CMDB + Incidents (now complete); still useful as a reference for future work (Sites, etc.).

---

## 1. Server Struct & Dependency Injection (router.go)

### Server struct fields
Each repository is a field on `*Server`:
```go
type Server struct {
    db              *pgxpool.Pool
    cfg             *config.Config
    templates       *template.Template
    sessions        *auth.SessionManager
    authMw          *auth.Middleware
    audit           *audit.Logger
    clients         *clients.Repository
    docs            *docs.Repository
    runbooks        *runbooks.Repository
    attachments     *attachments.Repo
    storage         attachments.Storage
    templates_repo  *tmplpkg.Repository
    checklists      *checklists.Repository
    loginLimiter    *auth.RateLimiter
}
```
**To add CMDB + Incidents:** Add `cmdb *cmdb.Repository` and `incidents *incidents.Repository` fields.

### NewRouter initialization
Repos are created in `NewRouter()` and assigned to server:
```go
func NewRouter(db *pgxpool.Pool, cfg *config.Config) http.Handler {
    // ... create repos ...
    checklistsRepo := checklists.NewRepository(db)
    s := &Server{
        // ... assign all fields ...
        checklists: checklistsRepo,
    }
```
**To add:** `cmdbRepo := cmdb.NewRepository(db)` and `incidentsRepo := incidents.NewRepository(db)`

### Route registration pattern
Protected routes use `r.Route("/path", func(r chi.Router) { ... })` groups:
```go
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
```
All inside the `r.Use(s.authMw.RequireAuth)` group.

### Template loading
Templates are loaded via `ParseFS` with glob patterns per directory:
```go
tmpl, err := template.New("").Funcs(funcMap).ParseFS(templatesFS,
    "templates/layout/*.html",
    "templates/docs/*.html",
    "templates/clients/*.html",
    "templates/checklists/*.html",
    // ... etc
)
```
**To add:** New glob lines for `"templates/cmdb/*.html"` and `"templates/incidents/*.html"`.

### Template FuncMap
Available template functions:
- `safeHTML` - renders raw HTML
- `formatSize` - formats bytes
- `deref` - dereferences *string, *time.Time, *uuid.UUID
- `tof` - int to float64
- `mulf`, `divf` - float multiplication/division
- `timeTag` - renders `<time datetime="...">display</time>`

---

## 2. PageData & Rendering (handlers.go)

### PageData struct
```go
type PageData struct {
    Title      string
    User       *auth.User
    Tenant     *auth.Tenant
    Membership *auth.Membership
    Content    any          // <-- main payload, type varies per page
    Error      string
    Success    string
    CSRFToken  string
    CSRFField  template.HTML
}
```

### Creating PageData
Always use `s.newPageData(r)` which auto-populates User, Tenant, Membership, CSRF:
```go
data := s.newPageData(r)
data.Title = "Checklists - Docstor"
data.Content = someDataSliceOrStruct
s.render(w, r, "template_name.html", data)
```

### render() method
- Reads flash cookies automatically (Success/Error)
- Calls `s.templates.ExecuteTemplate(w, name, data)`
- Logs errors via slog

---

## 3. CRUD Handler Pattern (handlers_checklists.go / handlers.go clients)

### List handler
```go
func (s *Server) handleXxxList(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    tenant := auth.TenantFromContext(ctx)

    items, err := s.repo.List(ctx, tenant.ID)
    if err != nil {
        slog.Error("failed to list xxx", "error", err)
        data := s.newPageData(r)
        data.Title = "Xxx - Docstor"
        data.Error = "Failed to load xxx"
        s.render(w, r, "xxx_list.html", data)
        return
    }

    data := s.newPageData(r)
    data.Title = "Xxx - Docstor"
    data.Content = items
    s.render(w, r, "xxx_list.html", data)
}
```

### New (show form) handler
```go
func (s *Server) handleXxxNew(w http.ResponseWriter, r *http.Request) {
    membership := auth.MembershipFromContext(r.Context())
    if !membership.IsEditor() {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    data := s.newPageData(r)
    data.Title = "New Xxx - Docstor"
    s.render(w, r, "xxx_form.html", data)
}
```

### Create (POST) handler
```go
func (s *Server) handleXxxCreate(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    membership := auth.MembershipFromContext(ctx)
    tenant := auth.TenantFromContext(ctx)
    user := auth.UserFromContext(ctx)

    if !membership.IsEditor() {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    if err := r.ParseForm(); err != nil {
        data := s.newPageData(r)
        data.Error = "Invalid form data"
        s.render(w, r, "xxx_form.html", data)
        return
    }

    name := strings.TrimSpace(r.FormValue("name"))
    // ... validate ...
    if name == "" {
        data := s.newPageData(r)
        data.Error = "Name is required"
        s.render(w, r, "xxx_form.html", data)
        return
    }

    item, err := s.repo.Create(ctx, repo.CreateInput{ ... })
    if err != nil {
        slog.Error("failed to create xxx", "error", err)
        data := s.newPageData(r)
        data.Error = "Failed to create xxx"
        s.render(w, r, "xxx_form.html", data)
        return
    }

    // Audit log
    _ = s.audit.Log(ctx, audit.Entry{
        TenantID:    tenant.ID,
        ActorUserID: &user.ID,
        Action:      audit.ActionXxxCreate,
        TargetType:  audit.TargetXxx,
        TargetID:    &item.ID,
        IP:          r.RemoteAddr,
        UserAgent:   r.UserAgent(),
        Metadata:    map[string]any{"name": name},
    })

    setFlashSuccess(w, "Xxx created successfully")
    http.Redirect(w, r, "/xxx/"+item.ID.String(), http.StatusSeeOther)
}
```

### View handler
```go
func (s *Server) handleXxxView(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    tenant := auth.TenantFromContext(ctx)

    id, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        http.NotFound(w, r)
        return
    }

    item, err := s.repo.Get(ctx, tenant.ID, id)
    if errors.Is(err, repo.ErrNotFound) {
        http.NotFound(w, r)
        return
    }
    if err != nil {
        slog.Error("failed to get xxx", "error", err)
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
        return
    }

    data := s.newPageData(r)
    data.Title = item.Name + " - Docstor"
    data.Content = item
    s.render(w, r, "xxx_view.html", data)
}
```

### Edit (show form with existing data) handler
```go
func (s *Server) handleXxxEdit(w http.ResponseWriter, r *http.Request) {
    // Same as View but:
    // - Check membership.IsEditor()
    // - Set data.Content = existing item
    // - Render xxx_form.html (same form template, checks .Content for edit vs new)
}
```

### Update (POST) handler
```go
func (s *Server) handleXxxUpdate(w http.ResponseWriter, r *http.Request) {
    // Parse ID from chi.URLParam(r, "id")
    // Check IsEditor()
    // ParseForm, validate
    // Call repo.Update(ctx, tenant.ID, id, UpdateInput{...})
    // Handle ErrNotFound
    // Audit log
    // setFlashSuccess + redirect to view page
}
```

### Delete (POST) handler
```go
func (s *Server) handleXxxDelete(w http.ResponseWriter, r *http.Request) {
    // Check IsEditor()
    // Parse ID
    // Optionally fetch item for audit metadata
    // Call repo.Delete(ctx, tenant.ID, id)
    // Audit log
    // setFlashSuccess + redirect to list page
}
```

---

## 4. Flash Messages (flash.go)

```go
// After mutation, before redirect:
setFlashSuccess(w, "Client created successfully")
setFlashError(w, "Failed to start checklist")

// render() reads them automatically from cookies
```

---

## 5. Audit Logging (audit/audit.go)

### Existing constants to add to:
```go
// Already defined for CMDB:
ActionSystemCreate  = "system.create"
ActionSystemUpdate  = "system.update"
ActionSystemDelete  = "system.delete"
ActionVendorCreate  = "vendor.create"
// ... etc for vendor, contact, circuit

// Already defined for Incidents:
ActionKnownIssueCreate = "known_issue.create"
ActionKnownIssueUpdate = "known_issue.update"
ActionKnownIssueDelete = "known_issue.delete"
ActionIncidentCreate   = "incident.create"
ActionIncidentUpdate   = "incident.update"
ActionIncidentDelete   = "incident.delete"
ActionIncidentEvent    = "incident.event"

// Target types already defined:
TargetSystem        = "system"
TargetVendor        = "vendor"
TargetContact       = "contact"
TargetCircuit       = "circuit"
TargetKnownIssue    = "known_issue"
TargetIncident      = "incident"
TargetIncidentEvent = "incident_event"
```
**All audit constants are already in place.** No changes needed to audit.go.

### Usage pattern:
```go
_ = s.audit.Log(ctx, audit.Entry{
    TenantID:    tenant.ID,
    ActorUserID: &user.ID,
    Action:      audit.ActionSystemCreate,
    TargetType:  audit.TargetSystem,
    TargetID:    &item.ID,
    IP:          r.RemoteAddr,
    UserAgent:   r.UserAgent(),
    Metadata:    map[string]any{"name": name},
})
```

---

## 6. Repository Pattern (cmdb/cmdb.go, incidents/incidents.go)

### CMDB Repository - Already complete:
- `ListSystems(ctx, tenantID, *clientID)` / `GetSystem` / `CreateSystem` / `UpdateSystem` / `DeleteSystem`
- `ListVendors(ctx, tenantID, *clientID)` / `GetVendor` / `CreateVendor` / `UpdateVendor` / `DeleteVendor`
- `ListContacts(ctx, tenantID, *clientID)` / `GetContact` / `CreateContact` / `UpdateContact` / `DeleteContact`
- `ListCircuits(ctx, tenantID, *clientID)` / `GetCircuit` / `CreateCircuit` / `UpdateCircuit` / `DeleteCircuit`

### Incidents Repository - Already complete:
- `ListKnownIssues(ctx, tenantID, status, *clientID)` / `GetKnownIssue` / `CreateKnownIssue` / `UpdateKnownIssue` / `DeleteKnownIssue`
- `ListIncidents(ctx, tenantID, status, *clientID)` / `GetIncident` / `CreateIncident` / `UpdateIncident` / `DeleteIncident`
- `ListEvents(ctx, tenantID, incidentID)` / `CreateEvent`

### Null-string pattern:
Both repos use `ns()` helper to coalesce `*string` → `string`, and convert empty strings to `nil` for INSERT/UPDATE.

### List methods support optional clientID filter:
```go
func (r *Repository) ListSystems(ctx context.Context, tenantID uuid.UUID, clientID *uuid.UUID) ([]System, error) {
    query := `SELECT ... FROM systems WHERE tenant_id = $1`
    args := []any{tenantID}
    if clientID != nil {
        query += " AND client_id = $2"
        args = append(args, *clientID)
    }
```

### Incidents have joined fields (ClientName, CreatedByName) via LEFT JOIN.

---

## 7. Shortcode Rendering (cmdb/shortcodes.go)

Pattern: `{{system:uuid}}`, `{{vendor:uuid}}`, `{{contact:uuid}}`, `{{circuit:uuid}}`

Called as: `r.RenderShortcodes(ctx, tenantID, renderedHTML)` — applied AFTER markdown rendering.

**Currently NOT wired into any handler.** The doc read handler at `handlers_docs.go:147` calls `docs.RenderMarkdown()` but doesn't call `RenderShortcodes` after. This would need to be integrated.

Shortcode links assume routes like `/systems/{id}`, `/vendors/{id}`, `/contacts/{id}`, `/circuits/{id}`.

---

## 8. Template Pattern (clients/ templates as reference)

### Common structure for ALL page templates:
```html
{{define "xxx_list.html"}}
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    {{template "head_common" .}}
    <title>{{.Title}}</title>
    <link rel="stylesheet" href="/static/css/main.css">
    <script src="https://unpkg.com/htmx.org@1.9.10" defer></script>
</head>
<body>
    <div class="app-container">
        {{template "sidebar" .}}
        <main class="main-content">
            {{template "topbar" .}}
            <div class="content">
                {{template "alerts" .}}

                <nav class="breadcrumbs">...</nav>
                <div class="page-header">
                    <div class="page-header-row">
                        <h2>Title</h2>
                        {{if .Membership.IsEditor}}
                        <a href="/xxx/new" class="btn btn-primary">New Xxx</a>
                        {{end}}
                    </div>
                </div>
                <div class="card">
                    <!-- table or empty state -->
                </div>
            </div>
        </main>
    </div>
{{template "page_scripts" .}}
</body>
</html>
{{end}}
```

### List template: table with empty state
```html
{{if .Content}}
<table>
    <thead><tr><th>Name</th><th>...</th></tr></thead>
    <tbody>
    {{range .Content}}
    <tr>
        <td><a href="/xxx/{{.ID}}">{{.Name}}</a></td>
        <td>{{timeTag .CreatedAt "Jan 2, 2006"}}</td>
        <td><a href="/xxx/{{.ID}}" class="btn btn-secondary btn-sm">View</a></td>
    </tr>
    {{end}}
    </tbody>
</table>
{{else}}
<div class="empty-state">
    <div class="empty-state-icon">🏢</div>
    <h3>No items yet</h3>
    <p>Description text.</p>
</div>
{{end}}
```

### Form template: shared for new/edit
Uses `{{if .Content}}` to distinguish edit from new:
```html
<form method="POST" action="{{if .Content}}/xxx/{{.Content.ID}}{{else}}/xxx{{end}}" class="form">
    {{.CSRFField}}
    <div class="form-group">
        <label for="name">Name</label>
        <input type="text" id="name" name="name" required class="form-control"
               value="{{if .Content}}{{.Content.Name}}{{end}}">
    </div>
    <div class="form-actions">
        <button type="submit" class="btn btn-primary">
            {{if .Content}}Save Changes{{else}}Create Xxx{{end}}
        </button>
        <a href="/xxx" class="btn btn-secondary">Cancel</a>
    </div>
</form>
```

### View template: detail list in card
```html
<div class="card">
    <dl class="detail-list">
        <dt>Field</dt>
        <dd>{{.Content.Field}}</dd>
        <dt>Notes</dt>
        <dd>{{if .Content.Notes}}{{.Content.Notes}}{{else}}<span class="text-muted">No notes</span>{{end}}</dd>
    </dl>
</div>
```

### Breadcrumbs pattern:
```html
<nav class="breadcrumbs">
    <a href="/xxx">Xxx</a>
    <span class="separator">/</span>
    {{if .Content}}{{.Content.Name}}{{else}}Xxx{{end}}
</nav>
```

---

## 9. Sidebar Navigation (layout/base.html)

Current nav items:
```html
<nav class="sidebar-nav">
    <a href="/" class="nav-item" data-nav="dashboard">Dashboard</a>
    <a href="/docs" class="nav-item" data-nav="docs">Docs</a>
    <a href="/runbooks" class="nav-item" data-nav="runbooks">Runbooks</a>
    <a href="/clients" class="nav-item" data-nav="clients">Clients</a>
    <a href="/templates" class="nav-item" data-nav="templates">Templates</a>
    <a href="/checklists" class="nav-item" data-nav="checklists">Checklists</a>
    <a href="/evidence-bundles" class="nav-item" data-nav="evidence-bundles">Evidence Bundles</a>
    <a href="/search" class="nav-item" data-nav="search">Search</a>
    <a href="/docs/health" class="nav-item" data-nav="doc-health">📊 Doc Health</a>
    {{if .Membership.IsAdmin}}
    <a href="/admin/users" class="nav-item" data-nav="admin">⚙️ Admin</a>
    {{end}}
</nav>
```

**To add CMDB & Incidents nav items** in appropriate position (after Clients makes sense for CMDB items).

The `data-nav` attribute is used for active-state highlighting via JS that matches `window.location.pathname.startsWith(href)`.

---

## 10. Key Integration Points for CMDB Shortcodes in Doc Rendering

In `handlers_docs.go` line ~147 (handleDocRead):
```go
rendered, err := docs.RenderMarkdown(doc.CurrentRevision.BodyMarkdown)
// After this, need to add:
// rendered = s.cmdb.RenderShortcodes(ctx, tenant.ID, rendered)
renderedBody = template.HTML(rendered)
```
Same pattern needed in `handleDocRevisionByID` (line ~626) and `handlePreview` (line ~482).

---

## 11. Content passed as map for complex views

When a view needs multiple data items, use a `map[string]any`:
```go
data.Content = map[string]any{
    "Checklist": cl,
    "Instances": relatedInstances,
}
```
Accessed in template as `{{$checklist := index .Content "Checklist"}}`.

---

## 12. Files Needed Per New Feature

For each CMDB entity (systems, vendors, contacts, circuits) and incident entity (known-issues, incidents):

1. **Handler file**: `internal/web/handlers_cmdb.go` (or split: `handlers_systems.go`, etc.)
2. **Template files**: `internal/web/templates/cmdb/systems_list.html`, `system_form.html`, `system_view.html` (etc.)
3. **Router registration**: Add routes in `router.go`
4. **Server struct**: Add repo fields in `router.go`
5. **Template loading**: Add glob in `loadTemplates()`
6. **Sidebar**: Add nav links in `layout/base.html`

---

## 13. Select/Dropdown Pattern for Foreign Keys (e.g., ClientID)

From `handlers_docs.go` — load clients for dropdown:
```go
clientsList, _ := s.clients.List(ctx, tenant.ID)
var clientOptions []ClientOption
for _, c := range clientsList {
    clientOptions = append(clientOptions, ClientOption{
        ID: c.ID, Name: c.Name, Code: c.Code,
        Selected: existingItem.ClientID != nil && *existingItem.ClientID == c.ID,
    })
}
```

In template:
```html
<select name="client_id" class="form-control">
    <option value="">-- None --</option>
    {{range .Clients}}
    <option value="{{.ID}}" {{if .Selected}}selected{{end}}>{{.Name}} ({{.Code}})</option>
    {{end}}
</select>
```

---

## 14. Incidents-specific: Severity/Status patterns

Incident types already define:
- **Severity**: used as string (e.g., "critical", "high", "medium", "low")
- **Status**: used as string (e.g., "open", "investigating", "resolved", "closed" for incidents; "open", "monitoring", "resolved" for known issues)
- **EventType**: string for incident timeline events

---

## 15. Error sentinel pattern

Both repos use:
```go
var ErrNotFound = errors.New("not found")
```
Handlers check: `errors.Is(err, cmdb.ErrNotFound)` → `http.NotFound(w, r)`
