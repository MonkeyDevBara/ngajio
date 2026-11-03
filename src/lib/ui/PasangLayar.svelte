<script lang="ts">
	import Icon from './Icon.svelte';
	import Sheet from './Sheet.svelte';

	let { open = $bindable(false) }: { open?: boolean } = $props();

	const KUNCI = 'ngajio:a2hs';
	let hp = $state<'android' | 'iphone'>(/iPhone|iPad|iPod/.test(navigator.userAgent) ? 'iphone' : 'android');

	function lewati() {
		localStorage.setItem(KUNCI, 'skip');
		open = false;
	}
</script>

<Sheet bind:open judul="Pasang di layar utama">
	<p class="text-sm text-ink-soft">Buka Ngajio langsung dari layar HP, seperti aplikasi biasa, dan tetap jalan tanpa sinyal.</p>

	<div class="mt-3 grid grid-cols-2 gap-1.5 rounded-2xl bg-surface-2 p-1.5" role="tablist">
		{#each [['android', 'Android'], ['iphone', 'iPhone']] as [id, l] (id)}
			<button
				role="tab"
				aria-selected={hp === id}
				class="min-h-11 rounded-xl text-sm font-bold {hp === id ? 'bg-surface text-brand shadow-sm' : 'text-ink-soft'}"
				onclick={() => (hp = id as 'android' | 'iphone')}>{l}</button
			>
		{/each}
	</div>

	<ol class="mt-4 space-y-3 text-sm">
		{#if hp === 'android'}
			<li class="flex gap-3"><span class="step">1</span><span>Buka Ngajio di <b>Chrome</b>.</span></li>
			<li class="flex gap-3">
				<span class="step">2</span>
				<span>Ketuk menu titik tiga <Icon n="dots" size={16} class="inline align-text-bottom" /> di pojok kanan atas.</span>
			</li>
			<li class="flex gap-3">
				<span class="step">3</span>
				<span>Pilih <b>Tambahkan ke layar utama</b> atau <b>Instal aplikasi</b>.</span>
			</li>
			<li class="flex gap-3"><span class="step">4</span><span>Ketuk <b>Instal</b> atau <b>Tambah</b>. Selesai.</span></li>
		{:else}
			<li class="flex gap-3"><span class="step">1</span><span>Buka Ngajio di <b>Safari</b> (bukan Chrome).</span></li>
			<li class="flex gap-3">
				<span class="step">2</span>
				<span>Ketuk tombol Bagikan <Icon n="share" size={16} class="inline align-text-bottom" /> di bagian bawah layar.</span>
			</li>
			<li class="flex gap-3">
				<span class="step">3</span>
				<span>Gulir lalu pilih <b>Tambah ke Layar Utama</b> <Icon n="plus" size={16} class="inline align-text-bottom" />.</span>
			</li>
			<li class="flex gap-3"><span class="step">4</span><span>Ketuk <b>Tambah</b> di pojok kanan atas. Selesai.</span></li>
		{/if}
	</ol>

	<div class="mt-5 grid grid-cols-2 gap-2">
		<button class="btn btn-line text-sm" onclick={lewati}>Jangan tampilkan lagi</button>
		<button class="btn btn-primary text-sm" onclick={() => (open = false)}>Mengerti</button>
	</div>
</Sheet>

<style>
	.step {
		display: flex;
		width: 1.5rem;
		height: 1.5rem;
		flex-shrink: 0;
		align-items: center;
		justify-content: center;
		border-radius: 9999px;
		background: var(--brand-tint);
		color: var(--brand-strong);
		font-size: 0.75rem;
		font-weight: 700;
	}
</style>
