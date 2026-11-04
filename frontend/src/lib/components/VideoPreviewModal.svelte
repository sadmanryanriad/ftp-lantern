<script lang="ts">
  import { X, AlertTriangle, Monitor, Smartphone, Download, ExternalLink } from '@lucide/svelte';
  import type { MediaFile } from '../types';
  import { downloadM3U, playOnAndroid, formatBytes } from '../api';

  let {
    file = null,
    titleName = '',
    onClose = () => {}
  }: {
    file: MediaFile | null;
    titleName: string;
    onClose: () => void;
  } = $props();

  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === 'Escape') onClose();
  }

  $effect(() => {
    if (file) {
      window.addEventListener('keydown', handleKeyDown);
      return () => window.removeEventListener('keydown', handleKeyDown);
    }
  });

  function handleWindowsPlay() {
    if (!file) return;
    downloadM3U(`/play/file/${file.id}.m3u`, file.id);
  }

  function handleAndroidPlay() {
    if (!file) return;
    const name = file.episode_code ? `${titleName} ${file.episode_code}` : file.name;
    playOnAndroid(file.id, file.file_url, name);
  }
</script>

{#if file}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-6 bg-zinc-950/90 backdrop-blur-md overflow-y-auto">
    <!-- Backdrop click -->
    <div
      role="button"
      tabindex="-1"
      onclick={onClose}
      onkeydown={(e) => { if (e.key === 'Enter') onClose(); }}
      class="fixed inset-0 cursor-default"
      aria-label="Close preview backdrop"
    ></div>

    <!-- Modal Box -->
    <div class="relative w-full max-w-4xl rounded-2xl bg-zinc-900 border border-zinc-800 shadow-2xl overflow-hidden z-10 animate-in fade-in zoom-in-95 duration-200">
      
      <!-- Video Player Frame -->
      <div class="relative aspect-video bg-black flex items-center justify-center">
        <!-- Close button on top-right -->
        <button
          type="button"
          onclick={onClose}
          class="absolute top-3 right-3 z-20 p-2 rounded-full bg-zinc-900/80 hover:bg-zinc-800 text-zinc-300 hover:text-white transition-colors"
          aria-label="Close preview"
        >
          <X class="w-5 h-5" />
        </button>

        <video
          src={file.file_url}
          controls
          autoplay
          class="w-full h-full"
        >
          <track kind="captions" />
          Your browser does not support the video tag.
        </video>
      </div>

      <!-- Warning Banner on HEVC / Browser Compatibility -->
      <div class="p-3.5 bg-amber-950/40 border-y border-amber-500/20 flex items-start gap-3 text-xs text-amber-200/90">
        <AlertTriangle class="w-4 h-4 text-amber-400 flex-shrink-0 mt-0.5" />
        <div class="flex-1">
          <span class="font-bold text-amber-300">Browser Compatibility Notice:</span>
          Most BDIX ISP files are encoded in <strong>HEVC / x265</strong> with <strong>AC3 / Dolby audio</strong>. If playback fails, stutters, or is silent in your web browser, launch via an external player below:
        </div>
      </div>

      <!-- Footer Info & External Player Launchers -->
      <div class="p-4 sm:p-5 flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-zinc-950/60">
        <div class="min-w-0">
          <h3 class="text-sm font-semibold text-zinc-100 truncate">
            {file.episode_code ? `${file.episode_code}: ` : ''}{file.name}
          </h3>
          <div class="flex items-center gap-2 mt-1 text-xs font-mono text-zinc-400">
            <span>{formatBytes(file.file_size_bytes)}</span>
            {#if file.resolution}
              <span>• {file.resolution}</span>
            {/if}
            {#if file.source}
              <span>• {file.source}</span>
            {/if}
          </div>
        </div>

        <div class="flex items-center gap-2 flex-shrink-0">
          <button
            type="button"
            onclick={handleWindowsPlay}
            class="inline-flex items-center gap-2 px-3 py-1.5 rounded-xl bg-zinc-800 hover:bg-zinc-700 text-zinc-200 text-xs font-medium border border-zinc-700 transition-colors"
          >
            <Monitor class="w-3.5 h-3.5 text-zinc-400" />
            <span>Launch VLC (Windows)</span>
          </button>

          <button
            type="button"
            onclick={handleAndroidPlay}
            class="inline-flex items-center gap-2 px-3 py-1.5 rounded-xl bg-zinc-800 hover:bg-zinc-700 text-zinc-200 text-xs font-medium border border-zinc-700 transition-colors"
          >
            <Smartphone class="w-3.5 h-3.5 text-zinc-400" />
            <span>Launch VLC / MX (Android)</span>
          </button>
        </div>
      </div>

    </div>
  </div>
{/if}
