<script lang="ts">
	import { onMount } from 'svelte';
	import Icon from './Icon.svelte';

	/** Jarak tarik (px, sudah diredam) yang memicu muat ulang. */
	const AMBANG = 64;
	const MAKS = 110;

	let { aktif = true }: { aktif?: boolean } = $props();

	let tarik = $state(0);
	let memuat = $state(false);

	onMount(() => {
		let y0: number | null = null;
		let x0 = 0;

		/** Tarik hanya dimulai di puncak halaman, di luar sheet dan kolom isian. */
		const mulai = (e: TouchEvent) => {
			const el = e.target as Element | null;
			if (!aktif || memuat || e.touches.length !== 1 || window.scrollY > 0) return;
			if (el?.closest('[role="dialog"], input, textarea, select')) return;
			y0 = e.touches[0].clientY;
			x0 = e.touches[0].clientX;
		};
		const geser = (e: TouchEvent) => {
			if (y0 === null) return;
			const dy = e.touches[0].clientY - y0;
			// Gerak ke atas atau dominan menyamping: bukan tarik-untuk-muat-ulang.
			if (dy <= 0 || Math.abs(e.touches[0].clientX - x0) > dy) {
				y0 = null;
				tarik = 0;
				return;
			}
			tarik = Math.min(MAKS, dy * 0.5);
		};
		const lepas = () => {
			if (y0 === null) return;
			y0 = null;
			if (tarik >= AMBANG) {
				memuat = true;
				tarik = 56;
				location.reload();
			} else tarik = 0;
		};

		window.addEventListener('touchstart', mulai, { passive: true });
		window.addEventListener('touchmove', geser, { passive: true });
		window.addEventListener('touchend', lepas);
		window.addEventListener('touchcancel', lepas);
		return () => {
			window.removeEventListener('touchstart', mulai);
			window.removeEventListener('touchmove', geser);
			window.removeEventListener('touchend', lepas);
			window.removeEventListener('touchcancel', lepas);
		};
	});

	const siap = $derived(tarik >= AMBANG);
</script>

{#if tarik > 0 || memuat}
	<div
		class="no-print pointer-events-none fixed inset-x-0 top-0 z-50 flex justify-center"
		style="transform: translateY({tarik - 48}px); transition: {memuat ? 'transform 150ms' : 'none'}"
		aria-live="polite"
	>
		<div class="safe-t mt-2 flex items-center gap-2 rounded-full border border-line bg-surface px-4 py-2 text-sm font-semibold text-ink-soft shadow-md">
			<span class:animate-spin={memuat} class="inline-flex"><Icon n="refresh" size={18} /></span>
			{memuat ? 'Memuat ulang…' : siap ? 'Lepas untuk muat ulang' : 'Tarik untuk muat ulang'}
		</div>
	</div>
{/if}
