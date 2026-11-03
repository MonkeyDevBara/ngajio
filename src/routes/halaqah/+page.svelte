<script lang="ts">
	import { page } from '$app/state';
	import { app } from '#lib/store.svelte.ts';
	import { nilaiLabel, jam } from '#lib/format.ts';
	import { NAMA_JENIS, posisiLabel } from '#lib/posisi.ts';
	import Avatar from '#lib/ui/Avatar.svelte';
	import Icon from '#lib/ui/Icon.svelte';
	import PageHeader from '#lib/ui/PageHeader.svelte';
	import Progress from '#lib/ui/Progress.svelte';

	const kelompokId = $derived(
		page.url.searchParams.get('k') ?? app.kelompokHariIni()[0]?.id ?? app.kelompokAktif[0]?.id ?? ''
	);
	const kelompok = $derived(app.kelompokById(kelompokId));
	const daftar = $derived(app.urutanHalaqah(kelompokId));
	const hariIni = $derived(app.setoranHariIni());

	// Satu santri bisa setor lebih dari sekali; tampilkan setoran terbaru.
	const terakhir = $derived.by(() => {
		const m = new Map<string, (typeof hariIni)[number]>();
		for (const z of hariIni) {
			const lama = m.get(z.santriId);
			if (!lama || z.dicatatPada > lama.dicatatPada) m.set(z.santriId, z);
		}
		return m;
	});
	const sudah = $derived(daftar.filter((s) => terakhir.has(s.id)).length);
	const berikutnya = $derived(daftar.find((s) => !terakhir.has(s.id)));

	let atur = $state(false);
	const tujuan = (id: string) => `/setoran?santri=${id}&dari=halaqah&k=${kelompokId}`;
</script>

<PageHeader judul="Mode Halaqah" sub={kelompok?.nama} kembali="/">
	{#snippet aksi()}
		<button
			type="button"
			class="btn min-h-11 px-3 text-sm {atur ? 'btn-primary' : 'btn-soft'}"
			onclick={() => (atur = !atur)}
			aria-pressed={atur}
		>
			{#if atur}<Icon n="check" size={18} />Selesai{:else}<Icon n="repeat" size={18} />Urutan{/if}
		</button>
	{/snippet}
</PageHeader>

<div class="px-4 pb-28">
	{#if app.kelompokAktif.length > 1}
		<div class="no-scrollbar -mx-4 mb-3 flex gap-2 overflow-x-auto px-4">
			{#each app.kelompokAktif as k (k.id)}
				<a
					href="/halaqah?k={k.id}"
					data-sveltekit-replacestate
					class="chip min-h-10 shrink-0 px-4 text-sm {k.id === kelompokId
						? 'bg-brand text-on-brand'
						: 'border border-line bg-surface text-ink-soft'}">{k.nama}</a
				>
			{/each}
		</div>
	{/if}

	<div class="mb-4 rounded-2xl border border-line bg-surface p-4">
		<div class="mb-2 flex items-baseline justify-between">
			<span class="text-sm font-semibold text-ink-soft">Sudah setor</span>
			<span class="tabular text-lg font-bold">{sudah}<span class="text-ink-muted"> / {daftar.length}</span></span>
		</div>
		<Progress persen={daftar.length ? sudah / daftar.length : 0} />
		{#if kelompok?.jadwal}<p class="mt-2 text-xs text-ink-muted">{kelompok.jadwal}</p>{/if}
	</div>

	{#if daftar.length === 0}
		<div class="card py-10 text-center">
			<p class="font-semibold">Kelompok ini belum punya santri.</p>
			<a href="/santri?tambah=1" class="btn btn-primary mt-4">Tambah santri</a>
		</div>
	{:else}
		<ul class="space-y-2">
			{#each daftar as s, i (s.id)}
				{@const z = terakhir.get(s.id)}
				<li class="flex items-stretch gap-2">
					<a
						href={tujuan(s.id)}
						class="flex min-h-[64px] min-w-0 flex-1 items-center gap-3 rounded-2xl border px-3 transition active:scale-[0.99] {z
							? 'border-brand/30 bg-brand-tint'
							: 'border-line bg-surface'}"
					>
						<span class="relative">
							<Avatar nama={s.nama} size={46} />
							{#if z}
								<span
									class="absolute -right-1 -bottom-1 flex size-5 items-center justify-center rounded-full bg-brand text-on-brand ring-2 ring-paper"
									><Icon n="check" size={12} /></span
								>
							{/if}
						</span>
						<span class="min-w-0 flex-1">
							<span class="block truncate text-lg leading-tight font-bold">{s.nama}</span>
							<span class="block truncate text-sm text-ink-muted">
								{#if z}
									{NAMA_JENIS[z.jenis]} · {jam(z.dicatatPada)}
								{:else}
									{posisiLabel(s.id)}
								{/if}
							</span>
						</span>
						{#if z}
							<span class="chip bg-surface text-brand-strong">{nilaiLabel(z.nilai, app.pengajar.skala)}</span>
						{:else}
							<Icon n="chevR" size={22} class="text-ink-muted" />
						{/if}
					</a>
					{#if atur}
						<div class="flex w-12 shrink-0 flex-col gap-1">
							<button
								type="button"
								class="flex flex-1 items-center justify-center rounded-xl bg-surface-2 text-ink disabled:opacity-30"
								disabled={i === 0}
								aria-label="Naikkan {s.nama}"
								onclick={() => app.geser(kelompokId, s.id, -1)}><Icon n="up" size={18} /></button
							>
							<button
								type="button"
								class="flex flex-1 items-center justify-center rounded-xl bg-surface-2 text-ink disabled:opacity-30"
								disabled={i === daftar.length - 1}
								aria-label="Turunkan {s.nama}"
								onclick={() => app.geser(kelompokId, s.id, 1)}><Icon n="down" size={18} /></button
							>
						</div>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
</div>

{#if berikutnya && !atur}
	<div class="no-print safe-b fixed inset-x-0 bottom-0 z-30 bg-gradient-to-t from-paper via-paper to-transparent pt-6">
		<div class="mx-auto max-w-[480px] px-4 pb-4">
			<a href={tujuan(berikutnya.id)} class="btn btn-primary min-h-14 w-full text-base shadow-lg">
				Berikutnya: {berikutnya.nama}
				<Icon n="arrowR" size={20} />
			</a>
		</div>
	</div>
{:else if !berikutnya && daftar.length > 0}
	<div class="no-print safe-b fixed inset-x-0 bottom-0 z-30 bg-gradient-to-t from-paper via-paper to-transparent pt-6">
		<div class="mx-auto max-w-[480px] px-4 pb-4">
			<a href="/" class="btn btn-soft min-h-14 w-full text-base">
				<Icon n="check" size={20} /> Semua sudah setor, selesai
			</a>
		</div>
	</div>
{/if}
