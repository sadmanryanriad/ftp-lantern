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
    getAndroidIntentUrl,
    playOnMPV,
    getMPVCommand,
    copyToClipboard,
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
  let copiedIspFileId = $state<number | null>(null);
  let copiedCliFileId = $state<number | null>(null);
  let copiedPlaylistCli = $state(false);
  let mpvLaunchedFileId = $state<number | null>(null);
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

  function handlePlaySeries() {
    if (!title) return;
    downloadM3U(`/play/series/${title.id}.m3u`);
  }

  async function copyMPVPlaylistCommand() {
    if (!title) return;
    const url = `${window.location.origin}/play/season/${title.id}.m3u?season=${activeSeason || 1}`;
    const cmd = `mpv "${url}"`;
    const success = await copyToClipboard(cmd);
    if (success) {
      copiedPlaylistCli = true;
      setTimeout(() => { copiedPlaylistCli = false; }, 2000);
    }
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
    const success = await copyToClipboard(file.file_url);
    if (success) {
      copiedFileId = file.id;
      setTimeout(() => {
        if (copiedFileId === file.id) copiedFileId = null;
      }, 2000);
    }
  }

  async function copyMPVCommand(file: MediaFile) {
    const success = await copyToClipboard(getMPVCommand(file.file_url));
    if (success) {
      copiedCliFileId = file.id;
      setTimeout(() => {
        if (copiedCliFileId === file.id) copiedCliFileId = null;
      }, 2000);
    }
  }

  async function openISPSite(file: MediaFile) {
    await copyToClipboard(file.file_url);
    window.open(file.file_url, '_blank', 'noopener,noreferrer');
    copiedIspFileId = file.id;
    setTimeout(() => {
      if (copiedIspFileId === file.id) copiedIspFileId = null;
    }, 2000);
  }

  function handlePlayMPV(file: MediaFile) {
    playOnMPV(file.file_url, file.id);
    mpvLaunchedFileId = file.id;
    setTimeout(() => {
      if (mpvLaunchedFileId === file.id) mpvLaunchedFileId = null;
    }, 2500);
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
              {#if title.category && title.category !== `(${title.year})` && title.category !== String(title.year)}
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
              <!-- Play Season / Complete Series M3U Playlist (MPV / VLC / Android) -->
              {#if title.type === 'series'}
                <button
                  type="button"
                  onclick={handlePlaySeason}
                  class="inline-flex items-center gap-2 px-3.5 py-2 rounded-xl bg-amber-500 hover:bg-amber-400 text-zinc-950 font-bold text-xs shadow-md shadow-amber-500/20 transition-all hover:scale-105 active:scale-95"
                  title="Download single .m3u playlist file for Season {activeSeason} — opens entire season in MPV or VLC with auto-next and full playlist drawer"
                >
                  <Play class="w-4 h-4 fill-current" />
                  <span>Play Season {activeSeason} (.m3u)</span>
                </button>

                {#if availableSeasons.length > 1}
                  <button
                    type="button"
                    onclick={handlePlaySeries}
                    class="inline-flex items-center gap-2 px-3.5 py-2 rounded-xl bg-zinc-800 hover:bg-zinc-750 text-amber-300 border border-amber-500/30 font-semibold text-xs shadow-sm transition-all hover:scale-105 active:scale-95"
                    title="Download complete series playlist (.m3u) across all seasons for continuous binge watching in MPV / VLC"
                  >
                    <Tv class="w-3.5 h-3.5 text-amber-400" />
                    <span>Play All Seasons (.m3u)</span>
                  </button>
                {/if}

                <button
                  type="button"
                  onclick={copyMPVPlaylistCommand}
                  class="inline-flex items-center gap-1.5 px-3 py-2 rounded-xl bg-zinc-800/80 hover:bg-zinc-750 text-zinc-300 border border-zinc-700 text-xs font-mono transition-colors shadow-sm"
                  title="Copy MPV command to launch this playlist directly from terminal"
                >
                  {#if copiedPlaylistCli}
                    <Check class="w-3.5 h-3.5 text-emerald-400" />
                    <span class="text-emerald-400 font-sans font-semibold">Copied!</span>
                  {:else}
                    <Copy class="w-3.5 h-3.5 text-zinc-400" />
                    <span class="font-sans font-semibold">MPV CLI</span>
                  {/if}
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

              <!-- Direct ISP Folder Link -->
              {#if title.folder_url}
                <a
                  href={title.folder_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  onclick={() => copyToClipboard(title.folder_url!)}
                  class="inline-flex items-center gap-2 px-3.5 py-2 rounded-xl bg-zinc-800/80 hover:bg-zinc-750 text-zinc-300 hover:text-amber-300 border border-zinc-700 text-xs font-semibold transition-colors shadow-sm"
                  title="Open directory directly on ISP Server (and copy folder URL)"
                >
                  <ExternalLink class="w-3.5 h-3.5 text-amber-400" />
                  <span>ISP Directory</span>
                </a>
              {/if}
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
                  <span class="text-amber-300 font-bold">{ep.size_human || formatBytes(ep.file_size_bytes || ep.size_bytes || 0)}</span>
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

              <!-- MOBILE ACTION BAR: Shown only on smartphones (sm:hidden). Big touch targets, no Windows clutter -->
              <div class="flex items-center gap-2 w-full mt-2 sm:hidden">
                <!-- 1-Click Android Player Launch (Opens native Android 'Open with' chooser: VLC, MX Player, MPV, etc.) -->
                <a
                  href={ep.intent_url || getAndroidIntentUrl(ep.file_url, ep.name)}
                  onclick={() => recordPlay(ep.id)}
                  class="flex-1 min-h-[42px] inline-flex items-center justify-center gap-2 px-3.5 py-2 rounded-xl bg-gradient-to-r from-amber-500 to-orange-500 hover:from-amber-600 hover:to-orange-600 text-zinc-950 font-black text-xs shadow-md shadow-amber-500/15 active:scale-95 transition-all text-center"
                >
                  <Play class="w-4 h-4 fill-current flex-shrink-0" />
                  <span class="truncate">Open in Player (VLC / MX / MPV)</span>
                </a>

                <!-- In-Browser Video Preview -->
                <button
                  type="button"
                  onclick={() => onPreview(ep)}
                  class="min-h-[42px] min-w-[42px] flex items-center justify-center rounded-xl bg-zinc-800 hover:bg-zinc-700 text-zinc-200 border border-zinc-700 active:scale-95 transition-all"
                  title="Preview in Browser"
                  aria-label="Preview in Browser"
                >
                  <Eye class="w-4 h-4" />
                </button>

                <!-- Copy Link Button -->
                <button
                  type="button"
                  onclick={() => copyStreamLink(ep)}
                  class="min-h-[42px] min-w-[42px] flex items-center justify-center rounded-xl border active:scale-95 transition-all {copiedFileId === ep.id ? 'text-emerald-400 border-emerald-500/40 bg-emerald-500/10' : 'bg-zinc-800 hover:bg-zinc-700 text-zinc-400 border-zinc-700'}"
                  title="Copy Stream URL"
                  aria-label="Copy Stream URL"
                >
                  {#if copiedFileId === ep.id}
                    <Check class="w-4 h-4 text-emerald-400" />
                  {:else}
                    <Copy class="w-4 h-4" />
                  {/if}
                </button>

                <!-- Open / Copy directly on ISP Media Server -->
                <button
                  type="button"
                  onclick={() => openISPSite(ep)}
                  class="min-h-[42px] min-w-[42px] flex items-center justify-center rounded-xl border active:scale-95 transition-all {copiedIspFileId === ep.id ? 'text-emerald-400 border-emerald-500/40 bg-emerald-500/10' : 'bg-zinc-800 hover:bg-zinc-700 text-zinc-300 hover:text-amber-300 border-zinc-700'}"
                  title="Open directly on ISP Media Server (copies link)"
                  aria-label="Open on ISP Media Server"
                >
                  {#if copiedIspFileId === ep.id}
                    <Check class="w-4 h-4 text-emerald-400" />
                  {:else}
                    <ExternalLink class="w-4 h-4 text-amber-400" />
                  {/if}
                </button>
              </div>

              <!-- DESKTOP ACTION BAR: Shown on PC / larger screens (hidden sm:flex). Complete desktop player suite -->
              <div class="hidden sm:flex items-center gap-1.5 sm:gap-2 flex-wrap flex-shrink-0">
                <!-- 1-Click MPV Launch (Primary) -->
                <button
                  type="button"
                  onclick={() => handlePlayMPV(ep)}
                  class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-gradient-to-r from-amber-500/20 to-orange-500/20 hover:from-amber-500/30 hover:to-orange-500/30 text-amber-300 text-xs font-bold border border-amber-500/40 shadow-sm transition-all hover:scale-105 active:scale-95"
                  title="Direct 1-Click Launch in MPV Player"
                >
                  {#if mpvLaunchedFileId === ep.id}
                    <Check class="w-3.5 h-3.5 text-emerald-400" />
                    <span class="text-emerald-400">Opening...</span>
                  {:else}
                    <Play class="w-3.5 h-3.5 fill-current text-amber-400" />
                    <span>MPV</span>
                  {/if}
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

                <!-- In-Browser Video Preview -->
                <button
                  type="button"
                  onclick={() => onPreview(ep)}
                  class="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-zinc-300 text-xs font-medium border border-zinc-700 transition-colors"
                  title="Preview in Browser player"
                >
                  <Eye class="w-3.5 h-3.5 text-zinc-400" />
                  <span>Preview</span>
                </button>

                <!-- Copy Link Button -->
                <button
                  type="button"
                  onclick={() => copyStreamLink(ep)}
                  class="p-2 rounded-lg border transition-colors {copiedFileId === ep.id ? 'text-emerald-400 border-emerald-500/40 bg-emerald-500/10' : 'bg-zinc-800 hover:bg-zinc-700 text-zinc-400 hover:text-zinc-200 border-zinc-700'}"
                  title={copiedFileId === ep.id ? 'Copied URL to Clipboard!' : 'Copy Direct Stream URL'}
                  aria-label="Copy stream URL"
                >
                  {#if copiedFileId === ep.id}
                    <Check class="w-3.5 h-3.5 text-emerald-400" />
                  {:else}
                    <Copy class="w-3.5 h-3.5" />
                  {/if}
                </button>

                <!-- Open / Copy directly on ISP Media Server -->
                <button
                  type="button"
                  onclick={() => openISPSite(ep)}
                  class="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg border transition-colors {copiedIspFileId === ep.id
                    ? 'text-emerald-400 border-emerald-500/40 bg-emerald-500/10'
                    : 'bg-zinc-850 hover:bg-zinc-800 text-zinc-300 hover:text-amber-300 border-zinc-800'}"
                  title="Open directly on ISP site (also copies stream URL)"
                  aria-label="Open directly on ISP site"
                >
                  {#if copiedIspFileId === ep.id}
                    <Check class="w-3.5 h-3.5 text-emerald-400" />
                    <span>Opening...</span>
                  {:else}
                    <ExternalLink class="w-3.5 h-3.5 text-amber-400" />
                    <span>ISP</span>
                  {/if}
                </button>

                <!-- Copy MPV CLI Command -->
                <button
                  type="button"
                  onclick={() => copyMPVCommand(ep)}
                  class="inline-flex items-center gap-1 px-2 py-1.5 rounded-lg bg-zinc-850 hover:bg-zinc-800 text-[11px] font-mono border transition-colors {copiedCliFileId === ep.id ? 'text-emerald-400 border-emerald-500/40 bg-emerald-500/10' : 'text-zinc-400 hover:text-zinc-200 border-zinc-800'}"
                  title="Copy 'mpv URL' CLI command"
                >
                  {#if copiedCliFileId === ep.id}
                    <Check class="w-3 h-3 text-emerald-400" />
                    <span>Copied!</span>
                  {:else}
                    <span>CLI</span>
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
