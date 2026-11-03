<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';

	let {
		open = $bindable(false),
		judul,
		children,
		tinggi = 'auto'
	}: { open?: boolean; judul: string; children: Snippet; tinggi?: 'auto' | 'penuh' } = $props();

	$effect(() => {
		if (!open) return;
		const sebelum = document.body.style.overflow;
		document.body.style.overflow = 'hidden';
		return () => (document.body.style.overflow = sebelum);
	});
</script>

<svelte:window onkeydown={(e) => open && e.key === 'Escape' && (open = false)} />

{#if open}
	<div class="no-print fixed inset-0 z-50 flex items-end justify-center">
		<button
			type="button"
			class="absolute inset-0 bg-black/45"
			aria-label="Tutup"
			onclick={() => (open = false)}
		></button>
		<div
			role="dialog"
			aria-modal="true"
			aria-label={judul}
			class="rise safe-b relative flex w-full max-w-[480px] flex-col rounded-t-3xl bg-paper shadow-2xl"
			style:max-height={tinggi === 'penuh' ? '92dvh' : '88dvh'}
			style:height={tinggi === 'penuh' ? '92dvh' : undefined}
		>
			<div class="flex items-center justify-between px-5 pt-4 pb-2">
				<h2 class="text-lg font-bold">{judul}</h2>
				<button
					type="button"
					class="-mr-2 flex size-11 items-center justify-center rounded-full text-ink-soft"
					onclick={() => (open = false)}
					aria-label="Tutup"><Icon n="x" /></button
				>
			</div>
			<div class="min-h-0 flex-1 overflow-y-auto px-5 pb-5">
				{@render children()}
			</div>
		</div>
	</div>
{/if}
