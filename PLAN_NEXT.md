# Docstor — Next Phase Plan

This plan covers every deferred, unfinished, or under-built area identified by a
full audit of the codebase against the `claude.md` contract and `plan.md`
roadmap. Items are grouped by priority tier.

---

## Tier 1 — Contract Compliance & Reliability

These are gaps where the codebase either doesn't meet the `claude.md` contract
or where delivered features lack the safety net to be trusted in production.

### 1.1 Test Coverage Expansion

**Current state:** 25 tests across 5 files. Security-critical paths (tenant
isolation, RBAC, XSS, conflict detection, rate limiting) are well-tested.
~75% of production packages have zero test coverage.

**Required work:**

| Area | Tests needed | Priority |
|------|-------------|----------|
| **Audit actions** | Verify that doc create/edit/save, doc rename/move/delete, runbook verify, client create/update, membership changes, attachment upload each produce the expected audit_log row | High |
| **Doc rename/move/delete** | Happy path + role gating (reader blocked) + audit entry created | High |
| **Attachments** | Upload creates record + storage file; unlink; download audited; SHA256 dedup works; tenant isolation | Medium |
| **Evidence bundles** | Create, add/remove item, export ZIP, delete; tenant isolation | Medium |
| **Templates** | CRUD; create-doc-from-template produces correct initial revision | Medium |
| **Checklists** | Create checklist; start instance; toggle item; auto-complete on all-done; tenant isolation | Medium |
| **CMDB** | System/vendor/contact/circuit CRUD; tenant isolation; role gating (reader can't create) | Medium |
| **Known issues + incidents** | CRUD; incident event append-only; tenant isolation | Medium |
| **Sites** | CRUD; tenant isolation; client scoping | Medium |
| **Shortcode rendering** | Valid shortcode renders block; missing entity renders warning; XSS in entity fields is escaped | Medium |
| **Search filters** | Filter by client, doc_type, recency; results respect sensitivity gating | Low |
| **Session lifecycle** | Login creates session; logout destroys it; expired session rejected | Low |

Estimate: 3–4 focused sessions. Use existing `testutil` infrastructure (real Postgres, parallel-safe fixtures).

### 1.2 Missing Audit Log Entries

**Current state:** 50 of 52 write handlers are audited.

| Handler | Action | Fix |
|---------|--------|-----|
| `handleLogout` | Session destroyed | Add `audit.ActionLogout` |
| `handleRunbookUpdateInterval` | Verification interval changed | Add `audit.ActionRunbookIntervalUpdate` |

Estimate: 30 minutes.

---

## Tier 2 — Incomplete Feature Wiring

Features where the schema exists but the Go code doesn't use it, or where
the plan described a capability that was never connected.

### 2.1 CMDB ↔ Sites Relationship

**Current state:** Migration 007 added `site_id` columns to `systems`,
`contacts`, and `circuits` tables. The CMDB Go code (`cmdb.go`) and all
CMDB handlers/templates have **zero references to `site_id`**. The columns
exist in Postgres but are always NULL.

**Required work:**
- Add `SiteID` field to `System`, `Contact`, `Circuit` structs
- Include `site_id` in INSERT/UPDATE/SELECT queries
- Add site dropdown to system/contact/circuit create/edit forms
- Add site filter to CMDB list pages
- Show linked site on CMDB detail views
- Filter CMDB entities on site detail view

Estimate: 2–3 hours.

### 2.2 Doc Links / Backlinks / Broken Links

**Current state:** `plan.md` describes a `doc_links` table and backlinks
feature. **Not implemented** — no table, no code, no UI. The `claude.md`
contract does not require this for MVP, but `plan.md` Phase 11 (Doc Health)
mentioned "broken links list" as a dashboard item.

**Required work (if approved):**
- Migration: `doc_links(id, tenant_id, from_document_id, to_document_id, link_path, created_at)`
- On doc save: parse markdown for internal links, rebuild `doc_links` rows
- Doc read view: "Backlinks" section showing docs that link to this one
- Doc health dashboard: "Broken links" list (links pointing to non-existent paths)
- Consider: relative link resolution, path normalization

Estimate: 4–6 hours.

### 2.3 Change Records

**Current state:** Described in `plan.md` with full schema (changes table,
rollback/validation plans, risk levels). Explicitly deferred. Not in `claude.md`
MVP scope.

**Required work (if approved):**
- Migration: `changes(id, tenant_id, title, risk_level, status, client_id, window_start, window_end, rollback_plan_markdown, validation_plan_markdown, created_by, created_at)`
- Handlers: CRUD + status transitions
- Templates: list, form, detail view
- Link changes to docs/runbooks, checklist instances, evidence bundles
- Audit logging for all writes

Estimate: 6–8 hours.

---

## Tier 3 — Robustness & Operational Quality

These are patterns that work at small scale but will cause problems as usage
grows.

### 3.1 Pagination

**Current state:** No pagination anywhere. All list operations return
unbounded results (except attachments capped at LIMIT 200).

**Affected endpoints (every list page):**
- `/docs` — document list
- `/search` — search results
- `/clients` — clients list
- `/systems`, `/vendors`, `/contacts`, `/circuits` — CMDB lists
- `/known-issues`, `/incidents` — issue/incident lists
- `/templates`, `/checklists`, `/checklist-instances` — template/checklist lists
- `/evidence-bundles` — bundle list
- `/admin/audit` — audit log
- `/admin/users` — user list
- `/docs/id/{id}/history` — revision history
- `/sites` — sites list

**Approach:**
- Add a shared `Pagination` struct (page, per_page, total, has_next)
- Add `LIMIT $x OFFSET $y` + `COUNT(*)` to repo list methods
- Add prev/next controls to list templates
- Default per_page: 50 (25 for audit log)

Estimate: 4–6 hours (repetitive but touches many files).

### 3.2 N+1 Query Patterns

**Current state:** Two known N+1 patterns:
1. `checklists.ListInstancesForDoc` — queries doc title inside a loop
2. `cmdb/shortcodes.go` — hits DB once per shortcode occurrence in a document

**Fix for shortcodes:**
- Pre-scan document for all shortcode UUIDs
- Batch-load all referenced entities in 4 queries (one per type)
- Render from the pre-loaded map

**Fix for checklists:**
- JOIN doc title in the list query

Estimate: 2 hours.

### 3.3 Attachment Storage Hardening

**Current state:** Local filesystem only. `ComputeSHA256` reads entire file
into memory. No size limits. No `DeleteAttachment` method (orphaned files
possible). No garbage collection.

**Required work:**
- Add upload size limit (e.g., 50MB) enforced in handler
- Stream SHA256 computation instead of `io.ReadAll`
- Add `DeleteAttachment` (with cascade: delete file if no other links reference it)
- Add a cleanup job for orphaned storage files (optional)
- Document the `Storage` interface for future S3 implementation

Estimate: 2–3 hours.

---

## Tier 4 — Hardening (Optional, from WORK_BREAKDOWN.md)

These were always listed as optional. Include only if pursuing production
readiness for external users.

| Item | Description | Estimate |
|------|-------------|----------|
| **S3 storage backend** | Implement `Storage` interface for S3-compatible backends (MinIO/AWS/Wasabi) | 4–6h |
| **PDF packet export** | Select docs/runbooks → generate PDF bundle for audits/handoffs | 6–8h |
| **Break-glass access** | Require ticket/reason for confidential doc access; log reason in audit | 3–4h |
| **Tamper-evident audit** | Hash chain on audit_log (each entry includes hash of previous entry) | 2–3h |

---

## Recommended Execution Order

```
Phase A (reliability):     1.2 → 1.1 (partial: audit + doc ops + core features)
Phase B (completeness):    2.1 → 3.2 → 3.3
Phase C (scale):           3.1
Phase D (completeness):    1.1 (remaining tests) → 2.2
Phase E (if approved):     2.3 (change records)
Phase F (hardening):       Tier 4 items as needed
```

Total estimate for Tiers 1–3: **~20–30 hours of implementation work.**

---

## What's Done and Solid

For reference, these areas are complete and well-built:

- ✅ Auth + tenancy + sessions + CSRF + rate limiting
- ✅ Documents full lifecycle (create/edit/save/rename/move/delete)
- ✅ Immutable revisions + conflict detection + revert + diff
- ✅ Markdown rendering + sanitization (XSS-tested)
- ✅ Search (Postgres FTS)
- ✅ Runbooks (verify workflow + overdue dashboard)
- ✅ Audit logging (50/52 handlers covered)
- ✅ Clients CRUD
- ✅ Attachments + evidence bundles
- ✅ Templates + checklists
- ✅ CMDB (4 entity types + shortcode rendering)
- ✅ Known issues + incidents (with event timeline)
- ✅ Sites (CRUD + client association)
- ✅ Doc health dashboard
- ✅ Admin panel (users, audit viewer, settings)
- ✅ UX polish (mobile, flash, breadcrumbs, folder tree, drag-drop)
- ✅ CodeMirror 6 editor with server preview
