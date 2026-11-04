<script lang="ts">
  import {
    X,
    Play,
    Download,
    Copy,
    Check,
    Star,
    ExternalLink,
    Smartphone,
    Monitor,
    Tv,
    Film,
    HardDrive,
    Eye
  } from '@lucide/svelte';
  import type { TitleDetail, MediaFile } from '../types';
  import {
    getThumbUrl,
    formatBytes,
    downloadM3U,
    playOnAndroid,
    playOnPotPlayer,
    playOnVLC,
    recordPlay,
    toggleFollow,
    displayTitleWithYear
  } from '../api';
  import PosterImage from './PosterImage.svelte';
  import ResolutionBadge from './ResolutionBadge.svelte';

  let {
    title = null,
    onClose = () => {},
    onPreview = (_file: MediaFile) => {}
  }: {
    title: TitleDetail | null;
    onClose: () => void;
    onPreview: (file: MediaFile) => void;
  } = $props();

  let activeSeason = $state<number>(1);
  let copiedFileId = $state<number | null>(null);
  let isFollowing = $state<boolean>(false);

  // Group files by season
  const allFiles = $derived.by(() => {
    if (!title) return [];
    return title.files || (title as any).episodes || [];
  });

  const seasonsMap = $derived.by(() => {
    const list = allFiles;
    if (!list || list.length === 0) return new Map<number, MediaFile[]>();
    const map = new Map<number, MediaFile[]>();

    for (const file of list) {
      const s = file.season_number ?? 1;
      if (!map.has(s)) map.set(s, []);
      map.get(s)!.push(file);
    }

    // Sort episodes within each season
    for (const [_s, l] of map.entries()) {
      l.sort((a, b) => {
        const epA = a.episode_number ?? 0;
        const epB = b.episode_number ?? 0;
        return epA - epB;
      });
    }

    return map;
  });

  const availableSeasons = $derived.by(() => {
    return Array.from(seasonsMap.keys()).sort((a, b) => a - b);
  });

  const currentEpisodes = $derived.by(() => {
    if (title?.type === 'movie') {
      return allFiles;
    }
    return seasonsMap.get(activeSeason) || allFiles;
  });

  $effect(() => {
    if (title) {
      isFollowing = Boolean(title.is_following);
      const keys = Array.from(seasonsMap.keys());
      if (keys.length > 0 && !keys.includes(activeSeason)) {
        activeSeason = keys[0];
      }
    }
  });

  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      onClose();
    }
  }

  $effect(() => {
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  });

  async function handleToggleFollow() {
    if (!title) return;
    const next = !isFollowing;
    isFollowing = next;
    try {
      await toggleFollow(title.id);
    } catch (err) {
      console.error('Failed to toggle follow:', err);
      isFollowing = !next;
    }
  }

  function handlePlaySeason() {
    if (!title) return;
    const seasonId = activeSeason || 1;
    downloadM3U(`/play/season/${title.id}.m3u?season=${seasonId}`);
  }

  function handleWindowsPlay(file: MediaFile) {
    downloadM3U(`/play/file/${file.id}.m3u`, file.id);
  }

  function handleAndroidPlay(file: MediaFile) {
    if (!title) return;
    const name = file.episode_code ? `${title.name} ${file.episode_code}` : file.name;
    playOnAndroid(file.id, file.file_url, name);
  }

  async function copyStreamLink(file: MediaFile) {
    try {
      await navigator.clipboard.writeText(file.file_url);
      copiedFileId = file.id;
      setTimeout(() => {
        if (copiedFileId === file.id) copiedFileId = null;
      }, 2000);
    } catch (err) {
      console.error('Failed to copy stream link:', err);
    }
  }
</script>

{#if title}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-6 bg-zinc-950/85 backdrop-blur-md overflow-y-auto" role="dialog" aria-modal="true">
    <!-- Backdrop click -->
    <div
      role="button"
      tabindex="-1"
      onclick={onClose}
      onkeydown={(e) => { if (e.key === 'Enter') onClose(); }}
      class="fixed inset-0 cursor-default"
      aria-label="Close modal backdrop"
    ></div>

    <!-- Modal Box -->
    <div class="relative w-full max-w-4xl max-h-[92vh] flex flex-col rounded-2xl bg-zinc-900 border border-zinc-800 shadow-2xl overflow-hidden z-10 animate-in fade-in zoom-in-95 duration-200">
      
      <!-- Modal Header Banner -->
      <div class="relative p-5 sm:p-6 border-b border-zinc-800 bg-gradient-to-r from-zinc-900 via-zinc-900/95 to-zinc-950">
        <!-- Close button -->
        <button
          type="button"
          onclick={onClose}
          class="absolute top-4 right-4 p-2 rounded-xl bg-zinc-800/80 hover:bg-zinc-700 text-zinc-400 hover:text-zinc-100 transition-colors"
          aria-label="Close modal"
        >
          <X class="w-5 h-5" />
        </button>

        <div class="flex flex-col sm:flex-row gap-5 items-start">
          <!-- Poster -->
          <div class="w-28 sm:w-36 flex-shrink-0 aspect-[2/3] rounded-xl overflow-hidden border border-zinc-750 shadow-xl bg-zinc-950">
            <PosterImage
              src={getThumbUrl(title)}
              alt={title.name}
              aspect="aspect-[2/3]"
            />
          </div>

          <!-- Metadata -->
          <div class="flex-1 min-w-0 pr-8">
            <div class="flex flex-wrap items-center gap-2 mb-1.5">
              <span class="px-2 py-0.5 rounded text-[11px] font-mono uppercase bg-zinc-800 text-zinc-300 border border-zinc-700">
                {title.type}
              </span>
              {#if title.year > 0}
                <span class="px-2 py-0.5 rounded text-[11px] font-mono bg-zinc-800 text-zinc-300 border border-zinc-700">
                  {title.year}
                </span>
              {/if}
              {#if title.size_human}
                <span class="px-2 py-0.5 rounded text-[11px] font-mono font-bold bg-amber-500/10 text-amber-400 border border-amber-500/30">
                  {title.size_human} Total
                </span>
              {/if}
              {#if title.resolution}
                <ResolutionBadge resolution={title.resolution} size="sm" />
              {/if}
              {#if title.category}
                <span class="px-2 py-0.5 rounded text-[11px] font-mono bg-zinc-800 text-zinc-300 border border-zinc-700">
                  {title.category}
                </span>
              {/if}
              {#if title.has_hdr}
                <span class="px-2 py-0.5 rounded text-[11px] font-mono font-bold bg-amber-500/20 text-amber-300 border border-amber-500/40">
                  HDR
                </span>
              {/if}
            </div>

            <!-- Title with Year in Bracket -->
            <h1 class="text-xl sm:text-2xl font-extrabold text-zinc-100 tracking-tight">
              {displayTitleWithYear(title.name, title.year)}
            </h1>

            <p class="text-xs text-zinc-400 mt-2 flex items-center gap-2">
              <HardDrive class="w-3.5 h-3.5 text-zinc-500" />
              <span>Direct streaming from local ISP cache • Zero proxy bandwidth bottleneck</span>
            </p>

            <!-- Actions row -->
            <div class="flex flex-wrap items-center gap-2.5 mt-4">
              <!-- Play Season M3U (Windows) -->
              {#if title.type === 'series'}
                <button
                  type="button"
                  onclick={handlePlaySeason}
                  class="inline-flex items-center gap-2 px-3.5 py-2 rounded-xl bg-amber-500 hover:bg-amber-400 text-zinc-950 font-bold text-xs shadow-md shadow-amber-500/20 transition-all hover:scale-105 active:scale-95"
                >
                  <Monitor class="w-4 h-4" />
                  <span>Download Season {activeSeason} Playlist (.m3u)</span>
                </button>
              {/if}

              <!-- Follow toggle with Instant Alert Badge -->
              <button
                type="button"
                onclick={handleToggleFollow}
                class="inline-flex items-center gap-2 px-3.5 py-2 rounded-xl border text-xs font-semibold transition-colors {isFollowing
                  ? 'bg-amber-500/20 text-amber-300 border-amber-500/40'
                  : 'bg-zinc-800/80 text-zinc-300 border-zinc-700 hover:bg-zinc-750'}"
                title={isFollowing ? 'Receiving instant alerts when new files arrive' : 'Follow to get instant alerts on new episodes'}
              >
                <Star class="w-3.5 h-3.5 {isFollowing ? 'fill-current text-amber-400' : ''}" />
                <span>{isFollowing ? 'Following (Instant Alerts)' : 'Follow for Instant Alerts'}</span>
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Season Selector Tabs (if series) -->
      {#if title.type === 'series' && availableSeasons.length > 1}
        <div class="px-5 sm:px-6 pt-3 pb-2 border-b border-zinc-800 bg-zinc-950/40 flex items-center gap-2 overflow-x-auto scrollbar-none">
          <span class="text-xs font-mono text-zinc-400 uppercase tracking-wider mr-2 flex-shrink-0">
            Seasons:
          </span>
          {#each availableSeasons as seasonNum}
            <button
              type="button"
              onclick={() => { activeSeason = seasonNum; }}
              class="px-3.5 py-1.5 rounded-lg text-xs font-semibold whitespace-nowrap transition-all {activeSeason === seasonNum
                ? 'bg-zinc-100 text-zinc-950 shadow'
                : 'bg-zinc-850 text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 border border-zinc-800'}"
            >
              Season {seasonNum}
            </button>
          {/each}
        </div>
      {/if}

      <!-- Episodes / Files List -->
      <div class="flex-1 overflow-y-auto p-4 sm:p-6 space-y-2.5">
        <div class="flex items-center justify-between text-xs font-mono text-zinc-400 px-1 mb-1">
          <span>{title.type === 'series' ? `EPISODES IN SEASON ${activeSeason} (${currentEpisodes.length})` : `AVAILABLE MEDIA FILES (${currentEpisodes.length})`}</span>
          <span>1-CLICK DIRECT LAUNCH</span>
        </div>

        {#if currentEpisodes.length > 0}
          {#each currentEpisodes as ep (ep.id)}
            <div class="group p-3 sm:p-3.5 rounded-xl bg-zinc-950/60 hover:bg-zinc-850/80 border border-zinc-800/80 hover:border-zinc-700 transition-all flex flex-col sm:flex-row sm:items-center justify-between gap-3">
              
              <!-- Episode metadata -->
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2.5">
                  {#if ep.episode_code}
                    <span class="px-2 py-0.5 rounded font-mono text-xs font-bold bg-amber-500/10 text-amber-400 border border-amber-500/20">
                      {ep.episode_code}
                    </span>
                  {/if}
                  <span class="text-sm font-semibold text-zinc-100 truncate group-hover:text-amber-300 transition-colors" title={ep.name}>
                    {ep.name}
                  </span>
                </div>

                <div class="flex items-center gap-3 mt-1.5 text-xs font-mono text-zinc-400">
                  <span class="text-amber-300 font-bold">{formatBytes(ep.file_size_bytes)}</span>
                  {#if ep.resolution}
                    <span>• {ep.resolution}</span>
                  {/if}
                  {#if ep.source}
                    <span>• {ep.source}</span>
                  {/if}
                  {#if ep.has_hdr}
                    <span class="text-amber-400 font-bold">• HDR</span>
                  {/if}
                </div>
              </div>

              <!-- Stream actions buttons: PotPlayer, VLC, Android, Preview, Copy -->
              <div class="flex items-center gap-1.5 sm:gap-2 flex-wrap flex-shrink-0">
                <!-- 1-Click PotPlayer Launch (No download!) -->
                <button
                  type="button"
                  onclick={() => playOnPotPlayer(ep.file_url, ep.id)}
                  class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-amber-500/15 hover:bg-amber-500/25 text-amber-300 text-xs font-semibold border border-amber-500/30 transition-colors"
                  title="Direct 1-Click Launch in Daum PotPlayer (Zero download required!)"
                >
                  <Play class="w-3.5 h-3.5 fill-current" />
                  <span>PotPlayer</span>
                </button>

                <!-- 1-Click VLC Launch -->
                <button
                  type="button"
                  onclick={() => playOnVLC(ep.file_url, ep.id)}
                  class="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-zinc-200 text-xs font-medium border border-zinc-700 transition-colors"
                  title="Direct 1-Click Launch in VLC Media Player"
                >
                  <Monitor class="w-3.5 h-3.5 text-orange-400" />
                  <span>VLC</span>
                </button>

                <!-- Android Intent -->
                <button
                  type="button"
                  onclick={() => handleAndroidPlay(ep)}
                  class="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-zinc-200 text-xs font-medium border border-zinc-700 transition-colors"
                  title="Stream on Android (VLC, MX Player)"
                >
                  <Smartphone class="w-3.5 h-3.5 text-zinc-400" />
                  <span class="hidden sm:inline">Android</span>
                </button>

                <!-- In-Browser Video Preview -->
                <button
                  type="button"
                  onclick={() => onPreview(ep)}
                  class="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-zinc-300 text-xs font-medium border border-zinc-700 transition-colors"
                  title="Preview in Browser player"
                >
                  <Eye class="w-3.5 h-3.5 text-zinc-400" />
                  <span class="hidden sm:inline">Preview</span>
                </button>

                <!-- Copy Link Button -->
                <button
                  type="button"
                  onclick={() => copyStreamLink(ep)}
                  class="p-2 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-zinc-400 hover:text-zinc-200 border border-zinc-700 transition-colors"
                  title={copiedFileId === ep.id ? 'Copied URL to Clipboard!' : 'Copy Direct Stream URL'}
                  aria-label="Copy stream URL"
                >
                  {#if copiedFileId === ep.id}
                    <Check class="w-3.5 h-3.5 text-emerald-400" />
                  {:else}
                    <Copy class="w-3.5 h-3.5" />
                  {/if}
                </button>

                <!-- Download M3U playlist file -->
                <button
                  type="button"
                  onclick={() => handleWindowsPlay(ep)}
                  class="p-2 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-zinc-400 hover:text-zinc-200 border border-zinc-700 transition-colors"
                  title="Download .m3u Playlist File"
                  aria-label="Download playlist"
                >
                  <Download class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          {/each}
        {:else}
          <div class="text-center py-8 text-zinc-500 text-xs font-mono">
            No media files available for this selection.
          </div>
        {/if}
      </div>

    </div>
  </div>
{/if}
