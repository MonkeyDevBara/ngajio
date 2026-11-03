<script lang="ts">
	import { app, sekarang } from '#lib/store.svelte.ts';
	import { angka, nilaiLabel, tanggal } from '#lib/format.ts';
	import { NAMA_JENIS, teksSetoran } from '#lib/posisi.ts';
	import Icon from '#lib/ui/Icon.svelte';
	import PageHeader from '#lib/ui/PageHeader.svelte';
	import Progress from '#lib/ui/Progress.svelte';

	type Tab = 'global' | 'kelompok' | 'santri';
	type Periode = 'pekan' | 'bulan' | 'semester' | 'custom';

	let tab = $state<Tab>('global');
	let periode = $state<Periode>('bulan');
	let kelompokId = $state(app.kelompokAktif[0]?.id ?? '');
	let santriId = $state(app.santriAktif[0]?.id ?? '');

	const isoLokal = (d: Date) =>
		`${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
	let cDari = $state(isoLokal(new Date(sekarang().getTime() - 14 * 86400000)));
	let cSampai = $state(isoLokal(sekarang()));

	const rentang = $derived.by(() => {
		const now = sekarang();
		if (periode === 'pekan') return { dari: new Date(now.getTime() - 7 * 86400000), sampai: now };
		if (periode === 'bulan') return { dari: new Date(now.getFullYear(), now.getMonth(), 1), sampai: now };
		if (periode === 'semester') {
			const m = now.getMonth() >= 6 ? 6 : 0;
			return { dari: new Date(now.getFullYear(), m, 1), sampai: now };
		}
		const d = new Date(cDari + 'T00:00:00');
		const s = new Date(cSampai + 'T23:59:59');
		return { dari: d, sampai: s };
	});

	const rekap = $derived(
		app.rekap({
			kelompokId: tab === 'kelompok' ? kelompokId : undefined,
			santriId: tab === 'santri' ? santriId : undefined,
			dari: rentang.dari,
			sampai: rentang.sampai
		})
	);

	const judulLingkup = $derived(
		tab === 'global'
			? 'Semua kelompok'
			: tab === 'kelompok'
				? (app.kelompokById(kelompokId)?.nama ?? '-')
				: (app.santriById(santriId)?.nama ?? '-')
	);
	const labelPeriode = $derived(
		`${tanggal(rentang.dari.toISOString(), { tahun: true })} – ${tanggal(rentang.sampai.toISOString(), { tahun: true })}`
	);
	const rataLabel = (v: number | null) =>
		v === null ? '-' : `${nilaiLabel(Math.round(v), app.pengajar.skala)} · ${angka(v, 1)}`;

	const riwayat = $derived(
		tab === 'santri'
			? app
					.setoranSantri(santriId)
					.filter((z) => z.dicatatPada >= rentang.dari.toISOString() && z.dicatatPada <= rentang.sampai.toISOString())
			: []
	);
	const targets = $derived(tab === 'santri' ? app.targetEfektif(santriId) : []);

	const cetak = () => window.print();
	const TAB: [Tab, string][] = [
		['global', 'Global'],
		['kelompok', 'Kelompok'],
		['santri', 'Santri']
	];
	const PER: [Periode, string][] = [
		['pekan', 'Pekan'],
		['bulan', 'Bulan'],
		['semester', 'Semester'],
		['custom', 'Custom']
	];
</script>

<PageHeader judul="Laporan" sub={judulLingkup}>
	{#snippet aksi()}
		<button class="btn btn-soft min-h-11 px-4 text-sm" onclick={cetak}><Icon n="printer" size={18} />PDF</button>
	{/snippet}
</PageHeader>

<div class="space-y-4 px-4 pb-8">
	<div class="no-print space-y-3">
		<div class="grid grid-cols-3 gap-1.5 rounded-2xl bg-surface-2 p-1.5" role="tablist">
			{#each TAB as [id, label] (id)}
				<button
					role="tab"
					aria-selected={tab === id}
					class="min-h-11 rounded-xl text-sm font-bold {tab === id ? 'bg-surface text-brand shadow-sm' : 'text-ink-soft'}"
					onclick={() => (tab = id)}>{label}</button
				>
			{/each}
		</div>

		{#if tab === 'kelompok'}
			<div class="no-scrollbar -mx-4 flex gap-2 overflow-x-auto px-4">
				{#each app.kelompokAktif as k (k.id)}
					<button
						class="chip min-h-10 shrink-0 px-4 text-sm {kelompokId === k.id ? 'bg-brand text-on-brand' : 'border border-line bg-surface text-ink-soft'}"
						onclick={() => (kelompokId = k.id)}>{k.nama}</button
					>
				{/each}
			</div>
		{:else if tab === 'santri'}
			<label class="sr-only" for="pilih-santri">Pilih santri</label>
			<select id="pilih-santri" class="field" bind:value={santriId}>
				{#each app.kelompokAktif as k (k.id)}
					<optgroup label={k.nama}>
						{#each app.urutanHalaqah(k.id) as s (s.id)}<option value={s.id}>{s.nama}</option>{/each}
					</optgroup>
				{/each}
			</select>
		{/if}

		<div class="grid grid-cols-4 gap-1.5">
			{#each PER as [id, label] (id)}
				<button
					class="chip min-h-10 justify-center text-sm {periode === id ? 'bg-brand text-on-brand' : 'border border-line bg-surface text-ink-soft'}"
					onclick={() => (periode = id)}>{label}</button
				>
			{/each}
		</div>
		{#if periode === 'custom'}
			<div class="grid grid-cols-2 gap-2">
				<div>
					<label class="label" for="cd">Dari</label>
					<input id="cd" type="date" class="field" bind:value={cDari} max={cSampai} />
				</div>
				<div>
					<label class="label" for="cs">Sampai</label>
					<input id="cs" type="date" class="field" bind:value={cSampai} min={cDari} />
				</div>
			</div>
		{/if}
	</div>

	<!-- Kop cetak -->
	<div class="print-only mb-2 border-b border-black pb-3">
		{#if !app.gratis}
			<p class="text-lg font-bold">{app.pengajar.lembaga || app.pengajar.nama}</p>
			<p class="text-sm">{app.pengajar.nama} · {app.pengajar.email}</p>
		{/if}
		<p class="text-xl font-bold">Laporan Setoran Tahfidz &amp; Tahsin</p>
		<p class="text-sm">{judulLingkup} · {labelPeriode}</p>
	</div>

	<p class="no-print px-1 text-xs text-ink-muted">{labelPeriode}</p>

	<!-- Angka dulu -->
	<section class="grid grid-cols-2 gap-3">
		<div class="card">
			<p class="text-xs font-semibold text-ink-muted uppercase">Total setoran</p>
			<p class="tabular mt-1 text-4xl font-extrabold">{rekap.jumlah}</p>
			<p class="mt-1 text-xs text-ink-muted">{rekap.hariAktif} hari ada setoran</p>
		</div>
		<div class="card">
			<p class="text-xs font-semibold text-ink-muted uppercase">Rata-rata nilai</p>
			<p class="mt-1 text-3xl font-extrabold text-brand">{rataLabel(rekap.rata)}</p>
			<p class="mt-1 text-xs text-ink-muted">skala {app.pengajar.skala}</p>
		</div>
		<div class="card">
			<p class="text-xs font-semibold text-ink-muted uppercase">Hafalan baru lulus</p>
			<p class="tabular mt-1 text-3xl font-extrabold">{rekap.ayatBaru}<span class="text-base font-semibold text-ink-muted"> ayat</span></p>
		</div>
		<div class="card">
			<p class="text-xs font-semibold text-ink-muted uppercase">Tahsin lulus</p>
			<p class="tabular mt-1 text-3xl font-extrabold">{rekap.halamanTahsin}<span class="text-base font-semibold text-ink-muted"> hal.</span></p>
		</div>
	</section>

	<section class="card">
		<div class="mb-3 flex items-center justify-between">
			<h2 class="font-bold">Per jenis</h2>
			{#if tab !== 'santri'}
				<span class="tabular text-sm text-ink-muted">{rekap.santriSetor} dari {rekap.santriTotal} santri setor</span>
			{/if}
		</div>
		<div class="space-y-3">
			{#each [['ziyadah', 'var(--brand)'], ['murojaah', 'var(--gold)'], ['tahsin', 'var(--ink-muted)']] as [j, warna] (j)}
				{@const n = rekap.perJenis[j as 'ziyadah' | 'murojaah' | 'tahsin']}
				<div>
					<div class="mb-1 flex justify-between text-sm">
						<span class="font-semibold">{NAMA_JENIS[j as 'ziyadah' | 'murojaah' | 'tahsin']}</span>
						<span class="tabular text-ink-soft">{n}</span>
					</div>
					<Progress persen={rekap.jumlah ? n / rekap.jumlah : 0} {warna} />
				</div>
			{/each}
		</div>
	</section>

	{#if tab === 'santri'}
		{#if targets.length}
			<section class="card space-y-4">
				<h2 class="font-bold">Progress target</h2>
				{#each targets as t (t.id)}
					{@const p = app.progressTarget(t, santriId)}
					<div>
						<div class="mb-1 flex justify-between text-sm">
							<span class="font-semibold">{t.jenis === 'ziyadah-pekan' ? 'Hafalan baru per pekan' : t.jenis === 'hafalan' ? 'Capaian hafalan' : 'Naik jilid'}</span>
							<span class="tabular font-bold">{Math.round(p.persen * 100)}%</span>
						</div>
						<Progress persen={p.persen} />
						<p class="mt-1 text-xs text-ink-muted">{p.label}</p>
					</div>
				{/each}
			</section>
		{/if}

		<section>
			<h2 class="mb-2 px-1 font-bold">Setoran pada periode ini · {riwayat.length}</h2>
			<ul class="divide-y divide-line overflow-hidden rounded-2xl border border-line bg-surface">
				{#each riwayat as z (z.id)}
					<li class="flex items-center gap-3 px-3 py-2.5 text-sm">
						<div class="min-w-0 flex-1">
							<p class="truncate font-semibold">{teksSetoran(z)}</p>
							<p class="text-xs text-ink-muted">{NAMA_JENIS[z.jenis]} · {tanggal(z.dicatatPada)}</p>
						</div>
						<span class="chip bg-surface-2">{nilaiLabel(z.nilai, app.pengajar.skala)}</span>
					</li>
				{:else}
					<li class="px-4 py-8 text-center text-ink-muted">Tidak ada setoran pada periode ini.</li>
				{/each}
			</ul>
		</section>
	{:else}
		<section>
			<h2 class="mb-2 px-1 font-bold">Per santri</h2>
			<div class="overflow-hidden rounded-2xl border border-line bg-surface">
				<table class="w-full text-sm">
					<thead class="bg-surface-2 text-xs text-ink-muted">
						<tr>
							<th class="py-2 pl-3 text-left font-semibold">Nama</th>
							<th class="px-1 py-2 text-right font-semibold">Setoran</th>
							<th class="px-1 py-2 text-right font-semibold">Ayat</th>
							<th class="py-2 pr-3 text-right font-semibold">Nilai</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-line">
						{#each rekap.perSantri as r (r.santri.id)}
							<tr class={r.jumlah === 0 ? 'text-ink-muted' : ''}>
								<td class="py-2.5 pl-3">
									<a href="/santri/{r.santri.id}" class="block leading-tight font-semibold">{r.santri.nama}</a>
									{#if r.santri.arsip}<span class="text-[10px] text-warn">diarsipkan</span>{/if}
								</td>
								<td class="tabular px-1 text-right">{r.jumlah}</td>
								<td class="tabular px-1 text-right">{r.ayat}</td>
								<td class="tabular py-2.5 pr-3 text-right">{r.rata === null ? '-' : nilaiLabel(Math.round(r.rata), app.pengajar.skala)}</td>
							</tr>
						{:else}
							<tr><td colspan="4" class="px-4 py-8 text-center text-ink-muted">Belum ada data.</td></tr>
						{/each}
					</tbody>
				</table>
			</div>
		</section>
	{/if}

	<p class="no-print px-1 text-xs text-ink-muted">
		Tombol PDF membuka dialog cetak: pilih “Simpan sebagai PDF”. {app.gratis ? 'PDF berkop tersedia di paket berbayar.' : ''}
	</p>
	<p class="print-only pt-4 text-xs">Dibuat dengan Ngajio · {tanggal(sekarang().toISOString(), { tahun: true })}</p>
</div>
