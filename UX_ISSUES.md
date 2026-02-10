# Docstor UX/UI Issues & Recommendations

## Test Date: Feb 10, 2026

---

## Critical Issues (Must Fix)

### 1. No Attachments Link on Document View
**Location**: Document view page  
**Issue**: Users can't access document attachments without manually typing the URL  
**Fix**: Add "Attachments" button next to Edit/History buttons

### 2. Evidence Bundle Add File UX
**Location**: `/evidence-bundles/{id}` - "Add File to Bundle" form  
**Issue**: Requires manually entering a UUID - users won't know this  
**Fix**: Either:
  - Add file picker that shows recent/all attachments
  - Add "Add to Bundle" button on document attachments page
  - Show attachment IDs on attachment pages for copy

### 3. Mobile Layout Broken
**Location**: All pages on viewport < 768px  
**Issue**: Sidebar doesn't collapse, content gets cut off  
**Fix**: 
  - Add hamburger menu for mobile
  - Make sidebar collapsible
  - Add responsive table styling

### 4. Form Submissions Go to Search
**Location**: New Document form, possibly others  
**Issue**: Some form submissions redirect to search page instead of intended destination  
**Root Cause**: Likely nested form issue with topbar search form
**Fix**: Ensure forms are not nested, add explicit form boundaries

---

## High Priority Issues

### 5. No Active State in Sidebar
**Location**: Sidebar navigation  
**Issue**: Can't tell which section you're in  
**Fix**: Add `.active` class based on current URL path

### 6. No Success/Error Flash Messages
**Location**: After form submissions  
**Issue**: No feedback when actions succeed or fail  
**Fix**: Add flash message system (session-based or query param)

### 7. Client Filter Dropdown Does Nothing
**Location**: Topbar  
**Issue**: "All Clients" dropdown has no functionality  
**Fix**: Either implement client filtering or remove the dropdown

### 8. Duplicate Search UIs
**Location**: Topbar search box vs Search page  
**Issue**: Confusing to have two search interfaces  
**Fix**: Topbar search should redirect to search page with query pre-filled

---

## Medium Priority Issues

### 9. Inconsistent Breadcrumbs
**Location**: Various pages  
**Issue**: Some pages have breadcrumbs, some don't  
**Status**:
  - ✅ Document attachments page - has breadcrumbs
  - ✅ Evidence bundle view - has breadcrumbs  
  - ❌ Docs list - no breadcrumbs
  - ❌ Edit page - no breadcrumbs
  - ❌ Clients list - no breadcrumbs
**Fix**: Add consistent breadcrumb pattern to all pages

### 10. No Loading States
**Location**: All forms  
**Issue**: No visual feedback during submission  
**Fix**: Disable submit button + show spinner on click

### 11. Table Styling Needs Work
**Location**: Docs list, Clients list, Attachments list  
**Issue**: Tables look plain, no row hover, no zebra striping  
**Fix**: Add hover states, alternating row colors, better borders

### 12. Empty States Are Plain
**Location**: Empty lists  
**Issue**: Just text, no visual appeal or CTA  
**Fix**: Add icons, better styling, prominent "Create" buttons

---

## Low Priority (Polish)

### 13. No Favicon
**Fix**: Add favicon.ico

### 14. No Page Titles Update
**Issue**: Browser tab doesn't show current page/doc name well  
**Fix**: Improve title tag content

### 15. Button Hover States
**Issue**: Some buttons lack hover feedback  
**Fix**: Add consistent hover styles

### 16. Code Block Styling
**Location**: Rendered markdown  
**Issue**: Code blocks could have better styling (syntax highlighting, copy button)  
**Fix**: Add better code block CSS, optional syntax highlighting

### 17. Date Formatting Inconsistent
**Location**: Various timestamps  
**Issue**: Some show full datetime, some just date  
**Fix**: Use consistent relative dates ("2 hours ago") or standardized format

---

## Missing Features (Consider Adding)

### 18. Keyboard Shortcuts
- `Ctrl+S` to save in editor
- `Ctrl+K` for quick search
- `Esc` to cancel/close modals

### 19. File Preview
- Image preview before download
- PDF inline viewer

### 20. Drag and Drop Upload
- Allow dragging files onto document page to attach

### 21. Bulk Operations
- Select multiple docs for bulk actions
- Bulk add attachments to bundle

### 22. Document Metadata Editing
- Can't change owner, sensitivity, type from view page
- Should have quick-edit or modal

---

## Recommended Fix Order

### Phase A: Critical Fixes (2-3 hours)
1. Add Attachments button to doc view
2. Fix mobile sidebar (add media queries + hamburger)
3. Fix form submission issue (isolate forms)

### Phase B: High Priority (2-3 hours)
4. Add sidebar active states
5. Implement flash messages
6. Fix/remove client filter dropdown
7. Unify search UX

### Phase C: Polish (2-3 hours)
8. Consistent breadcrumbs
9. Table styling improvements
10. Empty state improvements
11. Loading states

### Phase D: Nice to Have (ongoing)
12. Favicon
13. Keyboard shortcuts
14. Better code blocks
15. Date formatting

---

## CSS Variables Needed

For consistent theming, define:
```css
:root {
  --color-primary: #2563eb;
  --color-primary-hover: #1d4ed8;
  --color-danger: #dc2626;
  --color-success: #16a34a;
  --color-warning: #d97706;
  --color-text: #1f2937;
  --color-text-muted: #6b7280;
  --color-border: #e5e7eb;
  --color-bg: #f9fafb;
  --color-bg-card: #ffffff;
  --sidebar-width: 220px;
  --topbar-height: 60px;
}
```

---

## Mobile Breakpoints Needed

```css
/* Tablet */
@media (max-width: 1024px) {
  .sidebar { width: 200px; }
}

/* Mobile */
@media (max-width: 768px) {
  .sidebar { 
    position: fixed;
    transform: translateX(-100%);
    z-index: 100;
  }
  .sidebar.open { transform: translateX(0); }
  .main-content { margin-left: 0; }
  .table { display: block; overflow-x: auto; }
}
```
