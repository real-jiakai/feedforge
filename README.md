# FeedForge

**Turn any web page into an RSS feed** — a self-hosted, open-source recreation of the late, great [Feed43](https://en.wikipedia.org/wiki/Feed43).

English | [简体中文](README.zh-CN.md)

Feed43 let you point at any web page, describe what to extract with simple
`{%}` / `{*}` search patterns, and get a stable RSS URL. The service shut
down when its domain expired; FeedForge brings the same workflow back as a
single Go binary you run yourself — no accounts, no paid tiers, no quotas
beyond the ones you set.

## Features

- **Feed43-compatible pattern syntax** — `{%}` captures, `{*}` skips, global
  + item search patterns, `{%1}…{%n}` output templates.
- **Interactive 3-step wizard** — load a page, watch your patterns match
  live, preview the feed before saving. English / 中文 UI.
- **Built-in recipes** — working patterns for real sites
  ([OSSInsight](https://ossinsight.io/blog), [Bytes.dev](https://bytes.dev/archives),
  Hacker News) that fill in the whole wizard so you can read and adapt them.
  Each is covered by a test against a saved copy of the page.
- **RSS 2.0 and JSON Feed 1.1** output at stable URLs.
- **Stable item dates** — FeedForge remembers when it first saw each item
  and uses that as `pubDate` (Feed43 never dated items).
- **Charset-aware** — auto-detects encodings; GBK/GB18030/Big5/Shift_JIS
  and friends can be forced per feed.
- **Safe by default** — requests to private/internal networks are blocked
  (SSRF guard), fetched pages are size- and time-limited, XML output is
  properly escaped, non-http(s) links are stripped, optional API token
  protects editing.
- **Single static binary**, JSON-file storage, one-command Docker deploy.
- **Built-in demo page** at `/demo` for practicing patterns — a new fake
  article appears daily, so your practice feed visibly updates.

## Quick start

### Docker Compose (recommended)

```bash
git clone https://github.com/real-jiakai/feedforge.git
cd feedforge
docker compose up -d
```

Open <http://localhost:8080>, pick a recipe, and you'll have a working feed
in under a minute. Feed definitions live in `./data`.

That is the whole install — see [Docker Compose usage](#docker-compose-usage)
for tokens, HTTPS, updates and backups.

### From source

```bash
go build -o feedforge .
./feedforge -addr :8080 -data ./data
```

## How patterns work

Everything works on the page's raw HTML source (View Source, not the
rendered page). Two macros:

| Macro | Meaning |
|-------|---------|
| `{%}` | match any text and **capture** it |
| `{*}` | match any text and **skip** it |

1. The **global search pattern** (optional) is applied once and narrows the
   search area — e.g. `<ul class="news">{%}</ul>` limits everything that
   follows to the contents of that list. Its captures are available to the
   feed-level templates as `{%1}`, `{%2}`, …
2. The **item search pattern** is applied repeatedly to that area; every
   match becomes one feed item, and its captures fill the item templates.

Example. Page source:

```html
<ul class="news">
  <li><a href="/post/42">Big news</a><span>2026-07-31</span></li>
  <li><a href="/post/41">Old news</a><span>2026-07-30</span></li>
</ul>
```

Item search pattern:

```
<li><a href="{%}">{%}</a><span>{%}</span></li>
```

Item templates: title `{%2}`, link `{%1}`, content `{%3}` → a two-item feed.
Relative links like `/post/42` are resolved against the source URL
automatically.

Tips carried over from years of Feed43 practice:

- Macros are *lazy*: `{%}` stops at the first occurrence of whatever
  follows it. The one exception is a `{%}` at the very end of a pattern,
  which is greedy — that is what lets a bare `{%}` as the item pattern turn
  the whole global capture into a single item. A trailing `{*}` stays lazy.
- **Smart whitespace** (on by default, including for API clients that omit
  the field) makes any run of whitespace in your pattern match any run of
  whitespace on the page, so reformatted HTML and CRLF/LF differences don't
  break your feed. Turn it off for byte-exact matching.
- Titles are cleaned automatically: HTML tags stripped, entities decoded,
  whitespace collapsed. Item *content* keeps its HTML.
- If a page mixes several list markups, anchor your item pattern with
  something unique (a class name, a wrapper tag) rather than a plain `<li>`.
- **Watch out for a lead item with different markup.** Many blogs render
  the newest post as a "featured" block that looks nothing like the rest of
  the list. A pattern tuned to the list silently drops every new post until
  it scrolls into the list. The bundled Bytes.dev recipe shows the fix:
  skip the parts that vary (`<span class="font-{*}>` matches both
  `font-bold` and `font-semibold`) instead of matching them exactly.
- If the page lists **oldest first**, turn on *Reverse item order*.
  FeedForge then keeps the last N matches, so newly appended entries always
  make it into the feed.

### Tailwind-heavy pages

Modern sites carry enormous class attributes that change without warning.
Don't match them — skip them. Instead of

```
<h2 class="mt-2.5 line-clamp-2 text-[17px] font-semibold ...">{%}</h2>
```

write

```
<h2 class="{*}>{%}</h2>
```

which survives a restyle and matches every variant of the heading.

## Built-in recipes

Four ready-made definitions ship with FeedForge. Pick one on the home page
and the wizard fills in completely — they are meant to be read and modified,
not treated as magic.

| Recipe | Source | What it demonstrates |
|---|---|---|
| FeedForge demo page | built-in `/demo` | a clean, textbook pattern |
| Hacker News front page | news.ycombinator.com | how little is needed — one line |
| OSSInsight blog | ossinsight.io/blog | skipping volatile Tailwind classes |
| Bytes.dev archives | bytes.dev/archives | a lead item whose markup differs from the list |

The OSSInsight and Bytes.dev recipes are exercised in CI against saved
copies of those pages (`internal/server/testdata/`), so if a site's markup
drifts, a test fails instead of a feed quietly going empty.

For reference, the Bytes.dev item pattern is:

```
href="/archives/{%}">{*}<div class="grid gap-2"><span class="font-{*}>{%}</span><h3{*}Issue <!-- -->{%}</span><{*}>{%}</
```

`{%1}` is the issue number, `{%2}` the date, `{%3}` the issue number as
printed, `{%4}` the title. Note `<span class="font-{*}>` and `<{*}>`: those
skips are what let the same pattern match both the featured newest issue
(`font-bold`, title in a `<p>`) and every archived one (`font-semibold`,
title in a `<div>`).

## HTTP API

| Method & path | Description |
|---|---|
| `GET /api/recipes` | list built-in starter recipes |
| `GET /api/feeds` | list feeds |
| `POST /api/feeds` | create feed |
| `GET /api/feeds/{id}` | get one feed definition |
| `PUT /api/feeds/{id}` | update feed |
| `DELETE /api/feeds/{id}` | delete feed |
| `POST /api/feeds/{id}/refresh` | force refetch now |
| `POST /api/preview` | dry-run patterns against a page (powers the wizard) |
| `GET /feeds/{id}.xml` | RSS 2.0 output |
| `GET /feeds/{id}.json` | JSON Feed 1.1 output |
| `GET /demo` | practice page |
| `GET /healthz` | health check |

When `FEEDFORGE_TOKEN` is set, mutating calls (`POST`/`PUT`/`DELETE`)
require `Authorization: Bearer <token>`. Reads and feed outputs stay public.
Mutating calls must also send `Content-Type: application/json`, which is
what stops a cross-origin HTML form from driving a token-less instance.

## Configuration

| Env var | Flag | Default | Meaning |
|---|---|---|---|
| `FEEDFORGE_ADDR` | `-addr` | `:8080` | listen address |
| `FEEDFORGE_DATA` | `-data` | `./data` (`/data` in Docker) | data directory |
| `FEEDFORGE_TOKEN` | `-token` | *(empty = open)* | API token for editing |
| `FEEDFORGE_BASE_URL` | `-base-url` | *(derive from request)* | public origin for generated feed URLs |
| `FEEDFORGE_ALLOW_PRIVATE` | `-allow-private` | `false` | allow fetching private/LAN addresses |
| `FEEDFORGE_MAX_FETCH_MB` | `-max-fetch-mb` | `5` | max source page size |

Per-feed options: max items (1–500, default 25), refresh interval (feeds are
refetched at most once per interval, on demand), item order reversal,
encoding override.

Set `FEEDFORGE_BASE_URL` when running behind a reverse proxy: without it the
`atom:link rel="self"` in generated feeds is derived from the request's Host
header.

## Docker Compose usage

`docker compose up -d` compiles the Go binary inside the build stage and
starts one container — no Go toolchain, database, or reverse proxy required
on the host. Feeds are JSON files under `./data`, which is all you ever need
to back up.

```bash
git clone https://github.com/real-jiakai/feedforge.git
cd feedforge
cp .env.example .env      # optional; edit it before the first start
docker compose up -d
```

What you get out of the shipped `docker-compose.yml`:

- restarts on boot and on crash (`restart: unless-stopped`), with Docker
  watching `/healthz` so `docker compose ps` tells you the truth;
- an unprivileged runtime user, a read-only root filesystem, and every Linux
  capability dropped except the `CHOWN`/`SETUID`/`SETGID` the entrypoint
  needs to adopt a fresh `./data` and drop privileges;
- log rotation at 3 × 10 MB, so a chatty feed can't fill the disk;
- a 15 s stop grace period, long enough for in-flight feed builds to drain.

### Configure with `.env`

`.env` is read automatically by Compose; every key is optional. Copy
`.env.example`, which documents each one, and restart with
`docker compose up -d` to apply changes.

| Variable | Default | Purpose |
|---|---|---|
| `FEEDFORGE_TOKEN` | *(empty)* | require `Authorization: Bearer …` to create/edit feeds |
| `FEEDFORGE_BIND` | `0.0.0.0` | host interface the port is published on |
| `FEEDFORGE_PORT` | `8080` | host port (the container always uses 8080) |
| `FEEDFORGE_BASE_URL` | *(from request)* | public origin baked into feed URLs |
| `FEEDFORGE_ALLOW_PRIVATE` | `false` | allow scraping LAN/loopback addresses |
| `FEEDFORGE_MAX_FETCH_MB` | `5` | source page size limit |
| `FEEDFORGE_DOMAIN` | — | domain for the HTTPS overlay below |
| `ACME_EMAIL` | *(empty)* | certificate expiry contact for that overlay |

`FEEDFORGE_BIND`, `FEEDFORGE_PORT`, `FEEDFORGE_DOMAIN` and `ACME_EMAIL` are
Compose-level settings; the rest map to the flags in
[Configuration](#configuration).

> **Publishing on a public host?** Set `FEEDFORGE_TOKEN`. Without it, anyone
> who can reach the port can create feeds and make your server fetch URLs on
> their behalf. And note that Docker inserts its own iptables rules ahead of
> ufw/firewalld: a port published on `0.0.0.0` is reachable from the internet
> even when your firewall lists no rule allowing it. Set
> `FEEDFORGE_BIND=127.0.0.1` whenever a proxy fronts FeedForge.

### Everyday commands

```bash
docker compose ps                 # status + health
docker compose logs -f            # follow logs
docker compose up -d              # apply a changed .env (recreates the container)
docker compose restart            # restart in place — does NOT re-read .env
docker compose stop               # stop, keep the container
docker compose down               # stop and remove; ./data survives
docker compose up -d --build      # rebuild after `git pull`
```

`restart` only stops and starts the existing container. Environment variables
and port bindings are fixed when a container is *created*, so a token you just
added to `.env` takes effect only after `docker compose up -d` recreates it.

Upgrading is `git pull && docker compose up -d --build`. Feed definitions and
first-seen history live in `./data` and are untouched by a rebuild.

Back up and restore the whole service:

```bash
# Back up — safe to run while the service is up.
tar czf feedforge-$(date +%F).tar.gz data/

# Restore. Check the archive before destroying the only other copy.
BACKUP=feedforge-2026-07-31.tar.gz
tar tzf "$BACKUP" >/dev/null
docker compose down
sudo rm -rf data/
sudo tar xzf "$BACKUP"
docker compose up -d
```

`sudo` is needed because the entrypoint chowns `./data` to the container's
`feedforge` user (uid 100); drop it if you drive Docker as root. Extracting as
root also preserves that ownership, so the container starts without a re-chown.

### HTTPS on your own domain

`docker-compose.caddy.yml` adds a [Caddy](https://caddyserver.com) front end
that obtains and renews a certificate automatically:

```bash
echo "FEEDFORGE_DOMAIN=feeds.example.com" >> .env
echo "ACME_EMAIL=you@example.com"         >> .env   # optional
docker compose -f docker-compose.yml -f docker-compose.caddy.yml up -d
```

The overlay stops publishing FeedForge's own port — it is reachable only
through Caddy — and defaults `FEEDFORGE_BASE_URL` to `https://$FEEDFORGE_DOMAIN`
so generated feed URLs are right. Leave `FEEDFORGE_BASE_URL` empty in `.env`
unless you moved `CADDY_HTTPS_PORT` off 443, in which case set it explicitly
*including the port* — an explicit value still wins.

The domain's DNS record must already point at the host and ports 80 and 443
must be free, or the certificate order fails. Certificates persist in the
`caddy_data` volume; keep it across upgrades to avoid re-issuing (and hitting
Let's Encrypt rate limits). Every later command needs both `-f` flags, so it
is worth exporting `COMPOSE_FILE=docker-compose.yml:docker-compose.caddy.yml`
in your shell or `.env`.

### Behind a reverse proxy you already run

Publish on loopback only and tell FeedForge its public origin:

```dotenv
FEEDFORGE_BIND=127.0.0.1
FEEDFORGE_BASE_URL=https://feeds.example.com
```

```caddy
feeds.example.com {
	reverse_proxy 127.0.0.1:8080
}
```

```nginx
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host              $host;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

`X-Forwarded-Proto` matters only if you leave `FEEDFORGE_BASE_URL` unset —
it is what stops feeds from advertising `http://` URLs while served over TLS.

### Troubleshooting

| Symptom | Cause and fix |
|---|---|
| `bind: address already in use` | something else owns the host port — set `FEEDFORGE_PORT` in `.env` |
| `warning: could not chown /data` | `./data` belongs to another UID; `sudo chown -R 100:101 data/` or delete it and let the entrypoint recreate it |
| status stuck at `health: starting` | the first check runs after 5 s; `docker compose logs` shows the real error |
| feeds show stale items | each feed refetches at most once per its TTL — `POST /api/feeds/{id}/refresh` forces it |
| feed URLs point at `localhost` | set `FEEDFORGE_BASE_URL` (or use the Caddy overlay) |
| `401` from the API | `FEEDFORGE_TOKEN` is set; send `Authorization: Bearer <token>` |

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

Storage is plain JSON files under the data directory — trivially backed up,
diffed, and inspected. No database required.

## Security notes

- The fetcher refuses to connect to loopback, RFC1918, CGNAT/Tailscale
  (100.64/10), link-local (including cloud metadata at 169.254.169.254),
  and other non-public addresses **at dial time**, so redirects and DNS
  rebinding are covered too. IPv6 forms that embed an IPv4 address
  (IPv4-mapped, IPv4-compatible, NAT64, 6to4) are resolved before the check.
  Set `FEEDFORGE_ALLOW_PRIVATE=true` only if you trust everyone who can
  create feeds.
- `HTTP_PROXY`/`HTTPS_PROXY` are honoured **only** when
  `FEEDFORGE_ALLOW_PRIVATE=true`: a proxy would otherwise defeat the
  dial-time guard, since the only address dialed is the proxy's.
- Set `FEEDFORGE_TOKEN` whenever the server is reachable from the internet;
  otherwise anyone can create feeds and use your server to fetch pages.
- Scraped content is escaped in XML/JSON output and rendered as plain text
  in the editor preview. Item links that aren't `http`/`https` — such as a
  `javascript:` href lifted from a hostile page — are dropped rather than
  passed on to subscribers' readers.

## License

MIT.
