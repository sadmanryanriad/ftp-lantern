<script lang="ts">
  import { Zap, Settings, Flame, Star, Film, Tv, Compass, Clapperboard, Sparkles } from '@lucide/svelte';
  import type { SystemStatus, SchedulerStatus } from '../types';

  let {
    status = null,
    scheduler = null,
    activeTab = 'home',
    onSelectTab = (_tab: string) => {},
    onOpenSettings = () => {}
  }: {
    status: SystemStatus | null;
    scheduler: SchedulerStatus | null;
    activeTab: string;
    onSelectTab: (tab: string) => void;
    onOpenSettings: () => void;
  } = $props();

  const isConnected = $derived(status ? Boolean(status.bdix_connected) : true);
  const isTurbo = $derived(scheduler?.turbo_mode ?? false);
  const isCrawling = $derived(status?.is_crawling ?? false);
  const crawlSpeed = $derived(Math.round(status?.items_per_second || 0));

  const navItems = [
    { id: 'home', label: 'Home', icon: Flame },
    { id: 'movies', label: 'Movies', icon: Film },
    { id: 'series', label: 'TV Series', icon: Tv },
    { id: 'anime', label: 'Anime & Cartoons', icon: Sparkles },
    { id: 'documentary', label: 'Docs', icon: Compass },
    { id: 'following', label: 'Following', icon: Star },
  ];
</script>

<header class="sticky top-0 z-40 w-full border-b border-zinc-800/80 bg-zinc-950/95 backdrop-blur-md">
  <div class="max-w-7xl mx-auto px-3 sm:px-6 lg:px-8 h-16 flex items-center justify-between gap-2 sm:gap-4">
    <!-- Brand / Logo -->
    <a
      href="#/"
      onclick={(e) => { e.preventDefault(); onSelectTab('home'); }}
      class="flex items-center gap-2.5 flex-shrink-0 group"
    >
      <div class="relative flex items-center justify-center w-9 h-9 sm:w-10 sm:h-10 rounded-xl bg-gradient-to-br from-amber-500/20 via-orange-500/10 to-transparent border border-amber-500/30 glow-amber transition-transform group-hover:scale-105">
        <Flame class="w-5 h-5 text-amber-400" />
        <span class="absolute -top-0.5 -right-0.5 flex h-2 w-2">
          <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75"></span>
          <span class="relative inline-flex rounded-full h-2 w-2 bg-amber-500"></span>
        </span>
      </div>
      <div>
        <div class="flex items-center gap-1.5 sm:gap-2">
          <span class="text-base sm:text-lg font-extrabold tracking-tight bg-gradient-to-r from-zinc-100 via-zinc-200 to-amber-200 bg-clip-text text-transparent">
            FTP-Lantern
          </span>
          <span class="px-1.5 py-0.2 rounded text-[10px] font-mono font-semibold tracking-wider bg-amber-500/10 text-amber-400 border border-amber-500/20">
            BDIX
          </span>
        </div>
      </div>
    </a>

    <!-- Center Navigation Tabs -->
    <nav class="hidden md:flex items-center gap-1 bg-zinc-900/60 p-1 rounded-xl border border-zinc-800/60" aria-label="Main navigation">
      {#each navItems as item}
        {@const Icon = item.icon}
        {@const isActive = activeTab === item.id}
        <button
          type="button"
          onclick={() => onSelectTab(item.id)}
          class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all {isActive
            ? 'bg-amber-500/15 text-amber-300 border border-amber-500/30 shadow-sm'
            : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/50'}"
        >
          <Icon class="w-3.5 h-3.5 {isActive ? (item.id === 'following' ? 'text-amber-400 fill-amber-400' : 'text-amber-400') : 'text-zinc-400'}" />
          <span>{item.label}</span>
        </button>
      {/each}
    </nav>

    <!-- Right Actions / Status Badges -->
    <div class="flex items-center gap-2 sm:gap-3 flex-shrink-0">
      
      <!-- Live Indexing Indicator -->
      {#if isCrawling}
        <div
          class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-amber-500/10 border border-amber-500/30 text-amber-300 animate-pulse"
          title={`Currently indexing: ${status?.current_source || 'Dhaka-Flix'}`}
        >
          <span class="relative flex h-2 w-2">
            <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75"></span>
            <span class="relative inline-flex rounded-full h-2 w-2 bg-amber-500"></span>
          </span>
          <span class="font-mono text-[11px] hidden sm:inline">
            Indexing {crawlSpeed > 0 ? `~${crawlSpeed} files/s` : 'Active'}
          </span>
        </div>
      {/if}

      <!-- BDIX Health Badge -->
      <div
        class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium border transition-colors {status === null
          ? 'bg-zinc-900/80 text-zinc-400 border-zinc-800'
          : isConnected
          ? 'bg-emerald-950/50 text-emerald-300 border-emerald-500/30'
          : 'bg-rose-950/50 text-rose-300 border-rose-500/30'}"
        title="ISP Media Server Reachability"
      >
        <span class="relative flex h-2 w-2">
          {#if status === null}
            <span class="relative inline-flex rounded-full h-2 w-2 bg-zinc-500"></span>
          {:else if isConnected}
            <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
            <span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
          {:else}
            <span class="relative inline-flex rounded-full h-2 w-2 bg-rose-500"></span>
          {/if}
        </span>
        <span class="hidden lg:inline font-mono text-[11px]">
          {#if status === null}
            Checking...
          {:else if isConnected}
            {status?.isp_latency_ms ? `BDIX ${status.isp_latency_ms}ms` : 'BDIX 10Gbps'}
          {:else}
            Offline
          {/if}
        </span>
      </div>

      <!-- Turbo Mode Chip -->
      <button
        type="button"
        onclick={onOpenSettings}
        class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium border transition-all hover:scale-105 active:scale-95 {isTurbo
          ? 'bg-amber-500/20 text-amber-300 border-amber-500/40 glow-amber'
          : 'bg-zinc-900/80 text-zinc-400 border-zinc-800 hover:text-zinc-200 hover:border-zinc-700'}"
        title={isTurbo ? 'Turbo mode active (refreshing every 15m)' : 'Click to configure crawl rhythm in Settings'}
      >
        <Zap class="w-3.5 h-3.5 {isTurbo ? 'text-amber-400 fill-amber-400/30 animate-pulse' : 'text-zinc-400'}" />
        <span class="font-mono text-[11px]">
          {isTurbo ? 'Turbo: 15m' : (scheduler?.frequency === '15m' ? 'Pulse: 15m' : scheduler?.frequency === '1h' ? 'Hourly' : scheduler?.frequency === 'manual' ? 'Manual' : 'Daily: 7am')}
        </span>
      </button>

      <!-- Settings Button -->
      <button
        type="button"
        onclick={onOpenSettings}
        class="p-2 rounded-lg bg-zinc-900/80 text-zinc-400 hover:text-zinc-100 border border-zinc-800 hover:border-zinc-700 transition-colors"
        title="Settings & System Telemetry"
        aria-label="Settings"
      >
        <Settings class="w-4 h-4" />
      </button>
    </div>
  </div>

  <!-- Mobile Navigation Bar -->
  <div class="flex md:hidden items-center justify-around border-t border-zinc-800/60 bg-zinc-950 px-2 py-1.5 overflow-x-auto">
    {#each navItems as item}
      {@const Icon = item.icon}
      {@const isActive = activeTab === item.id}
      <button
        type="button"
        onclick={() => onSelectTab(item.id)}
        class="flex flex-col items-center gap-0.5 px-2.5 py-1 rounded-lg text-[10px] font-medium transition-colors {isActive
          ? 'text-amber-400 font-bold'
          : 'text-zinc-400'}"
      >
        <Icon class="w-4 h-4 {isActive ? (item.id === 'following' ? 'text-amber-400 fill-amber-400' : 'text-amber-400') : 'text-zinc-400'}" />
        <span>{item.label}</span>
      </button>
    {/each}
  </div>
</header>
