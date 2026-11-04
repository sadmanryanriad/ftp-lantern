<script lang="ts">
  import { Play, Check, Clock, ChevronRight } from '@lucide/svelte';
  import type { ContinueWatchingItem, Title } from '../types';
  import { getThumbUrl, markWatchedUpTo, downloadM3U, recordPlay } from '../api';
  import ResolutionBadge from './ResolutionBadge.svelte';

  let {
    items = [],
    onSelectTitle = (_id: number) => {},
    onPlayNext = (_item: ContinueWatchingItem) => {}
  }: {
    items: ContinueWatchingItem[];
    onSelectTitle: (id: number) => void;
    onPlayNext: (item: ContinueWatchingItem) => void;
  } = $props();

  let markedIds = $state<Set<number>>(new Set());
  let loadingMark = $state<number | null>(null);

  async function handleMarkWatched(e: MouseEvent, item: ContinueWatchingItem) {
    e.stopPropagation();
    const targetFileId = item.next_file_id || item.last_played_file_id;
    if (!targetFileId) return;

    loadingMark = targetFileId;
    try {
      await markWatchedUpTo(targetFileId);
      markedIds = new Set(markedIds).add(item.title_id);
    } catch (err) {
      console.error('Failed to mark watched:', err);
    } finally {
      loadingMark = null;
    }
  }

  function handleQuickPlay(e: MouseEvent, item: ContinueWatchingItem) {
    e.stopPropagation();
    onPlayNext(item);
  }
</script>

{#if items.length > 0}
  <section class="py-6 border-b border-zinc-800/60">
    <div class="flex items-center justify-between mb-4">
      <div class="flex items-center gap-2">
        <Clock class="w-4 h-4 text-amber-400" />
        <h2 class="text-base font-bold text-zinc-100 uppercase tracking-wider">
          Continue Watching
        </h2>
        <span class="px-2 py-0.5 rounded-full text-[11px] font-mono bg-zinc-800 text-zinc-400">
          {items.length}
        </span>
      </div>
    </div>

    <!-- Horizontal scroll shelf -->
    <div class="flex gap-4 overflow-x-auto pb-3 pt-1 scrollbar-thin">
      {#each items as item (item.title_id)}
        <div
          role="button"
          tabindex="0"
          onclick={() => onSelectTitle(item.title_id)}
          onkeydown={(e) => { if (e.key === 'Enter') onSelectTitle(item.title_id); }}
          class="group relative flex-shrink-0 w-72 sm:w-80 rounded-xl bg-zinc-900/80 border border-zinc-800 hover:border-amber-500/40 hover:bg-zinc-850/80 p-3 flex gap-3 transition-all duration-200 cursor-pointer shadow-lg hover:shadow-amber-500/5 text-left"
        >
          <!-- Thumbnail -->
          <div class="w-20 h-28 rounded-lg overflow-hidden bg-zinc-950 flex-shrink-0 border border-zinc-800 relative">
            {#if item.thumb_hash || item.poster_url}
              <img
                src={getThumbUrl({ id: item.title_id, thumb_hash: item.thumb_hash, poster_url: item.poster_url })}
                alt={item.title_name}
                loading="lazy"
                class="w-full h-full object-cover group-hover:scale-105 transition-transform duration-300"
                onerror={(e) => {
                  (e.currentTarget as HTMLImageElement).style.display = 'none';
                }}
              />
            {/if}
            <div class="absolute inset-0 bg-gradient-to-t from-zinc-950/80 to-transparent"></div>
          </div>

          <!-- Info & Actions -->
          <div class="flex-1 min-w-0 flex flex-col justify-between">
            <div>
              <div class="flex items-center justify-between gap-1 mb-1">
                <span class="text-xs font-mono text-zinc-400 truncate">
                  {item.last_played_at ? new Date(item.played_at || item.last_played_at).toLocaleDateString() : 'Recent'}
                </span>
                {#if item.resolution}
                  <ResolutionBadge resolution={item.resolution} size="xs" />
                {/if}
              </div>

              <h3 class="text-sm font-semibold text-zinc-100 truncate group-hover:text-amber-300 transition-colors">
                {item.title_name}
              </h3>

              <!-- Last played -> Next indicator -->
              <div class="mt-1.5 p-1.5 rounded-lg bg-zinc-950/70 border border-zinc-800/60 font-mono text-[11px]">
                <div class="text-zinc-400 truncate">
                  <span class="text-zinc-500">Played:</span> {item.last_played_code || 'Episode'}
                </div>
                {#if item.next_episode_code}
                  <div class="text-amber-400 font-semibold truncate flex items-center gap-1 mt-0.5">
                    <span>Next:</span> {item.next_episode_code}
                  </div>
                {/if}
              </div>
            </div>

            <!-- Action buttons -->
            <div class="flex items-center justify-between gap-2 mt-3 pt-2 border-t border-zinc-800/60">
              <!-- 1-Click Play Next -->
              <button
                type="button"
                onclick={(e) => handleQuickPlay(e, item)}
                class="flex-1 inline-flex items-center justify-center gap-1.5 px-3 py-1.5 rounded-lg bg-amber-500 hover:bg-amber-400 text-zinc-950 font-semibold text-xs transition-colors"
                title="Launch Next Episode"
              >
                <Play class="w-3.5 h-3.5 fill-current" />
                <span>Play Next</span>
              </button>

              <!-- Mark Watched Up to Here Checkmark -->
              <button
                type="button"
                onclick={(e) => handleMarkWatched(e, item)}
                class="p-1.5 rounded-lg border transition-colors {markedIds.has(item.title_id)
                  ? 'bg-emerald-500/20 text-emerald-400 border-emerald-500/40'
                  : 'bg-zinc-800/80 text-zinc-400 border-zinc-700/60 hover:text-zinc-100 hover:bg-zinc-700'}"
                title="Mark watched up to here"
                aria-label="Mark watched"
              >
                <Check class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        </div>
      {/each}
    </div>
  </section>
{/if}
