<script lang="ts">
	import { avatarWarna, inisial } from '../format';
	import { fotoAman } from '../foto';
	let { nama, size = 44, foto }: { nama: string; size?: number; foto?: string } = $props();
	const [fg, bg] = $derived(avatarWarna(nama));
</script>

{#if fotoAman(foto)}
	<img
		src={foto}
		alt=""
		width={size}
		height={size}
		class="shrink-0 rounded-full object-cover ring-1 ring-black/10 ring-inset"
		style="width:{size}px;height:{size}px"
	/>
{:else}
<span
	class="inline-flex shrink-0 items-center justify-center rounded-full font-bold ring-1 ring-black/10 ring-inset"
	style="width:{size}px;height:{size}px;background:{bg};color:{fg};font-size:{Math.round(size * 0.36)}px"
	aria-hidden="true">{inisial(nama)}</span
>
{/if}
