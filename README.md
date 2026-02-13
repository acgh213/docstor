Docstor

documentation for MSPs that doesn’t feel like punishment

Docstor is a self-hosted, server-rendered documentation system built specifically for managed service providers.

no electron wrapper
no SaaS ransom
no “search is an enterprise feature” nonsense

just markdown documents, living runbooks with verification tracking, and an audit trail that actually behaves like an audit trail.

it’s built by someone who has been on-call at 2am trying to find firewall rules buried in a SharePoint mausoleum. we’re not doing that again.







---

why this exists

MSP documentation tools usually fall into one of three buckets:

expensive SaaS that slowly converts your data into leverage

a wiki from 2008 held together by muscle memory

Confluence (no further comment)


Docstor is none of those.

It’s a Go binary.
It uses Postgres.
It renders HTML on the server.

It loads fast. It searches fast. It doesn’t decide your session expired while you were mid-edit.

That’s the bar.


---

what it does

the solid, already-shipped stuff

multi-tenant — actual isolation. not “we forgot a WHERE clause.”

immutable revisions — every save is permanent. reverting creates a new revision. auditors sleep better.

server-side markdown rendering — goldmark + bluemonday. sanitized. boring. correct.

full-text search — Postgres FTS. fast. filterable. scoped by client, type, owner, recency.

living runbooks — verification cadence tracking with an overdue dashboard so you know which ones are fiction.

clients + sites — first-class entities. client → site hierarchy for real-world MSP mess.

CMDB-lite — configuration items linked properly. shortcode references inside docs.

attachments — stored on disk, metadata in Postgres. evidence stays attached.

templates + checklists — reusable structure with tracked instances.

incidents + known issues

change records — because audits are not hypothetical.

doc links + backlinks — internal link detection and broken link reporting.

CodeMirror 6 editor — with vim mode. because yes.

HTMX for the interactive bits — previews, inline edits, tree expansion. no SPA.

append-only audit log — logins, edits, verification events, role changes. immutable.

role-based access — TenantAdmin / Editor / Reader. simple.

CSRF protection — works with HTMX. nosurf is doing honest work.



---

scale snapshot

~18k lines of Go (including ~4k tests)

~6k lines of HTML templates

17 test files covering tenant isolation, role gating, XSS regression, revision conflicts

9 migrations

51 commits of accumulated stubbornness

0 npm dependencies at runtime


the editor bundle is built separately and committed. runtime stays clean.


---

stack

Language: Go 1.22
Compiles fast. Single binary. Trusted at 3am.

HTTP: chi
Basically net/http with routing. Predictable.

Database: Postgres 16
FTS, JSONB, stability. No ORM. Just SQL.

Migrations: golang-migrate
Embedded. Runs on startup.

Templates: html/template
Server-rendered. Works without JavaScript.

CSS: custom and minimal
No Tailwind. No Bootstrap. Just restraint.

Markdown: goldmark + bluemonday
Render server-side. Sanitize everything.

Editor: CodeMirror 6
Loaded only on edit pages. One allowed JS island.

HTMX:
For previews and inline interactions. That’s it.

Auth:
bcrypt + secure cookies. Sessions stored in DB. Rate limiting on login.


---

quickstart

prerequisites

Go 1.22+

Docker + Docker Compose (for Postgres)


that’s it.

run

git clone https://github.com/acgh213/docstor.git
cd docstor

cp .env.example .env

make dev

App runs at http://localhost:8080.
A seed user is created on first run.

build

make build

Produces ./bin/docstor.

test

make test

Tests use a real Postgres instance via Docker. Mocking your database mostly just tests your imagination.


---

project structure

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

decisions i will defend calmly but firmly

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

contributing

It’s early and opinionated. PRs welcome if you align with the philosophy.

Read claude.md. It’s the build contract. It is not optional.

If you open an issue suggesting a React rewrite, I will close it with enthusiasm.


---

license

MIT.

Do what you want.

Just don’t deploy it without changing the session keys in .env and then blame me when you invent chaos.


---

built with stubbornness, caffeine, and the belief that documentation should be infrastructure — not a SaaS funnel ✨
