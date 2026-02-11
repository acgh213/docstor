# Docstor Work Breakdown

## Completed

| Phase | Description | Status |
|-------|-------------|--------|
| 0-7.5 | MVP (Auth, Docs, Search, Runbooks, Editor) | ✅ |
| 8 | Attachments + Evidence Bundles | ✅ |
| Security | CSRF, rate limiting, sensitivity gating | ✅ |
| UX A-D | Mobile, flash, breadcrumbs, editor polish | ✅ |
| Tests | Tenant isolation, role gating, XSS, conflict, audit (33 tests) | ✅ |
| Admin | User management, audit viewer, tenant settings | ✅ |
| Landing | Public landing page + improved dashboard | ✅ |
| Doc Ops | Rename/move/delete documents | ✅ |
| 12 | Doc Health Dashboard (stale, unowned, health %) | ✅ |

---

## Remaining: Feature Phases

### Phase 9 — Templates + Checklists (3-4 hours)
- Template library (reusable doc/runbook templates)
- Checklist items with trackable instances
- Schema: templates, checklists, checklist_items, checklist_instances
- Migration + handlers + templates + audit

### Phase 10 — CMDB-lite + Live Blocks (4-6 hours)
- Systems/vendors/contacts/circuits tables
- `{{system:uuid}}` shortcodes in markdown (server-side render)
- Directory browse UI
- Migration + CRUD handlers

### Phase 11 — Known Issues + Incidents (3-4 hours)
- Known issues board (open/investigating/resolved)
- Incident timeline with events
- Link to docs
- Migration + handlers + templates

---

## Remaining: Polish

- [ ] Drag-and-drop file upload
- [ ] HTMX folder tree navigation (`GET /tree?folder=...`)
- [ ] Image/PDF preview before download
- [ ] Quick-edit doc metadata from view page
