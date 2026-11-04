# 🏮 FTP-Lantern

> Ultra-lightweight self-hosted media indexer, instant search, and native streaming launcher for BDIX / ISP HTTP media portals.

FTP-Lantern indexes ISP HTTP file directories (such as Dhaka-FLIX), extracts clean metadata (title, year, resolution badges like 1080p/720p/4K), caches poster thumbnails, provides sub-millisecond full-text search with typo tolerance, and generates 1-click native player streams (PotPlayer / VLC on Windows, VLC / MX Player on Android) with watch-history tracking and Telegram alerts for new uploads.

---

## ⚡ Key Highlights

- **Zero Video Transcoding:** Media bytes stream directly from the ISP server to client players. Zero CPU load on the host.
- **Resource Discipline:** Single static Go binary + embedded Svelte 5 frontend. Runs in `< 30 MB` RAM in a minimal Docker container (`< 25 MB`).
- **Instant Search:** SQLite FTS5 trigram full-text search with parameterized `LIKE` acceleration and sliding-window typo tolerance (< 3ms query response).
- **1-Click Native Streaming:** 
  - **Windows:** Continuous full-season auto-next playlists (`.m3u`) for PotPlayer and VLC.
  - **Android:** Native player launch via `intent://` with `navigator.sendBeacon()` play tracking.
- **Smart Incremental Crawler:** Category-level mtime inspection + dynamic Hot-Set tracking detects new episodes in `< 5 seconds` without scanning the entire library.
- **Telegram Notifications:** Photo alerts & grouped upload digests with Telegram admin bot commands (`/status`, `/turbo [hours]`, `/scan`).
- **Dokploy & Homelab Ready:** Simple Docker deployment with host networking to guarantee direct routing to private ISP subnets.

---

## 📐 Architecture & Roadmap

- Deep technical architecture: [architecture.md](./architecture.md)
- Development task backlog: [Backlog.md](./Backlog.md)
- Operational homelab guidelines: [AGENTS.md](./AGENTS.md)

---

## 📄 License

MIT
