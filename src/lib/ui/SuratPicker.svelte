<script lang="ts">
	import { SURAHS, searchSurah, type Surah } from '../quran';
	import Sheet from './Sheet.svelte';
	import Icon from './Icon.svelte';

	let {
		open = $bindable(false),
		dipilih,
		batasi,
		keterangan,
		pilih
	}: {
		open?: boolean;
		dipilih?: number;
		/** Bila diisi, hanya surat ini yang bisa dipilih (murojaah). */
		batasi?: number[];
		keterangan?: (n: number) => string | undefined;
		pilih: (s: Surah) => void;
	} = $props();

	let q = $state('');
	const daftar = $derived.by(() => {
		const dasar = batasi ? SURAHS.filter((s) => batasi.includes(s.n)) : SURAHS;
		if (!q.trim()) return dasar;
		const cocok = new Set(searchSurah(q, 40).map((s) => s.n));
		return dasar.filter((s) => cocok.has(s.n));
	});
</script>

<Sheet bind:open judul="Pilih surat" tinggi="penuh">
	<div class="sticky top-0 z-10 -mx-5 bg-paper px-5 pb-3">
		<div class="relative">
			<Icon n="search" size={18} class="absolute top-1/2 left-3.5 -translate-y-1/2 text-ink-muted" />
			<input
				class="field pl-10"
				placeholder="Cari: Ikhlas, Al-Ikhlas, Ihlas…"
				bind:value={q}
				autocomplete="off"
				inputmode="search"
			/>
		</div>
	</div>
	<ul class="space-y-1.5">
		{#each daftar as s (s.n)}
			<li>
				<button
					type="button"
					class="flex min-h-14 w-full items-center gap-3 rounded-xl border px-3 text-left {dipilih === s.n
						? 'border-brand bg-brand-tint'
						: 'border-line bg-surface'}"
					onclick={() => {
						pilih(s);
						open = false;
						q = '';
					}}
				>
					<span class="tabular w-8 text-center text-sm font-bold text-ink-muted">{s.n}</span>
					<span class="min-w-0 flex-1">
						<span class="block truncate font-semibold">{s.latin}</span>
						<span class="block text-xs text-ink-muted"
							>{s.ayat} ayat · Juz {s.juz}{#if keterangan?.(s.n)} · {keterangan(s.n)}{/if}</span
						>
					</span>
					<span class="font-arabic text-xl text-ink-soft" dir="rtl" lang="ar">{s.arab}</span>
				</button>
			</li>
		{:else}
			<li class="py-10 text-center text-ink-muted">Surat tidak ditemukan.</li>
		{/each}
	</ul>
</Sheet>
