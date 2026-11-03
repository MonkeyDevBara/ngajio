<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';

	let {
		judul,
		sub,
		kembali,
		aksi
	}: { judul: string; sub?: string; kembali?: string; aksi?: Snippet } = $props();
</script>

<header class="no-print safe-t sticky top-0 z-30 bg-paper/90 backdrop-blur">
	<div class="flex min-h-16 items-center gap-1 px-3">
		{#if kembali}
			<a
				href={kembali}
				class="flex size-11 shrink-0 items-center justify-center rounded-full text-ink-soft"
				aria-label="Kembali"><Icon n="chevL" size={24} /></a
			>
		{/if}
		<div class="min-w-0 flex-1 px-2 {kembali ? '' : 'pl-2'}">
			<h1 class="truncate text-xl leading-tight font-bold">{judul}</h1>
			{#if sub}<p class="truncate text-sm text-ink-muted">{sub}</p>{/if}
		</div>
		{#if aksi}<div class="flex shrink-0 items-center gap-1">{@render aksi()}</div>{/if}
	</div>
</header>
