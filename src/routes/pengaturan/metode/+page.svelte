<script lang="ts">
	import { app } from '#lib/store.svelte.ts';
	import type { Metode } from '#lib/types.ts';
	import Icon from '#lib/ui/Icon.svelte';
	import PageHeader from '#lib/ui/PageHeader.svelte';
	import Sheet from '#lib/ui/Sheet.svelte';

	let buka = $state(false);
	let baru = $state(false);
	let edit = $state<Metode | null>(null);
	let galat = $state('');

	function mulai(m: Metode) {
		edit = structuredClone($state.snapshot(m));
		baru = false;
		galat = '';
		buka = true;
	}
	function buatBaru() {
		edit = {
			id: app.buatId(),
			nama: '',
			template: false,
			level: [{ id: app.buatId(), nama: 'Level 1', jumlahHalaman: 30 }]
		};
		baru = true;
		galat = '';
		buka = true;
	}
	function tambahLevel() {
		edit?.level.push({ id: app.buatId(), nama: `Level ${edit.level.length + 1}`, jumlahHalaman: 30 });
	}
	function geser(i: number, d: -1 | 1) {
		if (!edit) return;
		const j = i + d;
		if (j < 0 || j >= edit.level.length) return;
		[edit.level[i], edit.level[j]] = [edit.level[j], edit.level[i]];
	}
	function simpan() {
		if (!edit) return;
		if (!edit.nama.trim()) return (galat = 'Nama metode wajib diisi.');
		if (!edit.level.length) return (galat = 'Minimal satu level.');
		if (edit.level.some((l) => !l.nama.trim())) return (galat = 'Semua level perlu nama.');
		edit.nama = edit.nama.trim();
		app.simpanMetode($state.snapshot(edit) as Metode);
		buka = false;
		app.tampilToast(`Metode ${edit.nama} disimpan`);
	}
	function hapus() {
		if (!edit) return;
		if (!app.hapusMetode(edit.id)) {
			galat = 'Metode ini masih dipakai santri atau kelompok, atau satu-satunya metode.';
			return;
		}
		buka = false;
		app.tampilToast('Metode dihapus');
	}
	const totalHal = (m: Metode) => m.level.reduce((a, l) => a + l.jumlahHalaman, 0);
</script>

<PageHeader judul="Metode tahsin" sub="Level dan jumlah halaman" kembali="/pengaturan">
	{#snippet aksi()}
		<button class="btn btn-primary min-h-11 px-4 text-sm" onclick={buatBaru}><Icon n="plus" size={18} />Baru</button>
	{/snippet}
</PageHeader>

<div class="space-y-3 px-4 pb-8">
	<p class="px-1 text-sm text-ink-muted">
		Metode bawaan hanya menyimpan nama level dan jumlah halaman. Sesuaikan dengan kondisi lembaga Anda.
	</p>
	{#each app.metode as m (m.id)}
		<div class="card p-0">
			<button class="flex min-h-[72px] w-full items-center gap-3 px-4 text-left" onclick={() => mulai(m)}>
				<span class="min-w-0 flex-1">
					<span class="flex items-center gap-2">
						<span class="truncate text-lg font-bold">{m.nama}</span>
						{#if m.template}<span class="chip bg-surface-2 text-ink-soft">Template</span>{/if}
						{#if app.pengajar.metodeId === m.id}<span class="chip bg-brand-tint text-brand-strong">Utama</span>{/if}
					</span>
					<span class="block text-sm text-ink-muted">{m.level.length} level · {totalHal(m)} halaman</span>
				</span>
				<Icon n="chevR" class="text-ink-muted" />
			</button>
			{#if app.pengajar.metodeId !== m.id}
				<button class="flex min-h-12 w-full items-center justify-center border-t border-line text-sm font-semibold text-brand" onclick={() => app.ubahPengajar({ metodeId: m.id })}
					>Jadikan metode utama</button
				>
			{/if}
		</div>
	{/each}
</div>

<Sheet bind:open={buka} judul={baru ? 'Metode baru' : 'Ubah metode'} tinggi="penuh">
	{#if edit}
		<label class="label" for="mn">Nama metode</label>
		<input id="mn" class="field" bind:value={edit.nama} placeholder="Contoh: Metode Pesantren" autocomplete="off" />

		<div class="mt-5 mb-2 flex items-center justify-between">
			<span class="text-sm font-bold text-ink-soft">Level · {edit.level.length}</span>
			<button class="btn btn-soft min-h-10 px-3 text-sm" onclick={tambahLevel}><Icon n="plus" size={16} />Level</button>
		</div>
		<ul class="space-y-2">
			{#each edit.level as lv, i (lv.id)}
				<li class="rounded-2xl border border-line bg-surface p-3">
					<div class="flex items-center gap-2">
						<span class="tabular w-6 text-center text-sm font-bold text-ink-muted">{i + 1}</span>
						<input class="field min-h-11 flex-1" bind:value={lv.nama} aria-label="Nama level {i + 1}" autocomplete="off" />
					</div>
					<div class="mt-2 flex items-center justify-between gap-2">
						<div class="flex items-center gap-1 rounded-xl bg-surface-2 p-1">
							<button class="flex size-10 items-center justify-center rounded-lg text-xl disabled:opacity-30" disabled={lv.jumlahHalaman <= 1} aria-label="Kurangi halaman" onclick={() => (lv.jumlahHalaman -= 1)}>−</button>
							<span class="tabular min-w-20 text-center text-sm font-bold">{lv.jumlahHalaman} hal.</span>
							<button class="flex size-10 items-center justify-center rounded-lg text-xl" aria-label="Tambah halaman" onclick={() => (lv.jumlahHalaman += 1)}>+</button>
						</div>
						<div class="flex">
							<button class="flex size-10 items-center justify-center rounded-lg text-ink-soft disabled:opacity-30" disabled={i === 0} aria-label="Naikkan" onclick={() => geser(i, -1)}><Icon n="up" size={18} /></button>
							<button class="flex size-10 items-center justify-center rounded-lg text-ink-soft disabled:opacity-30" disabled={i === edit.level.length - 1} aria-label="Turunkan" onclick={() => geser(i, 1)}><Icon n="down" size={18} /></button>
							<button class="flex size-10 items-center justify-center rounded-lg text-danger disabled:opacity-30" disabled={edit.level.length <= 1} aria-label="Hapus level" onclick={() => edit?.level.splice(i, 1)}><Icon n="trash" size={18} /></button>
						</div>
					</div>
				</li>
			{/each}
		</ul>

		{#if galat}<p class="mt-3 text-sm text-danger" role="alert">{galat}</p>{/if}
		<button class="btn btn-primary mt-5 min-h-14 w-full" onclick={simpan}>Simpan metode</button>
		{#if !baru && !edit.template}
			<button class="btn btn-danger mt-3 w-full" onclick={hapus}><Icon n="trash" size={18} />Hapus metode</button>
		{/if}
	{/if}
</Sheet>
