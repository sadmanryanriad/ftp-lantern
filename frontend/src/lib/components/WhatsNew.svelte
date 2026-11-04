<script lang="ts">
  import { Sparkles, ChevronLeft, ChevronRight, Play, Star } from '@lucide/svelte';
  import type { Title } from '../types';
  import { getThumbUrl, displayTitleWithYear } from '../api';
  import PosterImage from './PosterImage.svelte';
  import ResolutionBadge from './ResolutionBadge.svelte';

  let {
    items = [],
    onSelectTitle = (_title: Title) => {},
    onViewAll = () => {}
  }: {
    items: Title[];
    onSelectTitle: (title: Title) => void;
    onViewAll?: () => void;
  } = $props();

  let carouselRef = $state<HTMLDivElement | null>(null);

  function scroll(direction: 'left' | 'right') {
    if (!carouselRef) return;
    const scrollAmount = direction === 'left' ? -carouselRef.clientWidth * 0.75 : carouselRef.clientWidth * 0.75;
    carouselRef.scrollBy({ left: scrollAmount, behavior: 'smooth' });
  }

  function formatRelativeTime(dateStr?: string): string {
    if (!dateStr) return 'Recently';
    try {
      const diffMs = Date.now() - new Date(dateStr).getTime();
      const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
      if (diffHours < 1) return 'Just now';
      if (diffHours < 24) return `${diffHours}h ago`;
      const diffDays = Math.floor(diffHours / 24);
      if (diffDays === 1) return 'Yesterday';
      return `${diffDays}d ago`;
    } catch {
      return 'Recently';
    }
  }
</script>

{#if items.length > 0}
  <section class="py-5 border-b border-zinc-800/60">
    <!-- Header with controls -->
    <div class="flex items-center justify-between mb-3">
      <div class="flex items-center gap-2">
        <Sparkles class="w-4 h-4 text-amber-400" />
        <h2 class="text-base sm:text-lg font-bold text-zinc-100 uppercase tracking-wider">
          What's New on BDIX
        </h2>
        <span class="px-2 py-0.5 rounded-full text-[11px] font-mono bg-zinc-800 text-zinc-400">
          {items.length} new
        </span>
      </div>

      <!-- Controls -->
      <div class="flex items-center gap-2">
        {#if onViewAll}
          <button
            type="button"
            onclick={onViewAll}
            class="text-xs font-semibold text-amber-400 hover:text-amber-300 hover:underline px-2 py-1"
          >
            View All Releases
          </button>
        {/if}

        <div class="hidden sm:flex items-center gap-1">
          <button
            type="button"
            onclick={() => scroll('left')}
            class="p-1.5 rounded-lg bg-zinc-900 border border-zinc-800 text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800 transition-colors"
            aria-label="Scroll left"
          >
            <ChevronLeft class="w-4 h-4" />
          </button>
          <button
            type="button"
            onclick={() => scroll('right')}
            class="p-1.5 rounded-lg bg-zinc-900 border border-zinc-800 text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800 transition-colors"
            aria-label="Scroll right"
          >
            <ChevronRight class="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>

    <!-- Carousel container -->
    <div
      bind:this={carouselRef}
      class="flex gap-3 sm:gap-4 overflow-x-auto pb-3 pt-1 scroll-smooth scrollbar-thin snap-x"
    >
      {#each items as item (item.id)}
        <div
          role="button"
          tabindex="0"
          onclick={() => onSelectTitle(item)}
          onkeydown={(e) => { if (e.key === 'Enter') onSelectTitle(item); }}
          class="group relative flex-shrink-0 w-36 sm:w-44 rounded-xl overflow-hidden bg-zinc-900 border border-zinc-800/80 hover:border-amber-500/50 hover:shadow-xl hover:shadow-amber-500/5 transition-all duration-300 cursor-pointer text-left flex flex-col snap-start"
        >
          <!-- Poster -->
          <div class="relative w-full aspect-[2/3] overflow-hidden">
            <PosterImage
              src={getThumbUrl(item)}
              alt={item.name}
              aspect="aspect-[2/3]"
              class="group-hover:scale-105 transition-transform duration-500"
            />

            <!-- Top Badges -->
            <div class="absolute top-2 left-2 flex flex-col gap-1 items-start z-10">
              {#if item.resolution}
                <ResolutionBadge resolution={item.resolution} size="xs" />
              {/if}
            </div>

            <div class="absolute top-2 right-2 flex items-center gap-1 z-10">
              {#if item.is_following}
                <span class="p-1 rounded-md bg-zinc-950/80 text-amber-400 border border-amber-500/40">
                  <Star class="w-3 h-3 fill-amber-400 text-amber-400" />
                </span>
              {/if}
              {#if item.size_human}
                <span class="px-1.5 py-0.5 rounded text-[9px] font-mono font-bold bg-zinc-950/80 text-zinc-300 border border-zinc-800">
                  {item.size_human}
                </span>
              {/if}
            </div>

            <!-- Bottom badge on poster: added time -->
            <div class="absolute bottom-2 left-2 right-2 flex items-center justify-between text-[10px] font-mono text-zinc-300 z-10">
              <span class="px-1.5 py-0.5 rounded bg-zinc-900/80 backdrop-blur-sm border border-zinc-700/50">
                {formatRelativeTime(item.first_seen)}
              </span>
              <span class="px-1.5 py-0.5 rounded bg-zinc-900/80 backdrop-blur-sm border border-zinc-700/50 uppercase text-[9px]">
                {item.type}
              </span>
            </div>
          </div>

          <!-- Title Details with Year in Bracket -->
          <div class="p-2.5 flex-1 flex flex-col justify-between bg-zinc-900">
            <div>
              <h3 class="text-xs sm:text-sm font-bold text-zinc-100 truncate group-hover:text-amber-300 transition-colors" title={displayTitleWithYear(item.name, item.year)}>
                {displayTitleWithYear(item.name, item.year)}
              </h3>
              <div class="flex items-center justify-between mt-1 text-[11px] text-zinc-400 font-mono">
                <span>{item.category || item.source || ''}</span>
                {#if item.size_human}
                  <span class="text-zinc-500">{item.size_human}</span>
                {/if}
              </div>
            </div>

            <div class="mt-2 pt-2 border-t border-zinc-800/60 flex items-center justify-between">
              <span class="text-[10px] text-amber-400/90 font-medium group-hover:underline">
                Explore Releases
              </span>
              <div class="w-5 h-5 rounded-full bg-zinc-800 group-hover:bg-amber-500 group-hover:text-zinc-950 flex items-center justify-center transition-colors">
                <Play class="w-2.5 h-2.5 fill-current ml-0.5" />
              </div>
            </div>
          </div>
        </div>
      {/each}
    </div>
  </section>
{/if}
