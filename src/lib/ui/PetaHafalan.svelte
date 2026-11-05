<script lang="ts">
	import { app } from '../store.svelte';
	import { JUZ30_REVERSE, surah } from '../quran';
	import { relatif } from '../format';
	import Sheet from './Sheet.svelte';
	import type { StatusSurat } from '../types';

	let { santriId }: { santriId: string } = $props();

	const status = $derived(app.statusSurat(santriId));
	// Dari An-Naba' (78) ke An-Nas (114), urutan mushaf, 6 kolom.
	const urut = [...JUZ30_REVERSE].reverse();
	const jumlahHafal = $derived(urut.filter((s) => status.get(s.n) === 'hafal').length);

	let buka = $state(false);
	let pilih = $state<number | null>(null);
	const riwayat = $derived.by(() => {
		const n = pilih;
		if (n == null) return [];
		return app
			.setoranSantri(santriId)
			.filter(
				(z) =>
					z.jenis !== 'tahsin' && z.suratAwal != null && n >= z.suratAwal && n <= (z.suratAkhir ?? z.suratAwal)
			);
	});

	const gaya: Record<StatusSurat, string> = {
		hafal: 'background:var(--brand);color:var(--on-brand);border-color:var(--brand)',
		sedang: 'background:var(--gold-tint);color:var(--warn);border-color:var(--gold)',
		kosong: 'background:var(--surface);color:var(--ink-muted);border-color:var(--line)'
	};
</script>

<div class="card">
	<div class="mb-3 flex items-baseline justify-between">
		<h3 class="font-bold">Peta hafalan Juz 30</h3>
		<span class="tabular text-sm text-ink-muted">{jumlahHafal} dari {urut.length} surat</span>
	</div>
	<div class="grid grid-cols-6 gap-1.5">
		{#each urut as s (s.n)}
			{@const st = status.get(s.n) ?? 'kosong'}
			<button
				type="button"
				class="flex aspect-square flex-col items-center justify-center rounded-lg border text-[11px] font-bold transition active:scale-90"
				style={gaya[st]}
				aria-label="{s.latin}, {st === 'hafal' ? 'sudah hafal' : st === 'sedang' ? 'sedang dihafal' : 'belum dihafal'}"
				onclick={() => {
					pilih = s.n;
					buka = true;
				}}
			>
				<span class="tabular">{s.n}</span>
			</button>
		{/each}
	</div>
	<div class="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-ink-soft">
		<span class="flex items-center gap-1.5"
			><i class="size-3 rounded bg-brand"></i>Hafal</span
		>
		<span class="flex items-center gap-1.5"
			><i class="size-3 rounded border border-gold bg-gold-tint"></i>Sedang dihafal</span
		>
		<span class="flex items-center gap-1.5"
			><i class="size-3 rounded border border-line bg-surface"></i>Belum</span
		>
	</div>
</div>

<Sheet bind:open={buka} judul={pilih ? surah(pilih).latin : ''}>
	{#if pilih}
		{@const info = surah(pilih)}
		<div class="mb-4 flex items-center justify-between">
			<div class="text-sm text-ink-muted">{info.ayat} ayat · Juz {info.juz}</div>
			<div class="font-arabic text-3xl" dir="rtl" lang="ar">{info.arab}</div>
		</div>
		{#if riwayat.length}
			<ul class="divide-y divide-line rounded-2xl border border-line bg-surface">
				{#each riwayat as z (z.id)}
					<li class="flex items-center justify-between gap-3 px-4 py-3">
						<div>
							<div class="font-semibold">
								{z.jenis === 'ziyadah' ? 'Hafalan baru' : 'Murajaah'} · ayat {z.ayatAwal}–{z.ayatAkhir}
							</div>
							<div class="text-xs text-ink-muted">{relatif(z.dicatatPada)}</div>
						</div>
						<span class="chip bg-surface-2 text-ink-soft">Nilai {z.nilai}/5</span>
					</li>
				{/each}
			</ul>
		{:else}
			<p class="py-6 text-center text-ink-muted">
				{status.get(pilih) === 'hafal'
					? 'Sudah hafal sebelum riwayat di Ngajio dimulai.'
					: 'Belum ada setoran untuk surat ini.'}
			</p>
		{/if}
	{/if}
</Sheet>
