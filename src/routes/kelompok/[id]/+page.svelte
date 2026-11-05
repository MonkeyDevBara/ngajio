<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { app, sekarang } from '#lib/store.svelte.ts';
	import { nilaiLabel, tanggal } from '#lib/format.ts';
	import { NAMA_TARGET } from '#lib/posisi.ts';
	import type { JenisTarget } from '#lib/types.ts';
	import Avatar from '#lib/ui/Avatar.svelte';
	import Icon from '#lib/ui/Icon.svelte';
	import PageHeader from '#lib/ui/PageHeader.svelte';
	import Progress from '#lib/ui/Progress.svelte';
	import Sheet from '#lib/ui/Sheet.svelte';
	import TargetSheet from '#lib/ui/TargetSheet.svelte';

	const id = $derived(page.params.id ?? '');
	const k = $derived(app.kelompokById(id));
	const ang = $derived(app.urutanHalaqah(id));
	const sudah = $derived(new Set(app.setoranHariIni().map((z) => z.santriId)));

	const rekap = $derived.by(() => {
		const sampai = sekarang();
		const dari = new Date(sampai.getTime() - 7 * 86400000);
		return app.rekap({ kelompokId: id, dari, sampai });
	});
	const tertinggal = $derived(app.tertinggal(id));
	const targets = $derived(app.target.filter((t) => t.kelompokId === id));

	function ringkas(tid: string) {
		const t = app.target.find((x) => x.id === tid)!;
		const ps = ang.map((s) => app.progressTarget(t, s.id));
		const capai = ps.filter((p) => p.persen >= 1).length;
		const rata = ps.length ? ps.reduce((a, p) => a + p.persen, 0) / ps.length : 0;
		return { capai, rata, total: ps.length };
	}

	let bukaTarget = $state(false);
	/* --- Ubah kelompok --- */
	let bukaEdit = $state(false);
	let nama = $state('');
	let jadwal = $state('');
	let galat = $state('');
	function mulaiEdit() {
		nama = k?.nama ?? '';
		jadwal = k?.jadwal ?? '';
		galat = '';
		bukaEdit = true;
	}
	function simpanEdit() {
		if (!nama.trim()) return;
		app.ubahKelompok(id, { nama: nama.trim(), jadwal: jadwal.trim() });
		bukaEdit = false;
	}
	function arsipkan() {
		if (ang.length) {
			galat = 'Pindahkan atau arsipkan semua santri di kelompok ini dulu.';
			return;
		}
		app.ubahKelompok(id, { arsip: true });
		app.tampilToast('Kelompok diarsipkan');
		goto('/kelompok', { replace: true });
	}
</script>

{#if !k}
	<PageHeader judul="Kelompok" kembali="/kelompok" />
	<div class="px-4"><div class="card py-10 text-center text-ink-muted">Kelompok tidak ditemukan.</div></div>
{:else}
	<PageHeader judul={k.nama} sub={k.jadwal || app.metodeById(k.metodeId)?.nama} kembali="/kelompok">
		{#snippet aksi()}
			<button class="flex size-11 items-center justify-center rounded-full text-ink-soft" aria-label="Ubah kelompok" onclick={mulaiEdit}
				><Icon n="pencil" size={20} /></button
			>
		{/snippet}
	</PageHeader>

	<div class="space-y-4 px-4 pb-8">
		<a href="/halaqah?k={id}" class="btn btn-primary min-h-14 w-full text-base"><Icon n="play" size={20} />Mulai Halaqah</a>

		<section class="card">
			<h2 class="mb-3 font-bold">7 hari terakhir</h2>
			<div class="grid grid-cols-3 gap-3 text-center">
				<div>
					<p class="tabular text-3xl font-extrabold">{rekap.jumlah}</p>
					<p class="text-xs text-ink-muted">setoran</p>
				</div>
				<div>
					<p class="text-3xl font-extrabold text-brand">
						{rekap.rata === null ? '-' : nilaiLabel(Math.round(rekap.rata), app.pengajar.skala)}
					</p>
					<p class="text-xs text-ink-muted">rata-rata</p>
				</div>
				<div>
					<p class="tabular text-3xl font-extrabold {tertinggal.length ? 'text-warn' : ''}">{tertinggal.length}</p>
					<p class="text-xs text-ink-muted">tertinggal</p>
				</div>
			</div>
		</section>

		<section class="card">
			<div class="mb-3 flex items-center justify-between">
				<h2 class="font-bold">Target kelompok</h2>
				<button class="btn btn-soft min-h-10 px-3 text-sm" onclick={() => (bukaTarget = true)}
					><Icon n="plus" size={16} />Target</button
				>
			</div>
			{#if targets.length === 0}
				<p class="text-sm text-ink-muted">Belum ada target. Pilih juz, surat, atau naik level.</p>
			{:else}
				<div class="space-y-4">
					{#each targets as t (t.id)}
						{@const r = ringkas(t.id)}
						<div>
							<div class="mb-1.5 flex items-start justify-between gap-2">
								<div>
									<p class="text-sm font-semibold">{NAMA_TARGET[t.jenis]}</p>
									<p class="text-xs text-ink-muted">{t.nilaiTarget} {t.satuan} · tenggat {tanggal(t.tenggat)}</p>
								</div>
								<button
									class="flex size-9 shrink-0 items-center justify-center rounded-lg text-ink-muted"
									aria-label="Hapus target"
									onclick={() => {
										const salin = { ...t };
										app.hapusTarget(t.id);
										app.tampilToast('Target dihapus', {
											label: 'Urungkan',
											fn: () => app.tambahTarget({ ...salin })
										});
									}}><Icon n="trash" size={16} /></button
								>
							</div>
							<Progress persen={r.rata} />
							<p class="mt-1 text-xs text-ink-muted">{r.capai} dari {r.total} santri sudah mencapai · rata-rata {Math.round(r.rata * 100)}%</p>
						</div>
					{/each}
				</div>
			{/if}
		</section>

		<section>
			<h2 class="mb-2 px-1 font-bold">Anggota · {ang.length}</h2>
			<ul class="divide-y divide-line overflow-hidden rounded-2xl border border-line bg-surface">
				{#each ang as s (s.id)}
					<li>
						<a href="/santri/{s.id}" class="flex min-h-14 items-center gap-3 px-3 py-2 active:bg-surface-2">
							<Avatar nama={s.nama} size={40} />
							<span class="min-w-0 flex-1">
								<span class="block truncate font-semibold">{s.nama}</span>
								{#if app.ringkasTarget(s.id)}
									{@const r = app.ringkasTarget(s.id)!}
									<span class="mt-1 flex items-center gap-2">
										<span class="flex-1"><Progress persen={r.persen} warna={r.persen >= 1 ? 'var(--ok)' : 'var(--brand)'} /></span>
										<span class="tabular w-9 text-right text-xs font-bold text-ink-soft">{Math.round(r.persen * 100)}%</span>
									</span>
								{/if}
							</span>
							{#if sudah.has(s.id)}<span class="chip bg-brand-tint text-brand-strong"><Icon n="check" size={12} />Setor</span>{/if}
							<Icon n="chevR" size={18} class="text-ink-muted" />
						</a>
					</li>
				{:else}
					<li class="px-4 py-8 text-center text-sm text-ink-muted">
						Belum ada santri. <a href="/santri?tambah=1" class="font-semibold text-brand">Tambah santri</a>
					</li>
				{/each}
			</ul>
		</section>
	</div>

	<TargetSheet
		bind:open={bukaTarget}
		judul="Buat target"
		kelompokId={id}
		catatan="Target kelompok berlaku ke semua anggota. Target khusus santri menimpa target kelompok."
	/>

	<Sheet bind:open={bukaEdit} judul="Ubah kelompok">
		<label class="label" for="en">Nama kelompok</label>
		<input id="en" class="field" bind:value={nama} autocomplete="off" />
		<label class="label mt-4" for="ej">Jadwal</label>
		<input id="ej" class="field" bind:value={jadwal} placeholder="Sen, Rab, Jum · 16.00" autocomplete="off" />
		<button class="btn btn-primary mt-5 min-h-14 w-full" disabled={!nama.trim()} onclick={simpanEdit}>Simpan</button>
		<button class="btn btn-danger mt-3 w-full" onclick={arsipkan}><Icon n="archive" size={18} />Arsipkan kelompok</button>
		{#if galat}<p class="mt-3 text-sm text-danger" role="alert">{galat}</p>{/if}
	</Sheet>
{/if}
