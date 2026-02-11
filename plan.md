# Docstor Plan

## Implementation Status (as of Feb 2026)

### ✅ Complete

| Phase | What | Notes |
|-------|------|-------|
| 0–7.5 | MVP: Auth, docs, search, runbooks, editor | Core platform |
| 8 | Attachments + Evidence Bundles | Upload, link, bundle, ZIP export |
| Security | CSRF (nosurf), rate limiting, sensitivity gating | S-1, S-2, S-3 |
| UX A–D | Mobile, flash messages, breadcrumbs, editor polish | 22/25 UX issues resolved |
| Tests | Tenant isolation, role gating, XSS, conflict, audit | 33 tests, -race clean |
| Admin | User management, audit viewer, tenant settings | /admin section |
| Landing | Public landing page + improved dashboard | Stats, recent docs, overdue |
| Doc Ops | Rename/move/delete documents | Full CRUD complete |
| 12 | Doc Health Dashboard | Stale, unowned, health % |
| 9 | Templates + Checklists | Templates CRUD, use-template, checklists, instances, HTMX toggle |
| 10 | CMDB-lite + Live Blocks | Systems, vendors, contacts, circuits CRUD + shortcode rendering |
| 11 | Known Issues + Incidents | Known issues board, incident timeline, events |

### 🚧 Remaining

| Phase | What | Est. | Schema needed |
|-------|------|------|---------------|
| Polish | Drag-drop upload, folder tree, image preview, metadata edit | 2–3h | None |
| 13 | Sites (client → sites) | 2–3h | sites |

### Phase 9 — Templates + Checklists

Goal: Reusable doc/runbook templates and trackable checklists.

Schema:
```sql
templates(id, tenant_id, name, template_type, body_markdown, default_metadata_json, created_by, created_at)
  -- template_type: 'doc' | 'runbook' | 'incident_rca' | 'change'
checklists(id, tenant_id, name, description, created_by, created_at)
checklist_items(id, tenant_id, checklist_id, position, text)
checklist_instances(id, tenant_id, checklist_id, linked_type, linked_id, status, created_by, created_at, completed_at)
checklist_instance_items(id, tenant_id, instance_id, item_id, done_by_user_id, done_at, note)
```

Routes:
- `GET /templates` — Template library
- `GET /templates/new`, `POST /templates` — Create template
- `GET /templates/{id}` — View template
- `POST /docs/new?template={id}` — Create doc from template
- `GET /checklists` — Checklist library
- `POST /checklist-instances` — Start checklist instance
- `POST /checklist-instances/{id}/items/{itemId}/toggle` — Check/uncheck

### Phase 10 — CMDB-lite + Live Blocks ✅ COMPLETE

Delivered: Full CRUD for systems/vendors/contacts/circuits. Shortcode rendering (`{{system:uuid}}`, `{{vendor:uuid}}`, etc.) in markdown. 12 templates, migration 005.

### Phase 11 — Known Issues + Incidents ✅ COMPLETE

Delivered: Known issues board with status/severity filters. Incident timeline with events. 6 templates, migration 006.

### Polish Items

- [ ] Drag-and-drop file upload on edit/attachments pages
- [ ] HTMX folder tree navigation (`GET /tree?folder=...`)
- [ ] Image/PDF preview before download
- [ ] Quick-edit doc metadata from view page (inline HTMX)

---

## Original Architecture Document

> The following is the original architecture plan for reference.

---

Absolutely, Cassie. Here’s a full Docstor plan that includes the “well-stocked documentation + a little more” upgrades (evidence bundles, CMDB-lite directory, templates/checklists, known issues/incidents, doc health + linkbacks), while staying faithful to “no React, no obscene JavaScript.”

I’m writing this as something you can drop straight into a `plan.md` for review.

---

DOCSTOR FULL PLAN (for review)

Vision
Docstor is a web-first MSP documentation system built to survive real life: midnight outages, new techs, missing context, and audits. It starts as “markdown docs with revision history + audit log” and grows into “living runbooks” plus a small set of adjacent operational modules that attach to docs instead of becoming a PSA clone.

Non-negotiables

* Server-rendered pages. No SPA.
* HTMX only for small interactions (partials, inline edits, dialogs).
* Minimal JavaScript. The only serious JS “island” is the markdown editor (CodeMirror).
* Immutable revisions and append-only audit trail for meaningful actions.
* Tenant scoping everywhere (even if you run single-tenant at launch).
* Secrets never stored in Docstor. Only references/pointers to your password manager/vault.

Core pillars

1. Documentation: docs, folders, markdown rendering, editing
2. History & trust: revisions, diff, revert, audit log
3. Living runbooks: ownership, verification cadence, verification workflow
4. Evidence: attachments tied to revisions, evidence bundles, export packets
5. “A little more”: structured directory (CMDB-lite), templates/checklists, known issues + incidents, doc health + backlinks

---

Architecture & stack

Backend

* Go HTTP server (standard net/http or chi)
* HTML templates (Go html/template) + partials
* Postgres database + migrations
* Background jobs (optional later) via a simple in-process queue or cron-like runner

Frontend

* Server-rendered HTML + a small CSS baseline (simple custom CSS, or Pico.css-class minimal approach)
* HTMX for targeted interactions (inline edit, preview pane updates, modal forms, tree expand/collapse)
* CodeMirror 6 on edit pages only (the one JS island)

Markdown rendering

* Server-side markdown render with sanitization
* Add a tiny “shortcode” layer for live blocks (details below)

File storage

* MVP: local disk storage (per-tenant partitioning in directory layout)
* Later: S3-compatible backend (MinIO, Wasabi, AWS) behind an interface

Search

* Postgres full-text search (tsvector), indexes, ranking
* Optional: trigram index for titles/paths

Security

* Session cookies, secure flags, CSRF protection compatible with HTMX
* Rate-limited auth
* Centralized permission checks, tested
* Strong tenant scoping and “can I see this doc?” enforcement

---

Data model (MVP + planned expansions)

Tenancy & auth

* tenants(id, name, created_at)
* users(id, email, name, password_hash, created_at, last_login_at)
* memberships(id, tenant_id, user_id, role, created_at)

Optional but recommended early: “client context”
This is how Docstor becomes MSP-shaped without becoming a PSA.

* clients(id, tenant_id, name, code, notes, created_at)
* sites(id, tenant_id, client_id, name, address, notes)

Docs & runbooks

* documents(id, tenant_id, client_id nullable, path, title, doc_type, sensitivity, owner_user_id nullable, metadata_json, current_revision_id, created_by, created_at, updated_at)

  * doc_type: “doc” | “runbook” (or a tag, but a type makes dashboards easy)
  * sensitivity: “public-internal” | “restricted” | “confidential” (your naming)
* revisions(id, tenant_id, document_id, body_markdown, created_by, created_at, message, base_revision_id nullable)
* audit_log(id, tenant_id, actor_user_id, action, target_type, target_id, at, ip, user_agent, metadata_json)

Doc links + health signals

* doc_links(id, tenant_id, from_document_id, to_document_id, created_at) (rebuilt on save, used for backlinks and “broken links”)
* doc_events (optional): denormalized view tracking for “most viewed”

  * doc_views(id, tenant_id, document_id, user_id, at) (optional; can be a counter instead)

Runbook verification

* runbook_status(document_id PK, tenant_id, last_verified_at, last_verified_by_user_id, verification_interval_days, verification_checklist_json, next_due_at computed or stored)

Evidence + attachments

* attachments(id, tenant_id, filename, content_type, size_bytes, sha256, storage_key, created_by, created_at)
* attachment_links(id, tenant_id, attachment_id, linked_type, linked_id, created_at)

  * linked_type: “revision” | “document” | “incident” | “change” | “checklist_instance”
* evidence_bundles(id, tenant_id, name, description, created_by, created_at)
* evidence_bundle_items(id, tenant_id, bundle_id, attachment_id, note, created_at)

Templates + checklists

* templates(id, tenant_id, name, template_type, body_markdown, default_metadata_json, created_by, created_at)

  * template_type: “doc” | “runbook” | “incident_rca” | “change”
* checklists(id, tenant_id, name, description, created_by, created_at)
* checklist_items(id, tenant_id, checklist_id, position, text)
* checklist_instances(id, tenant_id, checklist_id, linked_type, linked_id, status, created_by, created_at, completed_at)
* checklist_instance_items(id, tenant_id, instance_id, item_id, done_by_user_id nullable, done_at nullable, note)

CMDB-lite structured directory
Keep it intentionally narrow: the “facts we always need,” not every laptop.

* systems(id, tenant_id, client_id, site_id nullable, system_type, name, fqdn nullable, ip nullable, environment, notes, owner_user_id nullable, created_at, updated_at)

  * system_type: server/firewall/switch/circuit/app/service/etc
* vendors(id, tenant_id, client_id nullable, name, type, phone, email, portal_url, escalation_notes, created_at)
* contacts(id, tenant_id, client_id, site_id nullable, name, role, phone, email, notes)
* circuits(id, tenant_id, client_id, site_id, provider, circuit_id, wan_ip, speed, notes, created_at, updated_at)

Known issues + incidents

* known_issues(id, tenant_id, title, severity, status, client_id nullable, description, created_by, created_at, updated_at, linked_document_id nullable)
* incidents(id, tenant_id, title, severity, status, client_id nullable, started_at, ended_at nullable, summary, created_by, created_at)
* incident_events(id, tenant_id, incident_id, at, event_type, detail, actor_user_id nullable)
* rca_documents are just documents created from an RCA template and linked to the incident

Change records (light but powerful)

* changes(id, tenant_id, title, risk_level, status, client_id nullable, window_start, window_end, rollback_plan_markdown, validation_plan_markdown, created_by, created_at)
* changes link to docs/runbooks and can have checklist instances and evidence attached

---

“Live blocks” (docs pulling structured facts)

Goal
Prevent drift between “the doc says X” and “reality says Y,” without heavy JS.

Approach (minimal and server-friendly)
Add a shortcode syntax that’s easy to parse server-side during markdown render, for example:

* {{system:123}} renders a small server-generated block with name, IP/FQDN, environment, and links to the system record
* {{circuit:55}} renders ISP/provider/circuit ID/WAN IP
* {{vendor:9}} renders escalation contact + portal link

Rules

* Shortcodes always degrade safely: if missing or permission-denied, show a warning block rather than breaking the page.
* Shortcodes are resolved server-side at render time, so the HTML output is consistent and auditable.

---

UI and page map (server-rendered)

Global layout

* Left sidebar: folder tree + quick links (Runbooks, Systems, Known Issues, Incidents, Search)
* Top bar: tenant switch (if needed), client/site context filter, search
* Main panel: content

Docs

* Docs home: recent docs, pinned docs, doc health highlights
* Folder view: browse docs and subfolders
* Doc read view: rendered markdown, metadata panel, backlinks, attachments/evidence section, quick actions
* Doc edit view: CodeMirror editor, explicit Save, preview (HTMX server preview endpoint), metadata edit
* History view: revision list, diff links, revert action
* Diff view: server-rendered unified diff or split view
* Runbook verify dialog: checklist + “verify” action stamps verifier and time

Evidence

* Attachment upload modal (link to doc/revision/incident/change/checklist instance)
* Evidence bundle builder (select existing attachments + notes)
* Evidence bundle export (zip)
* “Packet export” (phase later): select docs/runbooks → generate a PDF packet or HTML bundle

Templates & checklists

* Template library: create new doc/runbook/incident/change from template
* Checklist library: create checklists
* Checklist instance view: track completion, attach evidence, link to ticket/change/incident

CMDB-lite directory

* Systems list: filter by client/site/type/environment, click-through detail page
* Vendors list: global or per-client; detail page includes escalation and notes
* Contacts list: per client/site
* Circuits list: per site; useful for outages

Known issues & incidents

* Known issues board: status columns, filters by client/severity, link to related docs/runbooks
* Incident view: timeline events, attached evidence, linked RCA doc, export summary
* Incident timeline add event: simple form (HTMX append)

Changes

* Change list: upcoming/active/completed
* Change detail: scope, risk, window, rollback/validation sections, linked runbooks, checklist instance, evidence attachments
* Post-change validation: checklist completion + evidence attachments

Doc health dashboards

* Stale docs: “not updated in X days”
* Runbooks overdue for verification
* Docs without owners
* Broken links list (from doc_links)
* “Popular but stale” list (views vs updates)

---

Routes and HTMX patterns (keep it non-SPA)

Core routes

* GET/POST /login, POST /logout
* GET /docs (home)
* GET /docs/*path (read)
* GET /docs/*path/edit (edit)
* POST /docs/*path/save (create revision; requires base_revision_id)
* GET /docs/*path/history
* GET /docs/*path/diff?from=…&to=…
* POST /docs/*path/revert/:revisionID (creates new revision)
* POST /docs/*path/verify (runbook verify action)

HTMX endpoints

* POST /preview (markdown → rendered fragment)
* POST /docs/*path/metadata (inline updates)
* GET /tree?folder=… (expand folder partial)
* POST /attachments/upload (returns attachment card partial)
* POST /incident/:id/event (append timeline event partial)
* POST /checklist/:instance_id/toggle/:item_id (returns updated row partial)

Directory + boards

* GET /systems, GET /systems/:id
* GET /vendors, GET /vendors/:id
* GET /known-issues, GET /known-issues/:id
* GET /incidents, GET /incidents/:id
* GET /changes, GET /changes/:id
* GET /search?q=… (filters)

---

Permissions model (simple now, expandable later)

Roles

* TenantAdmin: manage users/roles/settings, full access
* Editor: create/edit docs, create incidents/changes, upload evidence, runbook verify
* Reader: read-only access to allowed sensitivity levels

Sensitivity gates

* “public-internal”: all tenant members
* “restricted”: editors/admins (or explicit allowlist later)
* “confidential”: explicit allowlist (phase later), or admin-only in early versions

Break-glass (later)
For confidential docs, require a reason tied to a ticket/change/incident and log it.

---

Audit policy (what gets logged)

Log these actions at minimum

* login success/failure
* document created/edited (revision created)
* document moved/renamed/deleted/restored
* runbook verified
* attachments uploaded/downloaded/deleted
* evidence bundle created/exported
* change created/updated/status changed
* incident created/status changed/timeline event added
* permission/role changes

Nice-to-have

* doc viewed events (for health dashboards), but keep it lightweight

---

Phased delivery plan (with acceptance criteria)

Phase 0: Repo + skeleton + boring UI
Deliver: Go app, template layout, DB migrations, login screen shell, sidebar/topbar layout.
Accept: a page loads server-rendered; no JS required; project runs via one command (compose or make).

Phase 1: Tenancy + auth + roles
Deliver: tenants/users/memberships, secure sessions, permission middleware.
Accept: every request is tenant-scoped; role checks are centralized; login rate limit works.

Phase 2: Docs MVP (read/create/edit)
Deliver: folder tree browsing, doc read view, create doc, edit doc with textarea (temporary), server markdown render.
Accept: docs are addressable by path; render is sanitized; basic navigation works.

Phase 3: Revisions + audit baseline
Deliver: revisions on save, history view, revert creates a new revision, audit_log entries for write actions.
Accept: no overwriting history; revert never deletes; audit entries are created for edits/moves.

Phase 4: Editor island (CodeMirror) + preview
Deliver: CodeMirror only on edit page; explicit save; server-preview via HTMX.
Accept: edit works without JS (textarea fallback), but with JS it’s pleasant; preview matches read view output.

Phase 5: Search
Deliver: full-text search over title/body/path + filters (client, tag/type, sensitivity).
Accept: search is fast and good enough for daily use; results link directly to docs.

Phase 6: Living runbooks v1
Deliver: doc_type runbook, owner, verification interval, verify workflow, runbook dashboard (overdue, unowned).
Accept: verification stamps are auditable; overdue list is accurate; runbooks feel “alive,” not ceremonial.

Phase 7: Evidence bundles + attachments
Deliver: upload attachments, link to revisions/docs/incidents/changes, evidence bundles, export zip.
Accept: attachments are permission-checked; downloads are audited; evidence stays tied to specific revisions.

Phase 8: Templates + checklists
Deliver: template library + “create from template”; checklist library; checklist instances linked to docs/incidents/changes; evidence attach to checklist instances.
Accept: a tech can spin up a runbook/checklist in under a minute; checklist completion is tracked and auditable.

Phase 9: CMDB-lite directory + live blocks
Deliver: systems/vendors/contacts/circuits directory; doc shortcodes that render live blocks; linking from systems to docs and back.
Accept: live blocks render server-side; missing references show safe warnings; directory browsing is fast and useful.

Phase 10: Known issues + incidents + lightweight change records
Deliver: known issues board; incident timeline builder; change records with rollback/validation sections; linking + evidence + checklists.
Accept: incident pages produce clean timelines; known issues reduce repeated troubleshooting; change records are usable without bureaucracy.

Phase 11: Doc health dashboards
Deliver: stale docs, runbooks overdue, broken links, popular-but-stale, docs without owners.
Accept: dashboards help you choose what to fix next; link integrity scan is reliable.

Phase 12: Hardening + export packets (optional)
Deliver: PDF/doc packet export; S3 storage backend; break-glass; tamper-evident audit (hash chain) if desired.
Accept: exports look professional; storage is pluggable; security posture is tighter.

---

MVP definition (the “ship this first” cut)

If we want a clean, believable first release that already feels MSP-grade, MVP should include:

* Tenancy + roles
* Docs (read/create/edit)
* Revisions + audit
* Search
* Runbooks v1 (owner + verify cadence + overdue dashboard)

Everything else is “Phase 7+” and can be layered without rewriting foundations.

---

Deliberate non-goals (keeps Docstor from becoming a monster)

* Full RMM/PSA replacement
* Storing secrets/passwords
* Real-time collaborative editing (later maybe, but not now)
* Asset inventory of every endpoint
* Heavy client-side rendering

---

Review checklist (what you should judge this plan on)

* Does it preserve the no-React/minimal-JS law? (yes: only CodeMirror)
* Does it keep scope layered so MVP is shippable? (yes: phases 0–6 are tight)
* Do the “little more” features stay attached to docs rather than becoming separate products? (yes)
* Does the data model support multi-client MSP reality without overbuilding? (yes: CMDB-lite, not CMDB-full)
* Is auditability first-class? (yes)


---

Absolutely. Here’s a **review-ready, PR-sized implementation checklist** you can paste directly into `plan.md` as an appendix. It’s designed to keep scope tight, keep reviews easy, and make handoff clean.

---

## Appendix A — Implementation Checklist (PR-sized)

Conventions used below:

* Each item is intended to be **one pull request** (or at most two if you hit a natural split).
* Every PR that changes data must include: **migration(s)** + **rollback** + **indexes** (when relevant).
* Every PR that changes permissions or tenancy must include: **tests** for isolation/gating.
* Every PR that adds a write action must include: **audit_log entry**.

### Phase 0 — Repo skeleton + “boring but real” app shell

**PR-000: Repo + local dev**

* Add: basic Go module layout, Makefile/task runner, docker compose for Postgres
* Add: `.env.example`, config loader, minimal logging
* Acceptance:

  * `make dev` (or equivalent) brings up app + DB
  * Health route responds (200 OK)

**PR-001: DB migrations framework**

* Add: migrations tool (or internal migration runner), initial migration folder
* Add: `schema_migrations` table strategy
* Acceptance:

  * Fresh DB bootstraps successfully
  * Migrations are repeatable and deterministic

**PR-002: Base layout + static assets**

* Add: server-rendered layout template, basic CSS file, nav shell placeholders
* Acceptance:

  * Any route renders inside the shared layout
  * No JS required to load and navigate

---

### Phase 1 — Auth + tenancy + simple role gating (non-negotiable foundation)

**PR-010: Tenants/users/memberships schema + seed**

* Migrations:

  * tenants, users, memberships
* Add: seed/bootstrapping command to create first tenant + admin user
* Acceptance:

  * You can create a tenant + admin and log into the system

**PR-011: Sessions + login/logout**

* Add: login form, POST login, logout
* Add: secure session cookies (HttpOnly/Secure/SameSite)
* Add: password hashing
* Audit:

  * login success/failure logged
* Acceptance:

  * Valid login works; invalid login doesn’t leak info; session persists

**PR-012: Role gating middleware**

* Add: role checks for Admin/Editor/Reader
* Add: route guards and “403” template
* Tests:

  * Reader blocked from edit routes
  * Editor allowed for doc edits
  * Admin allowed for admin-only actions
* Acceptance:

  * Permissions enforced consistently via centralized helpers

**PR-013: Tenant scoping middleware + guardrails**

* Add: tenant context resolution in every request
* Add: repository layer requires tenant_id for queries
* Tests:

  * tenant isolation: data from tenant A never visible in tenant B
* Acceptance:

  * No handler can query without tenant scope

---

### Phase 2 — Clients now (sites later)

**PR-020: Clients schema + CRUD (minimal)**

* Migrations:

  * clients (tenant_id, name, code, notes)
* Add: clients list page + create form (Admin/Editor)
* Audit:

  * create/update logged
* Acceptance:

  * Can create and browse clients
  * Reader can view clients; only Admin/Editor can create/edit

**PR-021: Client context filtering in UI (lightweight)**

* Add: top-bar “Client filter” (All / specific client)
* Add: docs list respects filter (even before docs exist, stub it)
* Acceptance:

  * Selecting a client persists through navigation (query param or session)

---

### Phase 3 — Docs MVP (read/create/edit) with server-rendered markdown

**PR-030: Documents + revisions schema**

* Migrations:

  * documents, revisions
  * include client_id nullable, path, title, doc_type, sensitivity, metadata_json
* Indexes:

  * (tenant_id, path) unique
  * (tenant_id, client_id)
* Acceptance:

  * Tables exist, indexes applied, constraints correct

**PR-031: Document browse (folder/path)**

* Add: left sidebar folder tree (basic, can be non-HTMX initially)
* Add: `/docs` landing showing recent docs + “new doc” button
* Acceptance:

  * Navigate folders and open docs by path

**PR-032: Doc read view (server markdown render + sanitization)**

* Add: Markdown render on server + sanitization
* Add: doc read template (title, metadata stub, rendered body)
* Tests:

  * XSS safety regression test(s)
* Acceptance:

  * Markdown renders consistently; unsafe HTML doesn’t execute

**PR-033: Doc create flow**

* Add: “new doc” form (path, title, client optional, doc_type)
* Create initial revision with body (empty or starter template)
* Audit:

  * doc created logged
* Acceptance:

  * New doc appears immediately in navigation and can be opened

**PR-034: Doc edit flow (textarea v0)**

* Add: edit page with `<textarea>` and explicit Save
* Add: POST save creates new revision
* Acceptance:

  * Editor/Admin can edit and save revisions, Reader cannot

---

### Phase 4 — Trust layer v1 (immutable history, conflict detection, audit)

**PR-040: Audit log table + writer helper**

* Migrations:

  * audit_log
* Add: small audit helper API used everywhere
* Acceptance:

  * Write actions append audit records with actor, IP, user-agent

**PR-041: Revision conflict detection**

* Add: base_revision_id required on save
* Add: conflict page when base != current
* Tests:

  * save conflict detected
  * cannot overwrite silently
* Acceptance:

  * Two editors cannot clobber each other without an explicit conflict flow

**PR-042: History list + revert**

* Add: history page listing revisions
* Add: revert endpoint that creates a new revision from old body
* Audit:

  * revert logged
* Acceptance:

  * Revert never deletes history, and always creates a new revision

**PR-043: Diff view (server-side)**

* Add: `/diff?from=&to=` view
* Acceptance:

  * Diffs are readable and stable (good enough for MVP)

---

### Phase 5 — Editor JS island (CodeMirror) + HTMX preview

**PR-050: CodeMirror integration (edit page only)**

* Add: CodeMirror loaded only on `/edit`
* Fallback: textarea works with JS disabled
* Acceptance:

  * Editing feels good; page still works without JS

**PR-051: Server-side preview endpoint + HTMX preview pane**

* Add: `/preview` endpoint renders sanitized HTML fragment
* Add: HTMX “Preview” toggle button on edit page
* Acceptance:

  * Preview matches read view rendering (same pipeline)

---

### Phase 6 — Search (MVP-grade)

**PR-060: Full-text search schema support**

* Add: tsvector columns or generated vectors (title/path + current revision body)
* Add: indexes for FTS
* Acceptance:

  * Search query is fast on a sample dataset

**PR-061: Search UI + filters**

* Add: search results page with filters:

  * client, doc_type, owner (stub if owner not yet), updated_at sort
* Acceptance:

  * Tech can find docs quickly; results link directly to doc read view

---

### Phase 7 — Living runbooks v1 (MVP finish line)

**PR-070: Runbook status schema + doc metadata fields**

* Migrations:

  * runbook_status
* Add: doc_type=runbook, owner_user_id, verification_interval_days
* Acceptance:

  * A doc can be flagged as runbook and shown as such in UI

**PR-071: Verify action + dashboard**

* Add: verify endpoint stamps last_verified + next_due
* Add: `/runbooks` dashboard:

  * overdue, unowned, recently verified
* Audit:

  * verify logged
* Acceptance:

  * Overdue list is correct; verify updates state and is auditable

**MVP Milestone Acceptance**

* Tenant isolation proven by tests
* Role gating enforced everywhere
* Clients exist and docs can be associated to clients
* Docs: create/read/edit with revisions + history + revert + diff
* Audit logs for meaningful actions
* Search works
* Runbooks: verify + overdue dashboard

---

## Post-MVP Phases (approved roadmap, not required for MVP)

### Phase 8 — Attachments + evidence bundles ✅ COMPLETE

**PR-080: Attachments schema + storage interface (local disk)** ✅
**PR-081: Upload UI + link attachments to revisions/docs** ✅
**PR-082: Evidence bundles + export zip** ✅
Acceptance: uploads permission-checked; downloads audited; evidence ties to specific revisions.

### Phase 9 — Templates + checklists ✅ COMPLETE

Delivered: Template library (doc/runbook types), create-from-template flow, checklist library with items, checklist instances linked to docs, HTMX toggle for items. Migration 004.

### Phase 10 — CMDB-lite directory + live blocks ✅ COMPLETE

Delivered: Systems/vendors/contacts/circuits full CRUD with client filtering. Live block shortcodes (`{{system:uuid}}`, `{{vendor:uuid}}`, `{{contact:uuid}}`, `{{circuit:uuid}}`) resolved server-side during markdown render. Missing refs show warning spans. Migration 005.

### Phase 11 — Known issues + incidents ✅ COMPLETE

Delivered: Known issues board with status columns and severity. Incident timeline builder with event types (detected/acknowledged/investigating/mitigated/resolved/note). Client and status filtering. Migration 006.

Note: Change records (PR-112) deferred — not in MVP scope.

### Phase 12 — Doc health dashboards ✅ COMPLETE

Delivered: Stale docs, unowned docs, health percentage dashboard at `/docs/health`.

### Phase 13 — Sites (deferred)

**PR-130: Sites schema + client->site relationship**
**PR-131: Optional site scoping for directory objects**
Acceptance: adds sites without breaking existing client/doc URLs.

---

## UX/UI Overhaul Phases

Identified from full-app UX test on Feb 10, 2026.

### Phase A — Critical UX Fixes ✅ COMPLETE

**A-1: Add Attachments button to document view page**
- Users have no way to access attachments from the doc view
- Add button alongside Edit/History in doc action bar

**A-2: Fix mobile responsive layout**
- Sidebar doesn't collapse on narrow viewports
- Add hamburger menu, collapsible sidebar
- Tables overflow on mobile — add horizontal scroll
- Media queries at 768px and 1024px breakpoints

**A-3: Fix form submission issue (forms landing on search)**
- Some form submissions redirect to the search page
- Root cause: topbar search form may be wrapping/intercepting other forms
- Fix: ensure all forms are properly isolated

**A-4: Fix evidence bundle "Add File" UX**
- Currently requires manually entering attachment UUID
- Replace with searchable attachment picker
- Show attachment filename, date, and size in picker

### Phase B — High Priority UX (2-3 hours) ✅ COMPLETE

**B-1: Sidebar active state**
- No visual indicator of current section
- Add `.active` class based on URL prefix matching

**B-2: Flash messages (success/error feedback)**
- No feedback after form submissions
- Add session-based flash message system
- Show success (green), error (red), info (blue) banners

**B-3: Fix/implement client filter dropdown**
- "All Clients" dropdown in topbar does nothing
- Either implement filtering or remove the dropdown

**B-4: Unify search UX**
- Topbar search and search page are disconnected
- Topbar search should redirect to /search?q=...

### Phase C — Visual Polish (2-3 hours) ✅ COMPLETE

**C-1: Consistent breadcrumbs on all pages**
- Some pages have breadcrumbs, some don't
- Add standard breadcrumb component to: docs list, edit, clients, runbooks

**C-2: Table styling improvements**
- Add row hover states, zebra striping, better borders
- Compact header styling

**C-3: Empty state improvements**
- Replace plain text with styled empty states
- Add icons and prominent "Create" CTAs

**C-4: Loading states on forms**
- Disable submit buttons during submission
- Show spinner or "Saving..." text

**C-5: CSS variables for consistent theming**

### Phase D — Nice to Have (ongoing) ✅ PARTIAL

**D-1: Favicon**
**D-2: Keyboard shortcuts** (Ctrl+S save, Ctrl+K search, Esc cancel)
**D-3: Better code block styling** (syntax highlighting, copy button)
**D-4: Consistent date formatting** (relative: "2h ago" or standardized)
**D-5: Drag-and-drop file upload**
**D-6: Image/PDF preview before download**
**D-7: Confirmation dialogs on destructive actions**

---

## Hand-off Notes for Implementation

* Keep PRs small and reviewable; favor vertical slices (schema + handler + template + tests).
* Enforce tenant scoping in repository methods, not ad-hoc in handlers.
* Every write path must add an audit entry (and tests should expect it where practical).
* Do not expand JS beyond CodeMirror + tiny helpers; HTMX is for partial HTML, not app state.

### Security Hardening ✅ COMPLETE

**S-1: CSRF Protection (nosurf)**
- justinas/nosurf v1.2 middleware on all POST routes
- Form tokens via {{.CSRFField}} in every POST form
- HTMX X-CSRF-Token header injection
- Proper TLS detection via SetIsTLSFunc (X-Forwarded-Proto)
- /preview endpoint exempted for HTMX markdown preview

**S-2: Login Rate Limiting**
- In-memory IP-based rate limiter (5 attempts/60s)
- Port stripped from RemoteAddr for consistent tracking
- X-Forwarded-For support
- Retry-After header
- Reset on successful login
- Tests with race detection

**S-3: Sensitivity Gating (claude.md §3)**
- docs.CanAccess(sensitivity, role) helper
- public-internal: all roles
- restricted/confidential: admin + editor only
- Applied to all doc handlers + list/search filtering
