/// <reference types="@sveltejs/kit" />
/// <reference no-default-lib="true" />
/// <reference lib="esnext" />
/// <reference lib="webworker" />

// Offline-first: cangkang SPA + semua aset dipasang di cache saat instal, jadi halaman
// terbuka tanpa sinyal. Data hidup di perangkat (IndexedDB); /api tidak pernah disentuh di sini.
import { self as sw } from '$app/service-worker';
import { version } from '$app/env';
import { immutable, assets } from '$app/manifest';

// Aplikasi dilayani dari root (tanpa base path); path manifest tanpa '/' di depan.
const abs = (p: string) => `/${p}`;
const CACHE = `ngajio-${version}`;
const SHELL = '/'; // cangkang SPA (index.html); '/' dilayani semua host statis, '/index.html' tidak selalu
const ASET = [SHELL, ...immutable.map((a) => abs(a.path)), ...assets.map((a) => abs(a.path))];
const ASET_SET = new Set(ASET);

// Mode dev: daftar build kosong, jangan menyimpan apa pun.
if (immutable.length > 0) {
	sw.addEventListener('install', (e) => {
		e.waitUntil(
			caches
				.open(CACHE)
				.then((c) => c.addAll(ASET.map((u) => new Request(u, { cache: 'reload' })))),
		);
	});

	sw.addEventListener('activate', (e) => {
		e.waitUntil(
			caches
				.keys()
				.then((ks) => Promise.all(ks.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
				.then(() => sw.clients.claim()),
		);
	});

	sw.addEventListener('fetch', (e) => {
		const req = e.request;
		if (req.method !== 'GET') return;
		const url = new URL(req.url);
		if (url.origin !== sw.location.origin || url.pathname.startsWith('/api/')) return;

		// Navigasi (buka beranda, muat ulang, rute mana pun): cangkang dari cache, tanpa menunggu jaringan.
		if (req.mode === 'navigate') {
			e.respondWith(caches.open(CACHE).then(async (c) => (await c.match(SHELL)) ?? fetch(req)));
			return;
		}

		// Aset terdaftar: cache dulu. Selain itu: jaringan, cadangan cache.
		e.respondWith(
			caches.open(CACHE).then(async (c) => {
				if (ASET_SET.has(url.pathname)) {
					const hit = await c.match(url.pathname);
					if (hit) return hit;
				}
				try {
					return await fetch(req);
				} catch (err) {
					const hit = await c.match(req);
					if (hit) return hit;
					throw err;
				}
			}),
		);
	});
}
