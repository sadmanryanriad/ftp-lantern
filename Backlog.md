# FTP-Lantern — Backlog & Task Management

Tracks roadmap items, technical debt, and future enhancements.

---

## 🚀 Active Sprint — MVP (Phased Implementation)

- [ ] **Phase 0: Crawler Spike & Server Verification**
  - [ ] Write a 50-line crawler test script against real ISP server (`http://172.16.50.12/DHAKA-FLIX-12/`)
  - [ ] Verify HTML table layout, DOM parsing with `goquery`, and confirm timestamp behavior
- [ ] **Phase 1: Project Scaffolding & Database Engine**
  - [ ] Go 1.23+ module initialization with `modernc.org/sqlite`
  - [ ] Schema migration (titles with `year DEFAULT 0`, releases with `category`, files, plays, events with `failed_at`, settings)
  - [ ] FTS5 trigram virtual table with parameterized `LIKE` acceleration
  - [ ] Embedded timezone data (`time/tzdata`) and CLI `healthcheck` subcommand
- [ ] **Phase 2: Resilient Crawler & Parser**
  - [ ] Multi-source HTTP crawler with non-blocking single-flight lock (`TryLock`)
  - [ ] Category-level mtime checking (free season detection for cold shows)
  - [ ] Dynamic hot-set scanner based on ISP server folder timestamps (`now - 45d` or `is_following`)
  - [ ] Granular regex: split `resolution`, `source`, and `has_hdr` flag
  - [ ] Subtree-scoped circuit breaker (abort deletions on HTTP 5xx or >20% loss in scanned subtree)
- [ ] **Phase 3: Image Pipeline & Caching**
  - [ ] Dedicated single-concurrency poster worker (`concurrency = 1`) with `GOMEMLIMIT=96MiB` guard
  - [ ] Content-hashed thumb URLs (`/api/thumbs/:id-:hash.jpg`) with `immutable` caching
- [ ] **Phase 4: Telegram Bot & Notifier**
  - [ ] HTML escaping (`html.EscapeString`) for dynamic strings
  - [ ] 4xx non-retryable error guard (mark `failed_at`, don't block queue)
  - [ ] Message batching & length truncation (4000-char text, 1000-char caption)
  - [ ] Dedicated long-polling bot listener for `/turbo`, `/status`, and `/scan` (whitelisted to Admin Chat ID)
- [ ] **Phase 5: API & Player Dispatcher**
  - [ ] Parameterized FTS trigram `LIKE` search + short-query prefix + sliding-window Levenshtein
  - [ ] Encoded M3U playlist generator (single file & full season) with natural `SxxExx` sorting
  - [ ] Android `intent://` launcher with `navigator.sendBeacon()` play tracking
  - [ ] Play tracking (`plays` table) powering "Last played: S01E04 → Next: S01E05" and manual "Mark watched up to here"
- [ ] **Phase 6: Modern Embedded Frontend**
  - [ ] Svelte 5 / Vite SPA embedded via `go:embed`
  - [ ] Spotlight search with instant typeahead dropdown, mini posters, and resolution badges
  - [ ] "Continue Watching" shelf + "What's New" row + full poster grid
  - [ ] Settings panel for Turbo Scan configuration
- [ ] **Phase 7: Packaging & Dokploy Integration**
  - [ ] Multi-stage Dockerfile (`golang:1.23-alpine` → `scratch` with tzdata + CA certs + `go build -o /ftp-lantern .`)
  - [ ] `docker-compose.yml` with `network_mode: host`, named volume `lantern-data:/data`, and 128MB RAM / 0.5 CPU limits

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
