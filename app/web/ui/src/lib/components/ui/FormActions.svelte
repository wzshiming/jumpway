<script lang="ts">
	import type { Snippet } from 'svelte';
	import IconLoader from '~icons/lucide/loader-circle';
	import IconSave from '~icons/lucide/save';
	import { t } from '../../i18n.svelte';

	interface Props {
		dirty: boolean;
		disabled?: boolean;
		// The write this form issued is in flight; the icon spins and the caption keeps its width.
		saving?: boolean;
		// Pinned to the bottom of the scroller for long forms; inline for short, stacked ones.
		sticky?: boolean;
		secondary?: Snippet;
	}

	let { dirty, disabled = false, saving = false, sticky = false, secondary }: Props = $props();
</script>

<!-- Sticky offsets stop at the scroller's padding edge; --main-inset (set on #main) reaches across it. -->
<div
	data-form-actions
	class="flex flex-wrap items-center gap-2 {sticky
		? 'sticky -bottom-(--main-inset,0px) z-10 border-t border-line bg-canvas py-3'
		: 'mt-4'}"
>
	<button type="submit" class="btn btn-primary" {disabled} aria-busy={saving || undefined}>
		{#if saving}
			<IconLoader class="size-4 motion-safe:animate-spin" aria-hidden="true" data-spinner />
		{:else}
			<IconSave class="size-4" aria-hidden="true" />
		{/if}
		{t('saveApply')}
	</button>
	{@render secondary?.()}
	{#if dirty}
		<span
			role="status"
			class="ml-auto rounded-md bg-warning-soft px-2 py-0.5 text-xs font-medium text-warning"
		>
			{t('unsaved')}
		</span>
	{/if}
</div>
