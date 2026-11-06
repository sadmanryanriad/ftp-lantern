import type { Title, TitleDetail, SystemStatus, SchedulerStatus, SearchResponse, ContinueWatchingItem } from './types';

export const API_BASE = '';

export async function fetchWithTimeout(url: string, opts: RequestInit = {}, timeoutMs = 5000): Promise<Response> {
  const controller = new AbortController();
  const id = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const res = await fetch(url, { ...opts, signal: controller.signal });
    return res;
  } finally {
    clearTimeout(id);
  }
}

export async function fetchStatus(): Promise<SystemStatus> {
  const res = await fetchWithTimeout(`${API_BASE}/api/status`, {}, 8000);
  if (!res.ok) throw new Error(`Status HTTP ${res.status}`);
  return res.json();
}

export async function fetchScheduler(): Promise<SchedulerStatus> {
  const res = await fetchWithTimeout(`${API_BASE}/api/scheduler`, {}, 8000);
  if (!res.ok) throw new Error(`Scheduler HTTP ${res.status}`);
  return res.json();
}

export function displayTitleWithYear(name: string, year?: number): string {
  if (!name) return '';
  if (year && year > 0 && !name.includes(`(${year})`)) {
    return `${name} (${year})`;
  }
  return name;
}

export async function searchTitles(params: {
  q?: string;
  resolution?: string;
  source?: string;
  type?: string;
  category?: string;
  year?: string;
  date_added?: string;
  following?: boolean;
  page?: number;
  limit?: number;
  sort?: string;
}): Promise<SearchResponse> {
  const query = new URLSearchParams();
  if (params.q) query.set('q', params.q);
  if (params.resolution && params.resolution !== 'All') query.set('resolution', params.resolution);
  if (params.source) query.set('source', params.source);
  if (params.type && params.type !== 'all') query.set('type', params.type);
  if (params.category && params.category !== 'All') query.set('category', params.category);
  if (params.year && params.year !== 'All') query.set('year', params.year);
  if (params.date_added && params.date_added !== 'all') query.set('date_added', params.date_added);
  if (params.following) query.set('following', 'true');
  if (params.page) query.set('page', params.page.toString());
  if (params.limit) query.set('limit', params.limit.toString());
  if (params.sort) query.set('sort', params.sort);

  const res = await fetchWithTimeout(`${API_BASE}/api/search?${query.toString()}`, {}, 10000);
  if (!res.ok) throw new Error(`Search HTTP ${res.status}`);
  const data = await res.json();
  // Handle both array response or { results: [...], total: ... }
  if (Array.isArray(data)) {
    return { results: data, total: data.length };
  }
  return data;
}

export async function fetchTitleDetail(id: number): Promise<TitleDetail> {
  const res = await fetchWithTimeout(`${API_BASE}/api/titles/${id}`, {}, 5000);
  if (!res.ok) throw new Error(`Title HTTP ${res.status}`);
  return res.json();
}

export async function toggleFollow(id: number): Promise<{ is_following: boolean }> {
  const res = await fetch(`${API_BASE}/api/titles/${id}/follow`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
  });
  if (!res.ok) throw new Error(`Toggle follow HTTP ${res.status}`);
  return res.json();
}

export function recordPlay(fileId: number, titleId?: number): void {
  const payload = JSON.stringify({ file_id: fileId, title_id: titleId });
  // Per architectural guardrail: sendBeacon guarantees play tracking even if focus switches
  if (typeof navigator !== 'undefined' && navigator.sendBeacon) {
    const blob = new Blob([payload], { type: 'text/plain' });
    navigator.sendBeacon(`${API_BASE}/api/plays`, blob);
  } else {
    fetch(`${API_BASE}/api/plays`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: payload,
    }).catch(err => console.warn('Failed to record play:', err));
  }
}

export async function markWatchedUpTo(fileId: number): Promise<void> {
  const res = await fetch(`${API_BASE}/api/plays/mark-watched`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ file_id: fileId }),
  });
  if (!res.ok) throw new Error(`Mark watched HTTP ${res.status}`);
}

export async function fetchWhatsNew(limit = 20): Promise<Title[]> {
  const res = await fetchWithTimeout(`${API_BASE}/api/new?limit=${limit}`, {}, 5000);
  if (!res.ok) throw new Error(`WhatsNew HTTP ${res.status}`);
  const data = await res.json();
  return Array.isArray(data) ? data : data.results || [];
}

export async function fetchContinueWatching(): Promise<ContinueWatchingItem[]> {
  try {
    const res = await fetchWithTimeout(`${API_BASE}/api/titles?continue_watching=true`, {}, 4000);
    if (!res.ok) {
      // Fallback: check search or plays endpoint
      return [];
    }
    const data = await res.json();
    return Array.isArray(data) ? data : data.results || [];
  } catch {
    return [];
  }
}

export async function setTurboMode(durationHours: number, intervalMinutes = 15): Promise<void> {
  const res = await fetch(`${API_BASE}/api/scheduler/turbo`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      duration_hours: durationHours,
      interval_minutes: intervalMinutes,
    }),
  });
  if (!res.ok) throw new Error(`Set turbo HTTP ${res.status}`);
}

export async function triggerCrawl(): Promise<{ message: string; started?: boolean }> {
  const res = await fetch(`${API_BASE}/api/crawl/trigger`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error(`Trigger crawl HTTP ${res.status}`);
  return res.json();
}

export async function setSchedulerFrequency(frequency: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/scheduler/frequency`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ frequency }),
  });
  if (!res.ok) throw new Error(`Set frequency HTTP ${res.status}`);
}

export async function triggerDbBackup(): Promise<{ ok: boolean; backup_path: string }> {
  const res = await fetch(`${API_BASE}/api/backup`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error(`Backup HTTP ${res.status}`);
  return res.json();
}

export async function importDbBackup(file: File): Promise<{ ok: boolean; message: string; titles_count: number }> {
  const formData = new FormData();
  formData.append('database', file);
  const res = await fetch(`${API_BASE}/api/backup/import`, {
    method: 'POST',
    body: formData,
  });
  if (!res.ok) {
    const txt = await res.text();
    throw new Error(txt || `Import HTTP ${res.status}`);
  }
  return res.json();
}

export async function getNotificationMode(): Promise<string> {
  try {
    const res = await fetch(`${API_BASE}/api/settings/notifications`);
    if (!res.ok) return 'following_only';
    const data = await res.json();
    return data.mode || 'following_only';
  } catch {
    return 'following_only';
  }
}

export async function setNotificationMode(mode: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/settings/notifications`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ mode }),
  });
  if (!res.ok) throw new Error(`Set notification mode HTTP ${res.status}`);
}

export async function copyToClipboard(text: string): Promise<boolean> {
  if (typeof navigator !== 'undefined' && navigator.clipboard && navigator.clipboard.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // Fall through to textarea fallback
    }
  }

  // Fallback for non-secure HTTP contexts on LAN
  try {
    const textArea = document.createElement('textarea');
    textArea.value = text;
    textArea.style.position = 'fixed';
    textArea.style.left = '-999999px';
    textArea.style.top = '-999999px';
    textArea.setAttribute('readonly', '');
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();
    const successful = document.execCommand('copy');
    textArea.remove();
    return successful;
  } catch (err) {
    console.error('Fallback copy failed:', err);
    return false;
  }
}

export function playOnMPV(streamUrl: string, fileId?: number): void {
  if (fileId) recordPlay(fileId);

  // 1. Copy direct mpv CLI command to clipboard automatically as a handy backup
  copyToClipboard(getMPVCommand(streamUrl));

  // 2. Direct 1-Click Launch via native mpv:// protocol handler (exactly like potplayer://)
  window.location.href = `mpv://${streamUrl}`;
}

export function getMPVCommand(streamUrl: string): string {
  return `mpv "${streamUrl}"`;
}

export function getThumbUrl(title: { id: number; thumb_hash?: string; poster_url?: string }): string {
  if (title.thumb_hash) {
    return `${API_BASE}/api/thumbs/${title.id}-${title.thumb_hash}.jpg`;
  }
  if (title.poster_url) {
    return title.poster_url;
  }
  return '';
}

export function formatBytes(bytes: number, decimals = 2): string {
  if (!bytes || bytes === 0) return '0 B';
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`;
}

export function getAndroidIntentUrl(streamUrl: string, titleName?: string): string {
  try {
    const cleanUrl = streamUrl.trim();
    let scheme = 'http';
    let rawWithoutScheme = cleanUrl;

    if (cleanUrl.startsWith('https://')) {
      scheme = 'https';
      rawWithoutScheme = cleanUrl.substring(8);
    } else if (cleanUrl.startsWith('http://')) {
      scheme = 'http';
      rawWithoutScheme = cleanUrl.substring(7);
    }

    const cleanTitle = titleName ? encodeURIComponent(titleName) : '';
    // Standard Chrome Android Intent URI syntax:
    // Triggers Android's native "Open with" system dialog listing all installed players (VLC, MX Player, MPV, Just Player, etc.)
    let intent = `intent://${rawWithoutScheme}#Intent;action=android.intent.action.VIEW;scheme=${scheme};type=video/*;`;
    if (cleanTitle) {
      intent += `S.title=${cleanTitle};`;
    }
    intent += 'end';
    return intent;
  } catch {
    return streamUrl;
  }
}

export function playOnAndroid(fileId: number, streamUrl: string, titleName?: string): void {
  if (fileId) recordPlay(fileId);
  const intent = getAndroidIntentUrl(streamUrl, titleName);
  window.location.href = intent;
}

export function playOnVLC(streamUrl: string, fileId?: number): void {
  if (fileId) {
    recordPlay(fileId);
  }
  // Native VLC URI scheme
  window.location.href = `vlc://${streamUrl}`;
}

export function downloadM3U(url: string, fileId?: number): void {
  if (fileId) {
    recordPlay(fileId);
  }
  const link = document.createElement('a');
  link.href = url;
  link.setAttribute('download', '');
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
}
