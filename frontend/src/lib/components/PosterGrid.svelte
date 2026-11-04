<script lang="ts">
  import { Star, ArrowUpDown, Calendar, Clock, Filter, X, Loader2 } from '@lucide/svelte';
  import type { Title } from '../types';
  import { getThumbUrl, toggleFollow, displayTitleWithYear } from '../api';
  import PosterImage from './PosterImage.svelte';
  import ResolutionBadge from './ResolutionBadge.svelte';

  let {
    titles = [],
    activeFilter = 'All',
    activeCategory = 'All',
    activeYear = 'All',
    activeDateAdded = 'all',
    activeSort = 'recent',
    searchQuery = '',
    totalCount = 0,
    loading = false,
    categories = [],
    onFilterChange = (_filter: string) => {},
    onCategoryChange = (_cat: string) => {},
    onYearChange = (_year: string) => {},
    onDateAddedChange = (_date: string) => {},
    onSortChange = (_sort: string) => {},
    onSelectTitle = (_title: Title) => {},
    onClearSearch = () => {},
    onLoadMore = () => {}
  }: {
    titles: Title[];
    activeFilter: string;
    activeCategory?: string;
    activeYear?: string;
    activeDateAdded?: string;
    activeSort: string;
    searchQuery: string;
    totalCount: number;
    loading: boolean;
    categories?: string[];
    onFilterChange: (filter: string) => void;
    onCategoryChange?: (cat: string) => void;
    onYearChange?: (year: string) => void;
    onDateAddedChange?: (date: string) => void;
    onSortChange: (sort: string) => void;
    onSelectTitle: (title: Title) => void;
    onClearSearch?: () => void;
    onLoadMore: () => void;
  } = $props();

  const resolutionFilters = [
    { label: 'All Res', value: 'All' },
    { label: '4K UHD', value: '4K' },
    { label: '1080p', value: '1080p' },
    { label: '720p', value: '720p' },
  ];

  const yearOptions = [
    { label: 'All Years', value: 'All' },
    { label: '2026', value: '2026' },
    { label: '2025', value: '2025' },
    { label: '2024', value: '2024' },
    { label: '2023', value: '2023' },
    { label: '2022', value: '2022' },
    { label: '2020–2025', value: '2020-2025' },
    { label: '2010–2019', value: '2010-2019' },
    { label: '2000–2009', value: '2000-2009' },
    { label: 'Before 2000', value: '<2000' },
  ];

  const dateAddedOptions = [
    { label: 'All Time', value: 'all' },
    { label: 'Today', value: 'today' },
    { label: 'Last 7 Days', value: '7d' },
    { label: 'Last 30 Days', value: '30d' },
    { label: 'Last 90 Days', value: '90d' },
  ];

  const sortOptions = [
    { label: 'Recently Added', value: 'recent' },
    { label: 'Year (Newest)', value: 'year_desc' },
    { label: 'Year (Oldest)', value: 'year_asc' },
    { label: 'Title (A–Z)', value: 'name_asc' },
    { label: 'Title (Z–A)', value: 'name_desc' },
  ];

  // Local optimistic follow states
  let followingMap = $state<Record<number, boolean>>({});

  async function handleToggleFollow(e: MouseEvent, title: Title) {
    e.stopPropagation();
    const current = followingMap[title.id] ?? Boolean(title.is_following);
    const next = !current;
    followingMap[title.id] = next;

    try {
      await toggleFollow(title.id);
    } catch (err) {
      console.error('Failed to toggle follow:', err);
      followingMap[title.id] = current;
    }
  }

  function isItemFollowing(title: Title): boolean {
    return followingMap[title.id] !== undefined ? followingMap[title.id] : Boolean(title.is_following);
  }
</script>

<section class="py-6">
  <!-- Search Results Banner (When searching) -->
  {#if searchQuery}
    <div class="mb-6 p-4 rounded-2xl bg-amber-500/10 border border-amber-500/30 flex flex-col sm:flex-row sm:items-center justify-between gap-3 shadow-lg">
      <div class="flex items-center gap-2.5">
        <span class="text-xl">🔍</span>
        <div>
          <h2 class="text-sm sm:text-base font-bold text-zinc-100">
            Search Results for <span class="text-amber-400 font-mono">"{searchQuery}"</span>
          </h2>
          <p class="text-xs text-zinc-400 font-mono">
            Found {totalCount} matching {totalCount === 1 ? 'title' : 'titles'} across the library
          </p>
        </div>
      </div>
      {#if onClearSearch}
        <button
          type="button"
          onclick={onClearSearch}
          class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-zinc-800 hover:bg-zinc-700 text-xs font-semibold text-zinc-200 border border-zinc-700 transition-colors self-start sm:self-auto"
        >
          <X class="w-3.5 h-3.5" />
          <span>Clear Search</span>
        </button>
      {/if}
    </div>
  {/if}

  <!-- Filter & Subcategory Chips -->
  {#if categories && categories.length > 0}
    <div
      class="flex items-center gap-2 overflow-x-auto whitespace-nowrap pb-2 mb-4 w-full min-w-0 scrollbar-thin scrollbar-thumb-zinc-700 scrollbar-track-zinc-900/50"
      style="-webkit-overflow-scrolling: touch;"
    >
      {#each categories as cat}
        {@const isSelected = (activeCategory || 'All') === cat}
        <button
          type="button"
          onclick={() => onCategoryChange && onCategoryChange(cat)}
          class="flex-shrink-0 px-3.5 py-1.5 rounded-full text-xs font-medium whitespace-nowrap transition-all {isSelected
            ? 'bg-amber-500 text-zinc-950 font-bold shadow-md shadow-amber-500/20 scale-[1.02]'
            : 'bg-zinc-900/90 text-zinc-300 hover:text-white hover:bg-zinc-800 border border-zinc-800'}"
        >
          {cat}
        </button>
      {/each}
    </div>
  {/if}

  <!-- Controls Bar: Resolution, Year, Date Added & Sorting -->
  <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-3 mb-6 p-3 rounded-2xl bg-zinc-900/60 border border-zinc-800/80">
    <!-- Left Filters: Resolution & Dropdowns -->
    <div class="flex flex-wrap items-center gap-2">
      <!-- Resolution Pills -->
      <div class="flex items-center gap-1 bg-zinc-950/80 p-0.5 rounded-xl border border-zinc-800">
        {#each resolutionFilters as res}
          {@const isResActive = activeFilter === res.value}
          <button
            type="button"
            onclick={() => onFilterChange(res.value)}
            class="px-2.5 py-1 rounded-lg text-xs font-semibold whitespace-nowrap transition-all {isResActive
              ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30'
              : 'text-zinc-400 hover:text-zinc-200'}"
          >
            {res.label}
          </button>
        {/each}
      </div>

      <!-- Year Filter Dropdown -->
      <div class="relative flex items-center">
        <Calendar class="w-3.5 h-3.5 absolute left-2.5 text-zinc-400 pointer-events-none" />
        <select
          value={activeYear || 'All'}
          onchange={(e) => onYearChange && onYearChange((e.target as HTMLSelectElement).value)}
          class="pl-8 pr-7 py-1.5 rounded-xl bg-zinc-950/90 text-xs font-medium text-zinc-200 border border-zinc-800 hover:border-zinc-700 focus:outline-none focus:border-amber-500/50 appearance-none cursor-pointer"
          title="Filter by Release Year"
        >
          {#each yearOptions as opt}
            <option value={opt.value}>{opt.label}</option>
          {/each}
        </select>
      </div>

      <!-- Date Added Filter Dropdown -->
      <div class="relative flex items-center">
        <Clock class="w-3.5 h-3.5 absolute left-2.5 text-zinc-400 pointer-events-none" />
        <select
          value={activeDateAdded || 'all'}
          onchange={(e) => onDateAddedChange && onDateAddedChange((e.target as HTMLSelectElement).value)}
          class="pl-8 pr-7 py-1.5 rounded-xl bg-zinc-950/90 text-xs font-medium text-zinc-200 border border-zinc-800 hover:border-zinc-700 focus:outline-none focus:border-amber-500/50 appearance-none cursor-pointer"
          title="Filter by Date Added to Server"
        >
          {#each dateAddedOptions as opt}
            <option value={opt.value}>{opt.label}</option>
          {/each}
        </select>
      </div>
    </div>

    <!-- Right Controls: Sort & Count -->
    <div class="flex items-center justify-between lg:justify-end gap-3">
      <span class="text-xs font-mono text-zinc-400">
        {totalCount > 0 ? `${titles.length} of ${totalCount} titles` : `${titles.length} titles`}
      </span>

      <div class="relative flex items-center">
        <ArrowUpDown class="w-3.5 h-3.5 absolute left-2.5 text-zinc-400 pointer-events-none" />
        <select
          value={activeSort}
          onchange={(e) => onSortChange((e.target as HTMLSelectElement).value)}
          class="pl-8 pr-8 py-1.5 rounded-xl bg-zinc-950/90 text-xs font-medium text-zinc-200 border border-zinc-800 hover:border-zinc-700 focus:outline-none focus:border-amber-500/50 appearance-none cursor-pointer"
        >
          {#each sortOptions as sort}
            <option value={sort.value}>{sort.label}</option>
          {/each}
        </select>
      </div>
    </div>
  </div>

  <!-- Grid of Poster Cards -->
  {#if titles.length > 0}
    <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-3 sm:gap-5">
      {#each titles as title (title.id)}
        <div
          role="button"
          tabindex="0"
          onclick={() => onSelectTitle(title)}
          onkeydown={(e) => { if (e.key === 'Enter') onSelectTitle(title); }}
          class="group relative rounded-xl overflow-hidden bg-zinc-900/90 border border-zinc-850 hover:border-amber-500/40 hover:shadow-2xl hover:shadow-amber-500/10 transition-all duration-300 cursor-pointer flex flex-col text-left focus:outline-none"
        >
          <!-- Poster container -->
          <div class="relative w-full aspect-[2/3] overflow-hidden bg-zinc-950">
            <PosterImage
              src={getThumbUrl(title)}
              alt={title.name}
              aspect="aspect-[2/3]"
              class="group-hover:scale-105 transition-transform duration-500"
            />

            <!-- Top Overlays: Follow Star & Resolution -->
            <div class="absolute inset-x-0 top-0 p-2 flex items-start justify-between z-10 bg-gradient-to-b from-zinc-950/80 via-transparent to-transparent">
              <!-- Follow Star -->
              <button
                type="button"
                onclick={(e) => handleToggleFollow(e, title)}
                class="p-1.5 rounded-full backdrop-blur-md transition-all {isItemFollowing(title)
                  ? 'bg-amber-500 text-zinc-950 shadow-md shadow-amber-500/30'
                  : 'bg-zinc-950/60 text-zinc-400 hover:text-amber-300 hover:bg-zinc-900/90'}"
                title={isItemFollowing(title) ? 'Following series (instant alerts)' : 'Follow for instant alerts'}
                aria-label="Follow title"
              >
                <Star class="w-3.5 h-3.5 {isItemFollowing(title) ? 'fill-current' : ''}" />
              </button>

              <!-- Resolution Badge -->
              {#if title.resolution}
                <ResolutionBadge resolution={title.resolution} size="xs" />
              {/if}
            </div>

            <!-- Bottom Gradient Overlay -->
            <div class="absolute inset-x-0 bottom-0 h-16 bg-gradient-to-t from-zinc-950 via-zinc-950/70 to-transparent"></div>

            <!-- Bottom Badges on Poster: Type & Size -->
            <div class="absolute bottom-2 left-2 right-2 flex items-center justify-between text-[10px] font-mono text-zinc-300 z-10">
              <span class="px-1.5 py-0.5 rounded bg-zinc-900/80 backdrop-blur-sm border border-zinc-700/50 uppercase text-[9px]">
                {title.type}
              </span>
              {#if title.size_human}
                <span class="px-1.5 py-0.5 rounded bg-zinc-900/80 backdrop-blur-sm border border-zinc-700/50 font-bold text-amber-300">
                  {title.size_human}
                </span>
              {/if}
            </div>
          </div>

          <!-- Title Details with Year in Bracket -->
          <div class="p-2.5 sm:p-3 flex-1 flex flex-col justify-between bg-zinc-900/80">
            <div>
              <h3 class="text-xs sm:text-sm font-semibold text-zinc-100 truncate group-hover:text-amber-300 transition-colors" title={displayTitleWithYear(title.name, title.year)}>
                {displayTitleWithYear(title.name, title.year)}
              </h3>
              <div class="flex items-center justify-between mt-1 text-[11px] text-zinc-400 font-mono">
                <span class="truncate">{title.category || title.source || ''}</span>
                {#if title.has_hdr}
                  <span class="text-amber-400 font-bold ml-1">HDR</span>
                {/if}
              </div>
            </div>
          </div>
        </div>
      {/each}
    </div>

    <!-- Infinite Scroll / Load More -->
    {#if titles.length < totalCount}
      <div class="mt-8 text-center">
        <button
          type="button"
          onclick={onLoadMore}
          disabled={loading}
          class="inline-flex items-center gap-2 px-6 py-2.5 rounded-xl bg-zinc-900 hover:bg-zinc-800 text-zinc-200 text-xs font-semibold border border-zinc-800 transition-all hover:scale-105 active:scale-95 disabled:opacity-50"
        >
          {#if loading}
            <Loader2 class="w-4 h-4 animate-spin text-amber-400" />
            <span>Loading releases...</span>
          {:else}
            <span>Load More ({titles.length} of {totalCount})</span>
          {/if}
        </button>
      </div>
    {/if}
  {:else if !loading}
    <div class="rounded-2xl border border-zinc-900 bg-zinc-950/40 p-12 text-center">
      <p class="text-zinc-400 text-sm font-mono mb-2">No matching releases found</p>
      <p class="text-zinc-400 text-xs">Try selecting a different category, year, or clearing filters.</p>
    </div>
  {/if}
</section>
