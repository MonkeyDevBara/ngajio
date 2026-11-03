<script lang="ts">
	import '../app.css';
	import favicon from '#lib/assets/favicon.svg';
	import type { LayoutProps } from './$types';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { app } from '#lib/store.svelte.ts';
	import Icon from '#lib/ui/Icon.svelte';
	import PasangLayar from '#lib/ui/PasangLayar.svelte';
	import Toast from '#lib/ui/Toast.svelte';

	let { children }: LayoutProps = $props();

	onMount(() => {
		app.muat();
		app.mulaiSinkron();
		// Minta Chrome tidak menghapus data lokal saat penyimpanan sesak.
		navigator.storage?.persist?.();
		const naik = () => app.setOnline(true);
		const turun = () => app.setOnline(false);
		const tampil = () => document.visibilityState === 'visible' && app.picuSinkron();
		window.addEventListener('online', naik);
		window.addEventListener('offline', turun);
		document.addEventListener('visibilitychange', tampil);
		return () => {
			window.removeEventListener('online', naik);
			window.removeEventListener('offline', turun);
			document.removeEventListener('visibilitychange', tampil);
		};
	});

	const path = $derived(page.url.pathname);
	const intro = $derived(path.startsWith('/selamat-datang'));
	const publik = $derived(path.startsWith('/masuk') || intro);
	const setup = $derived(path.startsWith('/mulai'));
	const penuh = $derived(publik || setup || path.startsWith('/setoran'));

	// Penjaga rute: belum masuk -> /masuk, belum setup -> /mulai.
	$effect(() => {
		if (!app.siap) return;
		if (!app.sesi && !publik) goto(app.introSelesai ? '/masuk' : '/selamat-datang', { replace: true });
		else if (!app.sesi && !app.introSelesai && !intro) goto('/selamat-datang', { replace: true });
		else if (app.sesi && !app.onboarded && !setup && !publik) goto('/mulai', { replace: true });
	});

	// Tema: otomatis mengikuti sistem, atau dipaksa terang/gelap oleh pengajar.
	$effect(() => {
		const t = app.pengajar.tema;
		const mq = window.matchMedia('(prefers-color-scheme: dark)');
		const terap = () =>
			document.documentElement.classList.toggle('dark', t === 'gelap' || (t === 'otomatis' && mq.matches));
		terap();
		mq.addEventListener('change', terap);
		return () => mq.removeEventListener('change', terap);
	});

	// Setelah masuk baru: tawarkan 'pasang di layar utama' sekali, di halaman tujuan (bukan saat muat ulang).
	let bukaPasang = $state(false);
	$effect(() => {
		if (!app.tawarkanPasang || !app.siap || !(path === '/' || setup)) return;
		app.tawarkanPasang = false;
		const terpasang = window.matchMedia('(display-mode: standalone)').matches || (navigator as { standalone?: boolean }).standalone;
		if (!terpasang && localStorage.getItem('ngajio:a2hs') !== 'skip') bukaPasang = true;
	});

	const tab = [
		{ href: '/', label: 'Beranda', ikon: 'home' },
		{ href: '/kelompok', label: 'Kelompok', ikon: 'users' },
		{ href: '/santri', label: 'Santri', ikon: 'user' },
		{ href: '/laporan', label: 'Laporan', ikon: 'chart' }
	];
	const aktif = (href: string) => (href === '/' ? path === '/' : path.startsWith(href));
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>Ngajio</title>
</svelte:head>

{#if !app.siap || (!app.sesi && !publik) || (app.sesi && !app.onboarded && !setup && !publik)}
	<div class="flex min-h-dvh items-center justify-center text-ink-muted">
		<span class="text-sm">Memuat…</span>
	</div>
{:else}
	<div class="print-wide mx-auto min-h-dvh w-full max-w-[480px] {penuh ? '' : 'pb-32'}">
		{#if !penuh && !app.efektifOnline}
			<div class="no-print flex items-center gap-2 bg-gold-tint px-4 py-2.5 text-sm font-medium text-warn">
				<Icon n="wifiOff" size={16} />
				<span>Tanpa sinyal. Setoran tetap tersimpan di HP dan terkirim saat online.</span>
			</div>
		{/if}
		{#if !penuh && app.sesiBerakhir}
			<a
				href="/masuk"
				class="no-print flex items-center gap-2 bg-danger-tint px-4 py-2.5 text-sm font-semibold text-danger"
			>
				<Icon n="alert" size={16} />
				<span>Sesi berakhir, masuk lagi. Data di HP ini aman.</span>
			</a>
		{/if}
		{#if !penuh && app.statusSinkron === 'gagal'}
			<div class="no-print space-y-2 bg-danger-tint px-4 py-3 text-sm text-danger">
				<div class="flex items-center gap-2 font-semibold">
					<Icon n="alert" size={16} />Sinkronisasi gagal setelah 5 percobaan
				</div>
				<p class="text-ink-soft">Data Anda aman di HP ini. Coba lagi, atau simpan cadangan dulu.</p>
				<div class="flex gap-2">
					<button class="btn btn-danger min-h-11 flex-1 text-sm" onclick={() => app.cobaLagiSinkron()}
						>Coba lagi</button
					>
					<a class="btn btn-line min-h-11 flex-1 text-sm" href="/pengaturan#ekspor">Ekspor cadangan</a>
				</div>
			</div>
		{/if}

		{@render children()}
	</div>

	{#if !penuh}
		<nav
			class="no-print safe-b fixed inset-x-0 bottom-0 z-40 border-t border-line bg-surface/95 backdrop-blur"
			aria-label="Navigasi utama"
		>
			<div class="mx-auto grid max-w-[480px] grid-cols-5 items-end px-1">
				{#each tab.slice(0, 2) as t (t.href)}
					{@render item(t)}
				{/each}
				<a
					href="/setoran"
					class="relative -mt-6 flex flex-col items-center gap-1 pb-2"
					aria-label="Catat setoran baru"
				>
					<span
						class="flex size-16 items-center justify-center rounded-[22px] bg-brand text-on-brand shadow-lg ring-4 ring-paper transition active:scale-95"
						><Icon n="plus" size={32} /></span
					>
					<span class="text-[11px] font-bold text-brand">Setoran</span>
				</a>
				{#each tab.slice(2) as t (t.href)}
					{@render item(t)}
				{/each}
			</div>
		</nav>
		<Toast />
	{:else}
		<Toast />
	{/if}
{/if}

{#if bukaPasang}
	<PasangLayar bind:open={bukaPasang} />
{/if}

{#snippet item(t: { href: string; label: string; ikon: string })}
	<a
		href={t.href}
		class="flex min-h-[64px] flex-col items-center justify-center gap-1 {aktif(t.href)
			? 'text-brand'
			: 'text-ink-muted'}"
		aria-current={aktif(t.href) ? 'page' : undefined}
	>
		<Icon n={t.ikon} size={24} />
		<span class="text-[11px] font-semibold">{t.label}</span>
	</a>
{/snippet}
