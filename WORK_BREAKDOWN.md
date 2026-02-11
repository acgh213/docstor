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
| 13 | Sites (client → sites) | ✅ |
| Security | CSRF, rate limiting, sensitivity gating | ✅ |
| UX A-D | Mobile, flash, breadcrumbs, editor polish | ✅ |
| Polish | Drag-drop upload, folder tree, image preview, metadata edit | ✅ |
| Tests | Tenant isolation, role gating, XSS, conflict, audit (33 tests) | ✅ |
| Admin | User management, audit viewer, tenant settings | ✅ |
| Landing | Public landing page + improved dashboard | ✅ |
| Doc Ops | Rename/move/delete documents | ✅ |

**Migrations**: 001-007 all applied. DB at version 7.

---

## Optional: Hardening Phase

- [ ] S3-compatible storage backend
- [ ] PDF packet export
- [ ] Break-glass access for confidential docs
- [ ] Tamper-evident audit (hash chain)
