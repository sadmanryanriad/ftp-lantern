<script lang="ts">
  import { onMount } from 'svelte';
  import Navbar from './lib/components/Navbar.svelte';
  import SearchBar from './lib/components/SearchBar.svelte';
  import ContinueWatching from './lib/components/ContinueWatching.svelte';
  import WhatsNew from './lib/components/WhatsNew.svelte';
  import PosterGrid from './lib/components/PosterGrid.svelte';
  import TitleDetailModal from './lib/components/TitleDetailModal.svelte';
  import SettingsModal from './lib/components/SettingsModal.svelte';
  import VideoPreviewModal from './lib/components/VideoPreviewModal.svelte';

  import type {
    Title,
    TitleDetail,
    MediaFile,
    SystemStatus,
    SchedulerStatus,
    ContinueWatchingItem
  } from './lib/types';
  import {
    fetchStatus,
    fetchScheduler,
    fetchWhatsNew,
    fetchContinueWatching,
    searchTitles,
    fetchTitleDetail,
    downloadM3U,
    playOnAndroid
  } from './lib/api';

  // Global state using Svelte 5 runes
  let status = $state<SystemStatus | null>(null);
  let scheduler = $state<SchedulerStatus | null>(null);
  let whatsNew = $state<Title[]>([]);
  let continueWatching = $state<ContinueWatchingItem[]>([]);
  let titles = $state<Title[]>([]);
  let totalTitles = $state<number>(0);
  let loadingGrid = $state<boolean>(false);

  // Filters & Search
  let activeFilter = $state<string>('All');
  let activeSort = $state<string>('recent');
  let searchQuery = $state<string>('');
  let currentPage = $state<number>(1);
  const pageSize = 24;

  // Modals
  let selectedTitleDetail = $state<TitleDetail | null>(null);
  let previewFile = $state<MediaFile | null>(null);
  let previewTitleName = $state<string>('');
  let isSettingsOpen = $state<boolean>(false);

  async function loadSystemHealth() {
    try {
      const [s, sch] = await Promise.allSettled([fetchStatus(), fetchScheduler()]);
      if (s.status === 'fulfilled') status = s.value;
      if (sch.status === 'fulfilled') scheduler = sch.value;
    } catch (err) {
      console.warn('System status check error:', err);
    }
  }

  async function loadShelves() {
    try {
      const [wnRes, cwRes] = await Promise.allSettled([
        fetchWhatsNew(16),
        fetchContinueWatching()
      ]);
      if (wnRes.status === 'fulfilled') whatsNew = wnRes.value;
      if (cwRes.status === 'fulfilled') continueWatching = cwRes.value;
    } catch (err) {
      console.warn('Shelves load error:', err);
    }
  }

  async function loadGrid(reset = true) {
    if (reset) {
      currentPage = 1;
    }
    loadingGrid = true;
    try {
      let filterType: string | undefined;
      let filterRes: string | undefined;

      if (activeFilter === 'series' || activeFilter === 'movie') {
        filterType = activeFilter;
      } else if (activeFilter === '4K' || activeFilter === '1080p' || activeFilter === '720p') {
        filterRes = activeFilter;
      }

      const res = await searchTitles({
        q: searchQuery || undefined,
        type: filterType,
        resolution: filterRes,
        sort: activeSort,
        page: currentPage,
        limit: pageSize,
      });

      if (reset) {
        titles = res.results || [];
      } else {
        titles = [...titles, ...(res.results || [])];
      }
      totalTitles = res.total ?? titles.length;
    } catch (err) {
      console.error('Failed to load grid titles:', err);
      if (reset) titles = [];
    } finally {
      loadingGrid = false;
    }
  }

  function handleFilterChange(newFilter: string) {
    activeFilter = newFilter;
    loadGrid(true);
  }

  function handleSortChange(newSort: string) {
    activeSort = newSort;
    loadGrid(true);
  }

  function handleFullSearch(q: string) {
    searchQuery = q;
    loadGrid(true);
  }

  function handleLoadMore() {
    currentPage += 1;
    loadGrid(false);
  }

  async function openTitleDetail(title: Title | number) {
    const id = typeof title === 'number' ? title : title.id;
    try {
      const detail = await fetchTitleDetail(id);
      selectedTitleDetail = detail;
    } catch (err) {
      console.error('Failed to load title details:', err);
      // Fallback with minimal info if title object is available
      if (typeof title !== 'number') {
        selectedTitleDetail = {
          ...title,
          releases: [],
          files: [],
        };
      }
    }
  }

  function handleQuickPlay(title: Title) {
    // If has next episode or can open detail modal
    openTitleDetail(title);
  }

  function handlePlayNext(item: ContinueWatchingItem) {
    if (item.next_file_id) {
      downloadM3U(`/play/file/${item.next_file_id}.m3u`, item.next_file_id);
    } else {
      openTitleDetail(item.title_id);
    }
  }

  function handlePreviewVideo(file: MediaFile) {
    previewFile = file;
    previewTitleName = selectedTitleDetail?.name || '';
  }

  onMount(() => {
    loadSystemHealth();
    loadShelves();
    loadGrid(true);

    // Periodic telemetry polling every 15 seconds
    const interval = setInterval(loadSystemHealth, 15000);
    return () => clearInterval(interval);
  });
</script>

<div class="min-h-screen bg-zinc-950 flex flex-col text-zinc-100">
  <!-- Sticky Navbar -->
  <Navbar
    {status}
    {scheduler}
    onOpenSettings={() => { isSettingsOpen = true; }}
  />

  <!-- Main Hero & Content -->
  <main class="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-6">
    <!-- Hero / Search Section -->
    <div class="py-6 text-center">
      <h1 class="text-2xl sm:text-3xl font-extrabold tracking-tight text-zinc-100 mb-2">
        Instant BDIX Streaming
      </h1>
      <p class="text-xs sm:text-sm text-zinc-400 mb-6 max-w-md mx-auto">
        Zero-proxy direct playback from local ISP caches in 4K, 1080p and HDR
      </p>

      <!-- Instant Search Bar with 100ms Debounce & Spotlight -->
      <SearchBar
        onSelectTitle={(t) => openTitleDetail(t)}
        onFullSearch={handleFullSearch}
        onQuickPlay={handleQuickPlay}
      />
    </div>

    <!-- Continue Watching Shelf -->
    <ContinueWatching
      items={continueWatching}
      onSelectTitle={(id) => openTitleDetail(id)}
      onPlayNext={handlePlayNext}
    />

    <!-- What's New Shelf / Carousel -->
    <WhatsNew
      items={whatsNew}
      onSelectTitle={(t) => openTitleDetail(t)}
    />

    <!-- Main Filterable & Sortable Poster Grid -->
    <PosterGrid
      {titles}
      {activeFilter}
      {activeSort}
      {searchQuery}
      totalCount={totalTitles}
      loading={loadingGrid}
      onFilterChange={handleFilterChange}
      onSortChange={handleSortChange}
      onSelectTitle={(t) => openTitleDetail(t)}
      onLoadMore={handleLoadMore}
    />
  </main>

  <!-- Footer -->
  <footer class="mt-auto border-t border-zinc-900 bg-zinc-950/80 py-6 text-center text-xs font-mono text-zinc-400">
    <div class="max-w-7xl mx-auto px-4 flex flex-col sm:flex-row items-center justify-between gap-2">
      <div>
        <span>FTP-Lantern • Lightweight BDIX Media Gateway</span>
      </div>
      <div>
        <span>Direct Stream Mode: <strong class="text-zinc-300">Host Network</strong></span>
      </div>
    </div>
  </footer>

  <!-- Modals -->
  {#if selectedTitleDetail}
    <TitleDetailModal
      title={selectedTitleDetail}
      onClose={() => { selectedTitleDetail = null; }}
      onPreview={handlePreviewVideo}
    />
  {/if}

  {#if isSettingsOpen}
    <SettingsModal
      isOpen={isSettingsOpen}
      {status}
      {scheduler}
      onClose={() => { isSettingsOpen = false; }}
      onRefresh={loadSystemHealth}
    />
  {/if}

  {#if previewFile}
    <VideoPreviewModal
      file={previewFile}
      titleName={previewTitleName}
      onClose={() => { previewFile = null; }}
    />
  {/if}
</div>
