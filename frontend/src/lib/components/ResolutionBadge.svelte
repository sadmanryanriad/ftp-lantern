<script lang="ts">
  let { resolution = 'Unknown', size = 'sm' }: { resolution?: string; size?: 'xs' | 'sm' | 'md' } = $props();

  const resUpper = $derived((resolution || '').toUpperCase());

  const badgeStyle = $derived.by(() => {
    if (resUpper.includes('4K') || resUpper.includes('2160P')) {
      return 'bg-purple-900/60 text-purple-300 border-purple-500/40 ring-purple-500/20';
    }
    if (resUpper.includes('1080P') || resUpper.includes('FHD')) {
      return 'bg-emerald-900/60 text-emerald-300 border-emerald-500/40 ring-emerald-500/20';
    }
    if (resUpper.includes('720P') || resUpper.includes('HD')) {
      return 'bg-blue-900/60 text-blue-300 border-blue-500/40 ring-blue-500/20';
    }
    return 'bg-zinc-800/80 text-zinc-300 border-zinc-700 ring-zinc-700/20';
  });

  const sizeClasses = $derived.by(() => {
    switch (size) {
      case 'xs':
        return 'px-1.5 py-0.5 text-[10px]';
      case 'md':
        return 'px-2.5 py-1 text-xs';
      case 'sm':
      default:
        return 'px-2 py-0.5 text-[11px]';
    }
  });
</script>

<span class="inline-flex items-center font-mono font-semibold uppercase tracking-wider rounded border ring-1 {badgeStyle} {sizeClasses}">
  {resolution || 'SD'}
</span>
