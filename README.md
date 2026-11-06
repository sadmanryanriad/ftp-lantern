# 🏮 FTP-Lantern

> Ultra-lightweight self-hosted media indexer, instant search, and native streaming launcher for BDIX / ISP HTTP media portals.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](./LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8.svg?logo=go)](https://go.dev)
[![Svelte 5](https://img.shields.io/badge/Frontend-Svelte%205-FF3E00.svg?logo=svelte)](https://svelte.dev)
[![Docker Image](https://img.shields.io/badge/Image%20Size-~20MB-blue.svg?logo=docker)](https://hub.docker.com)
[![RAM Target](https://img.shields.io/badge/RAM%20Usage-%3C30MB-brightgreen.svg)](#performance--resource-discipline)

FTP-Lantern indexes ISP HTTP media directories (such as Dhaka-FLIX, SamOnline, CircleFTP), extracts clean metadata (titles, years, resolution badges like 1080p / 720p / 4K, audio tags), caches poster thumbnails, provides sub-millisecond full-text search with typo tolerance, and generates 1-click native player streams (PotPlayer / VLC on Windows, VLC / MX Player on Android) with watch-history tracking and Telegram alerts for new uploads.

---

## ⚡ Key Features

- **Zero Video Transcoding:** Media bytes stream directly from the ISP server (`172.16.50.12`) to client players. Zero server CPU load or proxy overhead.
- **Resource Discipline:** Single static Go binary with an embedded Svelte 5 single-page application. Runs inside a `< 25 MB` minimal container using `< 30 MB` RAM.
- **Instant Search:** SQLite FTS5 trigram full-text search with parameterized `LIKE` acceleration and sliding-window Levenshtein typo fallback (< 3ms query response).
- **1-Click Native Streaming:**
  - **Windows Desktop:** Continuous full-season auto-next playlists (`.m3u`) launching PotPlayer or VLC with a single click.
  - **Android:** Native player launch via standard `intent://` URIs with `navigator.sendBeacon()` background play tracking.
- **Smart Incremental Crawler:** 
  - Direct children mtime inspection catches new titles and new seasons of cold shows for free in `< 200 ms`.
  - Dynamic **Hot-Set** tracking re-checks season subfolders of recent series, detecting new episodes in `< 5 seconds`.
  - Subtree-scoped mass deletion circuit breaker protects against ISP timeouts or HTTP 5xx errors.
- **Volume Cap & Disk LRU:** The `/data/thumbs` directory is capped (default: `250 MB`). Automatically evicts least-recently-accessed posters with on-demand dynamic re-fetching.
- **Telegram Notifications:** Photo alerts & grouped upload digests with Telegram admin bot commands (`/status`, `/turbo [hours]`, `/scan`).
- **Dokploy & Homelab Ready:** Simple Docker deployment configured with `network_mode: host` to guarantee direct routing to private ISP subnets without Docker NAT collisions.

---

## 🛠 Architecture & Tech Stack

```
ISP BDIX Server (172.16.50.12/DHAKA-FLIX-12/...)
          │  Incremental HTTP crawl (Category Mtime + Hot-Set Seasons + Nightly Deep Crawl)
          ▼
┌───────────────────────── ftp-lantern (Single Go Binary) ─────────────────────────┐
│                                                                                  │
│  ┌────────────────────┐     ┌──────────────────┐     ┌────────────────────────┐  │
│  │   Crawler Engine   │ ──► │   Diff Engine    │ ──► │  SQLite 3 (WAL mode)   │  │
│  │  (DOM + Hot-Set)   │     │ (CircuitBreaker) │     │  - titles, releases    │  │
│  └─────────┬──────────┘     └────────┬─────────┘     │  - files, plays        │  │
│            │                         │               │  - events, settings    │  │
│            ▼                         ▼               │  - FTS5 trigram index  │  │
│    Poster Worker (Q=1)         Telegram Bot          └───────────┬────────────┘  │
│  (Single worker, max 96MB,  (Digest & Photo alerts               ▲               │
│   /data/thumbs/:id-:hash)    + Long-Polling Bot)                 │ REST API      │
│                                                                  │               │
│                                                      ┌───────────┴────────────┐  │
│                                                      │  Embedded Web UI       │  │
│                                                      │  (Svelte 5 / Vite SPA) │  │
└──────────────────────────────────────────────────────┴────────────────────────┴──┘
          │
          ├─► Android Devices: intent://... (navigator.sendBeacon() play tracking)
          ├─► Windows Desktops: /play/season/:id.m3u (Direct stream in PotPlayer / VLC)
          ├─► Telegram Client: Rich HTML alerts (escaped) + bot commands (/turbo, /status)
          └─► "Continue Watching": "Last played: S01E04 → Next: S01E05" + manual mark
```

For complete technical specifications, review [architecture.md](./architecture.md).

---

## 🚀 Quick Start (Docker Compose / Dokploy)

### 1. Clone & Configure
```bash
git clone https://github.com/sadmanryanriad/ftp-lantern.git
cd ftp-lantern
cp .env.example .env
```

Edit `.env` with your settings:
```ini
PORT=8085
SOURCE_URLS=http://172.16.50.12/DHAKA-FLIX-12/TV-WEB-Series/
BASE_CRAWL_INTERVAL_MINUTES=60
MAX_THUMBS_MB=250
PUBLIC_BASE_URL=http://192.168.0.x:8085
TELEGRAM_BOT_TOKEN=123456789:ABCdefGHIjklMNOpqrSTUvwxYZ
TELEGRAM_ADMIN_CHAT_ID=123456789
```

### 2. Deploy with Docker Compose
```bash
docker compose up -d --build
```

Access the dashboard at `http://<YOUR_HOST_IP>:8085`.

### 3. Deploy via Dokploy
1. In Dokploy, create a new **Compose Application** (or **Application with Dockerfile**).
2. Point the repository to `https://github.com/sadmanryanriad/ftp-lantern`.
3. Add the environment variables from `.env.example` in Dokploy's UI.
4. Click **Deploy**.

> [!IMPORTANT]
> **Host Networking:** The container MUST run with `network_mode: host`. This guarantees uninterrupted routing to private ISP subnets (`172.16.x.x`) without colliding with Docker's internal bridge network.

---

## 🎮 Native Player Setup

### Windows (PotPlayer / VLC)
1. In Chrome, Edge, or Firefox, click the **"Play Season in PotPlayer/VLC"** button on any show.
2. In the browser download bar, right-click the `.m3u` file and select **"Always open files of this type"**.
3. Choose **PotPlayer** or **VLC** as your default player for `.m3u` files.
4. Single-clicking "Play" or "Play Season" will now launch your media player immediately with full-season playlist and auto-next support!

### Android (VLC / MX Player / Just Player)
Tapping any episode's play button triggers an Android `intent://` URI. Android will automatically open your default video player (VLC or MX Player) and stream the file directly over local broadband Wi-Fi.

---

## 📡 External Search API (Homelab & Telegram Bot Integration)

FTP-Lantern exposes a high-performance, low-latency Search API designed to be accessed locally from anywhere across your homelab network (e.g. from your central **Homelab Dashboard**, custom scripts, or external bots).

> [!NOTE]
> **Homelab Bot Architecture:** If your homelab already owns and manages the primary Telegram bot, you do not need FTP-Lantern to manage inbound commands. Your homelab bot can simply query this API locally to provide `/search <query>` or interactive inline media search directly within Telegram.

### Endpoints
- `GET /api/search`
- `GET /api/v1/search` (versioned alias)

CORS is globally enabled (`Access-Control-Allow-Origin: *`), allowing direct browser-based calls from any web dashboard or cross-origin app.

### Query Parameters

| Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `q` | string | *none* | Search keyword (accelerated by SQLite FTS5 trigram + Levenshtein typo fallback) |
| `limit` | integer | `30` | Number of items to return (1 – 100) |
| `page` | integer | `1` | Pagination page number |
| `type` | string | `all` | Filter by type: `movie`, `series`, or `all` |
| `resolution` | string | *all* | Filter by resolution: `1080p`, `720p`, `4K` |
| `category` | string | *all* | Match specific category / folder string |
| `year` | string | *all* | Specific year (`2024`), range (`2020-2024`), or older (`<2010`) |
| `include_files`| boolean| `false` | When `true` (or `1`), embeds all media files & streaming URLs in the response |

### Response Schema

```json
{
  "query": "dexter",
  "total": 4,
  "page": 1,
  "limit": 2,
  "results": [
    {
      "id": 478,
      "name": "Dexter",
      "year": 2006,
      "type": "series",
      "resolution": "1080p",
      "source": "Unknown",
      "has_hdr": false,
      "category": "TV Series ♥  A  —  L",
      "size_bytes": 86651417600,
      "size_human": "80.7 GB",
      "poster_url": "http://172.16.50.12/DHAKA-FLIX-12/.../Cover1.jpg",
      "thumb_url": "/api/thumbs/478-62e12226.jpg",
      "thumb_full_url": "http://192.168.0.134:8087/api/thumbs/478-62e12226.jpg",
      "folder_url": "http://172.16.50.12/DHAKA-FLIX-12/TV-WEB-Series/TV%20Series%20%E2%99%A5%20%20A%20%20%E2%80%94%20%20L/Dexter%20%28TV%20Series%202006%E2%80%932013%29%201080p/",
      "web_url": "http://192.168.0.134:8087/#/title/478",
      "is_following": false,
      "files": [
        {
          "id": 14201,
          "season_number": 1,
          "episode_number": 1,
          "episode_code": "S01E01",
          "name": "Dexter.S01E01.1080p.mkv",
          "file_url": "http://172.16.50.12/DHAKA-FLIX-12/.../Dexter.S01E01.1080p.mkv",
          "intent_url": "intent://172.16.50.12/...#Intent;type=video/*;end",
          "size_bytes": 1423456789,
          "size_human": "1.3 GB",
          "resolution": "1080p",
          "source": "WEBRip",
          "has_hdr": false
        }
      ]
    }
  ]
}
```

### Homelab Dashboard & Telegram Bot Integration Examples

#### 1. Quick Local Search (cURL)
```bash
curl -s "http://192.168.0.134:8087/api/v1/search?q=severance&limit=5" | jq
```

#### 2. Telegram Bot Search Extension (Python)
If your central homelab bot handles Telegram updates:
```python
import requests

def handle_telegram_search(query: str, chat_id: int):
    # Query FTP-Lantern Search API locally
    res = requests.get(
        "http://192.168.0.134:8087/api/v1/search",
        params={"q": query, "limit": 5}
    ).json()
    
    if not res["results"]:
        return "No titles found matching your query."

    msg = f"🔍 <b>FTP-Lantern Search Results for '{query}':</b>\n\n"
    for i, item in enumerate(res["results"], start=1):
        year_str = f" ({item['year']})" if item['year'] else ""
        msg += f"{i}. <a href=\"{item['web_url']}\"><b>{item['name']}{year_str}</b></a> — [{item['resolution']}]"
        if item.get("folder_url"):
            msg += f" • <a href=\"{item['folder_url']}\">ISP</a>"
        msg += f" (<i>{item['size_human']}</i>)\n"
    
    return msg
```

#### 3. Homelab Dashboard Integration (Node.js / Express / Next.js)
```typescript
// Fetch directly from any microservice or dashboard backend
export async function searchBDIX(query: string) {
  const resp = await fetch(`http://192.168.0.134:8087/api/v1/search?q=${encodeURIComponent(query)}&limit=10`);
  const data = await resp.json();
  return data.results;
}
```

---

## ⌨️ Dashboard & API Reference

| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/api/search`, `/api/v1/search` | `GET` | Instant search & filtering (`q`, `type`, `category`, `resolution`, `year`, `include_files`, `page`, `limit`) |
| `/api/titles` | `GET` | Catalog list (`continue_watching=true` returns active watch shelf) |
| `/api/titles/:id` | `GET` | Detailed title payload with releases, files, streaming URLs, and `web_url` |
| `/api/titles/:id/follow` | `POST` | Toggle watchlist status |
| `/api/plays` | `POST` | Record playback event (`file_id`) via `navigator.sendBeacon()` |
| `/api/plays/mark-watched`| `POST` | Mark episodes watched up to specified `file_id` |
| `/api/new` | `GET` | Recently indexed additions with `web_url` and `folder_url` |
| `/api/thumbs/:filename`| `GET` | Content-hashed cached poster thumbnail (`immutable` headers) |
| `/play/file/:id.m3u` | `GET` | Ephemeral single-file M3U playlist |
| `/play/season/:id.m3u`| `GET` | Full season M3U playlist file with natural episode ordering |
| `/api/scheduler` | `GET` | Scheduler status, active interval, and Turbo Mode countdown |
| `/api/scheduler/turbo` | `POST` | Activate Turbo Mode (`{ "interval_minutes": 15, "duration_hours": 10 }`) |
| `/api/crawl/trigger` | `POST` | Non-blocking manual crawl trigger |
| `/api/status` | `GET` | Health telemetry (BDIX gateway ping, RAM, item counts) |

---

## 🤖 Telegram Bot Commands

When configured with `TELEGRAM_BOT_TOKEN` and `TELEGRAM_ADMIN_CHAT_ID`:

- `/status` — Displays server uptime, BDIX connection status, RAM usage, and library counts.
- `/turbo [hours]` — Instantly activates high-frequency Turbo Scan (e.g., `/turbo 10` checks every 15 mins for the next 10 hours).
- `/scan` — Manually triggers an immediate crawl.

---

## 📄 License

MIT License. Built for personal homelab use.
