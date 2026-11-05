<script lang="ts">
	let {
		value = $bindable<number | null>(null),
		label,
		placeholder = '',
		max = 9999,
		id
	}: { value?: number | null; label: string; placeholder?: string; max?: number; id: string } = $props();

	/** Hanya angka; angka di atas batas dipotong ke batas. Kosong berarti belum diisi. */
	function masuk(e: Event & { currentTarget: HTMLInputElement }) {
		const digits = e.currentTarget.value.replace(/\D/g, '').slice(0, 5);
		const n = digits ? Math.min(max, Number(digits)) : null;
		value = n !== null && n < 1 ? null : n;
		e.currentTarget.value = value === null ? '' : String(value);
	}
</script>

<div>
	<label class="label" for={id}>{label}</label>
	<input
		{id}
		class="field tabular text-center text-xl font-bold"
		type="text"
		inputmode="numeric"
		autocomplete="off"
		{placeholder}
		value={value ?? ''}
		oninput={masuk}
	/>
</div>
