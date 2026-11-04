<script lang="ts">
  import { Zap, Settings, Activity, Flame } from '@lucide/svelte';
  import type { SystemStatus, SchedulerStatus } from '../types';

  let {
    status = null,
    scheduler = null,
    onOpenSettings = () => {}
  }: {
    status: SystemStatus | null;
    scheduler: SchedulerStatus | null;
    onOpenSettings: () => void;
  } = $props();

  const isConnected = $derived(status?.bdix_connected ?? false);
  const isTurbo = $derived(scheduler?.turbo_mode ?? false);
</script>

<header class="sticky top-0 z-40 w-full border-b border-zinc-800/80 bg-zinc-950/90 backdrop-blur-md">
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between gap-4">
    <!-- Brand / Logo -->
    <div class="flex items-center gap-3">
      <div class="relative flex items-center justify-center w-10 h-10 rounded-xl bg-gradient-to-br from-amber-500/20 via-orange-500/10 to-transparent border border-amber-500/30 glow-amber">
        <Flame class="w-5 h-5 text-amber-400" />
        <span class="absolute -top-0.5 -right-0.5 flex h-2 w-2">
          <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75"></span>
          <span class="relative inline-flex rounded-full h-2 w-2 bg-amber-500"></span>
        </span>
      </div>
      <div>
        <div class="flex items-center gap-2">
          <span class="text-lg font-extrabold tracking-tight bg-gradient-to-r from-zinc-100 via-zinc-200 to-amber-200 bg-clip-text text-transparent">
            FTP-Lantern
          </span>
          <span class="px-1.5 py-0.2 rounded text-[10px] font-mono font-semibold tracking-wider bg-amber-500/10 text-amber-400 border border-amber-500/20">
            BDIX
          </span>
        </div>
        <p class="text-[11px] text-zinc-400 hidden sm:block">Direct High-Speed Media Gateway</p>
      </div>
    </div>

    <!-- Right Actions / Status Badges -->
    <div class="flex items-center gap-2 sm:gap-3">
      <!-- BDIX Health Badge -->
      <div
        class="inline-flex items-center gap-2 px-2.5 py-1 rounded-full text-xs font-medium border transition-colors {isConnected
          ? 'bg-emerald-950/50 text-emerald-300 border-emerald-500/30'
          : 'bg-rose-950/50 text-rose-300 border-rose-500/30'}"
        title={status?.isp_gateway_ip ? `Gateway: ${status.isp_gateway_ip}` : 'ISP Gateway Reachability'}
      >
        <span class="relative flex h-2 w-2">
          {#if isConnected}
            <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
            <span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
          {:else}
            <span class="relative inline-flex rounded-full h-2 w-2 bg-rose-500"></span>
          {/if}
        </span>
        <span class="hidden sm:inline font-mono text-[11px]">
          {isConnected ? 'BDIX Connected' : 'Offline'}
        </span>
      </div>

      <!-- Turbo Scan Status Chip -->
      <button
        type="button"
        onclick={onOpenSettings}
        class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium border transition-all hover:scale-105 active:scale-95 {isTurbo
          ? 'bg-amber-500/20 text-amber-300 border-amber-500/40 glow-amber'
          : 'bg-zinc-900/80 text-zinc-400 border-zinc-800 hover:text-zinc-200 hover:border-zinc-700'}"
      >
        <Zap class="w-3.5 h-3.5 {isTurbo ? 'text-amber-400 fill-amber-400/30 animate-pulse' : 'text-zinc-400'}" />
        <span class="font-mono text-[11px]">
          {isTurbo ? 'Turbo: 15m' : 'Scan: 2h'}
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
</header>
