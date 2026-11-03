<script lang="ts">
	import { app } from '../store.svelte';
	import Icon from './Icon.svelte';
</script>

{#if app.toast}
	{#key app.toast.id}
		<div
			class="no-print rise pointer-events-none fixed inset-x-0 bottom-[104px] z-[60] flex justify-center px-4"
			role="status"
			aria-live="polite"
		>
			<div
				class="pointer-events-auto flex w-full max-w-[448px] items-center gap-3 rounded-2xl bg-ink py-2 pr-2 pl-4 text-paper shadow-xl"
			>
				<span class="min-w-0 flex-1 text-[15px] leading-snug">{app.toast.pesan}</span>
				{#if app.toast.aksi}
					{@const aksi = app.toast.aksi}
					<button
						type="button"
						class="flex min-h-11 items-center gap-1.5 rounded-xl px-3 text-[15px] font-bold text-gold"
						onclick={() => {
							aksi.fn();
							app.tutupToast();
						}}><Icon n="undo" size={18} />{aksi.label}</button
					>
				{/if}
			</div>
		</div>
	{/key}
{/if}
