<script lang="ts">
  import {
    X,
    Zap,
    RefreshCw,
    HardDrive,
    Database,
    Clock,
    AlertCircle,
    CheckCircle2,
    Film,
    Tv,
    FolderKanban,
    Flame,
    Gauge,
    Bell
  } from '@lucide/svelte';
  import type { SystemStatus, SchedulerStatus } from '../types';
  import { triggerCrawl, setTurboMode } from '../api';

  let {
    isOpen = false,
    status = null,
    scheduler = null,
    onClose = () => {},
    onRefresh = () => {}
  }: {
    isOpen: boolean;
    status: SystemStatus | null;
    scheduler: SchedulerStatus | null;
    onClose: () => void;
    onRefresh: () => void;
  } = $props();

  let crawlTriggering = $state(false);
  let crawlFeedback = $state<string | null>(null);
  let turboLoading = $state<number | null>(null);

  // Turbo durations: 1h, 3h, 6h, 12h, 24h (1d), 72h (3d), 168h (7d max)
  const turboOptions = [
    { label: '1 Hour', hours: 1 },
    { label: '3 Hours', hours: 3 },
    { label: '6 Hours', hours: 6 },
    { label: '12 Hours', hours: 12 },
    { label: '24 Hours (1 Day)', hours: 24 },
    { label: '3 Days', hours: 72 },
    { label: '7 Days (Max)', hours: 168 },
  ];

  async function handleTriggerCrawl() {
    crawlTriggering = true;
    crawlFeedback = null;
    try {
      const res = await triggerCrawl();
      crawlFeedback = res.message || 'Crawl initiated successfully!';
      onRefresh();
    } catch (err: any) {
      crawlFeedback = err.message || 'Failed to trigger crawl';
    } finally {
      crawlTriggering = false;
    }
  }

  async function handleTurboSelect(hours: number) {
    turboLoading = hours;
    try {
      await setTurboMode(hours, 15);
      onRefresh();
    } catch (err) {
      console.error('Failed to set turbo mode:', err);
    } finally {
      turboLoading = null;
    }
  }

  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === 'Escape') onClose();
  }

  $effect(() => {
    if (isOpen) {
      window.addEventListener('keydown', handleKeyDown);
      return () => window.removeEventListener('keydown', handleKeyDown);
    }
  });
</script>

{#if isOpen}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-6 bg-zinc-950/85 backdrop-blur-md overflow-y-auto" role="dialog" aria-modal="true">
    <!-- Backdrop click -->
    <div
      role="button"
      tabindex="-1"
      onclick={onClose}
      onkeydown={(e) => { if (e.key === 'Enter') onClose(); }}
      class="fixed inset-0 cursor-default"
      aria-label="Close settings backdrop"
    ></div>

    <!-- Modal Box -->
    <div class="relative w-full max-w-2xl rounded-2xl bg-zinc-900 border border-zinc-800 shadow-2xl overflow-hidden z-10 animate-in fade-in zoom-in-95 duration-200">
      
      <!-- Header -->
      <div class="p-5 border-b border-zinc-800 flex items-center justify-between bg-zinc-950/60">
        <div class="flex items-center gap-2.5">
          <div class="p-2 rounded-xl bg-amber-500/10 text-amber-400 border border-amber-500/20">
            <Flame class="w-5 h-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-zinc-100">Index & System Settings</h2>
            <p class="text-xs text-zinc-400">Library breakdown, Turbo crawl & storage persistence</p>
          </div>
        </div>

        <button
          type="button"
          onclick={onClose}
          class="p-2 rounded-xl bg-zinc-850 hover:bg-zinc-800 text-zinc-400 hover:text-zinc-100 transition-colors"
          aria-label="Close modal"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- Content -->
      <div class="p-5 space-y-5 max-h-[82vh] overflow-y-auto">

        <!-- Library Index Breakdown Section -->
        <div>
          <h3 class="text-xs font-mono uppercase text-zinc-400 tracking-wider mb-2.5 flex items-center gap-2">
            <Database class="w-3.5 h-3.5 text-amber-400" />
            <span>Indexed Library Breakdown</span>
          </h3>

          <div class="grid grid-cols-2 sm:grid-cols-4 gap-2.5 font-mono text-xs">
            <!-- Movies -->
            <div class="p-3 rounded-xl bg-zinc-950/80 border border-zinc-800">
              <div class="flex items-center gap-1.5 text-zinc-400 mb-1">
                <Film class="w-3.5 h-3.5 text-amber-400" />
                <span>Movies</span>
              </div>
              <div class="text-base font-bold text-zinc-100">
                {status?.movies_count?.toLocaleString() ?? 0}
              </div>
            </div>

            <!-- Series -->
            <div class="p-3 rounded-xl bg-zinc-950/80 border border-zinc-800">
              <div class="flex items-center gap-1.5 text-zinc-400 mb-1">
                <Tv class="w-3.5 h-3.5 text-sky-400" />
                <span>TV Series</span>
              </div>
              <div class="text-base font-bold text-zinc-100">
                {status?.series_count?.toLocaleString() ?? 0}
              </div>
            </div>

            <!-- Others -->
            <div class="p-3 rounded-xl bg-zinc-950/80 border border-zinc-800">
              <div class="flex items-center gap-1.5 text-zinc-400 mb-1">
                <FolderKanban class="w-3.5 h-3.5 text-emerald-400" />
                <span>Others & Docs</span>
              </div>
              <div class="text-base font-bold text-zinc-100">
                {status?.others_count?.toLocaleString() ?? 0}
              </div>
            </div>

            <!-- Video Files -->
            <div class="p-3 rounded-xl bg-zinc-950/80 border border-zinc-800">
              <div class="flex items-center gap-1.5 text-zinc-400 mb-1">
                <HardDrive class="w-3.5 h-3.5 text-violet-400" />
                <span>Media Files</span>
              </div>
              <div class="text-base font-bold text-amber-300">
                {status?.files_count?.toLocaleString() ?? 0}
              </div>
            </div>
          </div>
        </div>

        <!-- Storage & Persistence Telemetry -->
        <div class="p-4 rounded-xl bg-zinc-950/60 border border-zinc-800 text-xs">
          <div class="flex items-center justify-between mb-2">
            <span class="font-bold text-zinc-200">Storage Persistence (Dokploy Volume)</span>
            <span class="font-mono text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded border border-emerald-500/20">
              DISK WAL MODE (PERSISTENT)
            </span>
          </div>
          <p class="text-zinc-400 mb-3 text-[11px] leading-relaxed">
            Index is saved permanently to host disk at <code class="text-zinc-300 bg-zinc-800 px-1 py-0.5 rounded">ftp-lantern-data:/data/lantern.db</code>. It is NOT kept in RAM and survives all deployments and restarts.
          </p>

          <div class="grid grid-cols-2 sm:grid-cols-3 gap-2 font-mono text-[11px]">
            <div class="p-2.5 rounded-lg bg-zinc-900 border border-zinc-800">
              <span class="text-zinc-500">Database Size</span>
              <div class="text-sm font-bold text-zinc-200 mt-0.5">{status?.db_size_human || '< 15 MB'}</div>
            </div>
            <div class="p-2.5 rounded-lg bg-zinc-900 border border-zinc-800">
              <span class="text-zinc-500">Thumbnail Cache</span>
              <div class="text-sm font-bold text-zinc-200 mt-0.5">{status?.thumbs_size_human || '< 10 MB'}</div>
            </div>
            <div class="p-2.5 rounded-lg bg-zinc-900 border border-zinc-800 col-span-2 sm:col-span-1">
              <span class="text-zinc-500">RAM Allocation</span>
              <div class="text-sm font-bold text-zinc-200 mt-0.5">{status?.memory_alloc_mb ? `${status.memory_alloc_mb.toFixed(1)} MB` : '< 25 MB'}</div>
            </div>
          </div>
        </div>

        <!-- Turbo Scan Mode Section -->
        <div class="p-4 rounded-xl bg-zinc-950/80 border border-amber-500/30 glow-amber">
          <div class="flex items-center justify-between mb-2">
            <div class="flex items-center gap-2">
              <Zap class="w-4 h-4 text-amber-400 fill-amber-400/20" />
              <h3 class="text-sm font-bold text-zinc-100">15-Minute Turbo Crawl Mode</h3>
            </div>
            {#if scheduler?.turbo_mode}
              <span class="px-2 py-0.5 rounded-full text-[10px] font-mono font-bold bg-amber-500/20 text-amber-300 border border-amber-500/40">
                ACTIVE (15m interval)
              </span>
            {:else}
              <span class="px-2 py-0.5 rounded-full text-[10px] font-mono bg-zinc-800 text-zinc-400">
                DAILY (7:00 AM)
              </span>
            {/if}
          </div>

          <p class="text-xs text-zinc-400 mb-3 leading-relaxed">
            Normally, index auto-refreshes <strong>once everyday at 7:00 AM</strong>, followed by a Telegram summary at <strong>8:01 AM</strong>. Followed titles always receive instant alerts.
            <br />
            When you want rapid live updates, turn on Turbo mode to scan <strong>every 15 minutes</strong> for up to 7 days:
          </p>

          <div class="flex flex-wrap items-center gap-1.5">
            {#each turboOptions as opt}
              <button
                type="button"
                onclick={() => handleTurboSelect(opt.hours)}
                disabled={turboLoading === opt.hours}
                class="px-2.5 py-1.5 rounded-lg text-xs font-mono font-semibold transition-all {scheduler?.turbo_mode
                  ? 'bg-zinc-800 hover:bg-amber-500 hover:text-zinc-950 text-zinc-300'
                  : 'bg-amber-500/10 hover:bg-amber-500 hover:text-zinc-950 text-amber-300 border border-amber-500/30'}"
              >
                {turboLoading === opt.hours ? '...' : opt.label}
              </button>
            {/each}
          </div>
        </div>

        <!-- Manual Crawl Trigger -->
        <div class="p-4 rounded-xl bg-zinc-950/60 border border-zinc-800 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div>
            <div class="flex items-center gap-2">
              <RefreshCw class="w-4 h-4 text-zinc-300" />
              <h3 class="text-sm font-semibold text-zinc-200">Manual Quick Scan</h3>
            </div>
            <p class="text-xs text-zinc-400 mt-1">
              Checks mtimes and indexes only newly added files without re-indexing unchanged folders.
            </p>
          </div>

          <button
            type="button"
            onclick={handleTriggerCrawl}
            disabled={crawlTriggering || status?.is_crawling}
            class="inline-flex items-center justify-center gap-2 px-4 py-2 rounded-xl bg-amber-500 hover:bg-amber-400 text-zinc-950 font-bold text-xs transition-all hover:scale-105 active:scale-95 disabled:opacity-50 flex-shrink-0"
          >
            <RefreshCw class="w-3.5 h-3.5 {crawlTriggering || status?.is_crawling ? 'animate-spin' : ''}" />
            <span>{status?.is_crawling ? 'Indexing...' : 'Run Scan Now'}</span>
          </button>
        </div>

        {#if crawlFeedback}
          <div class="p-3 rounded-xl bg-zinc-800/80 border border-zinc-700 text-xs font-mono text-zinc-200 flex items-center gap-2">
            <CheckCircle2 class="w-4 h-4 text-emerald-400 flex-shrink-0" />
            <span>{crawlFeedback}</span>
          </div>
        {/if}

      </div>
    </div>
  </div>
{/if}
