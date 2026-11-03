<script lang="ts">
	import Icon from './Icon.svelte';

	let {
		value = $bindable(1),
		min = 1,
		max = 999,
		label
	}: { value?: number; min?: number; max?: number; label: string } = $props();

	const set = (v: number) => (value = Math.min(max, Math.max(min, v)));
</script>

<div class="flex items-center justify-between gap-2 rounded-2xl border border-line bg-surface p-1.5">
	<button
		type="button"
		class="flex size-12 items-center justify-center rounded-xl bg-surface-2 text-ink disabled:opacity-35"
		onclick={() => set(value - 1)}
		disabled={value <= min}
		aria-label="Kurangi {label}"
	>
		<span class="text-2xl leading-none">−</span>
	</button>
	<div class="min-w-0 flex-1 text-center">
		<div class="text-[11px] font-semibold tracking-wide text-ink-muted uppercase">{label}</div>
		<div class="tabular text-2xl leading-tight font-bold">{value}</div>
	</div>
	<button
		type="button"
		class="flex size-12 items-center justify-center rounded-xl bg-brand-tint text-brand-strong disabled:opacity-35"
		onclick={() => set(value + 1)}
		disabled={value >= max}
		aria-label="Tambah {label}"
	>
		<Icon n="plus" size={22} />
	</button>
</div>
