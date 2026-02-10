# Docstor Implementation Progress

## Current Status: Phase 5 Complete (Partial)

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

### Phase 5 - Editor Island ✅ (Partial)
**Status**: Complete (enhanced textarea, CodeMirror deferred)

**Implemented:**
- Enhanced textarea with monospace font
- localStorage draft saving with recovery
- Tab key indentation support
- HTMX preview already worked from Phase 3

**Deferred:**
- Full CodeMirror 6 integration (requires bundled build)
- Vim keybindings (requires CodeMirror)

### Phase 6 - Search (Not Started)
- Full-text search using Postgres tsvector
- Search UI with filters (client, doc_type, owner)

### Phase 7 - Living Runbooks (Not Started)
- Runbook verification workflow
- Overdue runbooks dashboard
- Verification interval tracking

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

## Phase 6 Implementation Notes (Search)

### Approach
- Add tsvector column to documents table for full-text search
- Index title, path, and current revision body
- Create search endpoint with filters (client, doc_type)
- Search results page with highlighting

### Migration Plan
1. Add `search_vector` tsvector column to documents
2. Create GIN index on search_vector
3. Add trigger to update search_vector on document/revision changes
4. Implement search repository method
5. Add search handler and template

---

## Phase 7 Implementation Notes (Living Runbooks)

### Approach
- runbook_status table already exists in schema
- Need to implement:
  - Runbook verification workflow (POST /docs/id/{id}/verify)
  - Runbook dashboard (/runbooks) showing overdue, unowned
  - UI for setting verification interval
  - Display verification status on doc read page for runbooks

### Key Features
1. Verify action stamps last_verified_at and computes next_due_at
2. Dashboard shows:
   - Overdue runbooks (next_due_at < NOW())
   - Unowned runbooks (owner_user_id IS NULL)
   - Recently verified
3. Verification interval setting in edit/metadata
