<script lang="ts">
	import { goto } from '$app/navigation';
	import { app } from '#lib/store.svelte.ts';
	import Icon from '#lib/ui/Icon.svelte';
	import Logo from '#lib/ui/Logo.svelte';

	let email = $state('');
	let terkirim = $state(false);
	let proses = $state(false);

	// Sudah punya sesi: langsung ke tujuan.
	$effect(() => {
		if (app.siap && app.sesi) goto(app.onboarded ? '/' : '/mulai', { replace: true });
	});

	const emailValid = $derived(/^\S+@\S+\.\S+$/.test(email.trim()));

	async function masukGoogle() {
		proses = true;
		await new Promise((r) => setTimeout(r, 500));
		app.masukBaru('pengajar@gmail.com');
		goto('/mulai');
	}
	function kirimTautan() {
		if (!emailValid) return;
		terkirim = true;
	}
	function bukaTautan() {
		app.masukBaru(email.trim());
		goto('/mulai');
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
			{#if !terkirim}
				<button class="btn btn-line min-h-14 w-full text-base" onclick={masukGoogle} disabled={proses}>
					<svg width="20" height="20" viewBox="0 0 24 24" aria-hidden="true">
						<path fill="#4285F4" d="M23 12.3c0-.8-.1-1.5-.2-2.2H12v4.2h6.2a5.3 5.3 0 0 1-2.3 3.5v2.9h3.7c2.2-2 3.4-5 3.4-8.4z" />
						<path fill="#34A853" d="M12 23.5c3.1 0 5.7-1 7.6-2.8l-3.7-2.9c-1 .7-2.3 1.1-3.9 1.1-3 0-5.5-2-6.4-4.7H1.8v3A11.5 11.5 0 0 0 12 23.5z" />
						<path fill="#FBBC05" d="M5.6 14.2a6.9 6.9 0 0 1 0-4.4v-3H1.8a11.5 11.5 0 0 0 0 10.4z" />
						<path fill="#EA4335" d="M12 5.4c1.7 0 3.2.6 4.4 1.7l3.3-3.3A11.4 11.4 0 0 0 12 .5 11.5 11.5 0 0 0 1.8 6.8l3.8 3C6.5 7.4 9 5.4 12 5.4z" />
					</svg>
					{proses ? 'Memproses…' : 'Masuk dengan Google'}
				</button>

				<div class="flex items-center gap-3 py-1 text-xs text-ink-muted">
					<span class="h-px flex-1 bg-line"></span>atau<span class="h-px flex-1 bg-line"></span>
				</div>

				<form
					onsubmit={(e) => {
						e.preventDefault();
						kirimTautan();
					}}
					class="space-y-3"
				>
					<label class="sr-only" for="em">Email</label>
					<input
						id="em"
						type="email"
						class="field min-h-14"
						placeholder="email@contoh.id"
						bind:value={email}
						autocomplete="email"
						inputmode="email"
					/>
					<button class="btn btn-primary min-h-14 w-full text-base" disabled={!emailValid}>
						<Icon n="mail" size={20} />Kirim tautan masuk
					</button>
				</form>
				<p class="text-center text-xs text-ink-muted">
					Tanpa password. Kami kirim tautan sekali pakai ke email Anda.
				</p>
			{:else}
				<div class="card rise text-center">
					<span
						class="mx-auto mb-3 flex size-14 items-center justify-center rounded-full bg-brand-tint text-brand-strong"
						><Icon n="mail" size={28} /></span
					>
					<h2 class="text-lg font-bold">Cek email Anda</h2>
					<p class="mt-1 text-sm text-ink-soft">Tautan masuk dikirim ke <b>{email.trim()}</b>.</p>
					<button class="btn btn-primary mt-4 min-h-14 w-full" onclick={bukaTautan}>Buka tautan (demo)</button>
					<button class="mt-2 min-h-11 text-sm font-semibold text-ink-soft" onclick={() => (terkirim = false)}
						>Ganti email</button
					>
				</div>
			{/if}
		</div>
	</div>

	<div class="space-y-4">
		<button class="btn btn-soft min-h-12 w-full" onclick={contoh}>
			<Icon n="play" size={18} />Coba dengan data contoh
		</button>
		<p class="text-center text-xs text-ink-muted">
			Dengan masuk, Anda menyetujui Ketentuan Layanan dan Kebijakan Privasi. Data tersimpan di HP Anda dan
			dicadangkan di server Singapura.
		</p>
	</div>
</main>
