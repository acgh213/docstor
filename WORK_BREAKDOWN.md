# Docstor Work Breakdown

## Completed

| Phase | Description | Status |
|-------|-------------|--------|
| 0-7.5 | MVP (Auth, Docs, Search, Runbooks, Editor) | ✅ |
| 8 | Attachments + Evidence Bundles | ✅ |
| 9 | Templates + Checklists | ✅ |
| 10 | CMDB-lite + Live Blocks (systems/vendors/contacts/circuits) | ✅ |
| 11 | Known Issues + Incidents | ✅ |
| 12 | Doc Health Dashboard (stale, unowned, health %) | ✅ |
| Security | CSRF, rate limiting, sensitivity gating | ✅ |
| UX A-D | Mobile, flash, breadcrumbs, editor polish | ✅ |
| Tests | Tenant isolation, role gating, XSS, conflict, audit (33 tests) | ✅ |
| Admin | User management, audit viewer, tenant settings | ✅ |
| Landing | Public landing page + improved dashboard | ✅ |
| Doc Ops | Rename/move/delete documents | ✅ |

**Migrations**: 001-006 all applied. DB at version 6.

---

## Remaining: Polish

- [ ] Drag-and-drop file upload on attachments/edit pages
- [ ] HTMX folder tree navigation (`GET /tree?folder=...`)
- [ ] Image/PDF preview before download
- [ ] Quick-edit doc metadata from view page (inline HTMX)

## Remaining: Phase 13 — Sites

- [ ] Sites table (client → sites relationship)
- [ ] Optional site scoping for CMDB objects
- [ ] Site filtering in directory views
- [ ] Migration 007
