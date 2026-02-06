# Docstor Implementation Progress

## Current Status: Phase 4 Complete

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

### Phase 5 - Editor Island (Not Started)
- CodeMirror 6 on edit pages (with textarea fallback)
- HTMX preview endpoint improvements

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
