# claude.md — Docstor build contract (no React, minimal JS)

You are implementing **Docstor**, a web-first MSP documentation system.
Your job is to produce clean, server-rendered software that techs can trust at 2:17 AM.

This file is a contract. If a request conflicts with it, you must call it out and propose a compliant alternative.

---

## 0) Product shape (what we are building)

Docstor = documentation + runbooks + trust layer.

Core features (MVP scope):
- Tenancy + auth + **simple role gating**
- **Clients exist now** (first-class)
- Sites are **explicitly deferred** (later)
- Docs (folder/path navigation), Markdown rendering, search
- Editing with immutable revisions
- Append-only audit log for meaningful actions
- Runbooks v1: owner + verification cadence + verify action + overdue dashboard

Post-MVP modules (planned, not MVP unless stated):
- Attachments/evidence bundles
- Templates + checklist instances
- CMDB-lite directory
- Known issues/incidents/changes
- Doc health dashboards
- Sites (client -> sites) later

---

## 1) Non-negotiables

Frontend rules:
- **No React/Vue/SPA.**
- Server-rendered HTML for all pages.
- HTMX is allowed for partial updates (previews, inline edits, modals, tree expand).
- Minimal JavaScript. The only serious JS “island” allowed is the Markdown editor (CodeMirror) on edit pages.
- Pages must be usable with JS disabled (editor can fall back to `<textarea>`).

Backend rules:
- Every request is tenant-scoped (always filter by tenant_id).
- Every meaningful write is auditable (append-only audit log).
- Revisions are immutable; “revert” creates a new revision.

Security rules:
- Never store secrets/passwords in Docstor. Only store references/pointers to external secret stores.
- Use server-side Markdown rendering + sanitization. Never trust client-rendered Markdown.
- CSRF protection must work with HTMX.

---

## 2) Tech stack (locked)

- Language: Go
- HTTP: net/http + chi (or standard mux if you prefer; keep it simple)
- Templates: Go `html/template` (partials encouraged)
- DB: Postgres + SQL migrations (golang-migrate or equivalent)
- CSS: minimal custom CSS (or a small classless stylesheet). No Tailwind.
- HTMX: used only where it reduces clicks; do not build SPA behavior.
- Markdown: server-side render + HTML sanitization
- Editor: CodeMirror 6 only on edit pages

Dependency policy:
- Prefer stdlib + small, boring libs.
- Avoid “framework sprawl.”
- Any new dependency must justify itself: security, stability, maintenance.

---

## 3) Roles & permissions (simple role gating)

Roles:
- TenantAdmin: manage users/roles/settings; full access
- Editor: create/edit docs + runbooks; verify runbooks
- Reader: read-only access

Sensitivity:
- Keep sensitivity labels in schema, but in MVP use **role-only gating**:
  - public-internal: everyone
  - restricted/confidential: Admin + Editor (same behavior initially)

No per-doc allowlists in MVP.

---

## 4) Tenancy and clients (MVP decisions)

- Tenants exist from day 1.
- Clients exist from day 1.
- Sites do not exist in MVP; do not add a sites table until Phase “Sites later”.

Client usage:
- A document may optionally be associated with a client.
- Navigation supports filtering by client (e.g. “All docs” vs “Client docs”).
- A doc path namespace may optionally include a client prefix (implementation choice), but do not overcomplicate:
  - acceptable: `path="clients/acme/firewall/runbook.md"` and client_id set
  - also acceptable: store client_id separately and keep path human-only
- Whatever choice you make, keep URLs stable and human-readable.

---

## 5) Database model (baseline)

Minimum tables for MVP:
- tenants(id, name, created_at)
- users(id, email, name, password_hash, created_at, last_login_at)
- memberships(id, tenant_id, user_id, role, created_at)
- clients(id, tenant_id, name, code, notes, created_at)

Docs + history:
- documents(
    id, tenant_id, client_id NULL,
    path, title,
    doc_type,                  -- "doc" | "runbook"
    sensitivity,               -- label, but MVP gating is role-only
    owner_user_id NULL,
    metadata_json,
    current_revision_id,
    created_by, created_at, updated_at
  )
- revisions(
    id, tenant_id, document_id,
    body_markdown,
    created_by, created_at,
    message,
    base_revision_id NULL
  )
- audit_log(
    id, tenant_id, actor_user_id,
    action, target_type, target_id,
    at, ip, user_agent,
    metadata_json
  )

Runbooks v1:
- runbook_status(
    document_id PK,
    tenant_id,
    last_verified_at NULL,
    last_verified_by_user_id NULL,
    verification_interval_days INT,
    next_due_at TIMESTAMP NULL
  )

Notes:
- Always enforce tenant_id matches between joined records.
- Indexes: (tenant_id, path), (tenant_id, client_id), (tenant_id, updated_at), FTS indexes later.

---

## 6) HTTP routes (baseline)

Auth:
- GET /login
- POST /login
- POST /logout

Docs:
- GET  /docs                          -- home / recent / dashboards
- GET  /docs/*path                    -- read
- GET  /docs/*path/edit               -- edit
- POST /docs/*path/save               -- create new revision (requires base_revision_id)
- GET  /docs/*path/history            -- revision list
- GET  /docs/*path/diff?from=..&to=.. -- diff view
- POST /docs/*path/revert/:revID      -- creates new revision

Runbooks:
- GET  /runbooks                      -- overdue, unowned, recent verify
- POST /docs/*path/verify             -- verify runbook (audited)

Search:
- GET /search?q=...                   -- later phase but route can exist early

Clients:
- GET /clients
- GET /clients/:id
- POST /clients (admin/editor only)

HTMX helper endpoints:
- POST /preview                       -- markdown -> rendered fragment (sanitized)
- GET  /tree?folder=...               -- partial folder tree expand (optional)
- POST /docs/*path/metadata           -- inline metadata updates (optional)

---

## 7) HTMX usage rules

Allowed:
- Server-preview of markdown while editing (swap a preview div).
- Inline metadata edits (owner, verification interval, doc_type).
- Modal forms (rename, move, create doc).
- Folder tree expand/collapse.

Not allowed:
- Building a client-side router.
- Replacing full navigation state with HTMX.
- Persistent SPA-like history management.
Full page loads are normal and fine.

---

## 8) Editing & revision conflict rules

- Edit page includes `base_revision_id`.
- Save endpoint must check base_revision_id == documents.current_revision_id.
- If mismatch:
  - return a conflict page that shows both versions and a next step:
    - “Reload latest” OR “Create new revision anyway (fork)” OR “Copy my text”
  - Do not silently overwrite.

Saving creates:
1) new revision row
2) update documents.current_revision_id
3) audit_log entry

Revert:
- Never deletes history.
- Creates a new revision whose body is copied from the old revision.

---

## 9) Markdown rendering & safety

- Render on server using a stable markdown library.
- Sanitize HTML output to prevent XSS.
- Disable raw HTML in markdown unless there is a very specific need and a robust sanitizer.
- Links:
  - internal doc links should be normalized (helpful but not required MVP)
  - external links should get rel="noopener noreferrer"

---

## 10) Audit logging policy (minimum)

Log:
- login success/failure
- doc create/edit/save
- doc rename/move/delete (if delete exists)
- runbook verify
- client create/update
- membership/role changes (admin)

Audit metadata_json should include:
- document path/title when relevant
- revision id when relevant
- client id when relevant
- conflict events if they occur

Audit must be append-only.

---

## 11) Search (when implemented)

- Use Postgres full-text search on:
  - documents.title, documents.path
  - revisions.body_markdown (current revision only for performance)
- Provide filters:
  - client
  - doc_type (doc/runbook)
  - owner
  - updated_at recency
- Keep search UI simple and fast.

---

## 12) Project structure (suggested)

/cmd/docstor/main.go
/internal/
  auth/         -- sessions, password hashing, middleware
  db/           -- queries/repo layer, migrations
  docs/         -- documents, revisions, rendering
  runbooks/     -- verification
  clients/      -- clients CRUD
  audit/        -- audit logger
  web/          -- handlers, templates, static
/web/templates/
  layout/
  docs/
  runbooks/
  clients/
/web/static/
  css/
  js/           -- ONLY CodeMirror + tiny helpers

---

## 13) Testing requirements

Must-have tests:
- tenant scoping cannot leak data across tenants
- role gating: Reader cannot edit; Editor can; Admin can manage roles
- revision conflict detection works
- revert creates new revision
- markdown rendering is sanitized (XSS regression tests)
- audit log is written for required actions

---

## 14) Performance & UX basics

- Every page should render quickly and predictably.
- Avoid N+1 queries when listing docs/history.
- Keep templates readable and maintainable.
- Don’t invent complex UI patterns; techs want speed and clarity.

---

## 15) Out of scope for MVP (do not implement unless explicitly approved)

- Sites table and site scoping
- Per-doc allowlists / break-glass reason prompts
- Attachments/evidence bundles
- Templates & checklist instances
- CMDB-lite directory
- Incidents/known issues/changes
- PDF packet export
- Real-time collaborative editing

If asked to implement any of the above, respond with:
- where it fits in the phase plan
- schema + routes needed
- minimal compliant implementation approach

---

## 16) Delivery expectations

When implementing a feature, provide:
- migration(s)
- handlers + templates
- permission checks
- audit logging
- tests where applicable
- brief notes on any tradeoffs

Prefer shipping a small, correct slice over a broad, fragile one.
