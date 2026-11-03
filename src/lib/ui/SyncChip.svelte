<script lang="ts">
	import { app } from '../store.svelte';
	import Icon from './Icon.svelte';

	const s = $derived(app.statusSinkron);
</script>

<a
	href="/pengaturan#sinkron"
	class="chip min-h-8 {s === 'tersinkron'
		? 'bg-brand-tint text-brand-strong'
		: s === 'menunggu'
			? 'bg-gold-tint text-warn'
			: 'bg-danger-tint text-danger'}"
	aria-label="Status sinkronisasi: {s}"
>
	<Icon n={s === 'tersinkron' ? 'check' : s === 'menunggu' ? 'refresh' : 'alert'} size={14} />
	{s === 'tersinkron' ? 'Tersinkron' : s === 'menunggu' ? `Menunggu${app.antrian ? ` · ${app.antrian}` : ''}` : 'Gagal'}
</a>
