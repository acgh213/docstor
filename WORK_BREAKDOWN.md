# Docstor Work Breakdown

## Completed

| Phase | Description | Status |
|-------|-------------|--------|
| 0-7.5 | MVP (Auth, Docs, Search, Runbooks, Editor) | ✅ |
| 8 | Attachments + Evidence Bundles | ✅ |
| Security | CSRF, rate limiting, sensitivity gating | ✅ |
| UX A | Critical fixes (mobile, attachments, forms) | ✅ |
| UX B | Flash messages, sidebar active, search unification | ✅ |
| UX C | Breadcrumbs, tables, empty states, loading, favicon | ✅ |
| UX D | Editor highlighting, code blocks, shortcuts | ✅ |

---

## Remaining: Testing (claude.md §13)

Estimate: 3-4 hours

- [ ] Tenant isolation tests (cross-tenant data leak prevention)
- [ ] Role gating tests (reader can't edit, editor can, admin manages roles)
- [ ] Revision conflict detection tests
- [ ] Revert creates new revision tests
- [ ] Markdown XSS regression tests
- [ ] Audit log write assertions
- [ ] Rate limiter tests (already done: internal/auth/ratelimit_test.go)

---

## Remaining: Feature Phases

### Phase 9 — Templates + Checklists (3-4 hours)
- Template library (reusable doc/runbook templates)
- Checklist items with trackable instances
- Schema: templates, checklists, checklist_items, checklist_instances

### Phase 10 — CMDB-lite + Live Blocks (4-6 hours)
- Systems/vendors/contacts/circuits tables
- `{{system:123}}` shortcodes in markdown (server-side render)
- Directory browse UI

### Phase 11 — Known Issues + Incidents (3-4 hours)
- Known issues board (open/investigating/resolved)
- Incident timeline with events
- Link to docs

### Phase 12 — Doc Health Dashboards (2-3 hours)
- Stale docs (updated_at > 90 days)
- Unowned docs (owner_user_id IS NULL)
- Broken internal links detection

---

## Remaining: Minor Gaps

- [ ] Doc rename/move/delete handlers
- [ ] HTMX folder tree navigation (`GET /tree?folder=...`)
- [ ] Drag-and-drop file upload
- [ ] Image/PDF preview before download
- [ ] Quick-edit doc metadata from view page
