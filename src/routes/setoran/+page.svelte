<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { app } from '#lib/store.svelte.ts';
	import { surah } from '#lib/quran.ts';
	import { nilaiLabel } from '#lib/format.ts';
	import { NAMA_JENIS, PENJELASAN_JENIS, TAG_KESALAHAN, posisiLabel } from '#lib/posisi.ts';
	import type { Jenis } from '#lib/types.ts';
	import Avatar from '#lib/ui/Avatar.svelte';
	import Icon from '#lib/ui/Icon.svelte';
	import NilaiPicker from '#lib/ui/NilaiPicker.svelte';
	import Sheet from '#lib/ui/Sheet.svelte';
	import Stepper from '#lib/ui/Stepper.svelte';
	import SuratPicker from '#lib/ui/SuratPicker.svelte';

	const q = page.url.searchParams;
	const edit = q.get('edit') ? app.setoran.find((z) => z.id === q.get('edit') && !z.hapusPada) : undefined;
	const dari = q.get('dari') ?? (edit ? 'santri' : '');
	const kelompokAsal = q.get('k') ?? '';

	const santriAwal = edit?.santriId ?? q.get('santri') ?? '';
	let santriId = $state(santriAwal);
	let jenis = $state<Jenis>(edit?.jenis ?? (q.get('jenis') as Jenis | null) ?? 'ziyadah');
	let surat = $state(0);
	let dariAyat = $state(1);
	let sampaiAyat = $state(1);
	let levelId = $state('');
	let halDari = $state(1);
	let halSampai = $state(1);
	let nilai = $state(edit?.nilai ?? 0);
	let catatan = $state(edit?.catatan ?? '');
	let tag = $state<string[]>(edit ? [...edit.tag] : []);
	let bukaCatatan = $state(!!edit?.catatan);
	let bukaTag = $state(!!edit?.tag.length);
	let bukaSantri = $state(!santriAwal && !edit);
	let bukaSurat = $state(false);
	let cari = $state('');

	const santri = $derived(app.santriById(santriId));
	const kelompok = $derived(app.kelompokById(santri?.kelompokId));
	const skala = $derived(app.pengajar.skala);
	const ambang = $derived(app.pengajar.ambangLulus);
	const info = $derived(surat ? surah(surat) : null);

	/** Isi otomatis posisi dari setoran terakhir santri. */
	function muat() {
		if (!santriId) return;
		if (jenis === 'ziyadah') {
			const p = app.posisiZiyadah(santriId);
			surat = p.surat;
			dariAyat = p.ayat;
			sampaiAyat = Math.min(surah(p.surat).ayat, p.ayat + app.spanDefault(santriId) - 1);
		} else if (jenis === 'murojaah') {
			const pertama = app.saranMurojaah(santriId, 1)[0]?.surat ?? poolMurojaah[0];
			if (pertama) pilihSuratMurojaah(pertama);
			else surat = 0;
		} else {
			const p = app.posisiTahsin(santriId);
			levelId = p.levelId;
			const lv = app.levelById(p.levelId);
			halDari = p.halaman;
			halSampai = Math.min(lv?.jumlahHalaman ?? p.halaman, p.halaman + (p.selesai ? 0 : 1));
		}
	}

	function pilihSuratMurojaah(n: number) {
		surat = n;
		dariAyat = 1;
		sampaiAyat = surah(n).ayat;
	}

	const poolMurojaah = $derived(
		santriId
			? [...app.statusSurat(santriId)].filter(([, st]) => st !== 'kosong').map(([n]) => n)
			: []
	);
	const saran = $derived(santriId && jenis === 'murojaah' ? app.saranMurojaah(santriId) : []);
	const tahsin = $derived(santriId ? app.posisiTahsin(santriId) : null);
	const metode = $derived(app.metodeById(santri?.metodeId));
	const level = $derived(app.levelById(levelId));
	const adaBerikutnya = $derived(
		!edit && jenis === 'tahsin' && tahsin?.selesai && tahsin.berikutnya && levelId === tahsin.levelId
	);

	// Nilai awal: dari setoran yang diedit, atau posisi otomatis santri.
	if (edit) {
		surat = edit.suratAwal ?? 0;
		dariAyat = edit.ayatAwal ?? 1;
		sampaiAyat = edit.ayatAkhir ?? 1;
		levelId = edit.levelId ?? '';
		halDari = edit.halamanAwal ?? 1;
		halSampai = edit.halamanAkhir ?? 1;
	} else {
		muat();
	}

	// Rentang tetap konsisten: dari <= sampai <= jumlah ayat/halaman.
	$effect(() => {
		if (sampaiAyat < dariAyat) sampaiAyat = dariAyat;
		if (info && sampaiAyat > info.ayat) sampaiAyat = info.ayat;
		if (info && dariAyat > info.ayat) dariAyat = info.ayat;
	});
	$effect(() => {
		if (halSampai < halDari) halSampai = halDari;
		if (level && halSampai > level.jumlahHalaman) halSampai = level.jumlahHalaman;
	});

	function pilihSantri(id: string) {
		santriId = id;
		nilai = 0;
		catatan = '';
		tag = [];
		bukaSantri = false;
		cari = '';
		muat();
	}
	function gantiJenis(j: Jenis) {
		if (edit || jenis === j) return;
		jenis = j;
		tag = [];
		muat();
	}
	function naikJilid() {
		if (!tahsin?.berikutnya) return;
		const lv = app.levelById(tahsin.berikutnya);
		levelId = tahsin.berikutnya;
		halDari = 1;
		halSampai = Math.min(lv?.jumlahHalaman ?? 2, 2);
	}
	function toggleTag(t: string) {
		tag = tag.includes(t) ? tag.filter((x) => x !== t) : tag.length < 4 ? [...tag, t] : tag;
	}

	const tujuanKembali = $derived(
		dari === 'halaqah'
			? `/halaqah?k=${kelompokAsal || santri?.kelompokId || ''}`
			: dari === 'santri' && santriId
				? `/santri/${santriId}`
				: '/'
	);

	const bisaSimpan = $derived(
		!!santriId && nilai > 0 && (jenis === 'tahsin' ? !!levelId : surat > 0)
	);
	const dibawahAmbang = $derived(nilai > 0 && nilai < ambang);

	function simpan() {
		if (!bisaSimpan || !santri) return;
		const dasar = {
			santriId,
			jenis,
			nilai,
			catatan: catatan.trim() || undefined,
			tag: [...tag]
		};
		const isi =
			jenis === 'tahsin'
				? { ...dasar, levelId, halamanAwal: halDari, halamanAkhir: halSampai }
				: { ...dasar, suratAwal: surat, ayatAwal: dariAyat, suratAkhir: surat, ayatAkhir: sampaiAyat };
		if (edit) {
			app.ubahSetoran(edit.id, isi);
			app.tampilToast('Setoran diperbarui');
		} else {
			const z = app.tambahSetoran(isi);
			app.tampilToast(`Tersimpan · ${santri.nama.split(' ')[0]}`, {
				label: 'Urungkan',
				fn: () => app.hapusSetoran(z.id)
			});
		}
		goto(tujuanKembali, { replace: true });
	}

	function hapus() {
		if (!edit) return;
		const id = edit.id;
		app.hapusSetoran(id);
		app.tampilToast('Setoran dihapus', { label: 'Urungkan', fn: () => app.pulihkanSetoran(id) });
		goto(tujuanKembali, { replace: true });
	}

	const daftarSantri = $derived.by(() => {
		const f = cari.trim().toLowerCase();
		return app.kelompokAktif
			.map((k) => ({
				k,
				ang: app.urutanHalaqah(k.id).filter((s) => !f || s.nama.toLowerCase().includes(f))
			}))
			.filter((g) => g.ang.length);
	});
	const sudahHariIni = $derived(
		new Set(app.setoranHariIni().map((z) => z.santriId))
	);
</script>

<div class="mx-auto flex min-h-dvh max-w-[480px] flex-col">
	<header class="safe-t sticky top-0 z-30 flex min-h-16 items-center gap-2 bg-paper/90 px-3 backdrop-blur">
		<a
			href={tujuanKembali}
			class="flex size-11 items-center justify-center rounded-full text-ink-soft"
			aria-label="Batal"><Icon n="x" size={24} /></a
		>
		<h1 class="flex-1 text-xl font-bold">{edit ? 'Ubah setoran' : 'Catat setoran'}</h1>
		{#if edit}
			<button
				type="button"
				class="flex size-11 items-center justify-center rounded-full text-danger"
				onclick={hapus}
				aria-label="Hapus setoran"><Icon n="trash" size={22} /></button
			>
		{/if}
	</header>

	<div class="flex-1 space-y-5 px-4 pt-1 pb-40">
		<!-- Santri -->
		<button
			type="button"
			class="card flex min-h-[72px] w-full items-center gap-3 text-left disabled:opacity-100"
			onclick={() => (bukaSantri = true)}
			disabled={!!edit}
		>
			{#if santri}
				<Avatar nama={santri.nama} size={48} />
				<span class="min-w-0 flex-1">
					<span class="block truncate text-lg leading-tight font-bold">{santri.nama}</span>
					<span class="block truncate text-sm text-ink-muted">{kelompok?.nama}</span>
				</span>
				{#if !edit}<span class="text-sm font-semibold text-brand">Ganti</span>{/if}
			{:else}
				<span class="flex size-12 items-center justify-center rounded-full bg-surface-2 text-ink-muted"
					><Icon n="user" /></span
				>
				<span class="flex-1 font-semibold text-ink-soft">Pilih santri</span>
				<Icon n="chevR" class="text-ink-muted" />
			{/if}
		</button>

		{#if santri}
			<!-- Jenis -->
			<div>
				<div class="grid grid-cols-3 gap-1.5 rounded-2xl bg-surface-2 p-1.5" role="radiogroup" aria-label="Jenis setoran">
					{#each ['ziyadah', 'murojaah', 'tahsin'] as j (j)}
						<button
							type="button"
							role="radio"
							aria-checked={jenis === j}
							disabled={!!edit && jenis !== j}
							onclick={() => gantiJenis(j as Jenis)}
							class="min-h-12 rounded-xl px-1 text-[14px] font-bold transition disabled:opacity-40 {jenis === j
								? 'bg-surface text-brand shadow-sm'
								: 'text-ink-soft'}">{NAMA_JENIS[j as Jenis]}</button
						>
					{/each}
				</div>
				<p class="mt-2 px-1 text-xs text-ink-muted">{PENJELASAN_JENIS[jenis]}</p>
			</div>

			<!-- Posisi -->
			{#if jenis !== 'tahsin'}
				<section class="space-y-3">
					<div class="card flex items-center gap-3">
						<div class="min-w-0 flex-1">
							{#if info}
								<p class="truncate text-xl leading-tight font-bold">{info.latin}</p>
								<p class="text-sm text-ink-muted">{info.ayat} ayat · Juz {info.juz}</p>
							{:else}
								<p class="font-semibold text-ink-soft">Belum ada surat yang bisa diulang</p>
							{/if}
						</div>
						{#if info}<span class="font-arabic text-3xl" dir="rtl" lang="ar">{info.arab}</span>{/if}
						<button
							type="button"
							class="btn btn-soft min-h-11 px-3 text-sm"
							onclick={() => (bukaSurat = true)}
							disabled={jenis === 'murojaah' && poolMurojaah.length === 0}>Ganti</button
						>
					</div>

					{#if jenis === 'ziyadah' && !edit}
						<p class="flex items-center gap-1.5 px-1 text-xs text-brand-strong">
							<Icon n="bolt" size={14} />Lanjut otomatis dari setoran terakhir ({posisiLabel(santriId).replace('Lanjut ', '')})
						</p>
					{/if}

					{#if jenis === 'murojaah' && saran.length}
						<div>
							<p class="mb-1.5 px-1 text-xs font-semibold text-ink-soft">Paling lama tidak diulang</p>
							<div class="flex flex-wrap gap-2">
								{#each saran as s (s.surat)}
									<button
										type="button"
										class="chip min-h-10 px-3 text-sm {surat === s.surat
											? 'bg-brand text-on-brand'
											: 'border border-line bg-surface text-ink'}"
										onclick={() => pilihSuratMurojaah(s.surat)}
										>{surah(s.surat).latin}
										<span class="opacity-70">{s.hari === null ? 'belum pernah' : `${s.hari} hr`}</span></button
									>
								{/each}
							</div>
						</div>
					{/if}

					{#if info}
						<div class="grid grid-cols-2 gap-2">
							<Stepper label="Dari ayat" bind:value={dariAyat} min={1} max={info.ayat} />
							<Stepper label="Sampai ayat" bind:value={sampaiAyat} min={dariAyat} max={info.ayat} />
						</div>
						{#if jenis === 'ziyadah' && sampaiAyat >= info.ayat && nilai >= ambang}
							<p class="px-1 text-xs text-ok">Surat selesai. Setoran berikutnya lanjut ke surat berikutnya.</p>
						{/if}
					{/if}
				</section>
			{:else}
				<section class="space-y-3">
					<div class="card">
						<p class="text-xs font-semibold text-ink-muted uppercase">{metode?.nama ?? 'Metode'}</p>
						<div class="mt-2 flex flex-wrap gap-2">
							{#each metode?.level ?? [] as lv (lv.id)}
								<button
									type="button"
									class="chip min-h-10 px-3 text-sm {levelId === lv.id
										? 'bg-brand text-on-brand'
										: 'border border-line bg-surface text-ink'}"
									onclick={() => {
										levelId = lv.id;
										halDari = 1;
										halSampai = Math.min(lv.jumlahHalaman, 2);
									}}>{lv.nama}</button
								>
							{/each}
						</div>
					</div>
					{#if level}
						<div class="grid grid-cols-2 gap-2">
							<Stepper label="Dari halaman" bind:value={halDari} min={1} max={level.jumlahHalaman} />
							<Stepper label="Sampai halaman" bind:value={halSampai} min={halDari} max={level.jumlahHalaman} />
						</div>
						<p class="px-1 text-xs text-ink-muted">{level.nama} · {level.jumlahHalaman} halaman</p>
					{/if}
					{#if adaBerikutnya}
						<div class="flex items-center gap-3 rounded-2xl bg-gold-tint p-4">
							<Icon n="flag" class="shrink-0 text-warn" />
							<p class="flex-1 text-sm text-warn">
								Halaman terakhir <b>{level?.nama}</b> sudah tercapai. Naik jilid?
							</p>
							<button type="button" class="btn btn-primary min-h-11 px-3 text-sm" onclick={naikJilid}
								>Naik jilid</button
							>
						</div>
					{/if}
				</section>
			{/if}

			<!-- Nilai -->
			<section>
				<h2 class="mb-2 px-1 text-sm font-bold text-ink-soft">Nilai</h2>
				<NilaiPicker bind:value={nilai} />
				{#if dibawahAmbang}
					<p class="mt-2 flex items-start gap-1.5 px-1 text-xs text-warn">
						<Icon n="info" size={14} class="mt-0.5 shrink-0" />
						Di bawah ambang lulus ({nilaiLabel(ambang, skala)}). Posisi tidak maju, setoran berikutnya mengulang bagian yang sama.
					</p>
				{/if}
			</section>

			<!-- Opsional -->
			<section class="space-y-3">
				{#if !bukaTag}
					<button type="button" class="flex min-h-11 items-center gap-2 px-1 text-sm font-semibold text-brand" onclick={() => (bukaTag = true)}
						><Icon n="plus" size={16} />Tag kesalahan</button
					>
				{:else}
					<div>
						<p class="mb-2 px-1 text-sm font-bold text-ink-soft">
							Tag kesalahan <span class="font-normal text-ink-muted">· opsional, maks. 4</span>
						</p>
						<div class="flex flex-wrap gap-2">
							{#each TAG_KESALAHAN[jenis] as t (t)}
								<button
									type="button"
									aria-pressed={tag.includes(t)}
									disabled={!tag.includes(t) && tag.length >= 4}
									onclick={() => toggleTag(t)}
									class="chip min-h-10 px-4 text-sm disabled:opacity-40 {tag.includes(t)
										? 'bg-danger-tint text-danger ring-1 ring-danger'
										: 'border border-line bg-surface text-ink-soft'}">{t}</button
								>
							{/each}
						</div>
					</div>
				{/if}

				{#if !bukaCatatan}
					<button type="button" class="flex min-h-11 items-center gap-2 px-1 text-sm font-semibold text-brand" onclick={() => (bukaCatatan = true)}
						><Icon n="plus" size={16} />Catatan</button
					>
				{:else}
					<div>
						<label class="label" for="catatan">Catatan</label>
						<textarea
							id="catatan"
							class="field min-h-24 py-3"
							rows="3"
							maxlength="280"
							placeholder="Contoh: ulang besok, sudah lancar"
							bind:value={catatan}
						></textarea>
					</div>
				{/if}
			</section>

			{#if !edit && santriId && sudahHariIni.has(santriId)}
				<p class="flex items-center gap-1.5 px-1 text-xs text-ink-muted">
					<Icon n="info" size={14} />{santri.nama.split(' ')[0]} sudah punya setoran hari ini. Setoran ini ditambahkan, bukan menimpa.
				</p>
			{/if}
		{/if}
	</div>

	<!-- Simpan: menempel di bawah, terjangkau satu tangan -->
	<div class="safe-b fixed inset-x-0 bottom-0 z-30 bg-gradient-to-t from-paper via-paper to-transparent pt-6">
		<div class="mx-auto max-w-[480px] px-4 pb-4">
			<button
				type="button"
				class="btn btn-primary min-h-14 w-full text-lg shadow-lg"
				disabled={!bisaSimpan}
				onclick={simpan}
			>
				<Icon n="check" size={22} />{edit ? 'Simpan perubahan' : 'Simpan setoran'}
			</button>
			{#if santri && nilai === 0}
				<p class="mt-2 text-center text-xs text-ink-muted">Pilih nilai untuk menyimpan.</p>
			{/if}
		</div>
	</div>
</div>

<!-- Pemilih santri -->
<Sheet bind:open={bukaSantri} judul="Pilih santri" tinggi="penuh">
	<div class="sticky top-0 z-10 -mx-5 bg-paper px-5 pb-3">
		<div class="relative">
			<Icon n="search" size={18} class="absolute top-1/2 left-3.5 -translate-y-1/2 text-ink-muted" />
			<input class="field pl-10" placeholder="Cari nama santri" bind:value={cari} autocomplete="off" />
		</div>
	</div>
	{#each daftarSantri as g (g.k.id)}
		<h3 class="mt-3 mb-1.5 px-1 text-xs font-bold tracking-wide text-ink-muted uppercase">{g.k.nama}</h3>
		<ul class="space-y-1.5">
			{#each g.ang as s (s.id)}
				<li>
					<button
						type="button"
						class="flex min-h-14 w-full items-center gap-3 rounded-xl border px-3 text-left {s.id === santriId
							? 'border-brand bg-brand-tint'
							: 'border-line bg-surface'}"
						onclick={() => pilihSantri(s.id)}
					>
						<Avatar nama={s.nama} size={40} />
						<span class="min-w-0 flex-1 truncate font-semibold">{s.nama}</span>
						{#if sudahHariIni.has(s.id)}<span class="chip bg-brand-tint text-brand-strong">Sudah setor</span>{/if}
					</button>
				</li>
			{/each}
		</ul>
	{:else}
		<p class="py-10 text-center text-ink-muted">
			{app.santriAktif.length ? 'Nama tidak ditemukan.' : 'Belum ada santri. Tambahkan dulu di menu Santri.'}
		</p>
	{/each}
</Sheet>

<SuratPicker
	bind:open={bukaSurat}
	dipilih={surat}
	batasi={jenis === 'murojaah' ? poolMurojaah : undefined}
	keterangan={(n) =>
		jenis === 'murojaah' ? (app.statusSurat(santriId).get(n) === 'hafal' ? 'hafal' : 'sedang dihafal') : undefined}
	pilih={(s) => {
		surat = s.n;
		dariAyat = 1;
		sampaiAyat = jenis === 'murojaah' ? s.ayat : Math.min(s.ayat, app.spanDefault(santriId));
	}}
/>
