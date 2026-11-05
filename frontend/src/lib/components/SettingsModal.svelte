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
    Bell,
    UploadCloud,
    DownloadCloud,
    Radio
  } from '@lucide/svelte';
  import type { SystemStatus, SchedulerStatus } from '../types';
  import {
    triggerCrawl,
    setTurboMode,
    setSchedulerFrequency,
    triggerDbBackup,
    importDbBackup,
    getNotificationMode,
    setNotificationMode
  } from '../api';

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
  let backupLoading = $state(false);
  let backupFeedback = $state<string | null>(null);
  let importLoading = $state(false);
  let importFeedback = $state<string | null>(null);
  let notifyMode = $state<string>('following_only');
  let fileInputRef: HTMLInputElement | null = $state(null);

  // Load notification preference on open
  $effect(() => {
    if (isOpen) {
      getNotificationMode().then((m) => {
        notifyMode = m;
      });
    }
  });

  const frequencyCards = [
    {
      id: '15m',
      title: '15-Min Pulse',
      subtitle: 'Continuous incremental mtime checks',
      recommended: true,
      icon: Zap
    },
    {
      id: '1h',
      title: 'Hourly Check',
      subtitle: 'Scans once every 60 minutes',
      recommended: false,
      icon: Clock
    },
    {
      id: 'daily',
      title: 'Daily at 7:00 AM',
      subtitle: 'Single morning scan + 8:01 AM digest',
      recommended: false,
      icon: Flame
    },
    {
      id: 'manual',
      title: 'Manual Only',
      subtitle: 'Zero automated scans. Standby only',
      recommended: false,
      icon: Gauge
    }
  ];

  async function handleSelectFrequency(freq: string) {
    try {
      await setSchedulerFrequency(freq);
      onRefresh();
    } catch (err) {
      console.error('Failed to set crawl frequency:', err);
    }
  }

  async function handleNotificationModeChange(mode: string) {
    notifyMode = mode;
    try {
      await setNotificationMode(mode);
    } catch (err) {
      console.error('Failed to set notification mode:', err);
    }
  }

  async function handleBackup() {
    backupLoading = true;
    backupFeedback = null;
    try {
      const res = await triggerDbBackup();
      backupFeedback = `Snapshot created: ${res.backup_path}`;
    } catch (err: any) {
      backupFeedback = `Backup error: ${err.message}`;
    } finally {
      backupLoading = false;
    }
  }

  async function handleImportFile(e: Event) {
    const input = e.target as HTMLInputElement;
    if (!input.files || input.files.length === 0) return;
    const file = input.files[0];

    importLoading = true;
    importFeedback = null;
    try {
      const res = await importDbBackup(file);
      importFeedback = `Restored ${res.titles_count} titles from ${res.filename}!`;
      onRefresh();
    } catch (err: any) {
      importFeedback = `Import failed: ${err.message}`;
    } finally {
      importLoading = false;
      if (input) input.value = '';
    }
  }

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
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-4 bg-black/80 backdrop-blur-md animate-fade-in"
    role="dialog"
    aria-modal="true"
    aria-labelledby="settings-title"
    tabindex="-1"
  >
    <!-- Backdrop dismiss -->
    <button
      type="button"
      onclick={onClose}
      class="absolute inset-0 w-full h-full cursor-default"
      aria-label="Close modal"
    ></button>

    <!-- Modal Content -->
    <div
      class="relative w-full max-w-2xl max-h-[90vh] flex flex-col rounded-2xl bg-zinc-900 border border-zinc-800 shadow-2xl overflow-hidden z-10"
    >
      <!-- Header -->
      <div class="px-5 py-4 border-b border-zinc-800 flex items-center justify-between bg-zinc-950/70">
        <div class="flex items-center gap-2.5">
          <div class="p-2 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-400">
            <Flame class="w-5 h-5" />
          </div>
          <div>
            <h2 id="settings-title" class="text-base sm:text-lg font-bold text-zinc-100 flex items-center gap-2">
              <span>Settings & System Telemetry</span>
              {#if status?.is_crawling}
                <span class="px-2 py-0.5 rounded-full text-[10px] font-mono font-bold bg-amber-500/20 text-amber-300 border border-amber-500/30 animate-pulse">
                  SCANNING
                </span>
              {/if}
            </h2>
            <p class="text-xs text-zinc-400">
              Live homelab resource usage, crawl automation rhythm & storage persistence
            </p>
          </div>
        </div>
        <button
          type="button"
          onclick={onClose}
          class="p-2 rounded-xl bg-zinc-850 hover:bg-zinc-800 text-zinc-400 hover:text-zinc-100 transition-colors"
          aria-label="Close"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- Scrollable Body -->
      <div class="flex-1 overflow-y-auto p-4 sm:p-6 space-y-5">

        <!-- Live Index Breakdown -->
        <div>
          <div class="flex items-center justify-between mb-2">
            <h3 class="text-xs font-mono font-bold uppercase tracking-wider text-zinc-400">
              Indexed Library Breakdown
            </h3>
            <span class="text-[11px] font-mono text-zinc-500">
              Uptime: {status?.uptime || 'Active'}
            </span>
          </div>

          <div class="grid grid-cols-2 sm:grid-cols-4 gap-2 text-xs">
            <!-- Movies -->
            <div class="p-3 rounded-xl bg-zinc-950/80 border border-zinc-800">
              <div class="flex items-center gap-1.5 text-zinc-400 mb-1">
                <Film class="w-3.5 h-3.5 text-amber-400" />
                <span>Movies</span>
              </div>
              <div class="text-lg font-bold text-zinc-100">
                {status?.movies_count?.toLocaleString() ?? 0}
              </div>
            </div>

            <!-- Series -->
            <div class="p-3 rounded-xl bg-zinc-950/80 border border-zinc-800">
              <div class="flex items-center gap-1.5 text-zinc-400 mb-1">
                <Tv class="w-3.5 h-3.5 text-sky-400" />
                <span>TV Series</span>
              </div>
              <div class="text-lg font-bold text-zinc-100">
                {status?.series_count?.toLocaleString() ?? 0}
              </div>
            </div>

            <!-- Others -->
            <div class="p-3 rounded-xl bg-zinc-950/80 border border-zinc-800">
              <div class="flex items-center gap-1.5 text-zinc-400 mb-1">
                <FolderKanban class="w-3.5 h-3.5 text-emerald-400" />
                <span>Others & Docs</span>
              </div>
              <div class="text-lg font-bold text-zinc-100">
                {status?.others_count?.toLocaleString() ?? 0}
              </div>
            </div>

            <!-- Video Files -->
            <div class="p-3 rounded-xl bg-zinc-950/80 border border-zinc-800">
              <div class="flex items-center gap-1.5 text-zinc-400 mb-1">
                <HardDrive class="w-3.5 h-3.5 text-violet-400" />
                <span>Media Files</span>
              </div>
              <div class="text-lg font-bold text-amber-300">
                {status?.files_count?.toLocaleString() ?? 0}
              </div>
            </div>
          </div>
        </div>

        <!-- Real Disk Storage Breakdown -->
        <div class="p-4 rounded-xl bg-zinc-950/70 border border-zinc-800 text-xs">
          <div class="flex items-center justify-between mb-2">
            <div class="flex items-center gap-2">
              <Database class="w-4 h-4 text-emerald-400" />
              <span class="font-bold text-zinc-200">Disk Storage & Hardware Usage</span>
            </div>
            <span class="font-mono text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded border border-emerald-500/20">
              DISK WAL MODE
            </span>
          </div>

          <div class="grid grid-cols-2 sm:grid-cols-4 gap-2 font-mono text-[11px] mb-3">
            <div class="p-2.5 rounded-lg bg-zinc-900 border border-zinc-800">
              <span class="text-zinc-500">Database on Disk</span>
              <div class="text-sm font-bold text-zinc-200 mt-0.5">{status?.db_size_human || '0 B'}</div>
            </div>
            <div class="p-2.5 rounded-lg bg-zinc-900 border border-zinc-800">
              <span class="text-zinc-500">Thumbnails</span>
              <div class="text-sm font-bold text-zinc-200 mt-0.5">{status?.thumbs_size_human || '0 B'}</div>
            </div>
            <div class="p-2.5 rounded-lg bg-zinc-900 border border-zinc-800">
              <span class="text-zinc-500">Total Volume Size</span>
              <div class="text-sm font-bold text-amber-300 mt-0.5">{status?.total_volume_human || status?.db_size_human || '0 B'}</div>
            </div>
            <div class="p-2.5 rounded-lg bg-zinc-900 border border-zinc-800">
              <span class="text-zinc-500">RAM Allocation</span>
              <div class="text-sm font-bold text-zinc-200 mt-0.5">{status?.ram_alloc_mb || '< 25 MB'}</div>
            </div>
          </div>

          <!-- Backup & Disaster Recovery Actions -->
          <div class="pt-3 border-t border-zinc-800/80 flex flex-col sm:flex-row sm:items-center justify-between gap-2.5">
            <div>
              <span class="font-semibold text-zinc-300 block">Doomsday Backup & Restore</span>
              <span class="text-[11px] text-zinc-500">Atomic non-locking SQLite snapshot (`VACUUM INTO`) or manual index restore</span>
            </div>

            <div class="flex items-center gap-2">
              <!-- Export Snapshot -->
              <button
                type="button"
                onclick={handleBackup}
                disabled={backupLoading}
                class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-zinc-850 hover:bg-zinc-800 text-zinc-200 text-xs font-semibold border border-zinc-700 transition-colors"
              >
                <DownloadCloud class="w-3.5 h-3.5 text-zinc-400" />
                <span>{backupLoading ? 'Backing up...' : 'Snapshot Backup'}</span>
              </button>

              <!-- Import Backup Button -->
              <button
                type="button"
                onclick={() => fileInputRef?.click()}
                disabled={importLoading}
                class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-amber-500/10 hover:bg-amber-500/20 text-amber-300 text-xs font-semibold border border-amber-500/30 transition-colors"
              >
                <UploadCloud class="w-3.5 h-3.5 text-amber-400" />
                <span>{importLoading ? 'Importing...' : 'Import Backup'}</span>
              </button>
              <input
                bind:this={fileInputRef}
                type="file"
                accept=".db,.sqlite,.sqlite3"
                class="hidden"
                onchange={handleImportFile}
              />
            </div>
          </div>

          {#if backupFeedback}
            <div class="mt-2 text-[10px] font-mono text-emerald-400">{backupFeedback}</div>
          {/if}
          {#if importFeedback}
            <div class="mt-2 text-[10px] font-mono text-amber-300">{importFeedback}</div>
          {/if}
        </div>

        <!-- Crawl Rhythm / Automation Mode Selector (UX Cards) -->
        <div class="p-4 rounded-xl bg-zinc-950/70 border border-zinc-800">
          <div class="flex items-center justify-between mb-3">
            <div class="flex items-center gap-2">
              <Clock class="w-4 h-4 text-amber-400" />
              <h3 class="text-sm font-bold text-zinc-100">Automatic Crawl Rhythm</h3>
            </div>
            <span class="text-[11px] font-mono text-zinc-400">
              Active: <strong class="text-amber-400">{scheduler?.frequency || '15m'}</strong>
            </span>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
            {#each frequencyCards as card}
              {@const Icon = card.icon}
              {@const isSelected = (scheduler?.frequency || '15m') === card.id}
              <button
                type="button"
                onclick={() => handleSelectFrequency(card.id)}
                class="p-3 rounded-xl border text-left transition-all relative {isSelected
                  ? 'bg-amber-500/10 border-amber-500/50 shadow-md shadow-amber-500/10 ring-1 ring-amber-500/30'
                  : 'bg-zinc-900/60 border-zinc-800 hover:border-zinc-700 hover:bg-zinc-900'}"
              >
                <div class="flex items-center justify-between mb-1">
                  <div class="flex items-center gap-2">
                    <Icon class="w-4 h-4 {isSelected ? 'text-amber-400' : 'text-zinc-400'}" />
                    <span class="font-bold text-xs {isSelected ? 'text-amber-300' : 'text-zinc-200'}">
                      {card.title}
                    </span>
                  </div>
                  {#if card.recommended}
                    <span class="px-1.5 py-0.5 rounded text-[9px] font-bold font-mono bg-emerald-500/20 text-emerald-400 border border-emerald-500/30">
                      RECOMMENDED
                    </span>
                  {/if}
                </div>
                <p class="text-[11px] text-zinc-400 leading-tight">
                  {card.subtitle}
                </p>
              </button>
            {/each}
          </div>
        </div>

        <!-- Telegram Notification Preference (Radio Group) -->
        <div class="p-4 rounded-xl bg-zinc-950/70 border border-zinc-800 text-xs">
          <div class="flex items-center gap-2 mb-2">
            <Bell class="w-4 h-4 text-amber-400" />
            <h3 class="text-sm font-bold text-zinc-100">Telegram Notification Filter</h3>
          </div>
          <p class="text-[11px] text-zinc-400 mb-3">
            Choose what sends instant push alerts to your Telegram chat:
          </p>

          <div class="space-y-2">
            <label class="flex items-center gap-2.5 p-2.5 rounded-lg bg-zinc-900/60 border border-zinc-800 cursor-pointer hover:bg-zinc-900 transition-colors">
              <input
                type="radio"
                name="notify_mode"
                value="following_only"
                checked={notifyMode === 'following_only'}
                onchange={() => handleNotificationModeChange('following_only')}
                class="text-amber-500 focus:ring-amber-500/30"
              />
              <div>
                <span class="font-semibold text-zinc-200 block text-xs">Starred / Followed Shows Only (Recommended)</span>
                <span class="text-[10px] text-zinc-500">Only receive instant alerts when new episodes land for series you clicked "Follow" on.</span>
              </div>
            </label>

            <label class="flex items-center gap-2.5 p-2.5 rounded-lg bg-zinc-900/60 border border-zinc-800 cursor-pointer hover:bg-zinc-900 transition-colors">
              <input
                type="radio"
                name="notify_mode"
                value="all"
                checked={notifyMode === 'all'}
                onchange={() => handleNotificationModeChange('all')}
                class="text-amber-500 focus:ring-amber-500/30"
              />
              <div>
                <span class="font-semibold text-zinc-200 block text-xs">All New Movies & Series</span>
                <span class="text-[10px] text-zinc-500">Receive an instant Telegram notification whenever any new movie or show is added to the server.</span>
              </div>
            </label>

            <label class="flex items-center gap-2.5 p-2.5 rounded-lg bg-zinc-900/60 border border-zinc-800 cursor-pointer hover:bg-zinc-900 transition-colors">
              <input
                type="radio"
                name="notify_mode"
                value="muted"
                checked={notifyMode === 'muted'}
                onchange={() => handleNotificationModeChange('muted')}
                class="text-amber-500 focus:ring-amber-500/30"
              />
              <div>
                <span class="font-semibold text-zinc-200 block text-xs">Mute Push Alerts (Daily 8:01 AM Digest Only)</span>
                <span class="text-[10px] text-zinc-500">Silence instant alerts entirely; only receive the consolidated morning report at 08:01 AM.</span>
              </div>
            </label>
          </div>
        </div>

        <!-- 15-Minute Turbo Crawl Mode -->
        <div class="p-4 rounded-xl bg-zinc-950/80 border border-amber-500/30 glow-amber">
          <div class="flex items-center justify-between mb-2">
            <div class="flex items-center gap-2">
              <Zap class="w-4 h-4 text-amber-400 fill-amber-400/20" />
              <h3 class="text-sm font-bold text-zinc-100">15-Minute Turbo Crawl Mode</h3>
            </div>
            {#if scheduler?.turbo_mode}
              <span class="px-2 py-0.5 rounded-full text-[10px] font-mono font-bold bg-amber-500/20 text-amber-300 border border-amber-500/40">
                ACTIVE
              </span>
            {:else}
              <span class="px-2 py-0.5 rounded-full text-[10px] font-mono bg-zinc-800 text-zinc-400">
                STANDBY
              </span>
            {/if}
          </div>

          <p class="text-xs text-zinc-400 mb-3 leading-relaxed">
            When you're waiting for new releases, activate Turbo to force a scan <strong>every 15 minutes</strong> for a set duration:
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

        <!-- Manual Quick Scan Trigger -->
        <div class="p-4 rounded-xl bg-zinc-950/60 border border-zinc-800 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div>
            <div class="flex items-center gap-2">
              <RefreshCw class="w-4 h-4 text-zinc-300" />
              <h3 class="text-sm font-semibold text-zinc-200">Manual Quick Scan</h3>
            </div>
            <p class="text-xs text-zinc-400 mt-1">
              Checks directory mtimes and indexes only newly added files without re-indexing unchanged folders.
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
