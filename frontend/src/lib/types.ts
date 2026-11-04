export interface Title {
  id: number;
  name: string;
  name_norm: string;
  year: number;
  type: 'series' | 'movie';
  poster_url?: string;
  thumb_hash?: string;
  is_following: boolean | number;
  first_seen: string;
  last_modified?: string;
  // Computed / aggregated fields from backend queries
  resolution?: string;
  source?: string;
  has_hdr?: boolean | number;
  episodes_count?: number;
  seasons_count?: number;
  last_played_code?: string;
  last_played_at?: string;
  next_file_id?: number;
  next_episode_code?: string;
  next_episode_name?: string;
  next_episode_url?: string;
}

export interface Release {
  id: number;
  title_id: number;
  source_server: string;
  category: string;
  folder_url: string;
  folder_name: string;
  resolution: string;
  source: string;
  has_hdr: boolean | number;
  first_seen: string;
  last_modified?: string;
  files?: MediaFile[];
}

export interface MediaFile {
  id: number;
  release_id: number;
  title_id: number;
  season_number?: number;
  episode_number?: number;
  episode_code?: string;
  name: string;
  file_url: string;
  file_size_bytes: number;
  resolution: string;
  source: string;
  has_hdr: boolean | number;
  first_seen: string;
  last_modified?: string;
  is_watched?: boolean;
}

export interface TitleDetail extends Title {
  releases: Release[];
  files: MediaFile[];
  seasons?: {
    season_number: number;
    episodes: MediaFile[];
  }[];
}

export interface ContinueWatchingItem {
  title_id: number;
  title_name: string;
  poster_url?: string;
  thumb_hash?: string;
  last_played_file_id: number;
  last_played_code: string;
  last_played_at: string;
  played_at?: string;
  next_file_id?: number;
  next_episode_code?: string;
  next_file_url?: string;
  resolution?: string;
}

export interface SystemStatus {
  bdix_connected: boolean;
  isp_gateway_ip?: string;
  titles_count: number;
  files_count: number;
  releases_count: number;
  memory_alloc_mb: number;
  memory_sys_mb: number;
  goroutines_count?: number;
  uptime_seconds?: number;
  last_scan_at?: string;
  is_crawling?: boolean;
  crawler_status?: string;
  telegram_error?: string;
}

export interface SchedulerStatus {
  interval_minutes: number;
  next_scan_time?: string;
  turbo_mode: boolean;
  turbo_until?: string;
  turbo_remaining_seconds?: number;
  is_crawling: boolean;
  last_scan_duration?: string;
}

export interface SearchResponse {
  results: Title[];
  total: number;
  page?: number;
  limit?: number;
}
