<script lang="ts">
	import { avatarWarna, inisial } from '../format';
	let { nama, size = 44 }: { nama: string; size?: number } = $props();
	const [fg, bg] = $derived(avatarWarna(nama));
</script>

<span
	class="inline-flex shrink-0 items-center justify-center rounded-full font-bold ring-1 ring-black/10 ring-inset"
	style="width:{size}px;height:{size}px;background:{bg};color:{fg};font-size:{Math.round(size * 0.36)}px"
	aria-hidden="true">{inisial(nama)}</span
>
