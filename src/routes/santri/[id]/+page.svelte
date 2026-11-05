<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { app, sekarang } from '#lib/store.svelte.ts';
	import { surah } from '#lib/quran.ts';
	import { jam, nilaiLabel, relatif, tanggal } from '#lib/format.ts';
	import { NAMA_JENIS, NAMA_TARGET, posisiLabel, tahsinLabel, teksSetoran } from '#lib/posisi.ts';
	import Avatar from '#lib/ui/Avatar.svelte';
	import Icon from '#lib/ui/Icon.svelte';
	import PageHeader from '#lib/ui/PageHeader.svelte';
	import PetaHafalan from '#lib/ui/PetaHafalan.svelte';
	import Progress from '#lib/ui/Progress.svelte';
	import Sheet from '#lib/ui/Sheet.svelte';
	import TargetSheet from '#lib/ui/TargetSheet.svelte';

	const id = $derived(page.params.id ?? '');
	const s = $derived(app.santriById(id));
	const kelompok = $derived(app.kelompokById(s?.kelompokId));
	const metode = $derived(app.metodeById(s?.metodeId));

	const riwayat = $derived(app.setoranSantri(id));
	let tampil = $state(10);
	const potong = $derived(riwayat.slice(0, tampil));
	const perHari = $derived.by(() => {
		const out: { key: string; iso: string; item: typeof riwayat }[] = [];
		for (const z of potong) {
			const k = app.hariBisnis(z.dicatatPada);
			const g = out.at(-1);
			if (g && g.key === k) g.item.push(z);
			else out.push({ key: k, iso: z.dicatatPada, item: [z] });
		}
		return out;
	});

	const targets = $derived(s ? app.targetEfektif(id) : []);
	const status = $derived(s ? app.statusSurat(id) : new Map());
	const hafal = $derived([...status.values()].filter((v) => v === 'hafal').length);
	const streak = $derived(s ? app.streak(id) : 0);
	const saran = $derived(s ? app.saranMurojaah(id) : []);

	const ringkas = $derived(s ? app.ringkasTarget(id) : null);

	/* Target khusus santri: buka sheet, pilih jenis, isi angka. Menimpa target kelompok dengan jenis sama. */
	let bukaTarget = $state(false);
	function hapusTarget(tid: string) {
		const salin = { ...app.target.find((x) => x.id === tid)! };
		app.hapusTarget(tid);
		app.tampilToast('Target dihapus', { label: 'Urungkan', fn: () => app.tambahTarget({ ...salin }) });
	}

	let bukaMenu = $state(false);
	let bukaPindah = $state(false);
	let bukaEdit = $state(false);
	let namaEdit = $state('');
	let waEdit = $state('');
	let galat = $state('');

	function mulaiEdit() {
		namaEdit = s?.nama ?? '';
		waEdit = s?.waWali ?? '';
		bukaMenu = false;
		bukaEdit = true;
	}
	function simpanEdit() {
		if (!namaEdit.trim()) return;
		app.ubahSantri(id, { nama: namaEdit.trim().replace(/\s+/g, ' '), waWali: waEdit.trim() || undefined });
		bukaEdit = false;
		app.tampilToast('Data santri diperbarui');
	}
	function pindah(kid: string) {
		const asal = kelompok?.nama;
		app.pindahKelompok(id, kid);
		bukaPindah = false;
		bukaMenu = false;
		app.tampilToast(`Dipindah dari ${asal} ke ${app.kelompokById(kid)?.nama}`);
	}
	function arsip(v: boolean) {
		if (!app.arsipkanSantri(id, v)) {
			galat = 'Paket gratis maksimal 50 santri aktif. Arsipkan santri lain dulu atau upgrade paket.';
			return;
		}
		bukaMenu = false;
		app.tampilToast(v ? 'Santri diarsipkan. Riwayat tetap ada di laporan.' : 'Santri diaktifkan kembali', {
			label: 'Urungkan',
			fn: () => app.arsipkanSantri(id, !v)
		});
		if (v) goto('/santri', { replace: true });
	}
	function hapusSetoran(zid: string) {
		app.hapusSetoran(zid);
		app.tampilToast('Setoran dihapus', { label: 'Urungkan', fn: () => app.pulihkanSetoran(zid) });
	}

	const ikonJenis = { ziyadah: 'book', murojaah: 'repeat', tahsin: 'flag' } as const;
</script>

{#if !s}
	<PageHeader judul="Santri" kembali="/santri" />
	<div class="px-4"><div class="card py-10 text-center text-ink-muted">Santri tidak ditemukan.</div></div>
{:else}
	<PageHeader judul={s.nama} sub="{kelompok?.nama} · {metode?.nama}" kembali="/santri">
		{#snippet aksi()}
			<button
				class="flex size-11 items-center justify-center rounded-full text-ink-soft"
				aria-label="Menu santri"
				onclick={() => (bukaMenu = true)}><Icon n="dots" size={24} /></button
			>
		{/snippet}
	</PageHeader>

	<div class="space-y-4 px-4 pb-8">
		{#if s.arsip}
			<div class="flex items-center gap-3 rounded-2xl bg-gold-tint p-4 text-sm text-warn">
				<Icon n="archive" class="shrink-0" />
				<span class="flex-1">Santri ini diarsipkan. Tidak muncul di daftar aktif, tetap ada di laporan.</span>
				<button class="btn btn-line min-h-11 px-3 text-sm" onclick={() => arsip(false)}>Aktifkan</button>
			</div>
			{#if galat}<p class="text-sm text-danger" role="alert">{galat}</p>{/if}
		{:else}
			<a href="/setoran?santri={id}&dari=santri" class="btn btn-primary min-h-14 w-full text-base">
				<Icon n="plus" size={22} />Catat setoran
			</a>
		{/if}

		<div class="grid grid-cols-2 gap-3">
			<div class="card">
				<p class="text-xs font-semibold text-ink-muted uppercase">Hafalan</p>
				<p class="mt-1 text-lg leading-tight font-bold">{posisiLabel(id).replace('Lanjut ', '')}</p>
				<p class="mt-1 text-xs text-ink-muted">{hafal} surat hafal · {surah(app.posisiZiyadah(id).surat).arab}</p>
			</div>
			<div class="card">
				<p class="text-xs font-semibold text-ink-muted uppercase">Tahsin</p>
				<p class="mt-1 text-lg leading-tight font-bold">{tahsinLabel(id)}</p>
				<p class="mt-1 text-xs text-ink-muted">{metode?.nama}</p>
			</div>
		</div>
		{#if streak > 0}
			<p class="px-1 text-sm text-ink-soft">Setor <b class="tabular">{streak}</b> hari berturut-turut.</p>
		{/if}

		<section class="card space-y-4">
			<div class="flex items-center justify-between">
				<h2 class="font-bold">Target</h2>
				{#if !s.arsip}
					<button class="btn btn-soft min-h-10 px-3 text-sm" onclick={() => (bukaTarget = true)}
						><Icon n="plus" size={16} />Target</button
					>
				{/if}
			</div>
			{#if ringkas}
				<div>
					<div class="mb-1.5 flex items-baseline justify-between">
						<span class="text-sm font-semibold text-ink-soft">Progress keseluruhan</span>
						<span class="tabular text-lg font-extrabold {ringkas.persen >= 1 ? 'text-ok' : 'text-brand'}">{Math.round(ringkas.persen * 100)}%</span>
					</div>
					<Progress persen={ringkas.persen} warna={ringkas.persen >= 1 ? 'var(--ok)' : 'var(--brand)'} />
				</div>
				<div class="space-y-4 border-t border-line pt-4">
					{#each targets as t (t.id)}
						{@const p = app.progressTarget(t, id)}
						<div>
							<div class="mb-1.5 flex items-start justify-between gap-2">
								<span class="text-sm font-semibold">
									{NAMA_TARGET[t.jenis]}
									<span class="font-normal text-ink-muted">· {t.santriId ? 'khusus santri' : 'dari kelompok'}</span>
								</span>
								<span class="flex shrink-0 items-center gap-1">
									<span class="tabular text-sm font-bold {p.persen >= 1 ? 'text-ok' : ''}">{Math.round(p.persen * 100)}%</span>
									{#if t.santriId}
										<button
											class="flex size-9 items-center justify-center rounded-lg text-ink-muted"
											aria-label="Hapus target"
											onclick={() => hapusTarget(t.id)}><Icon n="trash" size={16} /></button
										>
									{/if}
								</span>
							</div>
							<Progress persen={p.persen} warna={p.persen >= 1 ? 'var(--ok)' : 'var(--brand)'} />
							<p class="mt-1 text-xs text-ink-muted">{p.label} · tenggat {tanggal(t.tenggat)}</p>
						</div>
					{/each}
				</div>
			{:else}
				<p class="text-sm text-ink-muted">Belum ada target untuk santri ini. Pilih juz, surat, atau naik level.</p>
			{/if}
		</section>

		<PetaHafalan santriId={id} />

		{#if saran.length && !s.arsip}
			<section class="card">
				<h2 class="font-bold">Saran murajaah</h2>
				<p class="mb-3 text-xs text-ink-muted">Hafalan yang paling lama tidak diulang.</p>
				<div class="space-y-2">
					{#each saran as m (m.surat)}
						<a
							href="/setoran?santri={id}&jenis=murojaah&dari=santri"
							class="flex min-h-12 items-center justify-between rounded-xl bg-surface-2 px-3"
						>
							<span class="font-semibold">{surah(m.surat).latin}</span>
							<span class="text-xs text-ink-muted">{m.hari === null ? 'belum pernah diulang' : `${m.hari} hari lalu`}</span>
						</a>
					{/each}
				</div>
			</section>
		{/if}

		<section>
			<h2 class="mb-2 px-1 font-bold">Riwayat setoran</h2>
			{#if riwayat.length === 0}
				<div class="card py-8 text-center text-ink-muted">Belum ada setoran.</div>
			{:else}
				<div class="space-y-4">
					{#each perHari as g (g.key)}
						<div>
							<p class="mb-1.5 px-1 text-xs font-bold tracking-wide text-ink-muted uppercase">
								{relatif(g.iso)}{#if relatif(g.iso).includes('lalu') || relatif(g.iso) === 'Kemarin'} · {tanggal(g.iso)}{/if}
							</p>
							<ul class="divide-y divide-line overflow-hidden rounded-2xl border border-line bg-surface">
								{#each g.item as z (z.id)}
									<li class="flex items-start gap-3 px-3 py-3">
										<span class="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-xl bg-brand-tint text-brand-strong"
											><Icon n={ikonJenis[z.jenis]} size={18} /></span
										>
										<div class="min-w-0 flex-1">
											<p class="font-semibold">{teksSetoran(z)}</p>
											<p class="text-xs text-ink-muted">{NAMA_JENIS[z.jenis]} · {jam(z.dicatatPada)}</p>
											{#if z.tag.length}
												<div class="mt-1.5 flex flex-wrap gap-1">
													{#each z.tag as t (t)}<span class="chip bg-danger-tint py-0.5 text-danger">{t}</span>{/each}
												</div>
											{/if}
											{#if z.catatan}<p class="mt-1 text-sm text-ink-soft">“{z.catatan}”</p>{/if}
										</div>
										<div class="flex flex-col items-end gap-1">
											<span class="chip bg-surface-2 text-ink">{nilaiLabel(z.nilai, app.pengajar.skala)}</span>
											{#if app.bisaDiubah(z)}
												<div class="flex">
													<a
														href="/setoran?edit={z.id}&dari=santri"
														class="flex size-9 items-center justify-center rounded-lg text-ink-muted"
														aria-label="Ubah setoran"><Icon n="pencil" size={16} /></a
													>
													<button
														class="flex size-9 items-center justify-center rounded-lg text-ink-muted"
														aria-label="Hapus setoran"
														onclick={() => hapusSetoran(z.id)}><Icon n="trash" size={16} /></button
													>
												</div>
											{:else}
												<span class="text-[10px] text-ink-muted">terkunci</span>
											{/if}
										</div>
									</li>
								{/each}
							</ul>
						</div>
					{/each}
				</div>
				{#if riwayat.length > tampil}
					<button class="btn btn-line mt-4 w-full" onclick={() => (tampil += 10)}
						>Tampilkan lebih banyak ({riwayat.length - tampil})</button
					>
				{/if}
			{/if}
		</section>
	</div>

	<TargetSheet
		bind:open={bukaTarget}
		judul="Target {s.nama.split(' ')[0]}"
		santriId={id}
		catatan="Target khusus santri menimpa target kelompok dengan jenis yang sama."
	/>

	<Sheet bind:open={bukaMenu} judul={s.nama}>
		<div class="space-y-2">
			<button class="btn btn-line w-full justify-start" onclick={mulaiEdit}><Icon n="pencil" size={18} />Ubah nama & kontak wali</button>
			<button class="btn btn-line w-full justify-start" onclick={() => { bukaMenu = false; bukaPindah = true; }}
				><Icon n="users" size={18} />Pindah kelompok</button
			>
			<div class="rounded-2xl border border-line bg-surface p-3">
				<p class="mb-2 text-sm font-semibold">Arah hafalan</p>
				<div class="grid grid-cols-2 gap-1.5 rounded-xl bg-surface-2 p-1">
					{#each [['juz30', 'Juz 30 dari An-Nas'], ['mushaf', 'Urut mushaf']] as [v, l] (v)}
						<button
							class="min-h-11 rounded-lg px-1 text-xs font-bold {s.arah === v ? 'bg-surface text-brand shadow-sm' : 'text-ink-soft'}"
							onclick={() => app.ubahSantri(id, { arah: v as 'juz30' | 'mushaf' })}>{l}</button
						>
					{/each}
				</div>
			</div>
			{#if !s.arsip}
				<button class="btn btn-danger w-full justify-start" onclick={() => arsip(true)}
					><Icon n="archive" size={18} />Arsipkan santri</button
				>
			{/if}
		</div>
	</Sheet>

	<Sheet bind:open={bukaPindah} judul="Pindah kelompok">
		<p class="mb-3 text-sm text-ink-muted">Riwayat mengikuti santri. Laporan kelompok lama tetap menampilkan setoran saat santri masih di sana.</p>
		<ul class="space-y-2">
			{#each app.kelompokAktif as k (k.id)}
				<li>
					<button
						class="flex min-h-14 w-full items-center justify-between rounded-xl border px-4 {k.id === s.kelompokId
							? 'border-brand bg-brand-tint'
							: 'border-line bg-surface'}"
						disabled={k.id === s.kelompokId}
						onclick={() => pindah(k.id)}
					>
						<span class="font-semibold">{k.nama}</span>
						{#if k.id === s.kelompokId}<span class="text-xs text-brand-strong">Sekarang</span>{/if}
					</button>
				</li>
			{/each}
		</ul>
	</Sheet>

	<Sheet bind:open={bukaEdit} judul="Ubah data santri">
		<label class="label" for="en">Nama</label>
		<input id="en" class="field" bind:value={namaEdit} autocomplete="off" />
		<label class="label mt-4" for="ew">Nomor WA wali <span class="font-normal text-ink-muted">(opsional)</span></label>
		<input id="ew" class="field" bind:value={waEdit} inputmode="tel" autocomplete="off" />
		<button class="btn btn-primary mt-5 min-h-14 w-full" disabled={!namaEdit.trim()} onclick={simpanEdit}>Simpan</button>
	</Sheet>
{/if}
