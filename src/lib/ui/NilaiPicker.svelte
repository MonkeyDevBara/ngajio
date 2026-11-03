<script lang="ts">
	import { app } from '../store.svelte';
	import { nilaiLabel } from '../format';

	let { value = $bindable(0) }: { value?: number } = $props();

	const skala = $derived(app.pengajar.skala);
	// Tampil dari nilai tertinggi ke terendah. Disimpan ternormalisasi 1 sampai 5.
	const daftar = [5, 4, 3, 2, 1];
	const warna: Record<number, string> = {
		5: 'var(--ok)',
		4: 'var(--brand)',
		3: 'var(--gold)',
		2: 'var(--warn)',
		1: 'var(--danger)'
	};
</script>

<div class="grid grid-cols-5 gap-2" role="radiogroup" aria-label="Nilai setoran">
	{#each daftar as n (n)}
		{@const aktif = value === n}
		<button
			type="button"
			role="radio"
			aria-checked={aktif}
			onclick={() => (value = n)}
			class="flex min-h-[72px] flex-col items-center justify-center gap-0.5 rounded-2xl border-2 px-0.5 transition active:scale-95"
			style="border-color:{aktif ? warna[n] : 'var(--line)'};background:{aktif
				? `color-mix(in srgb, ${warna[n]} 16%, var(--surface))`
				: 'var(--surface)'};color:{aktif ? warna[n] : 'var(--ink-soft)'}"
		>
			{#if skala === 'huruf'}
				<span class="text-[28px] leading-none font-extrabold">{nilaiLabel(n, 'huruf')}</span>
			{:else if skala === 'bintang'}
				<span class="text-[22px] leading-none font-extrabold">{n}</span>
				<span class="text-[13px] leading-none">★</span>
			{:else if skala === 'angka'}
				<span class="text-[24px] leading-none font-extrabold">{n * 20}</span>
			{:else}
				<span class="text-[11px] leading-tight font-bold">{nilaiLabel(n, 'predikat')}</span>
			{/if}
		</button>
	{/each}
</div>
