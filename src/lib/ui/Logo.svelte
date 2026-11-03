<script lang="ts">
	// Logo Ngajio: huruf N putih di kotak hijau. Warna mengikuti tema.
	let { size = 56 }: { size?: number } = $props();
</script>

<svg width={size} height={size} viewBox="0 0 64 64" role="img" aria-label="Ngajio">
	<rect width="64" height="64" rx="16" fill="var(--brand)" />
	<path
		d="M20 47V17l24 30V17"
		fill="none"
		stroke="var(--on-brand)"
		stroke-width="7.5"
		stroke-linecap="round"
		stroke-linejoin="round"
	/>
</svg>
