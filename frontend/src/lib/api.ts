import type { Title, TitleDetail, SystemStatus, SchedulerStatus, SearchResponse, ContinueWatchingItem } from './types';

export const API_BASE = '';

export async function fetchStatus(): Promise<SystemStatus> {
  const res = await fetch(`${API_BASE}/api/status`);
  if (!res.ok) throw new Error(`Status HTTP ${res.status}`);
  return res.json();
}

export async function fetchScheduler(): Promise<SchedulerStatus> {
  const res = await fetch(`${API_BASE}/api/scheduler`);
  if (!res.ok) throw new Error(`Scheduler HTTP ${res.status}`);
  return res.json();
}

export async function searchTitles(params: {
  q?: string;
  resolution?: string;
  source?: string;
  type?: string;
  page?: number;
  limit?: number;
  sort?: string;
}): Promise<SearchResponse> {
  const query = new URLSearchParams();
  if (params.q) query.set('q', params.q);
  if (params.resolution && params.resolution !== 'All') query.set('resolution', params.resolution);
  if (params.source) query.set('source', params.source);
  if (params.type && params.type !== 'all') query.set('type', params.type);
  if (params.page) query.set('page', params.page.toString());
  if (params.limit) query.set('limit', params.limit.toString());
  if (params.sort) query.set('sort', params.sort);

  const res = await fetch(`${API_BASE}/api/search?${query.toString()}`);
  if (!res.ok) throw new Error(`Search HTTP ${res.status}`);
  const data = await res.json();
  // Handle both array response or { results: [...], total: ... }
  if (Array.isArray(data)) {
    return { results: data, total: data.length };
  }
  return data;
}

export async function fetchTitleDetail(id: number): Promise<TitleDetail> {
  const res = await fetch(`${API_BASE}/api/titles/${id}`);
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
  const res = await fetch(`${API_BASE}/api/new?limit=${limit}`);
  if (!res.ok) throw new Error(`WhatsNew HTTP ${res.status}`);
  const data = await res.json();
  return Array.isArray(data) ? data : data.results || [];
}

export async function fetchContinueWatching(): Promise<ContinueWatchingItem[]> {
  try {
    const res = await fetch(`${API_BASE}/api/titles?continue_watching=true`);
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

export function getAndroidIntentUrl(streamUrl: string, titleName: string): string {
  // VLC / MX Player Android intent scheme
  const cleanTitle = encodeURIComponent(titleName);
  return `intent:${streamUrl}#Intent;type=video/*;title=${cleanTitle};package=org.videolan.vlc;S.browser_fallback_url=${encodeURIComponent(streamUrl)};end;`;
}

export function playOnAndroid(fileId: number, streamUrl: string, titleName: string): void {
  recordPlay(fileId);
  const intent = getAndroidIntentUrl(streamUrl, titleName);
  window.location.href = intent;
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
