<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { app } from '#lib/store.svelte.ts';
	import { api, pesanGalat, type BalasanAuth } from '#lib/sinkron.ts';
	import { isiFotoGoogle, pasangGoogle } from '#lib/google.ts';
	import Icon from '#lib/ui/Icon.svelte';
	import Logo from '#lib/ui/Logo.svelte';

	let cfg = $state<{ googleClientId: string; devLogin: boolean } | null>(null);
	let emailDev = $state('dev@ngajio.local');
	let proses = $state(false);
	let tombolGoogle = $state<HTMLElement>();

	onMount(async () => {
		try {
			cfg = await api('/api/config', undefined, 'GET');
		} catch (e) {
			cfg = { googleClientId: '', devLogin: false };
			app.tampilToast(pesanGalat(e));
		}
	});

	// Sudah punya sesi: langsung ke tujuan. Sesi berakhir tetap di sini untuk masuk lagi.
	$effect(() => {
		if (app.siap && app.sesi && !app.sesiBerakhir) goto(app.onboarded ? '/' : '/mulai', { replace: true });
	});

	$effect(() => {
		if (cfg?.googleClientId && tombolGoogle) {
			pasangGoogle(tombolGoogle, cfg.googleClientId, selesai, (e) => app.tampilToast(pesanGalat(e)));
		}
	});

	async function selesai(r: BalasanAuth, picture?: string) {
		proses = true;
		try {
			await app.masukAkun(r.user);
			isiFotoGoogle(picture);
			goto(app.onboarded ? '/' : '/mulai');
		} catch (e) {
			app.tampilToast(pesanGalat(e));
		} finally {
			proses = false;
		}
	}
	async function masukDev() {
		proses = true;
		try {
			await selesai(await api<BalasanAuth>('/api/auth/dev', { email: emailDev.trim() }));
		} catch (e) {
			app.tampilToast(pesanGalat(e));
			proses = false;
		}
	}
	function contoh() {
		app.masukContoh();
		goto('/');
	}
</script>

<main class="mx-auto flex min-h-dvh max-w-[480px] flex-col px-6 pt-[max(2rem,env(safe-area-inset-top))] pb-8">
	<div class="flex flex-1 flex-col justify-center py-10">
		<div class="mb-8 flex items-center gap-3">
			<Logo size={56} />
			<span class="text-3xl font-extrabold tracking-tight">ngaji<span class="text-gold">o</span></span>
		</div>
		<h1 class="text-[28px] leading-tight font-extrabold">Buku mutaba'ah digital untuk pengajar.</h1>
		<p class="mt-3 text-lg text-ink-soft">
			Catat setoran dalam 3 ketukan. Rekap tanpa lembur. Tetap jalan tanpa sinyal.
		</p>

		<div class="mt-10 space-y-3">
			{#if cfg?.googleClientId}
				<div bind:this={tombolGoogle} class="flex min-h-14 justify-center" class:opacity-50={proses}></div>
			{/if}
			{#if cfg?.devLogin}
				<form
					class="space-y-2 rounded-2xl border border-dashed border-line p-3"
					onsubmit={(e) => {
						e.preventDefault();
						masukDev();
					}}
				>
					<label class="text-xs font-semibold text-ink-soft" for="em">Masuk dev (lokal), tanpa Google</label>
					<input id="em" type="email" class="field min-h-12" bind:value={emailDev} autocomplete="off" />
					<button class="btn btn-line min-h-12 w-full" disabled={proses || !emailDev.trim()}>
						{proses ? 'Memproses…' : 'Masuk dev (lokal)'}
					</button>
				</form>
			{/if}
			{#if cfg && !cfg.googleClientId && !cfg.devLogin}
				<button class="btn btn-line min-h-14 w-full text-base" disabled>Masuk Google belum dikonfigurasi</button>
			{/if}
		</div>
	</div>

	<div class="space-y-4">
		{#if !(app.sesi && app.mode === 'akun')}
			<button class="btn btn-soft min-h-12 w-full" onclick={contoh}>
				<Icon n="play" size={18} />Coba dengan data contoh
			</button>
		{/if}
		<p class="text-center text-xs text-ink-muted">
			Dengan masuk, Anda menyetujui Ketentuan Layanan dan Kebijakan Privasi. Data tersimpan di HP Anda dan
			dicadangkan di server Singapura.
		</p>
	</div>
</main>
