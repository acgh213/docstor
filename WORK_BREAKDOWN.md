# Docstor Work Breakdown

## Completed

| Phase | Description | Status |
|-------|-------------|--------|
| 0-7.5 | MVP (Auth, Docs, Search, Runbooks, Editor) | ✅ |
| 8 | Attachments + Evidence Bundles | ✅ |

---

## Remaining Feature Phases

### Phase 9: Templates + Checklists
**Effort**: Medium (3-4 hours)

**Database**:
- `templates` table
- `checklists` + `checklist_items` tables
- `checklist_instances` + `checklist_instance_items` tables

**Features**:
- [ ] Template library CRUD
- [ ] Create doc from template
- [ ] Checklist library CRUD
- [ ] Attach checklist to document
- [ ] Track checklist completion

**Routes**: 6-8 new routes

---

### Phase 10: CMDB-lite + Live Blocks
**Effort**: Medium-High (4-6 hours)

**Database**:
- `systems` table
- `vendors` table  
- `contacts` table
- `circuits` table

**Features**:
- [ ] Systems CRUD (servers, firewalls, switches, apps)
- [ ] Vendors CRUD with contact info
- [ ] Contacts CRUD
- [ ] Circuits CRUD
- [ ] `{{system:id}}` shortcodes in markdown
- [ ] `{{vendor:id}}` shortcodes
- [ ] `{{circuit:id}}` shortcodes

**Routes**: 12-16 new routes

---

### Phase 11: Known Issues + Incidents
**Effort**: Medium (3-4 hours)

**Database**:
- `known_issues` table
- `incidents` table
- `incident_events` table

**Features**:
- [ ] Known issues board (kanban-style)
- [ ] Link issues to documents
- [ ] Incident timeline view
- [ ] Add events to incidents
- [ ] Link incidents to clients

**Routes**: 8-10 new routes

---

### Phase 12: Doc Health Dashboards
**Effort**: Low-Medium (2-3 hours)

**Database**:
- `doc_links` table (parsed from markdown)
- Optional: `doc_views` for tracking

**Features**:
- [ ] Stale docs report (>90 days)
- [ ] Unowned docs report
- [ ] Broken links detection
- [ ] Health dashboard page

**Routes**: 2-3 new routes

---

## UX/UI Improvements Needed

### Critical Issues
1. **No document link to attachments** - Can't get to attachments from doc view
2. **Bundle add file UX** - Requires manual UUID entry (bad)
3. **No breadcrumbs consistency** - Some pages have them, some don't
4. **Mobile responsiveness** - Sidebar doesn't collapse
5. **No loading states** - Forms submit without feedback

### Navigation Issues
1. Sidebar doesn't show active state
2. No way to navigate back from nested pages
3. Client filter dropdown does nothing
4. Search in topbar vs search page confusion

### Form/Action Issues
1. No confirmation on destructive actions
2. No success messages after actions
3. Upload progress not shown
4. No drag-and-drop for files

### Visual Polish
1. Inconsistent card styling
2. Tables need better styling
3. Empty states are plain
4. No favicon
5. Buttons need hover states

### Missing Features
1. Can't edit document metadata from view page
2. Can't see who uploaded an attachment
3. No file preview (images, PDFs)
4. No keyboard shortcuts

---

## Recommended Priority

### Immediate (UX Critical)
1. Add "Attachments" link to document view page
2. Fix bundle file addition (use file picker, not UUID)
3. Add sidebar active state
4. Add success/error flash messages

### Short-term (Polish)
1. Consistent breadcrumbs
2. Mobile responsive sidebar
3. Loading states on forms
4. Better empty states

### Medium-term (Features)
1. Phase 9: Templates (high user value)
2. Phase 12: Doc Health (low effort, high visibility)
3. Phase 10: CMDB-lite (MSP core feature)
4. Phase 11: Incidents (can wait)

---

## UX Test Plan

### Test Scenarios
1. **New User Flow**: Login → Create doc → Edit → Verify runbook
2. **Attachment Flow**: Upload file → Link to doc → Add to bundle → Export
3. **Search Flow**: Find doc by content → Navigate to it
4. **Client Filtering**: Filter docs by client
5. **Mobile Test**: All flows on narrow viewport

### Pages to Screenshot
- [ ] Login
- [ ] Dashboard
- [ ] Docs list
- [ ] Doc view
- [ ] Doc edit
- [ ] Doc history
- [ ] Runbooks dashboard
- [ ] Clients list
- [ ] Client view
- [ ] Search results
- [ ] Evidence bundles list
- [ ] Bundle view
- [ ] Document attachments

### Metrics to Check
- Page load times
- Mobile layout breakpoints
- Form validation feedback
- Error handling
- Empty states
