# FTP-Lantern — System Architecture & Implementation Reference (v3.4)

**FTP-Lantern** is an ultra-lightweight, zero-transcoding media indexing, instant search engine, and native streaming launcher tailored for BDIX ISP HTTP media portals (specifically Dhaka-FLIX on `172.16.50.12`, `14`, `7`, `8`, `9`).

It compiles into a single static Go binary with an embedded Svelte 5 SPA, incrementally mirrors ISP directory trees using HTTP HTML autoindex `Last-Modified` change detection, caches content-hashed poster thumbnails, provides sub-millisecond FTS5 search with typo tolerance, launches direct external streaming (Windows MPV/VLC, Android VLC/MPV/MX Player) with zero media proxying, and sends scheduled daily digests as well as instant notifications for followed shows.

---

## 1. High-Level Architecture & Operating Model

```
ISP BDIX Server (172.16.50.12, 14, 7, 9 / DHAKA-FLIX-*)
          │  Incremental HTTP crawl (Autoindex Last-Modified check + Hot-Set Season re-list)
          ▼
┌───────────────────────── ftp-lantern (Single Go Binary) ─────────────────────────┐
│                                                                                  │
│  ┌────────────────────┐     ┌──────────────────┐     ┌────────────────────────┐  │
│  │   Crawler Engine   │ ──► │   Diff Engine    │ ──► │  SQLite 3 (WAL mode)   │  │
│  │  (Low-Duty Daemon) │     │ (CircuitBreaker) │     │  /data/lantern.db      │  │
│  └─────────┬──────────┘     └────────┬─────────┘     │  - titles, releases    │  │
│            │                         │               │  - files, settings     │  │
│            ▼                         ▼               │  - events (notif queue)│  │
│    Poster Worker (Q=1)         Telegram Bot          │  - FTS5 trigram index  │  │
│  (Single worker, max 96MB,  (08:01 AM Digest         └───────────┬────────────┘  │
│   /data/thumbs/:id-:hash)    + Instant Follow Alerts             ▲               │
│                              + Bot Listener)                     │ REST API      │
│                                                                  │               │
│                                                      ┌───────────┴────────────┐  │
│                                                      │  Embedded Web UI       │  │
│                                                      │  (Svelte 5 / Vite SPA) │  │
│                                                      │  - History API Nav     │  │
│                                                      │  - MPV / Android       │  │
│                                                      │  - Multi-Block Shelves │  │
│                                                      └────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────────────┘
          │
          ├─► Windows Desktops: mpv:// (Direct 1-Click Launch) or /play/season/:id.m3u in MPV / VLC
          ├─► Android Devices: intent://... direct native player chooser (VLC, MX Player, MPV)
          ├─► Dokploy Volume: ftp-lantern-data:/data (Persists lantern.db across deploys)
          └─► Telegram Client: Daily 08:01 AM digest + Instant episode alerts on Follow + Bot listener
```

> [!IMPORTANT]
> **Zero 24/7 Background Indexing Loops:**  
> The entire ISP library (130,409 files across 28,484 titles) is **already 100% indexed** in persistent storage.  
> The crawler engine does **NOT** run in a continuous indexing loop.  
> Routine scheduled execution runs **Twice Daily** (at **07:00 AM** and **07:00 PM Asia/Dhaka**), completing in **$< 3 \text{ seconds}$** using timestamp-based skipping.  
> Outside of these two runs, the crawler is completely dormant: **Host CPU usage is $0.0\%$, and RAM is $< 25 \text{ MB}$**.

---

## 2. Technology Stack & Runtime Bounds

| Layer | Selection | Justification & Performance Characteristics |
| :--- | :--- | :--- |
| **Backend Runtime** | **Go 1.26 (`net/http`)** | Single static binary, zero runtime dependencies. Idle RAM $< 25 \text{ MB}$. Goroutines handle concurrent crawling without OS thread overhead. |
| **Timezone & CLI** | **`_ "time/tzdata"` + CLI subcommands** | Embedded timezone database for accurate scheduled runs (`Asia/Dhaka`). Built-in `healthcheck` subcommand eliminates `curl` requirement in `scratch` containers. |
| **Database** | **SQLite + FTS5 (`modernc.org/sqlite`)** | Pure Go SQLite engine (zero CGO). Runs in WAL mode with `synchronous = NORMAL`. Indexed queries execute in **$< 1 \text{ ms}$**. Zero external database containers needed. |
| **Search Engine** | **FTS5 Trigram via Parameterized `LIKE`** | SQLite accelerates parameterized `LIKE '%' || ? || '%'` directly through FTS5 trigram indexes with automated sync triggers. Substring sliding-window Levenshtein fallback handles typos. |
| **Image Pipeline** | **`disintegration/imaging` (Concurrency = 1)** | Dedicated single-worker queue prevents memory spikes. Resizes posters to ~300px JPEG Q75. Immutable caching headers. |
| **Crawler Parsing** | **`PuerkitoBio/goquery` (HTML DOM)** | Robust table parser for Dhaka-FLIX custom HTML autoindex structures (`td.fb-n`, `td.fb-d`, `td.fb-s`). |
| **Frontend** | **Svelte 5 (Runes) + Tailwind CSS** | Bundled and embedded directly into the Go binary via `//go:embed`. Initial bundle $< 45 \text{ KB}$ gzipped. Multi-block home layout, horizontal scrolling categories, and History API modal navigation. |
| **Deployment** | **Dokploy / Docker (`network_mode: host`)** | Minimal `scratch` container ($< 25 \text{ MB}$). Direct host routing to bypass Docker bridge NAT collisions with `172.16.50.12`. Named persistent volume for `/data`. |

---

## 3. Storage Persistence & Dokploy Volume Setup

### 3.1 Understanding Dokploy Compose vs Application Volumes
- In Dokploy, when deploying via **Docker Compose** (`docker-compose.yml`), Dokploy's UI "Volumes / Mounts" dialog creates **File Mounts** (mounting custom config files like `.env` into the container).
- Compose volume storage is governed **directly inside `docker-compose.yml`**.
- If a volume is undeclared or anonymous, Dokploy commands like `up -d --build --remove-orphans` recreate containers with a clean ephemeral layer, losing `/data/lantern.db`.

### 3.2 Permanent Persistence Configuration (Two Approved Methods)

#### Method A: Named External Docker Volume (Recommended)
1. SSH into the homelab server or run once via Dokploy host terminal:
   ```bash
   docker volume create ftp-lantern-data
   ```
2. In `docker-compose.yml`, define the volume:
   ```yaml
   services:
     ftp-lantern:
       volumes:
         - lantern-data:/data
   
   volumes:
     lantern-data:
       name: ftp-lantern-data
   ```
   Because `ftp-lantern-data` is a persistent named volume, Docker and Dokploy will **NEVER** delete or wipe it during git pulls, rebuilds, or container redeployments.

#### Method B: Host Bind Mount
If direct host visibility is preferred:
```yaml
    volumes:
      - /var/lib/ftp-lantern/data:/data
```

### 3.3 Safe, Non-Locking Database Snapshots (`VACUUM INTO`)
Copying a live SQLite database in WAL mode (`lantern.db`) while active writes are happening can yield an inconsistent or corrupt copy.
FTP-Lantern provides a built-in snapshot command:
```sql
VACUUM INTO '/data/lantern_backup.db';
```
- Endpoint: `POST /api/backup` (available in Settings UI).
- Creates an atomic, standalone, defragmented snapshot of the entire database into `/data/lantern_backup.db` without locking queries or blocking the crawler.

---

## 4. Ultra-Fast Incremental Tree Indexing & Change Detection

### 4.1 The ISP Directory Tree Hierarchy
The BDIX servers organize media into a structured tree:
- **Level 0 (Source Roots):** `SOURCE_URLS` entries (e.g. `http://172.16.50.12/DHAKA-FLIX-12/TV-WEB-Series/`, `http://172.16.50.14/DHAKA-FLIX-14/English Movies (1080p)/`).
- **Level 1 (Subcategories / Alphabetical Folders):** e.g. `TV Series ♦ A — L/`, `TV Series ♦ M — R/`, `(2026) 1080p/`, or direct title listings.
- **Level 2 (Title Folders):** e.g. `Breaking Bad/`, `Inception (2010)/`.
- **Level 3 (Season Folders / Files):** `Season 01/`, `Season 02/` or direct `.mkv`/`.mp4` media files.
- **Level 4 (Episode Files):** `Breaking.Bad.S01E01.1080p.mkv`.

### 4.2 Autoindex Table Timestamps & Normalization Invariant
> [!IMPORTANT]
> **The Modernc SQLite DATETIME Normalization Invariant:**  
> - Dhaka-FLIX HTML autoindex tables (`td.fb-d`) provide timestamps formatted as: `YYYY-MM-DD HH:MM` (e.g., `2025-04-24 13:28`).  
> - SQLite's pure-Go driver (`modernc.org/sqlite`) serializes `DATETIME` columns into RFC3339 strings (`2025-04-24T13:28:00Z`).  
> - **CRITICAL FIX:** A direct raw string comparison (`it.LastModified == recordedMtime`) will **always fail**, erroneously forcing the crawler to re-descend into every single one of the 34,000+ folders on every crawl.  
> - Timestamps must be normalized (e.g., matching the canonical prefix `YYYY-MM-DD HH:MM` or stripping non-digits).  
> - When normalized, checking 34,000 folders against SQLite indexed `releases(folder_url)` executes in **$< 100 \text{ ms}$**, skipping 100% of unchanged folders without making recursive HTTP GET requests.

### 4.3 Two-Tier Incremental Crawl Mechanics

```mermaid
flowchart TD
    Start["Scheduled Crawl (07:00 AM / 07:00 PM)"] --> FetchCat["Fetch Source Category URLs (~12 fast HTTP GETs)"]
    FetchCat --> CheckTitle{"For each Title: Compare Normalized Timestamp with DB"}
    CheckTitle -- "Timestamp Matches" --> Skip["SKIP DESCENT (0 HTTP requests)"]
    CheckTitle -- "Brand New or Timestamp Changed" --> Descend["Descend into Title Folder"]
    Descend --> ParseFiles["Parse Seasons & Episodes -> Upsert into SQLite"]
    Skip --> Next["Next Title Folder"]
    ParseFiles --> Next
    Next --> HotSet["Check Hot-Set (Followed Shows + Top 150 Recent Releases)"]
    HotSet --> Finish["Crawl Finished (< 3 seconds total)"]
```

1. **Category Listing Check (Level-1 Skip):**
   - The crawler lists the ~12 source category URLs.
   - For every title folder found in the HTML table, it queries SQLite:
     ```sql
     SELECT id, last_modified FROM releases WHERE folder_url = ?
     ```
   - If the folder exists and its normalized timestamp matches the HTML table's timestamp, **the crawler skips descending completely**.
   - Checking 34,000 title folders takes **$< 100 \text{ ms}$** with 0 recursive requests.
2. **Descent Only on Modification:**
   - A descent occurs **only** when:
     - The folder URL is brand new (new movie or newly added series).
     - The folder timestamp changed (a new season was added to an existing show).
3. **Hot-Set Season Check (Catches Unfollowed Ongoing Shows):**
   - When an episode is added inside `Season 01/` (e.g. `S01E08.mkv`), `Season 01/`'s timestamp updates, but the title folder timestamp may not propagate.
   - The crawler re-lists season folders for followed shows and the top 150 releases from the last 14 days (`LIMIT 150`):
     ```sql
     SELECT t.id, r.id, r.folder_url, r.folder_name, r.category
     FROM titles t
     JOIN releases r ON r.title_id = t.id
     WHERE r.gone_at IS NULL AND (
         t.is_following = 1 OR (
             SELECT MAX(f.last_modified) FROM files f WHERE f.release_id = r.id AND f.gone_at IS NULL
         ) > datetime('now', '-14 days')
     )
     ORDER BY t.is_following DESC, r.last_modified DESC
     LIMIT 150
     ```
   - Completes in **$< 3 \text{ seconds}$**.
4. **Non-Blocking Crawl Lock:**
   - Protected by `sync.Mutex.TryLock()`. If a crawl is already running, triggering a new crawl returns `409 Conflict ("Crawl already in progress")`.
5. **Safe Diff & Subtree Mass-Deletion Guard:**
   - Network errors, ISP timeouts, or 5xx responses never trigger deletions.
   - If $> 20\%$ of items disappear within a scanned subtree (minimum threshold $\ge 10$ items), all deletions in that subtree are aborted, and an alert is logged.
6. **Strict URL Percent-Encoding:**
   - URLs are constructed strictly using Go's `net/url` package (`url.URL{Path: ...}`), never raw string concatenation.

---

## 5. Crawler Duty Cycle, Scheduler & Notification Queue

### 5.1 Startup Behavior & Catch-Up Logic
- **No Redundant Re-Scans:** When the container starts or restarts:
  - It checks `SELECT COUNT(*) FROM titles`.
  - If titles exist, it inspects `last_crawl_time`.
  - If the last crawl was recent ($< 24 \text{ hours}$ ago), it **skips the boot crawl** and enters standby mode immediately.
  - If the last crawl was missed ($> 24 \text{ hours}$ ago due to host downtime) or if the database is empty (fresh install), it performs a catch-up/seed crawl.

### 5.2 Configurable Crawl Frequency (Settings UI)
- `Twice Daily (7:00 AM & 7:00 PM)` — **Recommended Default**: Ideal for homelab resource discipline; checks for morning and evening uploads with zero continuous CPU overhead.
- `Daily at 7:00 AM`: Single morning scan + 08:01 AM digest.
- `Hourly Check`: Runs once every 60 minutes.
- `15-Min Pulse`: Continuous incremental checks.
- `Manual Scan Only`: Crawls only when the user clicks "Run Scan Now" or sends `/scan` on Telegram.

### 5.3 Decoupled Event Queue & Delivery Guarantees
```sql
CREATE TABLE IF NOT EXISTS events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    kind            TEXT NOT NULL,
    title_id        INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
    release_id      INTEGER REFERENCES releases(id) ON DELETE CASCADE,
    payload_json    TEXT NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    notified_at     DATETIME,
    instant_sent_at DATETIME,
    digest_sent_at  DATETIME,
    failed_at       DATETIME
);
```

1. **Silent Initial Seed:**
   - During the first crawl (`isSeed == true`), events are inserted with `instant_sent_at = CURRENT_TIMESTAMP` and `digest_sent_at = CURRENT_TIMESTAMP`.
   - Completely prevents blasting the entire library to Telegram on initial setup.
2. **Instant Follow Alerts (⭐ Starred Shows):**
   - As soon as a crawl detects a new episode for a followed title (`is_following = 1`), it dispatches immediately and sets `instant_sent_at = CURRENT_TIMESTAMP`.
3. **Daily Digest (08:01 AM Asia/Dhaka):**
   - Dispatches a consolidated HTML morning report summarizing all updates where `digest_sent_at IS NULL AND failed_at IS NULL`.
   - If Telegram is unreachable, items remain in the queue with `digest_sent_at IS NULL` and are delivered on the next successful run without data loss.

### 5.4 Telegram Bot & Notifier Specification
- **Strict HTML Escaping:** Dynamic title strings are escaped with `html.EscapeString()`.
- **Item-Boundary Truncation:** Appends items measuring escaped length; stops cleanly before 3,800 characters, appending `...and X more`.
- **Error Classification:**
  - `401 / 403`: Pauses notifier and alerts admin.
  - `429 Rate Limit`: Sleeps for backoff duration.
  - `400 Bad Entity`: Marks only that single event as `failed_at = CURRENT_TIMESTAMP` to prevent queue deadlocks.
- **Long-Polling Bot Listener:**
  - `/status` — Reports total titles, total files, and system health.
  - `/scan` — Triggers an immediate crawl.

---

## 6. Instant Search with Parameterized Trigram FTS5 & Typo Tolerance

```mermaid
flowchart TD
    Query["User Types Search Query"] --> Norm["Normalize: Lowercase & Strip Punctuation/Spaces"]
    Norm --> CheckLen{"Query Length >= 3?"}
    CheckLen -- "No (< 3 chars)" --> LikeSearch["Execute SQL: name_norm LIKE ?%"]
    CheckLen -- "Yes (>= 3 chars)" --> TrigramSearch["Execute FTS: name_norm LIKE '%' || ? || '%'"]
    TrigramSearch --> CheckCount{"Hits >= 3?"}
    CheckCount -- "Yes" --> Results["Return Ranked Results"]
    CheckCount -- "No (< 3 hits)" --> WindowFuzzy["Sliding-Window Levenshtein <= 2 Fallback"]
    LikeSearch --> Results
    WindowFuzzy --> Results
```

### 6.1 FTS5 Automated Sync Triggers
```sql
CREATE TRIGGER IF NOT EXISTS titles_ai AFTER INSERT ON titles BEGIN
    INSERT INTO titles_fts(rowid, name_norm) VALUES (new.id, new.name_norm);
END;

CREATE TRIGGER IF NOT EXISTS titles_ad AFTER DELETE ON titles BEGIN
    INSERT INTO titles_fts(titles_fts, rowid, name_norm) VALUES('delete', old.id, old.name_norm);
END;

CREATE TRIGGER IF NOT EXISTS titles_au AFTER UPDATE OF name_norm ON titles BEGIN
    INSERT INTO titles_fts(titles_fts, rowid, name_norm) VALUES('delete', old.id, old.name_norm);
    INSERT INTO titles_fts(rowid, name_norm) VALUES (new.id, new.name_norm);
END;
```

---

## 7. 1-Click Streaming & Native Player Launchers

### 7.1 Desktop Playback (Windows)
- **Primary Launcher — MPV Player:**
  - Button launches: `mpv://http://172.16.50.12/DHAKA-FLIX-12/.../video.mkv`
  - Uses the Windows `mpv://` URL protocol handler to trigger direct zero-download native streaming.
  - Automatically copies the terminal CLI command (`mpv "http://..."`) to clipboard as a backup.
- **Secondary Launcher — VLC Media Player:**
  - Button launches: `vlc://http://172.16.50.12/DHAKA-FLIX-12/.../video.mkv`.
- **M3U Playlist Continuous Streaming:**
  - `GET /play/season/:id.m3u?season=X` serves full-season playlist with auto-next and `F8` playlist view in MPV / VLC.
  - `GET /play/series/:id.m3u` serves complete multi-season playlist for binge watching.
- **Copy Stream URL:** 1-click clipboard copy of the direct HTTP stream link.

### 7.2 Mobile Playback (Android)
- Uses Chrome standard Intent URI format:
  `intent://172.16.50.12/.../video.mkv#Intent;action=android.intent.action.VIEW;scheme=http;type=video/*;end`
- Opens Android's native "Open with" app chooser, allowing direct streaming with VLC, MX Player, MPV, or Nova Video Player.
- Smartphone displays hide desktop buttons (MPV, VLC, CLI) to maintain a clean 44px touch-friendly mobile action bar.

### 7.3 No Watch History Policy
- Watch history recording is completely disabled per homelab privacy preference.
- No `plays` records are created on playback trigger.
- The "Continue Watching" shelf is omitted from the frontend UI.

---

## 8. Frontend UI Architecture

### 8.1 Multi-Block Home Architecture
1. **Hero & Instant Search:** Real-time search with 100ms debounce, quick play shortcut, and disk telemetry pill.
2. **Block 1 — What's New on BDIX:** Expandable shelf displaying newly discovered media.
3. **Block 2 — Featured Movies Shelf:** Direct shelf featuring top movies with ISP category chips (*English Movies*, *English 1080p*, *Hindi*, *South Hindi Dubbed*, *Bangla*, *Animation*, *IMDb Top-250*) and an "Explore All Movies" button.
4. **Block 3 — TV & Web Series Shelf:** Direct shelf featuring series with ISP quick filters (*English*, *Hindi*, *Bengali*, *Korean*, *Anime & Cartoons*, *Documentaries*) and an "Explore All Series" button.

### 8.2 Horizontal Scrolling Quick Filter Chips
- Container styles: `flex items-center gap-2 overflow-x-auto whitespace-nowrap w-full min-w-0 scrollbar-thin scrollbar-thumb-zinc-700 scrollbar-track-zinc-900/50`.
- Buttons enforce `flex-shrink-0` to guarantee text labels never truncate.
- Touch momentum enabled with `-webkit-overflow-scrolling: touch`.

### 8.3 History API Navigation & Deep-Link Guard
- Modals push state: `window.history.pushState({ modal: 'title', id }, '', '#/title/' + id)`.
- `window.addEventListener('popstate')` cleanly closes open modals.
- If a user opens a direct deep link from Telegram (`#/title/42`), closing the modal uses `window.history.replaceState` to clear the hash instead of `history.back()`, preventing accidental exit.

---

## 9. Full REST API Specification

| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/api/search`, `/api/v1/search` | `GET` | Instant search & filtering (`q`, `type`, `category`, `resolution`, `year`, `date_added`, `sort`, `include_files`, `page`, `limit`) |
| `/api/titles` | `GET` | Paginated library titles (`continue_watching=true` returns active watch shelf) |
| `/api/titles/:id` | `GET` | Detailed title payload with releases, files, streaming URLs, `folder_url`, and `web_url` |
| `/api/titles/:id/follow` | `POST` | Toggle watchlist follow status (⭐) |
| `/api/new` | `GET` | Feed of recently indexed content (`since`, `limit`) with `web_url` and `folder_url` |
| `/api/thumbs/:filename` | `GET` | Content-hashed poster thumbnail with `Cache-Control: immutable` |
| `/play/file/:id.m3u` | `GET` | Ephemeral single-file M3U playlist file |
| `/play/season/:id.m3u` | `GET` | Full season M3U playlist file with natural episode ordering |
| `/api/scheduler` | `GET` | Current crawl interval, frequency, and Turbo Mode status |
| `/api/scheduler/turbo` | `POST` | Enable Turbo Mode (`{ "interval_minutes": 15, "duration_hours": 10 }`) |
| `/api/scheduler/frequency` | `POST` | Set crawl frequency (`twice_daily`, `daily`, `1h`, `15m`, `manual`) |
| `/api/crawl/trigger` | `POST` | Immediate manual crawl execution (non-blocking `TryLock`) |
| `/api/backup` | `POST` | Generates a non-locking SQLite snapshot via `VACUUM INTO` |
| `/api/status` | `GET` | Health status: ISP reachability, item counts, memory, and disk usage |

### 9.1 External Integration Architecture (Homelab Dashboard & Telegram Bot Extension)

FTP-Lantern is designed as an independent microservice within a larger homelab ecosystem. In homelab setups where a central dashboard (e.g. `home.bakeartbd.com`) or a primary management bot already controls Telegram interactions:

- **Decoupled Bot Model:** FTP-Lantern handles high-performance indexing, metadata enrichment, poster caching, and stream link resolution. It does not need to own inbound Telegram polling when an upstream bot exists.
- **Local Network Querying:** Upstream services or bots query `http://<host>:8087/api/v1/search?q=<query>&limit=5` directly over the host network or internal bridge.
- **Universal CORS:** Requests from any local or external frontend (e.g., custom web dashboards) are permitted via `Access-Control-Allow-Origin: *`.
- **Enriched Links:** Search results return both `web_url` (deep link into FTP-Lantern title view) and `folder_url` (direct link to the ISP media server), along with `thumb_full_url` for immediate Telegram photo previews.
- **Atomic Single-Hop Retrieval:** Supplying `include_files=true` (or `1`) embeds the complete file/stream array into the search response, eliminating N+1 queries.

```
┌────────────────────────────────────────────────────────┐
│             Central Homelab Dashboard / Bot            │
│                 (Owns Telegram Session)                │
└───────────────────────────┬────────────────────────────┘
                            │
               GET /api/v1/search?q=query
                            │
                            ▼
┌────────────────────────────────────────────────────────┐
│                  FTP-Lantern Microservice              │
│       - SQLite Trigram FTS5 + Typo Tolerant Index      │
│       - Direct stream link & folder_url resolver       │
│       - Returns web_url, folder_url, thumb_full_url   │
└────────────────────────────────────────────────────────┘
```

---

## 10. Multi-Stage Dockerfile & Compose

### 10.1 `Dockerfile`
```dockerfile
ARG GO_VERSION=1.26

# Stage 1: Build Frontend Assets
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2: Build Static Go Binary
FROM golang:${GO_VERSION}-alpine AS backend-builder
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /app/frontend/dist ./server/ui/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /ftp-lantern .

# Stage 3: Minimal Scratch Container
FROM scratch
WORKDIR /
COPY --from=backend-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=backend-builder /ftp-lantern /ftp-lantern
VOLUME ["/data"]
EXPOSE 8085
HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
  CMD ["/ftp-lantern", "healthcheck"]
ENTRYPOINT ["/ftp-lantern"]
```

### 10.2 `docker-compose.yml`
```yaml
services:
  ftp-lantern:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: ftp-lantern
    network_mode: host
    restart: unless-stopped
    volumes:
      - lantern-data:/data
    environment:
      - PORT=8087
      - SOURCE_URLS=http://172.16.50.12/DHAKA-FLIX-12/TV-WEB-Series/,http://172.16.50.14/DHAKA-FLIX-14/English%20Movies%20%281080p%29/,http://172.16.50.7/DHAKA-FLIX-7/English%20Movies/,http://172.16.50.14/DHAKA-FLIX-14/Hindi%20Movies/,http://172.16.50.14/DHAKA-FLIX-14/IMDb%20Top-250%20Movies/,http://172.16.50.14/DHAKA-FLIX-14/Animation%20Movies%20%281080p%29/,http://172.16.50.14/DHAKA-FLIX-14/KOREAN%20TV%20%26%20WEB%20Series/,http://172.16.50.9/DHAKA-FLIX-9/Anime%20%26%20Cartoon%20TV%20Series/,http://172.16.50.7/DHAKA-FLIX-7/Kolkata%20Bangla%20Movies/,http://172.16.50.7/DHAKA-FLIX-7/Foreign%20Language%20Movies/,http://172.16.50.14/DHAKA-FLIX-14/SOUTH%20INDIAN%20MOVIES/South%20Movies/,http://172.16.50.14/DHAKA-FLIX-14/SOUTH%20INDIAN%20MOVIES/Hindi%20Dubbed/
      - BASE_CRAWL_INTERVAL_MINUTES=720
      - CRAWL_CONCURRENCY=4
      - GOMEMLIMIT=96MiB
      - MAX_THUMBS_MB=250
      - PUBLIC_BASE_URL=${PUBLIC_BASE_URL:-http://localhost:8087}
      - TELEGRAM_BOT_TOKEN=${TELEGRAM_BOT_TOKEN}
      - TELEGRAM_ADMIN_CHAT_ID=${TELEGRAM_ADMIN_CHAT_ID}
      - TZ=Asia/Dhaka
      - LOG_LEVEL=info
    deploy:
      resources:
        limits:
          cpus: '0.50'
          memory: 128M

volumes:
  lantern-data:
    name: ftp-lantern-data
```
