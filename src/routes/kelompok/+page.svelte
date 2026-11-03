<script lang="ts">
	import { goto } from '$app/navigation';
	import { app } from '#lib/store.svelte.ts';
	import Icon from '#lib/ui/Icon.svelte';
	import PageHeader from '#lib/ui/PageHeader.svelte';
	import Progress from '#lib/ui/Progress.svelte';
	import Sheet from '#lib/ui/Sheet.svelte';

	const sudah = $derived(new Set(app.setoranHariIni().map((z) => z.santriId)));

	let buka = $state(false);
	let nama = $state('');
	let jadwal = $state('');
	let metodeId = $state(app.pengajar.metodeId);

	function buat() {
		if (!nama.trim()) return;
		const k = app.tambahKelompok({ nama: nama.trim(), jadwal: jadwal.trim(), metodeId });
		nama = jadwal = '';
		buka = false;
		app.tampilToast(`Kelompok ${k.nama} dibuat`);
		goto(`/kelompok/${k.id}`);
	}
</script>

<PageHeader judul="Kelompok" sub="{app.kelompokAktif.length} halaqah aktif">
	{#snippet aksi()}
		<button class="btn btn-primary min-h-11 px-4 text-sm" onclick={() => (buka = true)}>
			<Icon n="plus" size={18} />Baru
		</button>
	{/snippet}
</PageHeader>

<div class="space-y-3 px-4 pb-6">
	{#each app.kelompokAktif as k (k.id)}
		{@const ang = app.anggota(k.id)}
		{@const n = ang.filter((s) => sudah.has(s.id)).length}
		<article class="card p-0">
			<a href="/kelompok/{k.id}" class="block px-4 pt-4 pb-3">
				<div class="flex items-start justify-between gap-3">
					<div class="min-w-0">
						<h2 class="truncate text-lg font-bold">{k.nama}</h2>
						<p class="truncate text-sm text-ink-muted">{k.jadwal || 'Jadwal belum diisi'}</p>
					</div>
					<span class="chip shrink-0 bg-surface-2 text-ink-soft">{app.metodeById(k.metodeId)?.nama}</span>
				</div>
				<div class="mt-3 flex items-center justify-between text-sm">
					<span class="text-ink-soft"><b class="tabular">{ang.length}</b> santri</span>
					<span class="tabular text-ink-muted">Hari ini {n}/{ang.length} setor</span>
				</div>
				<div class="mt-1.5"><Progress persen={ang.length ? n / ang.length : 0} /></div>
			</a>
			<div class="grid grid-cols-2 border-t border-line">
				<a href="/kelompok/{k.id}" class="flex min-h-12 items-center justify-center gap-1.5 border-r border-line text-sm font-semibold text-ink-soft"
					>Rincian <Icon n="chevR" size={16} /></a
				>
				<a href="/halaqah?k={k.id}" class="flex min-h-12 items-center justify-center gap-1.5 text-sm font-bold text-brand"
					><Icon n="play" size={16} />Mulai Halaqah</a
				>
			</div>
		</article>
	{:else}
		<div class="card py-10 text-center">
			<p class="font-semibold">Belum ada kelompok.</p>
			<p class="mt-1 text-sm text-ink-muted">Kelompok adalah halaqah atau kelas yang Anda ajar.</p>
			<button class="btn btn-primary mt-4" onclick={() => (buka = true)}>Buat kelompok</button>
		</div>
	{/each}
</div>

<Sheet bind:open={buka} judul="Kelompok baru">
	<label class="label" for="kn">Nama kelompok</label>
	<input id="kn" class="field" bind:value={nama} placeholder="Contoh: Halaqah Ashar" autocomplete="off" />
	<label class="label mt-4" for="kj">Jadwal <span class="font-normal text-ink-muted">(opsional)</span></label>
	<input id="kj" class="field" bind:value={jadwal} placeholder="Sen, Rab, Jum · 16.00" autocomplete="off" />
	<span class="label mt-4">Metode tahsin</span>
	<div class="flex flex-wrap gap-2">
		{#each app.metode as m (m.id)}
			<button
				type="button"
				class="chip min-h-10 px-4 text-sm {metodeId === m.id ? 'bg-brand text-on-brand' : 'border border-line bg-surface text-ink-soft'}"
				onclick={() => (metodeId = m.id)}>{m.nama}</button
			>
		{/each}
	</div>
	<button class="btn btn-primary mt-6 min-h-14 w-full" disabled={!nama.trim()} onclick={buat}>Buat kelompok</button>
</Sheet>
