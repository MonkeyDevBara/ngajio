<script lang="ts">
	import { goto } from '$app/navigation';
	import { app, BATAS_GRATIS, sekarang } from '#lib/store.svelte.ts';
	import { eksporCSV, eksporJSON } from '#lib/ekspor.ts';
	import { nilaiLabel, rupiah, tanggal } from '#lib/format.ts';
	import type { Arah, SkalaJenis, Tema } from '#lib/types.ts';
	import { fotoAman, fotoDariBerkas } from '#lib/foto.ts';
	import Avatar from '#lib/ui/Avatar.svelte';
	import Icon from '#lib/ui/Icon.svelte';
	import PageHeader from '#lib/ui/PageHeader.svelte';
	import Progress from '#lib/ui/Progress.svelte';
	import Sheet from '#lib/ui/Sheet.svelte';

	let nama = $state(app.pengajar.nama);
	let lembaga = $state(app.pengajar.lembaga);
	let bukaPaket = $state(false);
	let bukaKeluar = $state(false);
	let inputFoto = $state<HTMLInputElement>();

	const p = $derived(app.pengajar);
	const setoranBaru = $derived(app.setoranAktif.length - app.ekspor.jumlah);

	function simpanProfil() {
		if (nama.trim() !== p.nama || lembaga.trim() !== p.lembaga)
			app.ubahPengajar({ nama: nama.trim(), lembaga: lembaga.trim() });
	}

	async function pilihFoto(e: Event & { currentTarget: HTMLInputElement }) {
		const f = e.currentTarget.files?.[0];
		e.currentTarget.value = '';
		if (!f) return;
		if (!f.type.startsWith('image/')) return app.tampilToast('Pilih berkas gambar, misalnya JPG atau PNG.');
		try {
			app.ubahPengajar({ foto: await fotoDariBerkas(f) });
		} catch {
			app.tampilToast('Foto tidak bisa dibaca. Coba gambar lain.');
		}
	}

	const SKALA: [SkalaJenis, string][] = [
		['huruf', 'Huruf'],
		['bintang', 'Bintang'],
		['predikat', 'Predikat'],
		['angka', 'Angka']
	];
	const TEMA: [Tema, string][] = [
		['terang', 'Terang'],
		['gelap', 'Gelap'],
		['otomatis', 'Ikut sistem']
	];
	const ARAH: [Arah, string, string][] = [
		['juz30', 'Juz 30 dulu', 'Dari An-Nas naik ke An-Naba’'],
		['mushaf', 'Urut mushaf', 'Dari Al-Fatihah']
	];
	const TIER = [
		{ nama: 'Gratis', batas: 'Sampai 40 santri', maks: BATAS_GRATIS, harga: 0, isi: 'Semua fitur input, laporan dasar, PDF tanpa kop' },
		{ nama: '41–80 santri', batas: 'Sampai 80 santri', maks: 80, harga: 49000, isi: 'Peta hafalan 30 juz, PDF berkop pengajar' },
		{ nama: '81–150 santri', batas: 'Sampai 150 santri', maks: 150, harga: 99000, isi: 'Semua fitur paket sebelumnya' },
		{ nama: '151–300 santri', batas: 'Sampai 300 santri', maks: 300, harga: 249000, isi: 'Semua fitur paket sebelumnya' },
		{ nama: 'Di atas 300 santri', batas: 'Tanpa batas', maks: Infinity, harga: 499000, isi: 'Prioritas dukungan' }
	];

	// Paket aktif: gratis, atau tingkat berbayar menurut jumlah santri aktif.
	const aktifIdx = $derived(
		app.gratis ? 0 : Math.max(1, TIER.findIndex((t) => app.santriAktif.length <= t.maks))
	);

	function ambilNilai(v: number) {
		app.ubahPengajar({ ambangLulus: Math.min(5, Math.max(1, v)) });
	}
	function keluar() {
		app.keluar();
		goto('/masuk', { replace: true });
	}
	const segmen = (aktif: boolean) =>
		`min-h-11 rounded-xl text-sm font-bold ${aktif ? 'bg-surface text-brand shadow-sm' : 'text-ink-soft'}`;
</script>

<PageHeader judul="Pengaturan" kembali="/" />

<div class="space-y-5 px-4 pb-10">
	<section class="card space-y-4">
		<h2 class="font-bold">Profil</h2>
		<div class="flex items-center gap-4">
			<Avatar nama={p.nama || 'Ustadz'} foto={p.foto} size={72} />
			<div class="flex flex-wrap gap-2">
				<button class="btn btn-soft min-h-11 px-3 text-sm" onclick={() => inputFoto?.click()}>Ganti foto</button>
				{#if fotoAman(p.foto)}
					<button class="btn btn-line min-h-11 px-3 text-sm" onclick={() => app.ubahPengajar({ foto: undefined })}>Hapus foto</button>
				{/if}
			</div>
			<input bind:this={inputFoto} type="file" accept="image/*" class="hidden" onchange={pilihFoto} />
		</div>
		<div>
			<label class="label" for="pn">Nama</label>
			<input id="pn" class="field" bind:value={nama} onblur={simpanProfil} autocomplete="off" />
		</div>
		<div>
			<label class="label" for="pl">Lembaga <span class="font-normal text-ink-muted">(opsional)</span></label>
			<input id="pl" class="field" bind:value={lembaga} onblur={simpanProfil} autocomplete="off" />
		</div>
		<p class="text-sm text-ink-muted">Masuk sebagai {p.email || '-'}</p>
	</section>

	<section class="card space-y-4">
		<h2 class="font-bold">Setoran</h2>

		<div>
			<span class="label">Skala nilai</span>
			<div class="grid grid-cols-4 gap-1.5 rounded-2xl bg-surface-2 p-1.5">
				{#each SKALA as [id, l] (id)}
					<button class={segmen(p.skala === id)} onclick={() => app.ubahPengajar({ skala: id })}>{l}</button>
				{/each}
			</div>
			<p class="mt-2 text-sm text-ink-soft">
				Contoh: {[5, 4, 3, 2, 1].map((n) => nilaiLabel(n, p.skala)).join(' · ')}
			</p>
			<p class="mt-1 text-xs text-ink-muted">Nilai lama tetap tampil dalam skala saat dicatat. Laporan memakai nilai normal 1–5.</p>
		</div>

		<div>
			<span class="label">Ambang lulus</span>
			<div class="flex items-center justify-between rounded-2xl border border-line bg-surface p-1.5">
				<button class="flex size-12 items-center justify-center rounded-xl bg-surface-2 text-2xl disabled:opacity-35" disabled={p.ambangLulus <= 1} onclick={() => ambilNilai(p.ambangLulus - 1)} aria-label="Kurangi">−</button>
				<div class="text-center">
					<p class="text-2xl font-bold">{nilaiLabel(p.ambangLulus, p.skala)}</p>
					<p class="text-xs text-ink-muted">nilai {p.ambangLulus} dari 5</p>
				</div>
				<button class="flex size-12 items-center justify-center rounded-xl bg-brand-tint text-2xl text-brand-strong disabled:opacity-35" disabled={p.ambangLulus >= 5} onclick={() => ambilNilai(p.ambangLulus + 1)} aria-label="Tambah">+</button>
			</div>
			<p class="mt-2 text-xs text-ink-muted">Di bawah ambang, posisi tidak maju. Surat dianggap hafal bila semua ayatnya lulus.</p>
		</div>

		<div>
			<span class="label">Arah hafalan santri baru</span>
			<div class="grid grid-cols-2 gap-2">
				{#each ARAH as [id, l, k] (id)}
					<button
						class="rounded-2xl border p-3 text-left {p.arah === id ? 'border-brand bg-brand-tint' : 'border-line bg-surface'}"
						onclick={() => app.ubahPengajar({ arah: id })}
					>
						<span class="block text-sm font-bold">{l}</span>
						<span class="block text-xs text-ink-muted">{k}</span>
					</button>
				{/each}
			</div>
		</div>

		<div>
			<span class="label">Metode tahsin</span>
			<a href="/pengaturan/metode" class="flex min-h-14 items-center justify-between rounded-2xl border border-line bg-surface px-4">
				<span>
					<span class="block font-semibold">{app.metodeById(p.metodeId)?.nama}</span>
					<span class="block text-xs text-ink-muted">{app.metode.length} metode · atur level dan halaman</span>
				</span>
				<Icon n="chevR" class="text-ink-muted" />
			</a>
		</div>
	</section>

	<section class="card space-y-4">
		<h2 class="font-bold">Tampilan</h2>
		<div>
			<span class="label">Tema</span>
			<div class="grid grid-cols-3 gap-1.5 rounded-2xl bg-surface-2 p-1.5">
				{#each TEMA as [id, l] (id)}
					<button class={segmen(p.tema === id)} onclick={() => app.ubahPengajar({ tema: id })}>{l}</button>
				{/each}
			</div>
		</div>
	</section>

	<section id="sinkron" class="card space-y-3 scroll-mt-20">
		<h2 class="font-bold">Sinkronisasi</h2>
		<div class="flex items-center gap-3">
			<span
				class="flex size-10 items-center justify-center rounded-full {app.statusSinkron === 'tersinkron' ? 'bg-brand-tint text-brand-strong' : app.statusSinkron === 'menunggu' ? 'bg-gold-tint text-warn' : 'bg-danger-tint text-danger'}"
				><Icon n={app.statusSinkron === 'tersinkron' ? 'check' : app.statusSinkron === 'menunggu' ? 'refresh' : 'alert'} /></span
			>
			<div class="flex-1">
				<p class="font-semibold capitalize">{app.statusSinkron}</p>
				<p class="text-xs text-ink-muted">
					{app.mode === 'contoh'
						? 'Mode contoh: data hanya ada di HP ini'
						: app.sesiBerakhir
							? 'Sesi berakhir, masuk lagi untuk melanjutkan'
							: `${app.antrian ? `${app.antrian} perubahan menunggu dikirim. ` : ''}${app.sinkronTerakhir ? `Terakhir ${tanggal(app.sinkronTerakhir, { jam: true })}` : 'Belum pernah sinkron'}`}
				</p>
			</div>
			{#if app.sesiBerakhir}
				<a class="btn btn-soft min-h-11 px-3 text-sm" href="/masuk">Masuk lagi</a>
			{:else if app.statusSinkron === 'gagal'}
				<button class="btn btn-soft min-h-11 px-3 text-sm" onclick={() => app.cobaLagiSinkron()}>Coba lagi</button>
			{/if}
		</div>
	</section>

	<section class="card space-y-3">
		<div class="flex items-center justify-between">
			<h2 class="font-bold">Paket</h2>
			<span class="chip {app.gratis ? 'bg-surface-2 text-ink-soft' : 'bg-gold-tint text-warn'}">{app.gratis ? 'Gratis' : 'Berbayar'}</span>
		</div>
		{#if app.gratis}
			<div>
				<div class="mb-1.5 flex justify-between text-sm">
					<span>Santri aktif</span>
					<span class="tabular font-bold">{app.santriAktif.length} / {BATAS_GRATIS}</span>
				</div>
				<Progress persen={app.santriAktif.length / BATAS_GRATIS} warna={app.santriAktif.length >= BATAS_GRATIS * 0.9 ? 'var(--warn)' : 'var(--brand)'} />
			</div>
		{/if}
		<div class="grid grid-cols-2 gap-2">
			<button class="btn btn-line text-sm" onclick={() => (bukaPaket = true)}>Lihat paket</button>
			<button class="btn btn-soft text-sm" onclick={() => app.tampilToast('Terima kasih! Pembayaran aktif setelah backend tersambung.')}><Icon n="heart" size={18} />Infaq</button>
		</div>
	</section>

	<section id="ekspor" class="card space-y-3 scroll-mt-20">
		<h2 class="font-bold">Data saya</h2>
		<p class="text-sm text-ink-soft">
			{app.ekspor.terakhir ? `Ekspor terakhir ${tanggal(app.ekspor.terakhir, { tahun: true })}.` : 'Belum pernah ekspor.'}
			{#if setoranBaru > 0}{setoranBaru} setoran baru sejak itu.{/if}
		</p>
		<div class="grid grid-cols-2 gap-2">
			<button class="btn btn-primary text-sm" onclick={() => { eksporJSON(); app.tampilToast('Cadangan JSON diunduh'); }}><Icon n="download" size={18} />JSON</button>
			<button class="btn btn-line text-sm" onclick={() => { eksporCSV(); app.tampilToast('3 berkas CSV diunduh'); }}><Icon n="download" size={18} />CSV</button>
		</div>
		<p class="text-xs text-ink-muted">JSON berisi semua data. CSV berisi santri, kelompok, dan setoran, bisa dibuka di Excel.</p>
	</section>

	<section class="card space-y-2">
		<h2 class="mb-1 font-bold">Akun</h2>
		<button class="btn btn-line w-full justify-start" onclick={() => (bukaKeluar = true)}><Icon n="logout" size={18} />Keluar</button>
		{#if app.mode === 'contoh'}
			<button class="btn btn-line w-full justify-start text-ink-soft" onclick={() => { app.muatUlangContoh(); app.tampilToast('Data contoh dimuat ulang'); }}><Icon n="refresh" size={18} />Muat ulang data contoh</button>
		{/if}
	</section>

	<p class="pb-2 text-center text-xs text-ink-muted">Ngajio v0.1 · {tanggal(sekarang().toISOString(), { tahun: true })}</p>
</div>

<Sheet bind:open={bukaPaket} judul="Paket Ngajio">
	<div class="mb-3 rounded-2xl border border-brand bg-brand-tint p-4">
		<p class="text-xs font-semibold tracking-wide text-brand-strong uppercase">Paket aktif</p>
		<p class="text-xl font-extrabold text-brand-strong">{TIER[aktifIdx].nama}</p>
		{#if app.gratis}
			<p class="tabular mt-1 mb-1.5 text-sm text-ink-soft">{app.santriAktif.length} dari {BATAS_GRATIS} santri aktif</p>
			<Progress persen={app.santriAktif.length / BATAS_GRATIS} warna={app.santriAktif.length >= BATAS_GRATIS * 0.9 ? 'var(--warn)' : 'var(--brand)'} />
		{:else}
			<p class="tabular mt-1 text-sm text-ink-soft">{app.santriAktif.length} santri aktif · {TIER[aktifIdx].batas}</p>
		{/if}
	</div>
	<ul class="space-y-2">
		{#each TIER as t, i (t.nama)}
			<li class="rounded-2xl border bg-surface p-4 {i === aktifIdx ? 'border-brand' : 'border-line'}">
				<div class="flex items-baseline justify-between gap-2">
					<p class="font-bold">{t.nama} {#if i === aktifIdx}<span class="chip ml-1 bg-brand-tint text-brand-strong">Aktif</span>{/if}</p>
					<p class="tabular font-bold text-brand">{t.harga ? `${rupiah(t.harga)}/tahun` : 'Rp0'}</p>
				</div>
				<p class="text-sm text-ink-soft">{t.batas}</p>
				<p class="text-xs text-ink-muted">{t.isi}</p>
			</li>
		{/each}
	</ul>
	<p class="mt-3 text-xs text-ink-muted">Harga masih asumsi dan akan divalidasi. Santri diarsip tidak dihitung. Upgrade di tengah periode dihitung prorata.</p>
	<button class="btn btn-primary mt-4 min-h-14 w-full" onclick={() => { bukaPaket = false; app.tampilToast('Pembayaran aktif setelah backend tersambung.'); }}>Upgrade</button>
</Sheet>

<Sheet bind:open={bukaKeluar} judul="Keluar dari Ngajio?">
	<p class="text-sm text-ink-soft">Data di HP ini akan dihapus setelah Anda keluar.{#if app.mode === 'akun'} Data yang sudah tersinkron tetap ada di server dan kembali saat Anda masuk lagi.{/if}</p>
	{#if app.statusSinkron !== 'tersinkron'}
		<p class="mt-3 flex gap-2 rounded-xl bg-danger-tint p-3 text-sm text-danger" role="alert">
			<Icon n="alert" size={18} class="mt-0.5 shrink-0" />Ada {app.antrian || 'beberapa'} perubahan yang belum tersinkron. Ekspor cadangan dulu agar tidak hilang.
		</p>
	{/if}
	<div class="mt-5 grid grid-cols-2 gap-2">
		<button class="btn btn-line" onclick={() => (bukaKeluar = false)}>Batal</button>
		<button class="btn btn-danger" onclick={keluar}>Keluar</button>
	</div>
</Sheet>
