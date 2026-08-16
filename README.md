English | [简体中文](README.zh-CN.md)

# FeedForge

**Turn any web page into an RSS feed** — a self-hosted, open-source successor to [Feed43](https://en.wikipedia.org/wiki/Feed43).

Point it at a page, describe what to extract with `{%}` / `{*}` patterns, and get a stable RSS URL. A single Go binary with JSON-file storage — no accounts, no database.

## Features

- **Feed43-compatible patterns** — `{%}` captures, `{*}` skips, `{%1}…{%n}` output templates.
- **3-step wizard** with live match preview. English / 中文 UI.
- **Built-in recipes** for [Bytes.dev](https://bytes.dev/archives) and [OSSInsight](https://ossinsight.io/blog), each tested against a saved copy of the page.
- **RSS 2.0 and JSON Feed 1.1** at stable URLs, with stable first-seen item dates.
- **Charset-aware** — encodings auto-detected; GBK/Big5/Shift_JIS and friends can be forced per feed.
- **Multi-user** — every account keeps its own feeds. The first account is the admin, who decides whether registration is open (closed by default).
- **Safe by default** — SSRF guard, size/time limits, escaped output, session auth with CSRF protection.
- **Single static binary**, one-command Docker deploy, `/demo` practice page.

## Quick start

```bash
git clone https://github.com/real-jiakai/feedforge.git
cd feedforge
docker compose up -d        # → http://localhost:8080
```

Or from source:

```bash
go build -o feedforge .
./feedforge -addr :8080 -data ./data
```

The first visit asks you to create the **admin account**, and a fresh instance starts with the two recipe feeds (Bytes.dev, OSSInsight) already created for it. Everything lives in `./data` — backing that up backs up everything (`tar czf backup.tar.gz data/`).

## Patterns in a minute

Patterns run against the page's raw HTML source. `{%}` matches any text and captures it; `{*}` matches and skips it.

1. The optional **global pattern** is applied once to narrow the search area, e.g. `<ul class="news">{%}</ul>`.
2. The **item pattern** is applied repeatedly to that area; every match becomes one feed item.

Page source:

```html
<li><a href="/post/42">Big news</a><span>2026-07-31</span></li>
```

Item pattern:

```
<li><a href="{%}">{%}</a><span>{%}</span></li>
```

Item templates: title `{%2}`, link `{%1}`, content `{%3}`. Relative links are resolved automatically.

Tips:

- Macros are lazy — they stop at the first occurrence of what follows. Only a trailing `{%}` is greedy.
- **Smart whitespace** (on by default) lets any whitespace run match any other, so reformatted HTML doesn't break feeds.
- Don't match long Tailwind class lists — skip them: `<h2 class="{*}>{%}</h2>`.
- Watch for a featured first item with different markup; keep the pattern loose enough to catch both (the Bytes.dev recipe shows how).
- If the page lists oldest first, enable *Reverse item order* so new items are kept.

## Built-in recipes

| Recipe | Source | What it demonstrates |
|---|---|---|
| OSSInsight blog | ossinsight.io/blog | skipping volatile Tailwind classes |
| Bytes.dev archives | bytes.dev/archives | a lead item whose markup differs from the list |

These are the two feeds this instance is meant to provide. Both are tested against saved page copies (`internal/server/testdata/`), so markup drift fails a test instead of quietly emptying a feed.

## HTTP API

| Method & path | Description |
|---|---|
| `POST /api/auth/register`, `…/login`, `…/logout` | accounts & sessions (register: first user ever = admin, later users only while registration is enabled) |
| `GET /api/auth/me` | current user |
| `GET`, `PUT /api/admin/settings` | admin: toggle `registrationEnabled` |
| `GET /api/recipes` | list built-in recipes |
| `GET /api/feeds`, `POST /api/feeds` | list / create your feeds |
| `GET`, `PUT`, `DELETE /api/feeds/{id}` | read / update / delete one of your feeds |
| `POST /api/feeds/{id}/refresh` | force refetch now |
| `POST /api/preview` | dry-run patterns against a page |
| `GET /feeds/{id}.xml`, `GET /feeds/{id}.json` | RSS 2.0 / JSON Feed output |
| `GET /demo`, `GET /healthz` | practice page, health check |

Everything under `/api` except `config`, `recipes` and `auth` requires a signed-in session (HttpOnly cookie); feeds are visible only to their owner. Mutating calls must send `Content-Type: application/json`. Feed outputs stay public — subscribers never sign in.

## Configuration

| Env var | Flag | Default | Meaning |
|---|---|---|---|
| `FEEDFORGE_ADDR` | `-addr` | `:8080` | listen address |
| `FEEDFORGE_DATA` | `-data` | `./data` (`/data` in Docker) | data directory |
| `FEEDFORGE_BASE_URL` | `-base-url` | *(derive from request)* | public origin in generated feed URLs |
| `FEEDFORGE_ALLOW_PRIVATE` | `-allow-private` | `false` | allow fetching private/LAN addresses |
| `FEEDFORGE_MAX_FETCH_MB` | `-max-fetch-mb` | `5` | max source page size |

Per feed: max items (1–500), refresh interval, item order reversal, encoding override. Compose-only settings (`FEEDFORGE_BIND`, `FEEDFORGE_PORT`, `FEEDFORGE_DOMAIN`, `ACME_EMAIL`) are documented in [`.env.example`](.env.example).

## Deployment

- **Create the admin account right after the first start** — until it exists, whoever visits first can claim it. Note that Docker-published ports bypass ufw/firewalld.
- **HTTPS on your own domain**: set `FEEDFORGE_DOMAIN` in `.env`, then `docker compose -f docker-compose.yml -f docker-compose.caddy.yml up -d` — Caddy obtains and renews the certificate.
- **Behind your own proxy**: `FEEDFORGE_BIND=127.0.0.1` and set `FEEDFORGE_BASE_URL` to the public origin.
- **Upgrade**: `git pull && docker compose up -d --build`. **Backup**: the `./data` directory.

## Security notes

- Fetches to loopback, RFC1918, link-local (cloud metadata), CGNAT and other non-public addresses are refused **at dial time**, covering redirects and DNS rebinding. Proxy env vars are honoured only with `FEEDFORGE_ALLOW_PRIVATE=true`.
- Passwords are stored as bcrypt hashes; sessions are HttpOnly `SameSite=Lax` cookies whose tokens are stored hashed, and the JSON content-type requirement on mutating calls blocks cross-site form CSRF.
- Scraped content is escaped in XML/JSON output; item links that aren't `http`/`https` (such as `javascript:`) are dropped.

## Architecture

```
main.go                  flags/env, embedded web UI, graceful shutdown
internal/pattern/        {%}/{*} → regex compiler, template renderer
internal/fetch/          hardened HTTP client (SSRF guard, limits, charsets)
internal/store/          JSON-file persistence + first-seen item history
internal/feed/           RSS 2.0 / JSON Feed 1.1 rendering
internal/server/         REST API, preview, TTL cache, demo page
web/                     vanilla JS wizard (embedded, no build step)
```

## License

MIT.
