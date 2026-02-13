# Docstor

**documentation for MSPs that doesn’t feel like punishment**

Docstor is a self-hosted, server-rendered documentation system built specifically for managed service providers.

no electron wrapper  
no SaaS ransom  
no “search is an enterprise feature” nonsense  

just markdown documents, living runbooks with verification tracking, and an audit trail that actually behaves like an audit trail.

it’s built by someone who has been on-call at 2am trying to find firewall rules buried in a SharePoint mausoleum. we’re not doing that again.

![Go](https://img.shields.io/badge/Go-1.22-00ADD8?style=flat&logo=go)  
![Postgres](https://img.shields.io/badge/Postgres-16-4169E1?style=flat&logo=postgresql)  
![License](https://img.shields.io/badge/license-MIT-green)  
![vibes](https://img.shields.io/badge/vibes-immaculate-ff69b4)

---

## why this exists

MSP documentation tools usually fall into one of three buckets:

- expensive SaaS that slowly converts your data into leverage  
- a wiki from 2008 held together by muscle memory  
- Confluence (no further comment)

Docstor is none of those.

It’s a Go binary.  
It uses Postgres.  
It renders HTML on the server.  

It loads fast. It searches fast. It doesn’t decide your session expired while you were mid-edit.

That’s the bar.

---

## what it does

### the solid, already-shipped stuff

- **multi-tenant** — actual isolation. not “we forgot a WHERE clause.”
- **immutable revisions** — every save is permanent. reverting creates a new revision.
- **server-side markdown rendering** — goldmark + bluemonday. sanitized.
- **full-text search** — Postgres FTS. filterable and scoped by client, type, owner, recency.
- **living runbooks** — verification cadence tracking with an overdue dashboard.
- **clients + sites** — first-class entities. client → site hierarchy.
- **CMDB-lite** — configuration items linked properly with shortcode references.
- **attachments** — stored on disk, metadata in Postgres.
- **templates + checklists** — reusable structure with tracked instances.
- **incidents + known issues**
- **change records**
- **doc links + backlinks** — internal link detection and broken link reporting.
- **CodeMirror 6 editor** — with vim mode.
- **HTMX for the interactive bits** — previews, inline edits, tree expansion.
- **append-only audit log** — immutable logging of meaningful actions.
- **role-based access** — TenantAdmin / Editor / Reader.
- **CSRF protection** — works with HTMX.

---

## scale snapshot

- ~18k lines of Go (including ~4k tests)
- ~6k lines of HTML templates
- 17 test files covering tenant isolation, role gating, XSS regression, revision conflicts
- 9 migrations
- 51 commits
- 0 npm dependencies at runtime

the editor bundle is built separately and committed. runtime stays clean.

---

## stack

**Language:** Go 1.22  
Compiles fast. Single binary.

**HTTP:** chi  
Lightweight routing on top of net/http.

**Database:** Postgres 16  
FTS, JSONB, stable. No ORM. Just SQL.

**Migrations:** golang-migrate  
Embedded. Runs on startup.

**Templates:** html/template  
Server-rendered. Works without JavaScript.

**CSS:** custom and minimal  
No Tailwind. No Bootstrap.

**Markdown:** goldmark + bluemonday  
Rendered server-side. Sanitized.

**Editor:** CodeMirror 6  
Loaded only on edit pages.

**HTMX:**  
Used for previews and inline interactions.

**Auth:**  
bcrypt + secure cookies. Sessions stored in DB. Rate limiting on login.

---

## quickstart

### prerequisites

- Go 1.22+
- Docker + Docker Compose (for Postgres)

### run

```bash
git clone https://github.com/acgh213/docstor.git
cd docstor

cp .env.example .env

make dev

run
```


App runs at http://localhost:8080.
A seed user is created on first run.
```bash
build

make build
```
Produces ./bin/docstor.
```bash
test

make test
```
Tests use a real Postgres instance via Docker. Mocking your database mostly just tests your imagination.


---

# project structure

cmd/docstor/         → entrypoint

internal/
  auth/              → sessions, passwords, middleware
  db/                → connection + embedded migrations
  docs/              → documents, revisions, markdown, diffs
  runbooks/          → verification tracking
  clients/           → client CRUD
  sites/             → site hierarchy
  cmdb/              → configuration items
  attachments/       → file storage + metadata
  templates/         → templates + checklist instances
  incidents/         → incidents + known issues
  changes/           → change records
  doclinks/          → link extraction + backlinks
  audit/             → append-only logger
  pagination/        → shared helpers
  web/               → handlers, router, rendering
  config/            → env config
  testutil/          → shared test helpers

web/
  templates/         → layout + UI
  static/            → CSS + minimal JS

editor-bundle/       → CodeMirror build
migrations/          → SQL reference copies


---

# decisions i will defend calmly but firmly

No SPA.
Server-rendered HTML is faster, simpler, and doesn’t require inventing a state problem to justify a framework.

No ORM.
SQL is explicit. Queries are visible. Tenant scoping is enforced deliberately. Performance problems are found in review, not in production at 2am.

Immutable revisions.
History is append-only. Reverts create new revisions. Audit logs are not editable. If something changed, we can see exactly what and when.

No secret storage.
This is documentation. Store passwords in a vault. We store references, not credentials.

Tenant isolation everywhere.
Every query. Every handler. Every test.
Cross-tenant leakage is a failure condition.


---

# contributing

It’s early and opinionated. PRs welcome if you align with the philosophy.

Read claude.md. It’s the build contract. It is not optional.

If you open an issue suggesting a React rewrite, I will close it with enthusiasm.


---

# license

MIT.

Do what you want.

Just don’t deploy it without changing the session keys in .env and then blame me when you invent chaos.


---

built with stubbornness, caffeine, and the belief that documentation should be infrastructure — not a SaaS funnel ✨
