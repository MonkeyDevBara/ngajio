<script lang="ts">
	import { goto } from '$app/navigation';
	import { app } from '#lib/store.svelte.ts';
	import { nilaiLabel } from '#lib/format.ts';
	import { parseDaftarNama } from '#lib/parse.ts';
	import type { SkalaJenis } from '#lib/types.ts';
	import Icon from '#lib/ui/Icon.svelte';

	let langkah = $state(1);
	let nama = $state(app.pengajar.nama);
	let lembaga = $state(app.pengajar.lembaga);
	let metodeId = $state(app.metode[0]?.id ?? 'ummi');
	let skala = $state<SkalaJenis>('huruf');
	let kelompok = $state('Halaqah 1');
	let teks = $state('');
	let izin = $state(false);

	const hasil = $derived(parseDaftarNama(teks));
	const baru = $derived(hasil.filter((h) => h.status === 'ok'));
	const lewat = $derived(hasil.length - baru.length);

	const SKALA: [SkalaJenis, string, string][] = [
		['huruf', 'Huruf', 'A sampai E'],
		['bintang', 'Bintang', '★ 1 sampai 5'],
		['predikat', 'Predikat', 'Mumtaz, Baik…'],
		['angka', 'Angka', '1–100 (20, 40, 60, 80, 100)']
	];

	const bisaLanjut = $derived(
		langkah === 1
			? nama.trim().length > 1
			: langkah === 2
				? true
				: kelompok.trim().length > 0 && (baru.length === 0 || izin)
	);

	function selesai() {
		const kid = app.selesaikanSetup({
			nama: nama.trim(),
			lembaga: lembaga.trim(),
			metodeId,
			skala,
			kelompok: kelompok.trim(),
			namaSantri: baru.map((b) => b.nama)
		});
		if (baru.length) {
			app.tampilToast('Siap! Ketuk nama santri untuk setoran pertama.');
			goto(`/halaqah?k=${kid}`, { replace: true });
		} else {
			goto('/santri?tambah=1', { replace: true });
		}
	}
	function maju() {
		if (langkah < 3) langkah++;
		else selesai();
	}

	const JUDUL = ['Kenalan dulu', 'Atur cara menilai', 'Masukkan santri'];
	const WAKTU = ['sekitar 1 menit', 'sekitar 1 menit', 'tempel sekali, selesai'];
</script>

<main class="mx-auto flex min-h-dvh max-w-[480px] flex-col px-5 pt-[max(1rem,env(safe-area-inset-top))] pb-6">
	<header class="mb-6 flex items-center gap-3">
		{#if langkah > 1}
			<button
				class="flex size-11 items-center justify-center rounded-full text-ink-soft"
				onclick={() => langkah--}
				aria-label="Kembali"><Icon n="chevL" size={24} /></button
			>
		{/if}
		<div class="flex-1">
			<div class="mb-2 flex gap-1.5" aria-hidden="true">
				{#each [1, 2, 3] as n (n)}
					<span class="h-1.5 flex-1 rounded-full {n <= langkah ? 'bg-brand' : 'bg-surface-2'}"></span>
				{/each}
			</div>
			<p class="text-xs font-semibold text-ink-muted">Langkah {langkah} dari 3 · {WAKTU[langkah - 1]}</p>
		</div>
	</header>

	<h1 class="mb-5 text-[26px] leading-tight font-extrabold">{JUDUL[langkah - 1]}</h1>

	<div class="flex-1 space-y-5">
		{#if langkah === 1}
			<div>
				<label class="label" for="nm">Nama Anda</label>
				<input id="nm" class="field min-h-14" bind:value={nama} placeholder="Ustadz Bara" autocomplete="name" />
			</div>
			<div>
				<label class="label" for="lb">Lembaga <span class="font-normal text-ink-muted">(opsional)</span></label>
				<input
					id="lb"
					class="field min-h-14"
					bind:value={lembaga}
					placeholder="TPQ, Rumah Tahfidz, atau Yayasan"
					autocomplete="organization"
				/>
			</div>
		{:else if langkah === 2}
			<div>
				<span class="label">Metode tahsin</span>
				<div class="grid grid-cols-2 gap-2">
					{#each app.metode as m (m.id)}
						<button
							class="min-h-14 rounded-2xl border px-3 text-left {metodeId === m.id
								? 'border-brand bg-brand-tint'
								: 'border-line bg-surface'}"
							onclick={() => (metodeId = m.id)}
						>
							<span class="block font-bold">{m.nama}</span>
							<span class="block text-xs text-ink-muted">{m.level.length} level</span>
						</button>
					{/each}
				</div>
				<p class="mt-2 text-xs text-ink-muted">
					Bisa diubah kapan saja di Pengaturan, termasuk membuat metode sendiri.
				</p>
			</div>
			<div>
				<span class="label">Cara menilai</span>
				<div class="space-y-2">
					{#each SKALA as [id, l, k] (id)}
						<button
							class="flex min-h-16 w-full items-center justify-between rounded-2xl border px-4 text-left {skala === id
								? 'border-brand bg-brand-tint'
								: 'border-line bg-surface'}"
							onclick={() => (skala = id)}
						>
							<span>
								<span class="block font-bold">{l}</span>
								<span class="block text-xs text-ink-muted">{k}</span>
							</span>
							<span class="text-lg font-bold text-brand">{nilaiLabel(4, id)}</span>
						</button>
					{/each}
				</div>
			</div>
		{:else}
			<div>
				<label class="label" for="kg">Nama kelompok</label>
				<input id="kg" class="field min-h-14" bind:value={kelompok} autocomplete="off" />
			</div>
			<div>
				<label class="label" for="ds">Tempel daftar nama santri</label>
				<textarea
					id="ds"
					class="field min-h-40 py-3"
					rows="7"
					bind:value={teks}
					placeholder="bagas,hudan,yusuf,lukman,radja"
				></textarea>
				<p class="mt-1.5 text-xs text-ink-muted">
					Pisahkan dengan koma, atau satu nama per baris. Bisa salin dari WhatsApp atau Excel, nomor urut otomatis dibuang.
				</p>
				{#if hasil.length}
					<p class="mt-2 flex items-center gap-1.5 text-sm font-semibold text-brand-strong">
						<Icon n="check" size={16} />{baru.length} santri siap ditambahkan{#if lewat}<span
								class="font-normal text-warn"> · {lewat} dilewati (duplikat/kosong)</span
							>{/if}
					</p>
				{/if}
			</div>
			{#if baru.length}
				<label class="flex items-start gap-3 rounded-2xl border border-line bg-surface p-3.5 text-sm">
					<input type="checkbox" class="mt-0.5 size-5 shrink-0 accent-[var(--brand)]" bind:checked={izin} />
					<span class="text-ink-soft"
						>Saya sudah mendapat izin wali santri untuk mencatat nama dan hafalan anak mereka di aplikasi ini.</span
					>
				</label>
			{/if}
		{/if}
	</div>

	<button class="btn btn-primary mt-6 min-h-14 w-full text-base" disabled={!bisaLanjut} onclick={maju}>
		{#if langkah < 3}Lanjut <Icon n="arrowR" size={20} />{:else if baru.length}Mulai catat setoran
			<Icon n="arrowR" size={20} />{:else}Selesai, tambah santri nanti{/if}
	</button>
</main>
