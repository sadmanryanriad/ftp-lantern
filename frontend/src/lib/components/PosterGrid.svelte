<script lang="ts">
  import { Star, Film, Tv, SlidersHorizontal, ArrowUpDown, Flame, Loader2 } from '@lucide/svelte';
  import type { Title } from '../types';
  import { getThumbUrl, toggleFollow } from '../api';
  import PosterImage from './PosterImage.svelte';
  import ResolutionBadge from './ResolutionBadge.svelte';

  let {
    titles = [],
    activeFilter = 'All',
    activeSort = 'recent',
    searchQuery = '',
    totalCount = 0,
    loading = false,
    onFilterChange = (_filter: string) => {},
    onSortChange = (_sort: string) => {},
    onSelectTitle = (_title: Title) => {},
    onLoadMore = () => {}
  }: {
    titles: Title[];
    activeFilter: string;
    activeSort: string;
    searchQuery: string;
    totalCount: number;
    loading: boolean;
    onFilterChange: (filter: string) => void;
    onSortChange: (sort: string) => void;
    onSelectTitle: (title: Title) => void;
    onLoadMore: () => void;
  } = $props();

  const filterTabs = [
    { label: 'All', value: 'All' },
    { label: 'Series', value: 'series' },
    { label: 'Movies', value: 'movie' },
    { label: '4K UHD', value: '4K' },
    { label: '1080p FHD', value: '1080p' },
    { label: '720p HD', value: '720p' },
  ];

  const sortOptions = [
    { label: 'Recently Added', value: 'recent' },
    { label: 'Title (A-Z)', value: 'title' },
    { label: 'Year (Newest)', value: 'year' },
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
      // Revert on error
      followingMap[title.id] = current;
    }
  }

  function isItemFollowing(title: Title): boolean {
    return followingMap[title.id] !== undefined ? followingMap[title.id] : Boolean(title.is_following);
  }
</script>

<section class="py-8">
  <!-- Controls Bar: Filter tabs & Sort Dropdown -->
  <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-6">
    <!-- Filter Tabs -->
    <div class="flex items-center gap-1.5 overflow-x-auto pb-1 scrollbar-none">
      {#each filterTabs as tab}
        <button
          type="button"
          onclick={() => onFilterChange(tab.value)}
          class="px-3.5 py-1.5 rounded-lg text-xs font-semibold whitespace-nowrap transition-all {activeFilter === tab.value
            ? 'bg-amber-500 text-zinc-950 shadow-md shadow-amber-500/20'
            : 'bg-zinc-900/80 text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 border border-zinc-800'}"
        >
          {tab.label}
        </button>
      {/each}
    </div>

    <!-- Right: Sorting & Count -->
    <div class="flex items-center justify-between md:justify-end gap-3">
      <span class="text-xs font-mono text-zinc-400">
        {totalCount > 0 ? `${titles.length} of ${totalCount} titles` : `${titles.length} titles`}
      </span>

      <div class="relative flex items-center">
        <ArrowUpDown class="w-3.5 h-3.5 absolute left-2.5 text-zinc-400 pointer-events-none" />
        <select
          value={activeSort}
          onchange={(e) => onSortChange((e.target as HTMLSelectElement).value)}
          class="pl-8 pr-8 py-1.5 rounded-lg bg-zinc-900 text-xs font-medium text-zinc-200 border border-zinc-800 hover:border-zinc-700 focus:outline-none focus:border-amber-500/50 appearance-none cursor-pointer"
        >
          {#each sortOptions as sort}
            <option value={sort.value}>{sort.label}</option>
          {/each}
        </select>
      </div>
    </div>
  </div>

  {#if searchQuery}
    <div class="mb-6 p-3 rounded-xl bg-zinc-900/60 border border-zinc-800/80 flex items-center justify-between">
      <span class="text-xs text-zinc-300">
        Active search: <strong class="text-amber-400 font-mono font-bold">"{searchQuery}"</strong>
      </span>
      <button
        type="button"
        onclick={() => onFilterChange('All')}
        class="text-xs text-zinc-400 hover:text-zinc-200 underline"
      >
        Reset filters
      </button>
    </div>
  {/if}

  <!-- Grid of Poster Cards -->
  {#if titles.length > 0}
    <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4 sm:gap-5">
      {#each titles as title (title.id)}
        <div
          role="button"
          tabindex="0"
          onclick={() => onSelectTitle(title)}
          onkeydown={(e) => { if (e.key === 'Enter') onSelectTitle(title); }}
          class="group relative rounded-xl overflow-hidden bg-zinc-900/90 border border-zinc-850 hover:border-amber-500/40 hover:shadow-2xl hover:shadow-amber-500/10 transition-all duration-300 cursor-pointer flex flex-col text-left"
        >
          <!-- Poster container -->
          <div class="relative w-full aspect-[2/3] overflow-hidden bg-zinc-950">
            <PosterImage
              src={getThumbUrl(title)}
              alt={title.name}
              aspect="aspect-[2/3]"
              class="group-hover:scale-105 transition-transform duration-500"
            />

            <!-- Top Overlays: Resolution & Star -->
            <div class="absolute inset-x-0 top-0 p-2 flex items-start justify-between z-10 bg-gradient-to-b from-zinc-950/80 via-transparent to-transparent">
              <!-- Follow Star -->
              <button
                type="button"
                onclick={(e) => handleToggleFollow(e, title)}
                class="p-1.5 rounded-full backdrop-blur-md transition-all {isItemFollowing(title)
                  ? 'bg-amber-500 text-zinc-950 shadow-md shadow-amber-500/30'
                  : 'bg-zinc-950/60 text-zinc-400 hover:text-amber-300 hover:bg-zinc-900/90'}"
                title={isItemFollowing(title) ? 'Following updates' : 'Follow title'}
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
            <div class="absolute inset-x-0 bottom-0 h-16 bg-gradient-to-t from-zinc-950 via-zinc-950/60 to-transparent"></div>

            <!-- Bottom Info on Poster -->
            <div class="absolute bottom-2 left-2 right-2 flex items-center justify-between text-[11px] font-mono text-zinc-300 z-10">
              <span class="px-1.5 py-0.2 rounded bg-zinc-900/80 backdrop-blur-sm border border-zinc-700/50 uppercase text-[10px]">
                {title.type}
              </span>
              {#if title.year > 0}
                <span class="px-1.5 py-0.2 rounded bg-zinc-900/80 backdrop-blur-sm border border-zinc-700/50">
                  {title.year}
                </span>
              {/if}
            </div>
          </div>

          <!-- Title & Sub-metadata -->
          <div class="p-3 flex-1 flex flex-col justify-between bg-zinc-900/80">
            <div>
              <h3 class="text-sm font-semibold text-zinc-100 truncate group-hover:text-amber-300 transition-colors" title={title.name}>
                {title.name}
              </h3>
              <div class="flex items-center gap-2 mt-1 text-[11px] text-zinc-400 font-mono">
                {#if title.source}
                  <span class="truncate">{title.source}</span>
                {/if}
                {#if title.has_hdr}
                  <span class="text-amber-400 font-bold">HDR</span>
                {/if}
              </div>
            </div>
          </div>
        </div>
      {/each}
    </div>

    <!-- Load More Button -->
    {#if titles.length < totalCount}
      <div class="mt-10 text-center">
        <button
          type="button"
          onclick={onLoadMore}
          disabled={loading}
          class="inline-flex items-center gap-2 px-6 py-2.5 rounded-xl bg-zinc-900 hover:bg-zinc-800 text-zinc-200 border border-zinc-700 font-medium text-sm transition-all hover:scale-105 active:scale-95 disabled:opacity-50"
        >
          {#if loading}
            <Loader2 class="w-4 h-4 animate-spin text-amber-400" />
            <span>Loading titles...</span>
          {:else}
            <span>Load More ({totalCount - titles.length} remaining)</span>
          {/if}
        </button>
      </div>
    {/if}
  {:else if !loading}
    <!-- Empty State -->
    <div class="text-center py-16 px-4 rounded-2xl bg-zinc-900/30 border border-dashed border-zinc-800">
      <div class="w-12 h-12 mx-auto rounded-full bg-zinc-800/60 flex items-center justify-center text-zinc-500 mb-3">
        <Film class="w-6 h-6 text-zinc-400" />
      </div>
      <h3 class="text-base font-semibold text-zinc-200">No media titles found</h3>
      <p class="text-xs text-zinc-400 mt-1 max-w-sm mx-auto">
        No titles match your active filter or search. Try selecting "All" or trigger a scan from settings.
      </p>
      {#if searchQuery || activeFilter !== 'All'}
        <button
          type="button"
          onclick={() => onFilterChange('All')}
          class="mt-4 px-4 py-2 rounded-lg bg-amber-500 text-zinc-950 font-semibold text-xs transition-colors"
        >
          Reset Filters
        </button>
      {/if}
    </div>
  {:else}
    <!-- Loading skeleton -->
    <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
      {#each Array(12) as _}
        <div class="rounded-xl overflow-hidden bg-zinc-900 border border-zinc-800 animate-pulse">
          <div class="aspect-[2/3] bg-zinc-800"></div>
          <div class="p-3 space-y-2">
            <div class="h-4 bg-zinc-800 rounded w-3/4"></div>
            <div class="h-3 bg-zinc-800 rounded w-1/2"></div>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</section>
