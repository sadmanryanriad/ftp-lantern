<script lang="ts">
  import { Film } from '@lucide/svelte';

  let {
    src = '',
    alt = '',
    class: customClass = '',
    aspect = 'aspect-[2/3]'
  }: {
    src?: string;
    alt?: string;
    class?: string;
    aspect?: string;
  } = $props();

  let loaded = $state(false);
  let error = $state(false);

  // Reset states if src changes
  $effect(() => {
    if (src) {
      loaded = false;
      error = false;
    } else {
      error = true;
    }
  });
</script>

<div class="relative overflow-hidden bg-zinc-900 {aspect} {customClass}">
  {#if !loaded && !error}
    <div class="absolute inset-0 bg-gradient-to-br from-zinc-800/80 to-zinc-950 animate-pulse flex items-center justify-center">
      <Film class="w-8 h-8 text-zinc-700/60" />
    </div>
  {/if}

  {#if src && !error}
    <img
      {src}
      {alt}
      loading="lazy"
      class="w-full h-full object-cover transition-opacity duration-300 {loaded ? 'opacity-100' : 'opacity-0'}"
      onload={() => { loaded = true; }}
      onerror={() => { error = true; }}
    />
  {/if}

  {#if error || !src}
    <div class="absolute inset-0 flex flex-col items-center justify-center p-3 text-center bg-gradient-to-b from-zinc-800 to-zinc-950 border border-zinc-800/80">
      <div class="w-10 h-10 rounded-full bg-zinc-800/80 flex items-center justify-center mb-2 text-zinc-500">
        <Film class="w-5 h-5 text-amber-500/60" />
      </div>
      <span class="text-[11px] font-medium text-zinc-400 line-clamp-2 px-1">{alt || 'No Poster'}</span>
    </div>
  {/if}
</div>
