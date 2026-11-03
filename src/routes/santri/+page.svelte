<script lang="ts">
	import { page } from '$app/state';
	import { app, BATAS_GRATIS } from '#lib/store.svelte.ts';
	import { relatif } from '#lib/format.ts';
	import { posisiLabel } from '#lib/posisi.ts';
	import { parseDaftarNama } from '#lib/parse.ts';
	import Avatar from '#lib/ui/Avatar.svelte';
	import Icon from '#lib/ui/Icon.svelte';
	import Progress from '#lib/ui/Progress.svelte';
	import PageHeader from '#lib/ui/PageHeader.svelte';
	import Sheet from '#lib/ui/Sheet.svelte';

	let cari = $state('');
	let filter = $state(page.url.searchParams.get('filter') ?? 'semua');
	let bukaTambah = $state(page.url.searchParams.get('tambah') === '1');
	let mode = $state<'satu' | 'tempel'>('tempel');

	const tertinggal = $derived(new Map(app.tertinggal().map((t) => [t.santri.id, t.alasan])));

	const daftar = $derived.by(() => {
		const f = cari.trim().toLowerCase();
		let src = filter === 'arsip' ? app.santri.filter((s) => s.arsip) : app.santriAktif;
		if (filter === 'tertinggal') src = src.filter((s) => tertinggal.has(s.id));
		else if (filter.startsWith('k:')) src = src.filter((s) => s.kelompokId === filter.slice(2));
		if (f) src = src.filter((s) => s.nama.toLowerCase().includes(f));
		return [...src].sort((a, b) => a.nama.localeCompare(b.nama));
	});

	function terakhir(id: string) {
		const z = app.setoranSantri(id)[0];
		return z ? relatif(z.dicatatPada) : 'Belum setor';
	}

	/* --- Form tambah --- */
	let namaSatu = $state('');
	let waSatu = $state('');
	let teks = $state('');
	let kelompokBaru = $state(app.kelompokAktif[0]?.id ?? '');
	let galat = $state('');

	const hasil = $derived(
		parseDaftarNama(teks, new Set(app.santriAktif.map((s) => s.nama.trim().toLowerCase())))
	);
	const baru = $derived(hasil.filter((h) => h.status === 'ok'));

	function tambah() {
		galat = '';
		const daftarBaru =
			mode === 'satu'
				? namaSatu.trim()
					? [{ nama: namaSatu.trim().replace(/\s+/g, ' '), kelompokId: kelompokBaru, waWali: waSatu.trim() || undefined }]
					: []
				: baru.map((b) => ({ nama: b.nama, kelompokId: kelompokBaru }));
		if (!daftarBaru.length || !kelompokBaru) return;
		const r = app.tambahSantri(daftarBaru);
		if (!r.ok) {
			galat = r.pesan ?? 'Gagal menambah santri.';
			return;
		}
		app.tampilToast(`${r.jumlah} santri ditambahkan`);
		namaSatu = waSatu = teks = '';
		bukaTambah = false;
	}

	const jumlahSiap = $derived(mode === 'satu' ? (namaSatu.trim() ? 1 : 0) : baru.length);
</script>

<PageHeader
	judul="Santri"
	sub={app.gratis
		? `${app.santriAktif.length} dari ${BATAS_GRATIS} santri (paket gratis)`
		: `${app.santriAktif.length} santri aktif`}
>
	{#snippet aksi()}
		<button class="btn btn-primary min-h-11 px-4 text-sm" onclick={() => (bukaTambah = true)}>
			<Icon n="plus" size={18} />Tambah
		</button>
	{/snippet}
</PageHeader>

<div class="px-4 pb-6">
	<div class="relative mb-3">
		<Icon n="search" size={18} class="absolute top-1/2 left-3.5 -translate-y-1/2 text-ink-muted" />
		<input class="field pl-10" placeholder="Cari nama santri" bind:value={cari} autocomplete="off" />
	</div>

	<div class="no-scrollbar -mx-4 mb-4 flex gap-2 overflow-x-auto px-4">
		{#snippet chip(id: string, label: string)}
			<button
				type="button"
				class="chip min-h-10 shrink-0 px-4 text-sm {filter === id
					? 'bg-brand text-on-brand'
					: 'border border-line bg-surface text-ink-soft'}"
				onclick={() => (filter = id)}>{label}</button
			>
		{/snippet}
		{@render chip('semua', 'Semua')}
		{#each app.kelompokAktif as k (k.id)}
			{@render chip('k:' + k.id, k.nama)}
		{/each}
		{@render chip('tertinggal', `Tertinggal · ${tertinggal.size}`)}
		{@render chip('arsip', 'Arsip')}
	</div>

	{#if daftar.length === 0}
		<div class="card py-10 text-center">
			<p class="font-semibold">
				{app.santriAktif.length === 0 ? 'Belum ada santri.' : 'Tidak ada santri yang cocok.'}
			</p>
			{#if app.santriAktif.length === 0}
				<p class="mt-1 text-sm text-ink-muted">Tempel daftar nama sekaligus, selesai dalam hitungan detik.</p>
				<button class="btn btn-primary mt-4" onclick={() => (bukaTambah = true)}>Tambah santri</button>
			{/if}
		</div>
	{:else}
		<ul class="space-y-2">
			{#each daftar as s (s.id)}
				<li>
					<a
						href="/santri/{s.id}"
						class="flex min-h-[64px] items-center gap-3 rounded-2xl border border-line bg-surface px-3 transition active:scale-[0.99] {s.arsip
							? 'opacity-70'
							: ''}"
					>
						<Avatar nama={s.nama} size={44} />
						<span class="min-w-0 flex-1">
							<span class="block truncate font-bold">{s.nama}</span>
							<span class="block truncate text-xs text-ink-muted"
								>{app.kelompokById(s.kelompokId)?.nama} · {posisiLabel(s.id).replace('Lanjut ', '')}</span
							>
							{#if tertinggal.has(s.id) && !s.arsip}
								<span class="block truncate text-xs text-warn">{tertinggal.get(s.id)}</span>
							{/if}
							{#if !s.arsip}
								{@const r = app.ringkasTarget(s.id)}
								{#if r}
									<span class="mt-1.5 flex items-center gap-2">
										<span class="flex-1"><Progress persen={r.persen} warna={r.persen >= 1 ? 'var(--ok)' : 'var(--brand)'} /></span>
										<span class="tabular w-9 text-right text-xs font-bold text-ink-soft">{Math.round(r.persen * 100)}%</span>
									</span>
								{/if}
							{/if}
						</span>
						<span class="text-right text-xs text-ink-muted">{terakhir(s.id)}</span>
						<Icon n="chevR" size={18} class="text-ink-muted" />
					</a>
				</li>
			{/each}
		</ul>
	{/if}
</div>

<Sheet bind:open={bukaTambah} judul="Tambah santri" tinggi="penuh">
	<div class="mb-4 grid grid-cols-2 gap-1.5 rounded-2xl bg-surface-2 p-1.5">
		{#each [['tempel', 'Tempel banyak'], ['satu', 'Satu santri']] as [id, label] (id)}
			<button
				type="button"
				class="min-h-11 rounded-xl text-sm font-bold {mode === id ? 'bg-surface text-brand shadow-sm' : 'text-ink-soft'}"
				onclick={() => (mode = id as 'satu' | 'tempel')}>{label}</button
			>
		{/each}
	</div>

	{#if app.gratis}
		<p class="mb-4 rounded-xl bg-surface-2 px-3 py-2 text-sm text-ink-soft">
			Slot paket gratis: <b class="tabular">{app.santriAktif.length}</b> dari {BATAS_GRATIS} terpakai.
		</p>
	{/if}

	<div class="mb-4">
		<span class="label">Kelompok</span>
		{#if app.kelompokAktif.length}
			<div class="flex flex-wrap gap-2">
				{#each app.kelompokAktif as k (k.id)}
					<button
						type="button"
						class="chip min-h-10 px-4 text-sm {kelompokBaru === k.id
							? 'bg-brand text-on-brand'
							: 'border border-line bg-surface text-ink-soft'}"
						onclick={() => (kelompokBaru = k.id)}>{k.nama}</button
					>
				{/each}
			</div>
		{:else}
			<p class="text-sm text-warn">Buat kelompok dulu di menu Kelompok.</p>
		{/if}
	</div>

	{#if mode === 'tempel'}
		<label class="label" for="tempel">Tempel daftar nama (satu nama per baris)</label>
		<textarea
			id="tempel"
			class="field min-h-36 py-3"
			rows="6"
			placeholder="bagas,hudan,yusuf,lukman,radja"
			bind:value={teks}
		></textarea>
		<p class="mt-1.5 text-xs text-ink-muted">Nomor urut dan tanda "-" di depan nama otomatis dibuang.</p>

		{#if hasil.length}
			<ul class="mt-3 max-h-56 divide-y divide-line overflow-y-auto rounded-xl border border-line bg-surface">
				{#each hasil as h, i (i)}
					<li class="flex min-h-11 items-center gap-2 px-3 text-sm">
						<span class="min-w-0 flex-1 truncate {h.status !== 'ok' ? 'text-ink-muted line-through' : ''}">{h.nama}</span>
						{#if h.status === 'duplikat'}<span class="chip bg-gold-tint text-warn">Duplikat</span>
						{:else if h.status === 'kosong'}<span class="chip bg-danger-tint text-danger">Kosong</span>
						{:else}<Icon n="check" size={16} class="text-ok" />{/if}
					</li>
				{/each}
			</ul>
			{#if hasil.length !== baru.length}
				<p class="mt-2 text-xs text-warn">{hasil.length - baru.length} baris dilewati (duplikat atau kosong).</p>
			{/if}
		{/if}
	{:else}
		<label class="label" for="nama">Nama santri</label>
		<input id="nama" class="field" bind:value={namaSatu} placeholder="Nama lengkap" autocomplete="off" />
		<label class="label mt-4" for="wa">Nomor WA wali <span class="font-normal text-ink-muted">(opsional)</span></label>
		<input id="wa" class="field" bind:value={waSatu} inputmode="tel" placeholder="08xxxxxxxxxx" autocomplete="off" />
	{/if}

	{#if galat}
		<div class="mt-4 flex gap-2 rounded-xl bg-danger-tint p-3 text-sm text-danger" role="alert">
			<Icon n="alert" size={18} class="mt-0.5 shrink-0" /><span>{galat}</span>
		</div>
	{/if}

	<button
		class="btn btn-primary mt-5 min-h-14 w-full text-base"
		disabled={jumlahSiap === 0 || !kelompokBaru}
		onclick={tambah}
	>
		{jumlahSiap ? `Tambah ${jumlahSiap} santri` : 'Tambah santri'}
	</button>
</Sheet>
