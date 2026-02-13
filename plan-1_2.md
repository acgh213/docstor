# Docstor Plan 1.2 — All Phases Approved

All tiers from PLAN_NEXT.md are approved. This is the execution plan.

---

## Phase A — Audit Gaps + Test Foundation

### A-1: Fix 2 missing audit entries
- [ ] `handleLogout` → `audit.ActionLogout`
- [ ] `handleRunbookUpdateInterval` → `audit.ActionRunbookIntervalUpdate`

### A-2: Audit action integration tests
Verify that specific write operations produce the expected audit_log rows:
- [ ] doc create → `doc.create`
- [ ] doc edit/save → `doc.edit`
- [ ] doc rename → `doc.move`
- [ ] doc delete → `doc.delete`
- [ ] doc revert → `doc.revert`
- [ ] runbook verify → `runbook.verify`
- [ ] client create → `client.create`
- [ ] client update → `client.update`
- [ ] membership add → `membership.add`
- [ ] membership edit → `membership.edit`
- [ ] membership delete → `membership.delete`
- [ ] login success → `login.success`
- [ ] login failure → `login.failed`
- [ ] logout → `logout`

### A-3: Doc rename/move/delete tests
- [ ] Rename happy path (editor can rename, path updates, audit logged)
- [ ] Delete happy path (editor can delete, audit logged)
- [ ] Reader cannot rename or delete (403)

### A-4: Post-MVP feature tests (core CRUD + tenant isolation)
- [ ] Attachments: upload, unlink, download audit, SHA256 dedup, tenant isolation
- [ ] Evidence bundles: create, add/remove item, export, delete, tenant isolation
- [ ] Templates: CRUD, create-doc-from-template, tenant isolation
- [ ] Checklists: create, start instance, toggle item, auto-complete, tenant isolation
- [ ] CMDB (systems/vendors/contacts/circuits): CRUD, tenant isolation, role gating
- [ ] Known issues: CRUD, tenant isolation
- [ ] Incidents: CRUD, event append, tenant isolation
- [ ] Sites: CRUD, client scoping, tenant isolation
- [ ] Shortcodes: valid renders, missing entity warning, XSS escaped
- [ ] Search: filters by client/doc_type, respects sensitivity gating

---

## Phase B — Incomplete Feature Wiring

### B-1: CMDB ↔ Sites relationship
- [ ] Add SiteID to System/Contact/Circuit structs
- [ ] Update INSERT/UPDATE/SELECT queries in cmdb.go
- [ ] Add site dropdown to create/edit forms
- [ ] Add site filter to CMDB list pages
- [ ] Show linked site on CMDB detail views
- [ ] Filter CMDB entities on site detail view

### B-2: Fix N+1 query patterns
- [ ] Shortcodes: batch-load entities (pre-scan UUIDs, 4 bulk queries)
- [ ] Checklists: JOIN doc title in list query

### B-3: Attachment storage hardening
- [ ] Upload size limit (50MB) in handler
- [ ] Stream SHA256 instead of io.ReadAll
- [ ] Add DeleteAttachment with cascade (delete file if no links remain)
- [ ] Orphan cleanup utility (optional CLI command)

---

## Phase C — Pagination

- [ ] Shared Pagination struct (page, per_page, total, has_next, has_prev)
- [ ] Shared pagination partial template (prev/next + page info)
- [ ] Add to all list endpoints:
  - /docs, /search, /clients, /sites
  - /systems, /vendors, /contacts, /circuits
  - /known-issues, /incidents
  - /templates, /checklists, /checklist-instances
  - /evidence-bundles
  - /admin/audit, /admin/users
  - /docs/id/{id}/history
- [ ] Default 50 per page (25 for audit log)

---

## Phase D — Doc Links & Remaining Tests

### D-1: Doc links / backlinks / broken links
- [ ] Migration: doc_links(id, tenant_id, from_document_id, to_document_id, link_path, created_at)
- [ ] On doc save: parse markdown for internal links, rebuild doc_links
- [ ] Doc read view: backlinks section
- [ ] Doc health dashboard: broken links list
- [ ] Path normalization for relative links

### D-2: Remaining test coverage
- [ ] Session lifecycle (login creates, logout destroys, expired rejected)
- [ ] CSRF integration tests
- [ ] Edge cases identified during earlier phases

---

## Phase E — Change Records

- [ ] Migration: changes table (title, risk_level, status, client_id, window_start/end, rollback_plan_markdown, validation_plan_markdown)
- [ ] Handlers: CRUD + status transitions (draft→approved→in_progress→completed/rolled_back)
- [ ] Templates: list, form, detail view
- [ ] Link changes to docs/runbooks, checklist instances, evidence bundles
- [ ] Audit logging for all writes
- [ ] Tests: CRUD, tenant isolation, role gating

---

## Phase F — Hardening

- [ ] S3-compatible storage backend (implement Storage interface)
- [ ] PDF packet export (select docs → generate PDF bundle)
- [ ] Break-glass access (reason required for confidential docs, logged)
- [ ] Tamper-evident audit (hash chain on audit_log entries)

---

## Progress Tracking

| Phase | Status | Started | Completed |
|-------|--------|---------|-----------|
| A-1   | ✅     | 2026-02-13 | 2026-02-13 |
| A-2   | ✅     | 2026-02-13 | 2026-02-13 |
| A-3   | ✅     | 2026-02-13 | 2026-02-13 |
| A-4   | ✅     | 2026-02-13 | 2026-02-13 |
| B-1   | ✅     | Feb 13  | Feb 13    |
| B-2   | ✅     | Feb 13  | Feb 13    |
| B-3   | ✅     | Feb 13  | Feb 13    |
| C     | ✅     | Feb 13  | Feb 13    |
| D-1   | ✅     | Feb 13  | Feb 13    |
| D-2   | ✅     | Feb 13  | Feb 13    |
| E     | ✅     | Feb 13  | Feb 13    |
| F     |        |         |           |
