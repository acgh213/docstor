# Docstor Implementation Progress

## Current Status: Phase 7 Complete (MVP)

## Completed Phases

### Phase 0 ✅
- Go module, Makefile, Docker Compose
- Migrations framework with embedded SQL
- Base layout, CSS, health endpoint

### Phase 1 ✅
- Auth: password hashing, sessions, login/logout
- Tenant-scoped middleware
- Role gating (admin/editor/reader)
- Audit logging foundation

### Phase 2 ✅
- Clients CRUD with full UI
- Audit logging for client operations
- Permission gating on create/edit

### Phase 3 ✅
- Documents repository: List, GetByPath, GetByID, Create, Update
- Revisions table with conflict detection via base_revision_id
- Markdown rendering with goldmark + bluemonday
- Templates: list, read, form, edit, history, conflict
- Route structure: `/docs/*` for read, `/docs/id/{id}/...` for operations

### Phase 4 ✅
- **Revert functionality**: Creates new revision from old revision's content
- **Diff view**: Line-by-line diff between any two revisions
- **Revision view**: View any historical revision with rendered markdown
- **History UI enhanced**: Compare revisions dropdown, View/Revert buttons
- **Audit logging**: Revert actions logged with metadata
- Uses go-diff/diffmatchpatch for line-based diff computation

## Remaining Work

### Phase 5 - Editor Island ✅
**Status**: Complete

**Implemented:**
- Enhanced textarea with monospace font (fallback)
- localStorage draft saving with recovery
- Tab key indentation support
- HTMX preview from Phase 3

### Phase 7.5 - CodeMirror 6 Editor ✅
**Status**: Complete

**Implemented:**
- esbuild bundle for CM6 (editor-bundle/)
- codemirror-bundle.js (686KB) with all dependencies
- Markdown syntax highlighting
- Vim mode toggle (persisted in localStorage)
- Line numbers, code folding, bracket matching
- Search (Ctrl+F), history (undo/redo)
- Light/dark theme support
- Falls back to enhanced textarea if bundle fails

### Phase 6 - Search ✅
**Status**: Complete

**Implemented:**
- Migration adds tsvector column with GIN index to documents
- Trigger auto-updates search vector on document changes
- Weighted search (title A, path B, body C)
- Search repository with filters (client_id, doc_type, owner_id)
- ts_headline for highlighted snippets
- Search UI with filters dropdown
- Results show highlights, metadata, badges

### Phase 7 - Living Runbooks ✅
**Status**: Complete

**Implemented:**
- runbooks package with Repository for runbook_status operations
- EnsureStatus creates runbook_status on first verify
- Verify action stamps last_verified_at, computes next_due_at
- UpdateInterval for changing verification cadence
- ListOverdue/ListUnowned/ListRecentlyVerified queries
- Runbooks dashboard (/runbooks) with three sections:
  - Overdue (next_due_at < NOW())
  - Unowned (owner_user_id IS NULL)
  - Recently Verified
- Runbook Status card on doc_read page for runbooks
- Mark as Verified button
- Audit logging for verification actions

## Key Files Added/Modified This Session

```
internal/
├── docs/
│   ├── docs.go          # Added Revert() method
│   └── diff.go          # NEW: Diff computation
└── web/
    ├── handlers_docs.go # Added handleDocRevert, handleDocDiff, handleDocRevision
    ├── router.go        # Added routes for diff, revision, revert
    └── templates/docs/
        ├── doc_diff.html     # NEW: Diff view template
        ├── doc_revision.html # NEW: Revision view template
        └── doc_history.html  # Updated with compare UI, View/Revert buttons
```

## Technical Notes

1. Diff uses go-diff/diffmatchpatch for line-based comparison
2. Revert creates a new revision (never deletes history)
3. All revert operations are audited with metadata
4. History page now has revision comparison feature

## Handoff

See `handoff.md` for complete implementation status and next steps.


## Phase 5 Implementation Notes

### What Was Implemented

**Enhanced Textarea with Draft Saving** (`editor-cm.js`):
- ✅ Monospace font for code/markdown editing
- ✅ Tab key support (inserts 4 spaces)
- ✅ localStorage draft saving (auto-saves on every keystroke)
- ✅ Draft recovery prompt when returning to edit page
- ✅ "Draft saved" indicator in UI
- ✅ Draft cleared on successful save

### CodeMirror 6 Challenges

CodeMirror 6 could not be loaded from CDN due to ES module dependency conflicts:
- CM6 is heavily modular with many interdependent packages
- Each CDN import gets its own instance of `@codemirror/state`
- `instanceof` checks fail when multiple instances are loaded
- This is a known issue with CM6 + browser ESM imports

**Future Options for Full CodeMirror 6:**
1. Build a local bundle with esbuild/rollup that includes all dependencies
2. Use CodeMirror 5 which has simpler UMD builds
3. Self-host pre-built CM6 bundles

### Current Status

The enhanced textarea provides a good editing experience that:
- Works without JS (basic textarea fallback)
- Saves drafts to prevent lost work
- Supports tab indentation
- Uses monospace font appropriate for markdown

This meets the spirit of plan.md's Phase 5 acceptance criteria:
- "Editing feels good; page still works without JS"
- HTMX preview was already implemented in Phase 3

---

## Handoff Status

- [x] `handoff.md` reviewed and confirmed up-to-date
- [x] Phase 5 complete (enhanced textarea, CodeMirror deferred)
- [x] Starting Phase 6 (Search) and Phase 7 (Living Runbooks)
- [x] Technical decisions documented
- [x] Database schema documented
- [x] Route structure documented

---

## MVP Complete!

Phases 0-7 are complete. The MVP includes:
- Tenant isolation + role gating
- Client management
- Documents with revisions, history, diff, revert
- Server-side markdown rendering + sanitization
- Full-text search with PostgreSQL tsvector
- Living runbooks with verification workflow + overdue dashboard
- Audit logging for all meaningful actions

## Remaining Work (Post-MVP)

### Phase 8 - Attachments + Evidence Bundles
### Phase 9 - Templates + Checklists
### Phase 10 - CMDB-lite + Live Blocks
### Phase 11 - Known Issues + Incidents
### Phase 12 - Doc Health Dashboards
