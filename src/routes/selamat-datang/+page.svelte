<script lang="ts">
	import { goto } from '$app/navigation';
	import { app } from '#lib/store.svelte.ts';
	import Icon from '#lib/ui/Icon.svelte';
	import Logo from '#lib/ui/Logo.svelte';

	let layar = $state(1);

	function mulai() {
		app.selesaikanIntro();
		goto('/masuk', { replace: true });
	}

	const LANGKAH = [
		{
			ikon: 'users',
			judul: 'Tulis nama santri',
			isi: 'Tempel daftar nama dari WhatsApp. Cukup sekali di awal.'
		},
		{
			ikon: 'plus',
			judul: 'Catat setoran',
			isi: 'Ketuk nama santri, pilih surat dan ayat, lalu beri nilai. Posisi hafalan terisi sendiri.'
		},
		{
			ikon: 'chart',
			judul: 'Lihat perkembangan',
			isi: 'Bar kemajuan tiap santri dan laporan siap cetak untuk wali santri.'
		}
	];
</script>

<main class="mx-auto flex min-h-dvh max-w-[480px] flex-col px-6 pt-[max(1rem,env(safe-area-inset-top))] pb-6">
	<header class="flex h-12 items-center justify-between">
		{#if layar > 1}
			<button
				class="flex size-11 items-center justify-center rounded-full text-ink-soft"
				onclick={() => (layar = 1)}
				aria-label="Kembali"><Icon n="chevL" size={24} /></button
			>
		{:else}
			<span></span>
		{/if}
		<button class="min-h-11 px-2 text-sm font-semibold text-ink-muted" onclick={mulai}>Lewati</button>
	</header>

	{#if layar === 1}
		<section class="rise flex flex-1 flex-col justify-center py-8" aria-labelledby="j1">
			<Logo size={88} />
			<p class="mt-8 text-lg font-semibold text-brand-strong">Assalamu'alaikum warahmatullahi wabarakatuh</p>
			<h1 id="j1" class="mt-2 text-[32px] leading-tight font-extrabold">
				Selamat datang di ngaji<span class="text-gold">o</span>
			</h1>
			<p class="mt-4 text-lg text-ink-soft">
				Terima kasih sudah mengajarkan Al-Qur'an kepada anak-anak kita. Ngajio membantu Anda mencatat setoran
				hafalan dan bacaan dengan mudah, supaya waktu Anda untuk membimbing santri lebih banyak.
			</p>

			<figure class="mt-8 rounded-2xl border border-line bg-gold-tint p-4">
				<blockquote lang="ar" dir="rtl" class="text-center text-2xl leading-loose text-ink">
					خَيْرُكُمْ مَنْ تَعَلَّمَ الْقُرْآنَ وَعَلَّمَهُ
				</blockquote>
				<figcaption class="mt-2 text-center text-sm text-ink-soft">
					"Sebaik-baik kalian adalah yang belajar Al-Qur'an dan mengajarkannya." (HR. Bukhari)
				</figcaption>
			</figure>
		</section>
	{:else}
		<section class="rise flex-1 py-4" aria-labelledby="j2">
			<h1 id="j2" class="text-[28px] leading-tight font-extrabold">Apa itu Ngajio?</h1>
			<p class="mt-3 text-lg text-ink-soft">
				Buku catatan setoran di HP Anda. Seperti buku mutaba'ah di kertas, tapi lebih cepat, tidak mudah hilang, dan
				rekapnya dibuat otomatis.
			</p>

			<h2 class="mt-7 mb-3 text-sm font-bold tracking-wide text-ink-muted uppercase">Cara kerjanya</h2>
			<ol class="space-y-3">
				{#each LANGKAH as l, i (l.judul)}
					<li class="card flex items-start gap-3.5">
						<span
							class="flex size-11 shrink-0 items-center justify-center rounded-full bg-brand-tint text-brand-strong"
							><Icon n={l.ikon} size={22} /></span
						>
						<div>
							<p class="font-bold">{i + 1}. {l.judul}</p>
							<p class="mt-0.5 text-sm text-ink-soft">{l.isi}</p>
						</div>
					</li>
				{/each}
			</ol>

			<p class="mt-5 flex items-start gap-2.5 rounded-2xl bg-brand-tint p-3.5 text-sm text-brand-strong">
				<Icon n="wifiOff" size={20} />
				<span>Tidak ada sinyal di tempat mengaji? Tenang. Catatan tetap tersimpan di HP dan terkirim saat online.</span>
			</p>
		</section>
	{/if}

	<footer class="mt-4">
		<div class="mb-4 flex justify-center gap-2" aria-hidden="true">
			{#each [1, 2] as n (n)}
				<span class="h-2 rounded-full transition-all {n === layar ? 'w-6 bg-brand' : 'w-2 bg-surface-2'}"></span>
			{/each}
		</div>
		{#if layar === 1}
			<button class="btn btn-primary min-h-14 w-full text-base" onclick={() => (layar = 2)}>
				Lanjut <Icon n="arrowR" size={20} />
			</button>
		{:else}
			<button class="btn btn-primary min-h-14 w-full text-base" onclick={mulai}>
				Bismillah, mulai <Icon n="arrowR" size={20} />
			</button>
		{/if}
	</footer>
</main>
