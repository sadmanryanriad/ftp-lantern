# AGENTS.md — FTP-Lantern Project Guide

Operational guidelines and architectural reference for `ftp-lantern`.

---

## 1. Context & Homelab Environment

- **Primary Homelab Host:** Asus Server (`192.168.0.134`, Ubuntu 24.04, 8GB RAM, Dokploy).
- **Target ISP Media Server:** `http://172.16.50.12/DHAKA-FLIX-12/` (BDIX HTTP media directory).
- **Deployment Platform:** Dokploy (managed via Docker Compose).
- **Telegram Admin Chat ID:** `1240674937`.

---

## 2. Core Guardrails & Networking

- **Network Mode (CRITICAL):**
  - Containers MUST run with `network_mode: host`.
  - Docker's default bridge network (`172.16.0.0/12`) overlaps directly with the ISP server's `172.16.50.12` IP. Using `network_mode: host` guarantees uninterrupted direct routing to the ISP server without NAT collision.
- **Port Assignment:** Default web UI port `8085` (configurable via `PORT` env var).
- **Resource Discipline:**
  - Single Go binary + embedded static frontend.
  - Runtime memory target: `< 30 MB` RAM.
  - Image size target: `< 25 MB`.
  - Container resource limits: `128 MB` RAM cap, `0.50` CPU cap.
  - Direct media streaming: media bytes NEVER transit through FTP-Lantern; client players stream directly from `172.16.50.12`.
- **Public Open-Source Hygiene & Strict Zero-Secret / Zero-PII Policy:**
  - This repository is designed to be a clean, reusable open-source project.
  - NEVER hardcode personal IPs, Telegram Chat IDs, Bot Tokens, or private hostnames in source code.
  - All configurations MUST be ingested via environment variables or `.env`.
  - Provide a safe `.env.example` with dummy placeholders only (`http://172.16.x.x/...`, `<YOUR_BOT_TOKEN>`, `123456789`).

---

## 3. Storage Layout

- `/data/lantern.db`: SQLite database (WAL mode + FTS5 trigram).
- `/data/thumbs/`: Resized JPEG poster thumbnails (~300px, cached with immutable headers).
