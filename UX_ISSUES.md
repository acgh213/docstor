# Docstor UX/UI Issues & Recommendations

## Test Date: Feb 10, 2026
## Last Updated: Feb 10, 2026

---

## Status Summary

| Category | Total | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 4 | 4 | 0 |
| High | 4 | 4 | 0 |
| Medium | 4 | 4 | 0 |
| Low/Polish | 5 | 4 | 1 |
| Missing Features | 5 | 2 | 3 |

---

## Resolved Issues

### Critical (All Fixed)
1. ✅ **No Attachments Link on Document View** — Added Attachments button next to Edit/History (Phase A)
2. ✅ **Evidence Bundle Add File UX** — Replaced UUID input with select dropdown populated via /api/attachments (Phase A)
3. ✅ **Mobile Layout Broken** — Hamburger menu, collapsible sidebar with overlay, responsive tables (Phase A)
4. ✅ **Form Submissions Go to Search** — Isolated topbar search form, removed client filter dropdown (Phase A)

### High Priority (All Fixed)
5. ✅ **No Active State in Sidebar** — JS-based URL prefix matching with blue left border (Phase A)
6. ✅ **No Success/Error Flash Messages** — Cookie-based flash system with auto-dismiss (Phase B)
7. ✅ **Client Filter Dropdown Does Nothing** — Removed; unified with topbar search (Phase A)
8. ✅ **Duplicate Search UIs** — Topbar search redirects to /search with query (Phase A)

### Medium (All Fixed)
9. ✅ **Inconsistent Breadcrumbs** — Added to all pages: docs, clients, runbooks, attachments, bundles (Phase C)
10. ✅ **No Loading States** — Spinner on submit, double-submit prevention, 8s auto-reset (Phase C)
11. ✅ **Table Styling** — Zebra striping, row hover, responsive scroll on mobile (Phase C)
12. ✅ **Empty States** — Icons (📄🏢✅), descriptive text, prominent CTAs (Phase C)

### Low/Polish (4 of 5 Fixed)
13. ✅ **No Favicon** — Blue "D" SVG favicon (Phase C)
14. ✅ **Code Block Styling** — Dark navy background, copy button on hover, inline code pills (Phase D)
15. ✅ **Date Formatting** — Relative times ("26m ago"), dotted underline, hover for full date (Phase C)
16. ✅ **Keyboard Shortcuts** — Ctrl+K search, Ctrl+S save, Esc close (Phase C/D)
17. ⬜ **Button Hover States** — Mostly addressed via CSS transitions; some minor gaps remain

### Missing Features (2 of 5 Addressed)
18. ✅ **Keyboard Shortcuts** — Ctrl+K, Ctrl+S, Esc implemented (Phase C/D)
19. ✅ **Editor Syntax Highlighting** — Custom CM6 HighlightStyle with markdown-aware colors (Phase D)
20. ⬜ **File Preview** — Image/PDF preview before download (future)
21. ⬜ **Drag and Drop Upload** — Planned for attachments page (future)
22. ⬜ **Document Metadata Editing** — Quick-edit owner/sensitivity/type from view page (future)

---

## Security Issues (Added Post-Review, All Fixed)

23. ✅ **No CSRF Protection** — nosurf middleware with form tokens + HTMX header injection
24. ✅ **No Login Rate Limiting** — 5 attempts/60s per IP, in-memory rate limiter
25. ✅ **Sensitivity Not Enforced** — Role-based gating on restricted/confidential docs
