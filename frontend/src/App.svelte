<script lang="ts">
  import { onMount } from 'svelte';
  import Navbar from './lib/components/Navbar.svelte';
  import SearchBar from './lib/components/SearchBar.svelte';
  import ContinueWatching from './lib/components/ContinueWatching.svelte';
  import WhatsNew from './lib/components/WhatsNew.svelte';
  import MediaShelf from './lib/components/MediaShelf.svelte';
  import PosterGrid from './lib/components/PosterGrid.svelte';
  import TitleDetailModal from './lib/components/TitleDetailModal.svelte';
  import SettingsModal from './lib/components/SettingsModal.svelte';
  import VideoPreviewModal from './lib/components/VideoPreviewModal.svelte';
  import { Star, Film, Tv, Sparkles, Compass, ArrowLeft } from '@lucide/svelte';

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
    downloadM3U
  } from './lib/api';

  function getInitialStatus(): SystemStatus | null {
    if (typeof localStorage === 'undefined') return null;
    try {
      const c = localStorage.getItem('lantern_status_cache');
      return c ? JSON.parse(c) : null;
    } catch {
      return null;
    }
  }

  // Global state with localStorage cache to avoid offline/0-metrics flash on refresh
  let status = $state<SystemStatus | null>(getInitialStatus());
  let scheduler = $state<SchedulerStatus | null>(null);
  let whatsNew = $state<Title[]>([]);
  let continueWatching = $state<ContinueWatchingItem[]>([]);
  
  // Home block shelves data
  let homeMovies = $state<Title[]>([]);
  let homeMovieCategory = $state<string>('All');
  let homeSeries = $state<Title[]>([]);
  let homeSeriesCategory = $state<string>('All');

  // Dedicated Views / Main Grid state
  let activeTab = $state<string>('home'); // 'home' | 'movies' | 'series' | 'anime' | 'documentary' | 'following'
  let titles = $state<Title[]>([]);
  let totalTitles = $state<number>(0);
  let loadingGrid = $state<boolean>(false);

  // Filters & Search
  let activeFilter = $state<string>('All'); // Resolution filter: All, 4K, 1080p, 720p
  let activeCategory = $state<string>('All');
  let activeYear = $state<string>('All');
  let activeDateAdded = $state<string>('all');
  let activeSort = $state<string>('recent');
  let searchQuery = $state<string>('');
  let currentPage = $state<number>(1);
  const pageSize = 24;

  // Modals state
  let selectedTitleDetail = $state<TitleDetail | null>(null);
  let previewFile = $state<MediaFile | null>(null);
  let previewTitleName = $state<string>('');
  let isSettingsOpen = $state<boolean>(false);

  // ISP Category Definitions
  const movieCategories = [
    'All',
    'English Movies (1080p)',
    'English Movies',
    'Hindi Movies',
    'South Movies (Hindi Dubbed)',
    'South Indian Movies',
    'Bangla Movies',
    'Animation Movies',
    'IMDb Top-250',
    'Foreign Movies'
  ];

  const seriesCategories = [
    'All',
    'TV-WEB-Series',
    'Korean Series',
    'Anime & Cartoons',
    'Documentaries',
    'Wrestling',
    'Awards & TV Shows'
  ];

  async function loadSystemHealth() {
    try {
      const [s, sch] = await Promise.allSettled([fetchStatus(), fetchScheduler()]);
      if (s.status === 'fulfilled') {
        status = s.value;
        try { localStorage.setItem('lantern_status_cache', JSON.stringify(s.value)); } catch {}
      }
      if (sch.status === 'fulfilled') scheduler = sch.value;
    } catch (err) {
      console.warn('System status check error:', err);
    }
  }

  async function loadShelves() {
    try {
      const [wnRes, cwRes, movRes, serRes] = await Promise.allSettled([
        fetchWhatsNew(16),
        fetchContinueWatching(),
        searchTitles({ type: 'movie', limit: 16, sort: 'recent' }),
        searchTitles({ type: 'series', limit: 16, sort: 'recent' })
      ]);
      if (wnRes.status === 'fulfilled') whatsNew = wnRes.value;
      if (cwRes.status === 'fulfilled') continueWatching = cwRes.value;
      if (movRes.status === 'fulfilled') homeMovies = movRes.value.results || [];
      if (serRes.status === 'fulfilled') homeSeries = serRes.value.results || [];
    } catch (err) {
      console.warn('Shelves load error:', err);
    }
  }

  async function handleHomeMovieCategoryChange(cat: string) {
    homeMovieCategory = cat;
    try {
      const res = await searchTitles({
        type: 'movie',
        category: cat !== 'All' ? cat : undefined,
        limit: 16,
        sort: 'recent'
      });
      homeMovies = res.results || [];
    } catch (err) {
      console.error('Failed to filter home movies:', err);
    }
  }

  async function handleHomeSeriesCategoryChange(cat: string) {
    homeSeriesCategory = cat;
    try {
      const res = await searchTitles({
        type: 'series',
        category: cat !== 'All' ? cat : undefined,
        limit: 16,
        sort: 'recent'
      });
      homeSeries = res.results || [];
    } catch (err) {
      console.error('Failed to filter home series:', err);
    }
  }

  async function loadGrid(reset = true) {
    if (reset) {
      currentPage = 1;
    }
    loadingGrid = true;
    try {
      let filterType: string | undefined;
      let filterCategory = activeCategory !== 'All' ? activeCategory : undefined;
      let isFollowingOnly = false;

      if (activeTab === 'movies') {
        filterType = 'movie';
      } else if (activeTab === 'series') {
        filterType = 'series';
      } else if (activeTab === 'anime') {
        if (!filterCategory) filterCategory = 'Anime & Cartoons';
      } else if (activeTab === 'documentary') {
        if (!filterCategory) filterCategory = 'Documentaries';
      } else if (activeTab === 'following') {
        isFollowingOnly = true;
      }

      const res = await searchTitles({
        q: searchQuery || undefined,
        type: filterType,
        category: filterCategory,
        resolution: activeFilter !== 'All' ? activeFilter : undefined,
        year: activeYear !== 'All' ? activeYear : undefined,
        date_added: activeDateAdded !== 'all' ? activeDateAdded : undefined,
        following: isFollowingOnly || undefined,
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

  // Route & Navigation handlers
  function navigateToTab(tab: string, pushHistory = true) {
    activeTab = tab;
    searchQuery = '';
    activeCategory = 'All';
    activeFilter = 'All';
    activeYear = 'All';
    activeDateAdded = 'all';

    if (pushHistory) {
      const hash = tab === 'home' ? '#/' : `#/${tab}`;
      window.history.pushState({ tab }, '', hash);
    }

    if (tab !== 'home') {
      loadGrid(true);
    }
  }

  function handleFilterChange(newFilter: string) {
    activeFilter = newFilter;
    loadGrid(true);
  }

  function handleCategoryChange(newCategory: string) {
    activeCategory = newCategory;
    loadGrid(true);
  }

  function handleYearChange(newYear: string) {
    activeYear = newYear;
    loadGrid(true);
  }

  function handleDateAddedChange(newDate: string) {
    activeDateAdded = newDate;
    loadGrid(true);
  }

  function handleSortChange(newSort: string) {
    activeSort = newSort;
    loadGrid(true);
  }

  function handleFullSearch(q: string) {
    searchQuery = q.trim();
    if (searchQuery) {
      loadGrid(true);
    }
  }

  function handleClearSearch() {
    searchQuery = '';
    if (activeTab !== 'home') {
      loadGrid(true);
    }
  }

  function handleLoadMore() {
    currentPage += 1;
    loadGrid(false);
  }

  // Modal open & close with History API
  async function openTitleDetail(title: Title | number, pushHistory = true) {
    const id = typeof title === 'number' ? title : title.id;
    try {
      const detail = await fetchTitleDetail(id);
      selectedTitleDetail = detail;
      if (pushHistory) {
        window.history.pushState({ modal: 'title', id }, '', `#/title/${id}`);
      }
    } catch (err) {
      console.error('Failed to load title details:', err);
      if (typeof title !== 'number') {
        selectedTitleDetail = {
          ...title,
          releases: [],
          files: [],
        };
      }
    }
  }

  function closeTitleDetail(shouldGoBack = true) {
    selectedTitleDetail = null;
    if (shouldGoBack && window.location.hash.startsWith('#/title/')) {
      if (window.history.state && window.history.state.modal === 'title') {
        window.history.back();
      } else {
        window.history.replaceState(null, '', window.location.pathname);
      }
    }
  }

  function openSettings(pushHistory = true) {
    isSettingsOpen = true;
    if (pushHistory) {
      window.history.pushState({ modal: 'settings' }, '', '#/settings');
    }
  }

  function closeSettings(shouldGoBack = true) {
    isSettingsOpen = false;
    if (shouldGoBack && window.location.hash === '#/settings') {
      if (window.history.state && window.history.state.modal === 'settings') {
        window.history.back();
      } else {
        window.history.replaceState(null, '', window.location.pathname);
      }
    }
  }

  function handlePreviewVideo(file: MediaFile) {
    previewFile = file;
    previewTitleName = selectedTitleDetail?.name || '';
  }

  function handlePlayNext(item: ContinueWatchingItem) {
    if (item.next_file_id) {
      downloadM3U(`/play/file/${item.next_file_id}.m3u`, item.next_file_id);
    } else {
      openTitleDetail(item.title_id);
    }
  }

  // Popstate listener for mouse forward/backward & browser navigation
  function handlePopState(e: PopStateEvent) {
    const hash = window.location.hash || '#/';

    if (hash.startsWith('#/title/')) {
      const id = parseInt(hash.replace('#/title/', ''), 10);
      if (!isNaN(id)) {
        openTitleDetail(id, false);
      }
      return;
    }

    if (hash === '#/settings') {
      isSettingsOpen = true;
      selectedTitleDetail = null;
      return;
    }

    // Close any open modals
    selectedTitleDetail = null;
    isSettingsOpen = false;
    previewFile = null;

    if (hash === '#/movies') {
      navigateToTab('movies', false);
    } else if (hash === '#/series') {
      navigateToTab('series', false);
    } else if (hash === '#/anime') {
      navigateToTab('anime', false);
    } else if (hash === '#/documentary') {
      navigateToTab('documentary', false);
    } else if (hash === '#/following') {
      navigateToTab('following', false);
    } else {
      activeTab = 'home';
      searchQuery = '';
    }
  }

  onMount(() => {
    loadSystemHealth();
    loadShelves();

    // Check initial hash route
    const hash = window.location.hash;
    if (hash.startsWith('#/title/')) {
      const id = parseInt(hash.replace('#/title/', ''), 10);
      if (!isNaN(id)) openTitleDetail(id, false);
    } else if (hash === '#/settings') {
      isSettingsOpen = true;
    } else if (hash === '#/movies' || hash === '#/series' || hash === '#/anime' || hash === '#/documentary' || hash === '#/following') {
      navigateToTab(hash.replace('#/', ''), false);
    }

    window.addEventListener('popstate', handlePopState);

    // Live responsive polling: 2s when crawling, 6s when idle
    let isCurrentlyCrawling = false;
    let pollTimer: any = null;
    let crawlTick = 0;

    async function pollHealth() {
      await loadSystemHealth();
      crawlTick++;
      const wasCrawling = isCurrentlyCrawling;
      isCurrentlyCrawling = Boolean(status?.is_crawling);

      // Periodically refresh shelves every 10s while crawler is actively indexing
      if (isCurrentlyCrawling && crawlTick % 5 === 0) {
        loadShelves();
        if (activeTab !== 'home') {
          loadGrid(false);
        }
      }

      if (wasCrawling && !isCurrentlyCrawling) {
        loadShelves();
        if (activeTab !== 'home') {
          loadGrid(false);
        }
      }

      const nextDelay = isCurrentlyCrawling ? 2000 : 6000;
      pollTimer = setTimeout(pollHealth, nextDelay);
    }

    pollHealth();

    return () => {
      window.removeEventListener('popstate', handlePopState);
      if (pollTimer) clearTimeout(pollTimer);
    };
  });
</script>

<div class="min-h-screen bg-zinc-950 flex flex-col text-zinc-100">
  <!-- Sticky Navbar with live Indexing & Tab routing -->
  <Navbar
    {status}
    {scheduler}
    {activeTab}
    onSelectTab={(tab) => navigateToTab(tab, true)}
    onOpenSettings={() => openSettings(true)}
  />

  <!-- Main Container -->
  <main class="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-5">
    
    <!-- Hero / Search Section: Full on Home, Compact on Dedicated Tabs -->
    {#if activeTab === 'home'}
      <div class="py-4 text-center">
        <h1 class="text-2xl sm:text-3xl font-extrabold tracking-tight text-zinc-100 mb-1.5">
          Instant BDIX Streaming
        </h1>
        <p class="text-xs sm:text-sm text-zinc-400 mb-3 max-w-md mx-auto">
          Zero-proxy direct playback from local ISP caches in 4K, 1080p and HDR
        </p>

        <!-- Library Breakdown Stat Pills -->
        <div class="flex flex-wrap items-center justify-center gap-2 mb-5">
          <button
            type="button"
            onclick={() => navigateToTab('movies', true)}
            class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-mono font-medium bg-zinc-900/80 hover:bg-zinc-850 border border-zinc-800 text-zinc-300 transition-colors cursor-pointer"
          >
            <span>🎬</span>
            <span>{status?.movies_count?.toLocaleString() || 0} Movies</span>
          </button>

          <button
            type="button"
            onclick={() => navigateToTab('series', true)}
            class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-mono font-medium bg-zinc-900/80 hover:bg-zinc-850 border border-zinc-800 text-zinc-300 transition-colors cursor-pointer"
          >
            <span>📺</span>
            <span>{status?.series_count?.toLocaleString() || 0} Series</span>
          </button>

          <button
            type="button"
            onclick={() => navigateToTab('documentary', true)}
            class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-mono font-medium bg-zinc-900/80 hover:bg-zinc-850 border border-zinc-800 text-zinc-300 transition-colors cursor-pointer"
          >
            <span>📁</span>
            <span>{status?.others_count?.toLocaleString() || 0} Docs & Others</span>
          </button>

          <div class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-mono font-medium bg-zinc-900/80 border border-zinc-800 text-amber-300">
            <span>⚡</span>
            <span>{status?.files_count?.toLocaleString() || 0} Total Files</span>
          </div>

          <!-- Disk Storage Stat Pill -->
          <button
            type="button"
            onclick={() => openSettings(true)}
            class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-mono font-medium bg-zinc-900/80 hover:bg-zinc-850 border border-zinc-800 text-emerald-400 transition-colors cursor-pointer"
            title="Total persistent disk storage used by database & poster thumbnails"
          >
            <span>💾</span>
            <span>{status?.total_volume_human || status?.db_size_human || '0 B'} on Disk</span>
          </button>
        </div>

        <!-- Live Indexing Progress Banner (Appears dynamically while crawling) -->
        {#if status?.is_crawling}
          <div class="max-w-xl mx-auto mb-5 p-3 rounded-2xl bg-amber-500/10 border border-amber-500/30 flex items-center justify-between gap-3 text-xs font-mono shadow-lg shadow-amber-500/5 animate-pulse">
            <div class="flex items-center gap-2 text-amber-300 min-w-0">
              <span class="relative flex h-2 w-2 flex-shrink-0">
                <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75"></span>
                <span class="relative inline-flex rounded-full h-2 w-2 bg-amber-500"></span>
              </span>
              <span class="font-bold flex-shrink-0">Indexing:</span>
              <span class="truncate text-zinc-200">{status?.current_folder || status?.current_source || 'Dhaka-Flix Portal'}</span>
            </div>
            <div class="flex items-center gap-2 text-zinc-400 flex-shrink-0">
              {#if (status?.files_indexed_session || 0) > 0}
                <span class="text-emerald-400 font-bold">+{status?.files_indexed_session} new files</span>
                <span>•</span>
                <span class="text-amber-400 font-bold">{Math.round(status?.items_per_second || 0)} files/s</span>
              {:else}
                <span class="text-zinc-300 font-bold">{status?.folders_indexed_session || 0} folders checked</span>
                <span>•</span>
                <span class="text-emerald-400 font-semibold">Verifying</span>
              {/if}
            </div>
          </div>
        {/if}

        <!-- Instant Search Bar with 100ms Debounce & Spotlight -->
        <SearchBar
          onSelectTitle={(t) => openTitleDetail(t, true)}
          onFullSearch={handleFullSearch}
          onQuickPlay={(t) => openTitleDetail(t, true)}
        />
      </div>
    {:else}
      <!-- Compact Top Search on Dedicated Pages -->
      <div class="pt-2 pb-5 flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-zinc-850/80 mb-5">
        <div>
          <div class="flex items-center gap-2 text-xs font-mono text-zinc-400 mb-1">
            <button
              type="button"
              onclick={() => navigateToTab('home', true)}
              class="hover:text-amber-400 transition-colors cursor-pointer"
            >
              Home
            </button>
            <span>/</span>
            <span class="text-amber-400 capitalize">{activeTab === 'documentary' ? 'Docs' : activeTab}</span>
          </div>
          <h1 class="text-xl sm:text-2xl font-black text-zinc-100 flex items-center gap-2">
            {#if activeTab === 'movies'}
              <span>🎬</span>
              <span>Movies Library</span>
            {:else if activeTab === 'series'}
              <span>📺</span>
              <span>TV & Web Series</span>
            {:else if activeTab === 'anime'}
              <span>⛩️</span>
              <span>Anime & Cartoons</span>
            {:else if activeTab === 'documentary'}
              <span>🌍</span>
              <span>Documentaries & Specials</span>
            {:else if activeTab === 'following'}
              <span>⭐</span>
              <span>Followed Series</span>
            {/if}
          </h1>
        </div>

        <div class="w-full sm:w-80">
          <SearchBar
            onSelectTitle={(t) => openTitleDetail(t, true)}
            onFullSearch={handleFullSearch}
            onQuickPlay={(t) => openTitleDetail(t, true)}
          />
        </div>
      </div>
    {/if}

    <!-- VIEW 1: ACTIVE SEARCH QUERY (Hides Home blocks and focuses on search results) -->
    {#if searchQuery}
      <PosterGrid
        {titles}
        {activeFilter}
        {activeCategory}
        {activeYear}
        {activeDateAdded}
        {activeSort}
        {searchQuery}
        totalCount={totalTitles}
        loading={loadingGrid}
        categories={[]}
        onFilterChange={handleFilterChange}
        onCategoryChange={handleCategoryChange}
        onYearChange={handleYearChange}
        onDateAddedChange={handleDateAddedChange}
        onSortChange={handleSortChange}
        onSelectTitle={(t) => openTitleDetail(t, true)}
        onClearSearch={handleClearSearch}
        onLoadMore={handleLoadMore}
      />

    <!-- VIEW 2: HOMEPAGE (3 BLOCKS ARCHITECTURE) -->
    {:else if activeTab === 'home'}
      
      <!-- Continue Watching Shelf -->
      <ContinueWatching
        items={continueWatching}
        onSelectTitle={(id) => openTitleDetail(id, true)}
        onPlayNext={handlePlayNext}
      />

      <!-- BLOCK 1: What's New on BDIX (Recently Added horizontal shelf) -->
      <WhatsNew
        items={whatsNew}
        onSelectTitle={(t) => openTitleDetail(t, true)}
        onViewAll={() => navigateToTab('movies', true)}
      />

      <!-- BLOCK 2: Movies Shelf (with quick ISP category chips & Explore All Movies) -->
      <MediaShelf
        title="Featured Movies"
        icon="🎬"
        items={homeMovies}
        categories={movieCategories}
        activeCategory={homeMovieCategory}
        onSelectCategory={handleHomeMovieCategoryChange}
        onSelectTitle={(t) => openTitleDetail(t, true)}
        onExploreAll={() => navigateToTab('movies', true)}
      />

      <!-- BLOCK 3: TV & Web Series Shelf (with quick ISP series chips & Explore All Series) -->
      <MediaShelf
        title="TV & Web Series"
        icon="📺"
        items={homeSeries}
        categories={seriesCategories}
        activeCategory={homeSeriesCategory}
        onSelectCategory={handleHomeSeriesCategoryChange}
        onSelectTitle={(t) => openTitleDetail(t, true)}
        onExploreAll={() => navigateToTab('series', true)}
      />

    <!-- VIEW 3: DEDICATED MOVIES PAGE -->
    {:else if activeTab === 'movies'}
      <PosterGrid
        {titles}
        {activeFilter}
        {activeCategory}
        {activeYear}
        {activeDateAdded}
        {activeSort}
        {searchQuery}
        totalCount={totalTitles}
        loading={loadingGrid}
        categories={movieCategories}
        onFilterChange={handleFilterChange}
        onCategoryChange={handleCategoryChange}
        onYearChange={handleYearChange}
        onDateAddedChange={handleDateAddedChange}
        onSortChange={handleSortChange}
        onSelectTitle={(t) => openTitleDetail(t, true)}
        onLoadMore={handleLoadMore}
      />

    <!-- VIEW 4: DEDICATED TV & WEB SERIES PAGE -->
    {:else if activeTab === 'series'}
      <PosterGrid
        {titles}
        {activeFilter}
        {activeCategory}
        {activeYear}
        {activeDateAdded}
        {activeSort}
        {searchQuery}
        totalCount={totalTitles}
        loading={loadingGrid}
        categories={seriesCategories}
        onFilterChange={handleFilterChange}
        onCategoryChange={handleCategoryChange}
        onYearChange={handleYearChange}
        onDateAddedChange={handleDateAddedChange}
        onSortChange={handleSortChange}
        onSelectTitle={(t) => openTitleDetail(t, true)}
        onLoadMore={handleLoadMore}
      />

    <!-- VIEW 5: ANIME & CARTOONS PAGE -->
    {:else if activeTab === 'anime'}
      <PosterGrid
        {titles}
        {activeFilter}
        {activeCategory}
        {activeYear}
        {activeDateAdded}
        {activeSort}
        {searchQuery}
        totalCount={totalTitles}
        loading={loadingGrid}
        categories={['All', 'Animation Movies', 'Anime & Cartoons']}
        onFilterChange={handleFilterChange}
        onCategoryChange={handleCategoryChange}
        onYearChange={handleYearChange}
        onDateAddedChange={handleDateAddedChange}
        onSortChange={handleSortChange}
        onSelectTitle={(t) => openTitleDetail(t, true)}
        onLoadMore={handleLoadMore}
      />

    <!-- VIEW 6: DOCUMENTARIES PAGE -->
    {:else if activeTab === 'documentary'}
      <PosterGrid
        {titles}
        {activeFilter}
        {activeCategory}
        {activeYear}
        {activeDateAdded}
        {activeSort}
        {searchQuery}
        totalCount={totalTitles}
        loading={loadingGrid}
        categories={[]}
        onFilterChange={handleFilterChange}
        onCategoryChange={handleCategoryChange}
        onYearChange={handleYearChange}
        onDateAddedChange={handleDateAddedChange}
        onSortChange={handleSortChange}
        onSelectTitle={(t) => openTitleDetail(t, true)}
        onLoadMore={handleLoadMore}
      />

    <!-- VIEW 7: FOLLOWING MANAGEMENT PAGE -->
    {:else if activeTab === 'following'}
      <div class="mb-4 p-4 rounded-2xl bg-amber-500/10 border border-amber-500/30 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div class="flex items-center gap-3">
          <div class="p-2.5 rounded-xl bg-amber-500/20 text-amber-400 border border-amber-500/30">
            <Star class="w-5 h-5 fill-current" />
          </div>
          <div>
            <h2 class="text-base font-bold text-zinc-100">
              Followed Titles ({totalTitles})
            </h2>
            <p class="text-xs text-zinc-300">
              You receive instant Telegram alerts immediately when new episodes or files drop for these titles.
            </p>
          </div>
        </div>

        <button
          type="button"
          onclick={() => navigateToTab('home', true)}
          class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-zinc-850 hover:bg-zinc-800 text-xs font-semibold text-zinc-300 transition-colors self-start sm:self-auto"
        >
          <ArrowLeft class="w-3.5 h-3.5" />
          <span>Back to Home</span>
        </button>
      </div>

      <PosterGrid
        {titles}
        {activeFilter}
        {activeCategory}
        {activeYear}
        {activeDateAdded}
        {activeSort}
        {searchQuery}
        totalCount={totalTitles}
        loading={loadingGrid}
        categories={[]}
        onFilterChange={handleFilterChange}
        onCategoryChange={handleCategoryChange}
        onYearChange={handleYearChange}
        onDateAddedChange={handleDateAddedChange}
        onSortChange={handleSortChange}
        onSelectTitle={(t) => openTitleDetail(t, true)}
        onLoadMore={handleLoadMore}
      />
    {/if}

  </main>

  <!-- Footer -->
  <footer class="mt-auto border-t border-zinc-900 bg-zinc-950/80 py-6 text-center text-xs font-mono text-zinc-400">
    <div class="max-w-7xl mx-auto px-4 flex flex-col sm:flex-row items-center justify-between gap-2">
      <div>
        <span>FTP-Lantern • Lightweight BDIX Media Gateway</span>
      </div>
      <div>
        <span>Host Network Mode: <strong class="text-emerald-400">Direct ISP Subnet</strong></span>
      </div>
    </div>
  </footer>

  <!-- Modals with History API navigation -->
  {#if selectedTitleDetail}
    <TitleDetailModal
      title={selectedTitleDetail}
      onClose={() => closeTitleDetail(true)}
      onPreview={handlePreviewVideo}
    />
  {/if}

  {#if isSettingsOpen}
    <SettingsModal
      isOpen={isSettingsOpen}
      {status}
      {scheduler}
      onClose={() => closeSettings(true)}
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
