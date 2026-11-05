<script lang="ts">
	import { app, sekarang } from '../store.svelte';
	import { PILIHAN_TARGET } from '../posisi';
	import Icon from './Icon.svelte';
	import NumberField from './NumberField.svelte';
	import Sheet from './Sheet.svelte';

	let {
		open = $bindable(false),
		judul,
		kelompokId,
		santriId,
		catatan
	}: { open?: boolean; judul: string; kelompokId?: string; santriId?: string; catatan: string } = $props();

	let pilihan = $state(PILIHAN_TARGET[0].id);
	let jumlah = $state<number | null>(null);
	let durasi = $state(90);

	const p = $derived(PILIHAN_TARGET.find((x) => x.id === pilihan) ?? PILIHAN_TARGET[0]);

	function buat() {
		if (!jumlah) return;
		const mulai = sekarang();
		app.tambahTarget({
			...(santriId ? { santriId } : { kelompokId }),
			jenis: p.jenis,
			nilaiTarget: jumlah,
			satuan: p.satuan,
			mulai: mulai.toISOString(),
			tenggat: new Date(mulai.getTime() + durasi * 86400000).toISOString()
		});
		const baru = app.target.at(-1)!;
		app.tampilToast(`Target ${jumlah} ${p.satuan} dibuat`, { label: 'Urungkan', fn: () => app.hapusTarget(baru.id) });
		jumlah = null;
		open = false;
	}
</script>

<Sheet bind:open {judul}>
	<span class="label">Target</span>
	<div class="mb-4 grid grid-cols-3 gap-1.5 rounded-2xl bg-surface-2 p-1.5" role="radiogroup" aria-label="Jenis target">
		{#each PILIHAN_TARGET as x (x.id)}
			<button
				type="button"
				role="radio"
				aria-checked={pilihan === x.id}
				class="min-h-11 rounded-xl text-sm font-bold {pilihan === x.id ? 'bg-surface text-brand shadow-sm' : 'text-ink-soft'}"
				onclick={() => {
					pilihan = x.id;
					if (jumlah && jumlah > x.max) jumlah = x.max;
				}}>{x.nama}</button
			>
		{/each}
	</div>

	<NumberField id="target-jumlah" label={p.tanya} bind:value={jumlah} max={p.max} placeholder="Isi angka" />

	<span class="label mt-4">Tenggat</span>
	<div class="mb-5 grid grid-cols-3 gap-1.5 rounded-2xl bg-surface-2 p-1.5">
		{#each [[30, '1 bulan'], [90, '3 bulan'], [180, '6 bulan']] as [d, l] (d)}
			<button
				type="button"
				class="min-h-11 rounded-xl text-sm font-bold {durasi === d ? 'bg-surface text-brand shadow-sm' : 'text-ink-soft'}"
				onclick={() => (durasi = d as number)}>{l}</button
			>
		{/each}
	</div>

	<button type="button" class="btn btn-primary min-h-14 w-full" disabled={!jumlah} onclick={buat}
		><Icon n="target" size={20} />Buat target</button
	>
	<p class="mt-3 text-xs text-ink-muted">{catatan}</p>
</Sheet>
