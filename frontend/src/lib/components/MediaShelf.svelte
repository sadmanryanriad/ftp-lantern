<script lang="ts">
  import { ChevronLeft, ChevronRight, Play, ArrowRight, Star } from '@lucide/svelte';
  import type { Title } from '../types';
  import { getThumbUrl, displayTitleWithYear } from '../api';
  import PosterImage from './PosterImage.svelte';
  import ResolutionBadge from './ResolutionBadge.svelte';

  let {
    title = '',
    icon = '🎬',
    items = [],
    categories = [],
    activeCategory = 'All',
    onSelectCategory = (_cat: string) => {},
    onSelectTitle = (_title: Title) => {},
    onExploreAll = () => {}
  }: {
    title: string;
    icon?: string;
    items: Title[];
    categories?: string[];
    activeCategory?: string;
    onSelectCategory?: (cat: string) => void;
    onSelectTitle: (title: Title) => void;
    onExploreAll?: () => void;
  } = $props();

  let scrollContainer: HTMLDivElement | null = $state(null);

  function scroll(direction: 'left' | 'right') {
    if (!scrollContainer) return;
    const offset = direction === 'left' ? -550 : 550;
    scrollContainer.scrollBy({ left: offset, behavior: 'smooth' });
  }
</script>

<section class="py-5">
  <!-- Header Bar -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-3">
    <div class="flex items-center gap-2">
      <span class="text-xl">{icon}</span>
      <h2 class="text-lg sm:text-xl font-bold tracking-tight text-zinc-100">{title}</h2>
      <span class="text-xs font-mono text-zinc-400">({items.length} titles)</span>
    </div>

    <div class="flex items-center gap-2">
      {#if onExploreAll}
        <button
          type="button"
          onclick={onExploreAll}
          class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold text-amber-400 hover:text-amber-300 hover:bg-amber-400/10 transition-colors"
        >
          <span>Explore All</span>
          <ArrowRight class="w-3.5 h-3.5" />
        </button>
      {/if}

      <!-- Carousel Scroll Navigation -->
      <div class="hidden sm:flex items-center gap-1">
        <button
          type="button"
          onclick={() => scroll('left')}
          class="p-1.5 rounded-lg bg-zinc-900 border border-zinc-800 hover:bg-zinc-800 text-zinc-400 hover:text-zinc-100 transition-colors"
          aria-label="Scroll left"
        >
          <ChevronLeft class="w-4 h-4" />
        </button>
        <button
          type="button"
          onclick={() => scroll('right')}
          class="p-1.5 rounded-lg bg-zinc-900 border border-zinc-800 hover:bg-zinc-800 text-zinc-400 hover:text-zinc-100 transition-colors"
          aria-label="Scroll right"
        >
          <ChevronRight class="w-4 h-4" />
        </button>
      </div>
    </div>
  </div>

  <!-- Quick Subcategory Filter Chips -->
  {#if categories && categories.length > 0}
    <div class="flex items-center gap-1.5 overflow-x-auto pb-2 mb-3 scrollbar-none">
      {#each categories as cat}
        {@const isSelected = activeCategory === cat}
        <button
          type="button"
          onclick={() => onSelectCategory(cat)}
          class="px-2.5 py-1 rounded-full text-xs font-medium whitespace-nowrap transition-all {isSelected
            ? 'bg-amber-500 text-zinc-950 font-bold shadow-sm'
            : 'bg-zinc-900/80 text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 border border-zinc-800'}"
        >
          {cat}
        </button>
      {/each}
    </div>
  {/if}

  <!-- Horizontal Scrollable Cards Container -->
  {#if items.length > 0}
    <div
      bind:this={scrollContainer}
      class="flex gap-3 sm:gap-4 overflow-x-auto pb-3 scroll-smooth scrollbar-none snap-x"
    >
      {#each items as item (item.id)}
        <div
          role="button"
          tabindex="0"
          onclick={() => onSelectTitle(item)}
          onkeydown={(e) => { if (e.key === 'Enter') onSelectTitle(item); }}
          class="w-36 sm:w-44 flex-shrink-0 snap-start group cursor-pointer text-left focus:outline-none"
        >
          <div class="relative aspect-[2/3] w-full rounded-xl overflow-hidden bg-zinc-900 border border-zinc-800 shadow-md group-hover:border-amber-500/50 group-hover:shadow-amber-500/10 transition-all duration-300">
            <PosterImage
              src={getThumbUrl(item)}
              alt={item.name}
            />

            <!-- Top Badges -->
            <div class="absolute top-2 left-2 flex flex-col gap-1 items-start z-10 pointer-events-none">
              {#if item.resolution}
                <ResolutionBadge resolution={item.resolution} hasHDR={Boolean(item.has_hdr)} />
              {/if}
            </div>

            <!-- Size / Following Top Right Badge -->
            <div class="absolute top-2 right-2 flex items-center gap-1 z-10 pointer-events-none">
              {#if item.is_following}
                <span class="p-1 rounded-md bg-zinc-950/80 text-amber-400 border border-amber-500/40 shadow-sm" title="Following series">
                  <Star class="w-3 h-3 fill-amber-400 text-amber-400" />
                </span>
              {/if}
              {#if item.size_human}
                <span class="px-1.5 py-0.5 rounded text-[10px] font-mono font-bold bg-zinc-950/85 text-zinc-300 border border-zinc-800">
                  {item.size_human}
                </span>
              {/if}
            </div>

            <!-- Hover Play Overlay -->
            <div class="absolute inset-0 bg-gradient-to-t from-zinc-950 via-zinc-950/40 to-transparent opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center">
              <div class="w-11 h-11 rounded-full bg-amber-500 text-zinc-950 flex items-center justify-center shadow-lg transform group-hover:scale-110 transition-transform">
                <Play class="w-5 h-5 fill-zinc-950 ml-0.5" />
              </div>
            </div>
          </div>

          <!-- Title Name with Year in Bracket -->
          <div class="mt-2">
            <h3 class="text-xs sm:text-sm font-semibold text-zinc-200 group-hover:text-amber-400 transition-colors line-clamp-1" title={displayTitleWithYear(item.name, item.year)}>
              {displayTitleWithYear(item.name, item.year)}
            </h3>
            <div class="flex items-center justify-between text-[11px] font-mono text-zinc-400 mt-0.5">
              <span>{item.category || (item.type === 'movie' ? 'Movie' : 'Series')}</span>
              {#if item.size_human}
                <span class="text-zinc-400">{item.size_human}</span>
              {/if}
            </div>
          </div>
        </div>
      {/each}
    </div>
  {:else}
    <div class="rounded-xl border border-zinc-900 bg-zinc-950/50 p-8 text-center text-zinc-400 text-xs font-mono">
      No items found in this section.
    </div>
  {/if}
</section>
