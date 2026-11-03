<script lang="ts">
	let { persen, warna = 'var(--brand)' }: { persen: number; warna?: string } = $props();
</script>

<div
	class="h-2.5 w-full overflow-hidden rounded-full bg-surface-2"
	role="progressbar"
	aria-valuemin="0"
	aria-valuemax="100"
	aria-valuenow={Math.round(persen * 100)}
>
	<div
		class="h-full rounded-full transition-[width] duration-500"
		style="width:{Math.max(persen > 0 ? 4 : 0, persen * 100)}%;background:{warna}"
	></div>
</div>
