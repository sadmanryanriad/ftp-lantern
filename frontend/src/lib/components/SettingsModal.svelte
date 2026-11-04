<script lang="ts">
  import {
    X,
    Zap,
    RefreshCw,
    Activity,
    HardDrive,
    Layers,
    Clock,
    AlertCircle,
    CheckCircle2,
    Shield,
    Flame
  } from '@lucide/svelte';
  import type { SystemStatus, SchedulerStatus } from '../types';
  import { triggerCrawl, setTurboMode, fetchStatus, fetchScheduler } from '../api';

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

  const turboDurations = [2, 6, 10, 24];

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
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-zinc-950/80 backdrop-blur-md overflow-y-auto">
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
            <h2 class="text-base font-bold text-zinc-100">Crawler & System Telemetry</h2>
            <p class="text-xs text-zinc-400">Scheduler management, Turbo scan & BDIX gateway status</p>
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
      <div class="p-5 space-y-6 max-h-[80vh] overflow-y-auto">

        <!-- Turbo Scan Control Section -->
        <div class="p-4 rounded-xl bg-zinc-950/80 border border-amber-500/20 glow-amber">
          <div class="flex items-center justify-between mb-2">
            <div class="flex items-center gap-2">
              <Zap class="w-4 h-4 text-amber-400 fill-amber-400/20" />
              <h3 class="text-sm font-bold text-zinc-100">Turbo Scan Mode</h3>
            </div>
            {#if scheduler?.turbo_mode}
              <span class="px-2 py-0.5 rounded-full text-[11px] font-mono font-bold bg-amber-500/20 text-amber-300 border border-amber-500/40">
                ACTIVE (15 min interval)
              </span>
            {:else}
              <span class="px-2 py-0.5 rounded-full text-[11px] font-mono bg-zinc-800 text-zinc-400">
                NORMAL (120 min)
              </span>
            {/if}
          </div>

          <p class="text-xs text-zinc-400 mb-3.5">
            Accelerates crawler runs from 2 hours down to <strong>every 15 minutes</strong> for rapid indexing of new releases.
          </p>

          <div class="flex flex-wrap items-center gap-2">
            <span class="text-xs font-mono text-zinc-400">Run every 15 min for:</span>
            {#each turboDurations as hours}
              <button
                type="button"
                onclick={() => handleTurboSelect(hours)}
                disabled={turboLoading === hours}
                class="px-3 py-1.5 rounded-lg text-xs font-mono font-semibold transition-all {scheduler?.turbo_mode
                  ? 'bg-zinc-800 hover:bg-amber-500 hover:text-zinc-950 text-zinc-300'
                  : 'bg-amber-500/10 hover:bg-amber-500 hover:text-zinc-950 text-amber-300 border border-amber-500/30'}"
              >
                {turboLoading === hours ? '...' : `${hours}h`}
              </button>
            {/each}
          </div>
        </div>

        <!-- Manual Crawl Trigger -->
        <div class="p-4 rounded-xl bg-zinc-950/60 border border-zinc-800 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div>
            <div class="flex items-center gap-2">
              <RefreshCw class="w-4 h-4 text-zinc-300" />
              <h3 class="text-sm font-semibold text-zinc-200">Manual Crawl Trigger</h3>
            </div>
            <p class="text-xs text-zinc-400 mt-1">
              Trigger an immediate incremental diff scan across all ISP directories.
            </p>
          </div>

          <button
            type="button"
            onclick={handleTriggerCrawl}
            disabled={crawlTriggering || status?.is_crawling}
            class="inline-flex items-center justify-center gap-2 px-4 py-2 rounded-xl bg-zinc-100 hover:bg-white text-zinc-950 font-bold text-xs transition-all hover:scale-105 active:scale-95 disabled:opacity-50 flex-shrink-0"
          >
            <RefreshCw class="w-3.5 h-3.5 {crawlTriggering || status?.is_crawling ? 'animate-spin' : ''}" />
            <span>{status?.is_crawling ? 'Scanning...' : 'Scan Now'}</span>
          </button>
        </div>

        {#if crawlFeedback}
          <div class="p-3 rounded-xl bg-zinc-800/80 border border-zinc-700 text-xs font-mono text-zinc-200 flex items-center gap-2">
            <CheckCircle2 class="w-4 h-4 text-emerald-400 flex-shrink-0" />
            <span>{crawlFeedback}</span>
          </div>
        {/if}

        <!-- System Telemetry Grid -->
        <div class="space-y-3">
          <h3 class="text-xs font-mono uppercase text-zinc-400 tracking-wider">
            System & Gateway Metrics
          </h3>

          <div class="grid grid-cols-2 sm:grid-cols-3 gap-3 font-mono">
            <!-- BDIX Gateway -->
            <div class="p-3 rounded-xl bg-zinc-950/60 border border-zinc-800/80">
              <div class="text-[11px] text-zinc-500 uppercase">BDIX Gateway</div>
              <div class="text-sm font-bold mt-1 flex items-center gap-1.5 {status?.bdix_connected ? 'text-emerald-400' : 'text-rose-400'}">
                <span class="w-2 h-2 rounded-full {status?.bdix_connected ? 'bg-emerald-400' : 'bg-rose-400'}"></span>
                <span>{status?.bdix_connected ? 'ONLINE' : 'UNREACHABLE'}</span>
              </div>
            </div>

            <!-- Memory Usage -->
            <div class="p-3 rounded-xl bg-zinc-950/60 border border-zinc-800/80">
              <div class="text-[11px] text-zinc-500 uppercase">Memory RAM</div>
              <div class="text-sm font-bold text-zinc-200 mt-1">
                {status?.memory_alloc_mb ? `${status.memory_alloc_mb.toFixed(1)} MB` : '< 25 MB'}
                <span class="text-[10px] text-zinc-500 font-normal">/ 128 MB</span>
              </div>
            </div>

            <!-- Titles Indexed -->
            <div class="p-3 rounded-xl bg-zinc-950/60 border border-zinc-800/80">
              <div class="text-[11px] text-zinc-500 uppercase">Total Titles</div>
              <div class="text-sm font-bold text-zinc-200 mt-1">
                {status?.titles_count ?? 0}
              </div>
            </div>

            <!-- Files Indexed -->
            <div class="p-3 rounded-xl bg-zinc-950/60 border border-zinc-800/80">
              <div class="text-[11px] text-zinc-500 uppercase">Media Files</div>
              <div class="text-sm font-bold text-zinc-200 mt-1">
                {status?.files_count ?? 0}
              </div>
            </div>

            <!-- Crawl Interval -->
            <div class="p-3 rounded-xl bg-zinc-950/60 border border-zinc-800/80">
              <div class="text-[11px] text-zinc-500 uppercase">Interval</div>
              <div class="text-sm font-bold text-zinc-200 mt-1">
                {scheduler?.interval_minutes ? `${scheduler.interval_minutes}m` : '120m'}
              </div>
            </div>

            <!-- Next Scan -->
            <div class="p-3 rounded-xl bg-zinc-950/60 border border-zinc-800/80">
              <div class="text-[11px] text-zinc-500 uppercase">Next Crawl</div>
              <div class="text-sm font-bold text-zinc-200 mt-1 truncate" title={scheduler?.next_scan_time}>
                {scheduler?.next_scan_time ? new Date(scheduler.next_scan_time).toLocaleTimeString() : 'Auto'}
              </div>
            </div>
          </div>
        </div>

        <!-- Telegram Notification Status (if present) -->
        {#if status?.telegram_error}
          <div class="p-3 rounded-xl bg-rose-950/40 border border-rose-500/30 flex items-start gap-2.5 text-xs text-rose-300">
            <AlertCircle class="w-4 h-4 text-rose-400 flex-shrink-0 mt-0.5" />
            <div>
              <span class="font-bold">Telegram Notifier Alert:</span>
              <p class="mt-0.5 text-rose-300/90">{status.telegram_error}</p>
            </div>
          </div>
        {/if}

      </div>
    </div>
  </div>
{/if}
