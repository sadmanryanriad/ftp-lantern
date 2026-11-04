<script lang="ts">
  import { Search, X, Play, Film, Tv, Loader2 } from '@lucide/svelte';
  import type { Title } from '../types';
  import { searchTitles, getThumbUrl } from '../api';
  import ResolutionBadge from './ResolutionBadge.svelte';

  let {
    onSelectTitle = (_title: Title) => {},
    onFullSearch = (_query: string) => {},
    onQuickPlay = (_title: Title) => {}
  }: {
    onSelectTitle: (title: Title) => void;
    onFullSearch: (query: string) => void;
    onQuickPlay: (title: Title) => void;
  } = $props();

  let query = $state('');
  let results = $state<Title[]>([]);
  let isOpen = $state(false);
  let loading = $state(false);
  let selectedIndex = $state(-1);
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;
  let containerRef = $state<HTMLDivElement | null>(null);

  function handleInput(e: Event) {
    const val = (e.target as HTMLInputElement).value;
    query = val;

    if (debounceTimer) clearTimeout(debounceTimer);

    if (!val.trim()) {
      results = [];
      isOpen = false;
      loading = false;
      return;
    }

    loading = true;
    debounceTimer = setTimeout(async () => {
      try {
        const res = await searchTitles({ q: val.trim(), limit: 6 });
        results = res.results || [];
        isOpen = results.length > 0;
        selectedIndex = -1;
      } catch (err) {
        console.error('Search error:', err);
        results = [];
      } finally {
        loading = false;
      }
    }, 100);
  }

  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      isOpen = false;
      selectedIndex = -1;
      return;
    }

    if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (!isOpen && results.length > 0) {
        isOpen = true;
      }
      if (results.length > 0) {
        selectedIndex = (selectedIndex + 1) % results.length;
      }
      return;
    }

    if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (results.length > 0) {
        selectedIndex = (selectedIndex - 1 + results.length) % results.length;
      }
      return;
    }

    if (e.key === 'Enter') {
      e.preventDefault();
      if (selectedIndex >= 0 && selectedIndex < results.length) {
        selectItem(results[selectedIndex]);
      } else {
        // Full search submitted
        isOpen = false;
        onFullSearch(query.trim());
      }
    }
  }

  function selectItem(title: Title) {
    isOpen = false;
    onSelectTitle(title);
  }

  function clearSearch() {
    query = '';
    results = [];
    isOpen = false;
    selectedIndex = -1;
    onFullSearch('');
  }

  function handleClickOutside(e: MouseEvent) {
    if (containerRef && !containerRef.contains(e.target as Node)) {
      isOpen = false;
    }
  }

  $effect(() => {
    window.addEventListener('click', handleClickOutside);
    return () => {
      window.removeEventListener('click', handleClickOutside);
      if (debounceTimer) clearTimeout(debounceTimer);
    };
  });
</script>

<div class="relative w-full max-w-2xl mx-auto" bind:this={containerRef}>
  <!-- Search Input Bar -->
  <div class="relative flex items-center">
    <div class="absolute left-3.5 pointer-events-none text-zinc-400">
      {#if loading}
        <Loader2 class="w-4 h-4 animate-spin text-amber-400" />
      {:else}
        <Search class="w-4 h-4" />
      {/if}
    </div>

    <input
      type="text"
      bind:value={query}
      oninput={handleInput}
      onkeydown={handleKeyDown}
      onfocus={() => { if (results.length > 0) isOpen = true; }}
      placeholder="Instant search movies, series, 4K, 1080p... (Press Enter for full grid)"
      class="w-full pl-10 pr-10 py-2.5 rounded-xl bg-zinc-900/90 text-sm text-zinc-100 placeholder-zinc-500 border border-zinc-800 focus:border-amber-500/50 focus:ring-2 focus:ring-amber-500/20 focus:outline-none transition-all shadow-inner"
    />

    {#if query}
      <button
        type="button"
        onclick={clearSearch}
        class="absolute right-3 p-1 rounded-md text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition-colors"
        aria-label="Clear search"
      >
        <X class="w-3.5 h-3.5" />
      </button>
    {/if}
  </div>

  <!-- Spotlight Typeahead Dropdown -->
  {#if isOpen && results.length > 0}
    <div class="absolute left-0 right-0 mt-2 z-50 rounded-xl bg-zinc-900/95 border border-zinc-800/90 backdrop-blur-xl shadow-2xl overflow-hidden divide-y divide-zinc-800/60 animate-in fade-in slide-in-from-top-2 duration-150">
      <div class="px-3 py-1.5 bg-zinc-950/60 flex items-center justify-between text-[11px] font-mono text-zinc-400">
        <span>SPOTLIGHT MATCHES ({results.length})</span>
        <span>↑↓ Navigate · Enter to open</span>
      </div>

      <div class="max-h-96 overflow-y-auto">
        {#each results as title, index (title.id)}
          <div
            role="button"
            tabindex="0"
            onclick={() => selectItem(title)}
            onkeydown={(e) => { if (e.key === 'Enter') selectItem(title); }}
            class="group w-full px-3 py-2.5 flex items-center justify-between gap-3 text-left transition-colors cursor-pointer {selectedIndex === index
              ? 'bg-amber-500/10 text-amber-200 border-l-2 border-amber-500'
              : 'hover:bg-zinc-800/50 text-zinc-200'}"
          >
            <!-- Left: Mini poster & metadata -->
            <div class="flex items-center gap-3 min-w-0">
              <!-- Mini poster (~40x60px) -->
              <div class="w-10 h-14 rounded-md overflow-hidden bg-zinc-950 flex-shrink-0 border border-zinc-800 relative">
                {#if title.thumb_hash || title.poster_url}
                  <img
                    src={getThumbUrl(title)}
                    alt={title.name}
                    loading="lazy"
                    class="w-full h-full object-cover"
                    onerror={(e) => {
                      (e.currentTarget as HTMLImageElement).style.display = 'none';
                    }}
                  />
                {/if}
                <div class="absolute inset-0 flex items-center justify-center -z-10 bg-zinc-800">
                  {#if title.type === 'movie'}
                    <Film class="w-4 h-4 text-zinc-600" />
                  {:else}
                    <Tv class="w-4 h-4 text-zinc-600" />
                  {/if}
                </div>
              </div>

              <!-- Title details -->
              <div class="min-w-0">
                <div class="flex items-center gap-2">
                  <span class="text-sm font-semibold truncate group-hover:text-amber-300 transition-colors">
                    {title.name}
                  </span>
                  {#if title.year > 0}
                    <span class="text-xs text-zinc-400 font-mono">({title.year})</span>
                  {/if}
                </div>
                <div class="flex items-center gap-2 mt-1">
                  <span class="text-[10px] font-mono uppercase px-1.5 py-0.5 rounded bg-zinc-800 text-zinc-400">
                    {title.type}
                  </span>
                  {#if title.resolution}
                    <ResolutionBadge resolution={title.resolution} size="xs" />
                  {/if}
                  {#if title.source}
                    <span class="text-[10px] font-mono text-zinc-400">{title.source}</span>
                  {/if}
                </div>
              </div>
            </div>

            <!-- Right: Quick Play action -->
            <button
              type="button"
              onclick={(e) => {
                e.stopPropagation();
                onQuickPlay(title);
              }}
              class="flex-shrink-0 p-2 rounded-lg bg-zinc-800/80 hover:bg-amber-500 hover:text-zinc-950 text-zinc-300 transition-colors"
              title="Quick Stream"
              aria-label="Quick Play"
            >
              <Play class="w-3.5 h-3.5 fill-current" />
            </button>
          </div>
        {/each}
      </div>

      <!-- Footer action to trigger full search -->
      <button
        type="button"
        onclick={() => {
          isOpen = false;
          onFullSearch(query.trim());
        }}
        class="w-full px-3 py-2 text-center text-xs text-amber-400 hover:bg-amber-500/10 transition-colors font-medium"
      >
        View all search results for "{query}" →
      </button>
    </div>
  {/if}
</div>
