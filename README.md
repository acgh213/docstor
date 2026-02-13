# Docstor

**a documentation platform for MSPs that doesn't make you want to mass-quit IT**

Docstor is a self-hosted, server-rendered documentation system built for managed service providers. no electron apps. no SaaS upsells. no "please upgrade to Enterprise for basic search." just markdown docs, runbooks with verification tracking, and an audit trail that actually works.

built by someone who has been the on-call tech at 2am trying to find the firewall rules doc in a SharePoint graveyard. never again.

![Go](https://img.shields.io/badge/Go-1.22-00ADD8?style=flat&logo=go) ![Postgres](https://img.shields.io/badge/Postgres-16-4169E1?style=flat&logo=postgresql) ![License](https://img.shields.io/badge/license-MIT-green) ![vibes](https://img.shields.io/badge/vibes-immaculate-ff69b4)

---

## why does this exist

MSP documentation tools are either:
- overpriced SaaS that holds your data hostage
- wikis from 2008 that look like they run on prayer
- confluence (i will not elaborate)

Docstor is none of those things. it's a Go binary, a Postgres database, and server-rendered HTML. it loads fast, it searches fast, and it won't randomly decide your session expired while you're mid-edit.

## features

### the stuff that's done
- **multi-tenant** — proper tenant isolation, not just a `WHERE` clause someone forgot
- **docs with revision history** — every save is immutable. revert creates a new revision. your audit team will cry tears of joy
- **markdown rendering** — server-side, sanitized, no XSS. rendered with goldmark + bluemonday because we're not animals
- **full-text search** — postgres FTS. it's fast. it works. filters by client, doc type, owner, recency
- **living runbooks** — docs with verification cadence tracking. overdue dashboard so you know which runbooks are lying to you
- **clients** — first-class client entities. filter docs by client. associate everything properly
- **sites** — client → site hierarchy for when one client has 47 offices
- **CMDB-lite** — configuration items linked to clients/sites with shortcode references in docs
- **attachments** — evidence bundles, screenshots, whatever. stored on disk with metadata in postgres
- **templates & checklists** — reusable doc templates with checklist instances you can track
- **incidents & known issues** — track them, link them to docs and CIs
- **change records** — because your clients' auditors will ask
- **doc links & backlinks** — automatic detection of internal links with broken link reporting
- **CodeMirror 6 editor** — with vim mode because i have opinions
- **HTMX for the spicy bits** — markdown preview, inline edits, folder tree expand. no SPA. ever.
- **append-only audit log** — every meaningful action logged. login, edits, verifications, role changes. immutable.
- **role-based access** — TenantAdmin / Editor / Reader. simple and correct
- **CSRF protection** — works with HTMX. nosurf is doing the lord's work

### what it looks like as numbers
- ~18k lines of Go (14k app + 4k tests)
- ~6k lines of HTML templates
- 17 test files with tenant isolation, role gating, XSS regression, and revision conflict tests
- 9 database migrations
- 51 commits of accumulated intent
- 0 npm dependencies in the runtime (the editor bundle is built separately and committed)

## stack

| layer | choice | why |
|-------|--------|-----|
| language | Go 1.22 | compiles fast, deploys as a single binary, i trust it at 3am |
| http | chi | it's just net/http with better routing. boring. perfect. |
| database | Postgres 16 | FTS, JSONB, rock solid. no ORM, just SQL. |
| migrations | golang-migrate | embedded in the binary, runs on startup |
| templates | html/template | server-rendered. every page works without JS |
| css | custom minimal | no tailwind. no bootstrap. just vibes and `max-width` |
| markdown | goldmark + bluemonday | render server-side, sanitize everything |
| editor | CodeMirror 6 | only loaded on edit pages. the one JS island we allow |
| htmx | 1.x | for previews, inline edits, tree expand. that's it. |
| auth | bcrypt + secure cookies | sessions in the DB. rate limiting on login. |

## quickstart

### prerequisites
- Go 1.22+
- Docker + Docker Compose (for Postgres)
- that's literally it

### run it

```bash
# clone it
git clone https://github.com/acgh213/docstor.git
cd docstor

# copy the env file
cp .env.example .env

# start postgres and run the app
# (migrations run automatically on startup)
make dev
```

the app will be at `http://localhost:8080`. a seed user is created on first run.

### build it

```bash
make build
# produces ./bin/docstor
```

### test it

```bash
make test
```

tests use a real Postgres instance (via docker compose) because mocking your database is just testing your mocks.

## project structure

```
cmd/docstor/         → entrypoint
internal/
  auth/              → sessions, passwords, middleware, rate limiting
  db/                → database connection, migrations (embedded)
  docs/              → documents, revisions, markdown rendering, diffs
  runbooks/          → verification tracking
  clients/           → client CRUD
  sites/             → site management
  cmdb/              → configuration items + shortcodes
  attachments/       → file storage + metadata
  templates/         → doc templates + checklist instances
  incidents/         → incidents + known issues
  changes/           → change records
  doclinks/          → link extraction + backlinks
  audit/             → append-only audit logger
  pagination/        → shared pagination helpers
  web/               → HTTP handlers, router, template rendering
  config/            → env config
  testutil/          → shared test helpers
web/
  templates/         → Go HTML templates (layout, docs, clients, etc.)
  static/            → CSS, JS (just CodeMirror + tiny helpers)
editor-bundle/       → CodeMirror build (node, built separately, output committed)
migrations/          → reference copies of SQL migrations
```

## design decisions i will mass-defend

**no SPA.** server-rendered HTML loads faster, works without JS, and doesn't need a state management library named after a subatomic particle. HTMX handles the interactive bits.

**no ORM.** SQL is a feature, not a bug. every query is visible, tunable, and tenant-scoped. N+1s are caught in code review, not discovered in production at 2am.

**immutable revisions.** you can't edit history. revert copies the old content into a new revision. the audit log is append-only. if something went wrong, we know exactly what happened and who did it.

**no secrets storage.** Docstor stores documentation, not passwords. keep your secrets in a vault. we store *references* to where the secrets live.

**tenant isolation everywhere.** every query, every handler, every test. cross-tenant data leakage is a test failure, not a TODO.

## contributing

this is early and opinionated but PRs are welcome if you vibe with the approach. please read `claude.md` — it's the build contract and it's not optional.

if you open an issue that says "have you considered rewriting the frontend in React" i will mass-close it with a mass-emoji.

## license

MIT. do whatever you want. just don't mass-blame me if you deploy it without changing the session keys in `.env`.

---

*built with mass-stubbornness and mass-caffeination by someone who thinks documentation should be boring infrastructure, not a product with a pricing page* ✨
