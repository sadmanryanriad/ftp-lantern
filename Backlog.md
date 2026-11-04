# FTP-Lantern — Backlog & Task Management

Tracks roadmap items, technical debt, and future enhancements.

---

## 🚀 Active Sprint — MVP (Phased Implementation)

- [x] **Phase 0: Crawler Spike & Server Verification**
  - [x] Write a 50-line crawler test script against real ISP server (`http://172.16.50.12/DHAKA-FLIX-12/`)
  - [x] Verify HTML table layout, DOM parsing with `goquery`, and confirm timestamp behavior
- [x] **Phase 1: Project Scaffolding & Database Engine**
  - [x] Go 1.26 module initialization with `modernc.org/sqlite`
  - [x] Schema migration (titles with `year DEFAULT 0`, releases with `category`, files, plays, events with `failed_at`, settings)
  - [x] FTS5 trigram virtual table with parameterized `LIKE` acceleration
  - [x] Embedded timezone data (`time/tzdata`) and CLI `healthcheck` subcommand
- [x] **Phase 2: Resilient Crawler & Parser**
  - [x] Multi-source HTTP crawler with non-blocking single-flight lock (`TryLock`)
  - [x] Category-level mtime checking (free season detection for cold shows)
  - [x] Dynamic hot-set scanner based on ISP server folder timestamps (`now - 45d` or `is_following`)
  - [x] Granular regex: split `resolution`, `source`, and `has_hdr` flag
  - [x] Subtree-scoped circuit breaker (abort deletions on HTTP 5xx or >20% loss in scanned subtree)
- [x] **Phase 3: Image Pipeline & Caching**
  - [x] Dedicated single-concurrency poster worker (`concurrency = 1`) with `GOMEMLIMIT=96MiB` guard
  - [x] Content-hashed thumb URLs (`/api/thumbs/:id-:hash.jpg`) with `immutable` caching
  - [x] LRU storage volume cap (`MAX_THUMBS_MB=250`) with auto-eviction and on-demand re-fetch
- [x] **Phase 4: Telegram Bot & Notifier**
  - [x] HTML escaping (`html.EscapeString`) for dynamic strings
  - [x] 4xx/429/5xx non-retryable error guard (mark `failed_at`, don't block queue)
  - [x] Message batching & item-boundary length truncation (3800-char text threshold)
  - [x] Dedicated long-polling bot listener for `/turbo`, `/status`, and `/scan` (whitelisted to Admin Chat ID)
- [x] **Phase 5: API & Player Dispatcher**
  - [x] Parameterized FTS trigram `LIKE` search + short-query prefix + sliding-window Levenshtein
  - [x] Encoded M3U playlist generator (single file & full season) with natural `SxxExx` sorting
  - [x] Android `intent://` launcher with `navigator.sendBeacon()` play tracking
  - [x] Play tracking (`plays` table) powering "Last played: S01E04 → Next: S01E05" and manual "Mark watched up to here"
- [x] **Phase 6: Modern Embedded Frontend**
  - [x] Svelte 5 / Vite SPA embedded via `go:embed`
  - [x] Spotlight search with instant typeahead dropdown, mini posters, and resolution badges
  - [x] "Continue Watching" shelf + "What's New" row + full poster grid
  - [x] Settings panel for Turbo Scan configuration
- [x] **Phase 7: Packaging & Dokploy Integration**
  - [x] Multi-stage Dockerfile (`golang:1.26-alpine` → `scratch` with tzdata + CA certs + `go build -o /ftp-lantern .`)
  - [x] `docker-compose.yml` with `network_mode: host`, named volume `lantern-data:/data`, and 128MB RAM / 0.5 CPU limits

---

## 📌 Future Backlog (Post-MVP)

- [ ] **Remote / Outside-Home Streaming Relay**
  - Optional authenticated video proxy endpoint (`/api/stream/proxy?url=...`) for streaming outside home Wi-Fi
  - Range request streaming support for seeking
  - Direct Tailscale / WireGuard homelab integration guide
- [ ] **Multiple ISP Mirror Support**
  - Support simultaneous crawling across DHAKA-FLIX mirrors (`DHAKA-FLIX-7`, `DHAKA-FLIX-9`, `DHAKA-FLIX-14`)
  - Redundant link generation (if server 12 is slow, switch stream link to mirror)
- [ ] **Kodi / Infuse Strm File Generation**
  - Export library as `.strm` files for local Kodi / Apple TV Infuse integration
- [ ] **Push Notification Webhook / Gotify Integration**
  - Alternative push alerts for non-Telegram homelab users
