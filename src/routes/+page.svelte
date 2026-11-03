<script lang="ts">
	import { app, sekarang } from '#lib/store.svelte.ts';
	import { jam, nilaiLabel, tanggal } from '#lib/format.ts';
	import { posisiLabel } from '#lib/posisi.ts';
	import Avatar from '#lib/ui/Avatar.svelte';
	import Icon from '#lib/ui/Icon.svelte';
	import SyncChip from '#lib/ui/SyncChip.svelte';

	const hariIni = $derived(app.setoranHariIni());
	const kelompokHari = $derived(app.kelompokHariIni());
	const kelompokUtama = $derived(kelompokHari[0]);

	const stat = $derived.by(() => {
		const per = { ziyadah: 0, murojaah: 0, tahsin: 0 };
		let total = 0;
		for (const z of hariIni) {
			per[z.jenis]++;
			total += z.nilai;
		}
		return { per, rata: hariIni.length ? total / hariIni.length : null };
	});

	const sudahSetor = $derived(new Set(hariIni.map((z) => z.santriId)));
	const belum = $derived(
		kelompokHari.flatMap((k) => app.urutanHalaqah(k.id)).filter((s) => !sudahSetor.has(s.id))
	);
	const totalHari = $derived(kelompokHari.reduce((n, k) => n + app.anggota(k.id).length, 0));
	const tertinggal = $derived(app.tertinggal());

	const progressUtama = $derived.by(() => {
		if (!kelompokUtama) return { sudah: 0, total: 0 };
		const ang = app.anggota(kelompokUtama.id);
		return { sudah: ang.filter((s) => sudahSetor.has(s.id)).length, total: ang.length };
	});

	const jamTeks = $derived(jam(sekarang().toISOString()));
</script>

<div class="px-4 pt-[max(1rem,env(safe-area-inset-top))] pb-4">
	<header class="mb-5 flex items-start justify-between gap-3">
		<div class="min-w-0">
			<p class="text-sm text-ink-muted">
				{tanggal(sekarang().toISOString(), { hari: true, tahun: true })} · {jamTeks}
			</p>
			<h1 class="mt-0.5 text-[22px] leading-tight font-bold">
				Assalamu'alaikum, {app.pengajar.nama || 'Ustadz'}
			</h1>
		</div>
		<div class="flex shrink-0 items-center gap-1">
			<SyncChip />
			<a
				href="/pengaturan"
				class="flex size-11 items-center justify-center rounded-full text-ink-soft"
				aria-label="Pengaturan"><Icon n="sliders" size={22} /></a
			>
		</div>
	</header>

	{#if app.perluCadangan}
		<a
			href="/pengaturan#ekspor"
			class="mb-4 flex items-center gap-3 rounded-2xl bg-gold-tint px-4 py-3 text-sm text-warn"
		>
			<Icon n="download" size={20} class="shrink-0" />
			<span class="flex-1"
				><b>Simpan cadangan.</b> Sudah lebih dari 50 setoran baru sejak ekspor terakhir.</span
			>
			<Icon n="chevR" size={18} />
		</a>
	{/if}

	{#if kelompokUtama}
		<a
			href="/halaqah?k={kelompokUtama.id}"
			class="mb-4 flex min-h-[88px] items-center gap-4 rounded-3xl bg-brand p-4 text-on-brand shadow-sm transition active:scale-[0.99]"
		>
			<span class="flex size-14 shrink-0 items-center justify-center rounded-2xl bg-white/15">
				<Icon n="play" size={26} />
			</span>
			<span class="min-w-0 flex-1">
				<span class="block text-xs font-semibold tracking-wide uppercase opacity-80">Mode Halaqah</span>
				<span class="block truncate text-xl font-bold">{kelompokUtama.nama}</span>
				<span class="block text-sm opacity-90"
					>{progressUtama.sudah} dari {progressUtama.total} santri sudah setor</span
				>
			</span>
			<Icon n="chevR" size={24} />
		</a>
	{/if}

	<div class="space-y-4">
		<!-- Kartu 1: setoran hari ini -->
		<section class="card rise">
			<div class="flex items-start justify-between">
				<div>
					<h2 class="text-sm font-semibold text-ink-soft">Setoran hari ini</h2>
					<p class="tabular mt-1 text-5xl leading-none font-extrabold">{hariIni.length}</p>
				</div>
				{#if stat.rata !== null}
					<div class="text-right">
						<p class="text-xs text-ink-muted">Rata-rata nilai</p>
						<p class="text-2xl font-bold text-brand">
							{nilaiLabel(Math.round(stat.rata), app.pengajar.skala)}
						</p>
					</div>
				{/if}
			</div>
			<div class="mt-4 flex flex-wrap gap-2">
				<span class="chip bg-brand-tint text-brand-strong">Hafalan baru {stat.per.ziyadah}</span>
				<span class="chip bg-gold-tint text-warn">Murajaah {stat.per.murojaah}</span>
				<span class="chip bg-surface-2 text-ink-soft">Tahsin {stat.per.tahsin}</span>
			</div>
		</section>

		<!-- Kartu 2: belum setor -->
		<section class="card rise p-0">
			<div class="flex items-center justify-between px-4 pt-4 pb-2">
				<h2 class="font-bold">Belum setor hari ini</h2>
				<span class="chip bg-surface-2 text-ink-soft tabular">{belum.length} dari {totalHari}</span>
			</div>
			{#if belum.length === 0}
				<p class="flex items-center gap-2 px-4 pb-5 text-ok">
					<Icon n="check" size={20} /> Semua santri hari ini sudah setor. Alhamdulillah.
				</p>
			{:else}
				<ul class="divide-y divide-line">
					{#each belum.slice(0, 5) as s (s.id)}
						<li>
							<a
								href="/setoran?santri={s.id}&dari=beranda"
								class="flex min-h-14 items-center gap-3 px-4 py-2 active:bg-surface-2"
							>
								<Avatar nama={s.nama} size={40} />
								<span class="min-w-0 flex-1">
									<span class="block truncate font-semibold">{s.nama}</span>
									<span class="block truncate text-xs text-ink-muted">{posisiLabel(s.id)}</span>
								</span>
								<Icon n="plus" size={20} class="text-brand" />
							</a>
						</li>
					{/each}
				</ul>
				{#if belum.length > 5 && kelompokUtama}
					<a
						href="/halaqah?k={kelompokUtama.id}"
						class="flex min-h-12 items-center justify-center gap-1 border-t border-line text-sm font-semibold text-brand"
						>Lihat {belum.length - 5} lainnya di Mode Halaqah <Icon n="chevR" size={16} /></a
					>
				{/if}
			{/if}
		</section>

		<!-- Kartu 3: tertinggal target -->
		<section class="card rise p-0">
			<div class="flex items-center justify-between px-4 pt-4 pb-2">
				<h2 class="font-bold">Tertinggal dari target</h2>
				<span class="chip bg-gold-tint text-warn tabular">{tertinggal.length} santri</span>
			</div>
			{#if tertinggal.length === 0}
				<p class="flex items-center gap-2 px-4 pb-5 text-ok">
					<Icon n="check" size={20} /> Semua santri sesuai target.
				</p>
			{:else}
				<ul class="divide-y divide-line">
					{#each tertinggal.slice(0, 5) as t (t.santri.id)}
						<li>
							<a
								href="/santri/{t.santri.id}"
								class="flex min-h-14 items-center gap-3 px-4 py-2 active:bg-surface-2"
							>
								<Avatar nama={t.santri.nama} size={40} />
								<span class="min-w-0 flex-1">
									<span class="block truncate font-semibold">{t.santri.nama}</span>
									<span class="block truncate text-xs text-warn">{t.alasan}</span>
								</span>
								<Icon n="chevR" size={18} class="text-ink-muted" />
							</a>
						</li>
					{/each}
				</ul>
				{#if tertinggal.length > 5}
					<a
						href="/santri?filter=tertinggal"
						class="flex min-h-12 items-center justify-center gap-1 border-t border-line text-sm font-semibold text-brand"
						>Lihat semua {tertinggal.length} santri <Icon n="chevR" size={16} /></a
					>
				{/if}
			{/if}
		</section>
	</div>
</div>
